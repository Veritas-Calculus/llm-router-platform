package resolvers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"

	"github.com/gin-gonic/gin"
)

const gqlSSOStateCookieName = "sso_oidc_state"

type gqlSSOStatePayload struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	IDPID    string `json:"idp_id"`
}

// CaptchaConfig is the resolver for the captchaConfig field.
func (r *queryResolver) CaptchaConfig(ctx context.Context) (*model.CaptchaConfig, error) {
	cfg := r.Config().Turnstile
	return &model.CaptchaConfig{
		Enabled: cfg.Enabled,
		SiteKey: cfg.SiteKey,
	}, nil
}

// DiscoverSso is the resolver for the discoverSso field.
func (r *mutationResolver) DiscoverSso(ctx context.Context, email string) (*model.SsoDiscoveryResult, error) {
	domain := ssoDomainFromEmail(email)
	if domain == "" {
		return nil, fmt.Errorf("invalid email")
	}

	var idps []models.IdentityProvider
	if err := r.DB().WithContext(ctx).Where("is_active = ?", true).Find(&idps).Error; err != nil {
		return nil, fmt.Errorf("database error")
	}

	var matched *models.IdentityProvider
	for i := range idps {
		for _, d := range strings.Split(idps[i].Domains, ",") {
			if strings.TrimSpace(strings.ToLower(d)) == domain {
				matched = &idps[i]
				break
			}
		}
		if matched != nil {
			break
		}
	}
	if matched == nil {
		return nil, fmt.Errorf("no identity provider configured for this domain")
	}

	redirectURL := r.ssoCallbackURL(matched.ID.String())
	var authURL string

	switch strings.ToLower(matched.Type) {
	case "oidc":
		if matched.OIDCClientID == "" || matched.OIDCIssuerURL == "" {
			return nil, fmt.Errorf("identity provider is missing OIDC configuration")
		}
		gc, err := directives.GinContextFromContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("request context unavailable")
		}
		state, nonce, verifier, challenge, err := generateGraphQLOIDCState()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize SSO flow")
		}
		gc.SetSameSite(http.SameSiteLaxMode)
		setGraphQLOIDCStateCookie(gc, state, nonce, verifier, matched.ID.String())

		q := url.Values{}
		q.Set("client_id", matched.OIDCClientID)
		q.Set("response_type", "code")
		q.Set("redirect_uri", redirectURL)
		q.Set("scope", "openid email profile")
		q.Set("state", state)
		q.Set("nonce", nonce)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
		authURL = strings.TrimRight(matched.OIDCIssuerURL, "/") + "/authorize?" + q.Encode()
	case "saml":
		if matched.SAMLSSOURL == "" {
			return nil, fmt.Errorf("identity provider is missing SAML SSO URL")
		}
		authURL = matched.SAMLSSOURL
	default:
		return nil, fmt.Errorf("unsupported idp type")
	}

	return &model.SsoDiscoveryResult{RedirectURL: authURL}, nil
}

func ssoDomainFromEmail(email string) string {
	parts := strings.Split(strings.TrimSpace(email), "@")
	if len(parts) == 2 {
		return strings.ToLower(parts[1])
	}
	return strings.ToLower(strings.TrimSpace(email))
}

func (r *Resolver) ssoCallbackURL(idpID string) string {
	cfg := r.Config()
	base := cfg.Frontend.PublicBackendURL
	if base == "" {
		base = cfg.Frontend.URL
	}
	if base == "" {
		base = "http://localhost:8080"
	}
	return fmt.Sprintf("%s/api/v1/sso/callback/%s", strings.TrimRight(base, "/"), idpID)
}

func generateGraphQLOIDCState() (state, nonce, verifier, challenge string, err error) {
	state, err = randomGraphQLURLString(32)
	if err != nil {
		return "", "", "", "", err
	}
	nonce, err = randomGraphQLURLString(32)
	if err != nil {
		return "", "", "", "", err
	}
	verifier, err = randomGraphQLURLString(64)
	if err != nil {
		return "", "", "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return state, nonce, verifier, challenge, nil
}

func randomGraphQLURLString(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func setGraphQLOIDCStateCookie(c *gin.Context, state, nonce, verifier, idpID string) {
	payload, _ := json.Marshal(gqlSSOStatePayload{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		IDPID:    idpID,
	})
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	c.SetCookie(gqlSSOStateCookieName, encoded, 600, "/", "", true, true)
}
