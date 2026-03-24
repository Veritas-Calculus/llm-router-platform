package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SSOPayload input for initiating SSO
type SSOPayload struct {
	Email string `json:"email"`
}

type SSOResponse struct {
	RedirectURL string `json:"redirect_url"`
}

type SSOHandler struct {
	cfg    *config.Config
	db     *gorm.DB
	logger *zap.Logger
}

func NewSSOHandler(cfg *config.Config, db *gorm.DB, logger *zap.Logger) *SSOHandler {
	return &SSOHandler{cfg: cfg, db: db, logger: logger}
}

// Discover takes an email, finds the associated IdP via domain matching,
// and returns the OIDC/SAML authorize URL.
func (h *SSOHandler) Discover(c *gin.Context) {
	var payload SSOPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	parts := strings.Split(payload.Email, "@")
	var domain string
	if len(parts) == 2 {
		domain = parts[1]
	} else {
		domain = payload.Email
	}

	domain = strings.ToLower(strings.TrimSpace(domain))

	var idps []models.IdentityProvider
	if err := h.db.Where("is_active = true").Find(&idps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	var matchedIdp *models.IdentityProvider
	for _, idp := range idps {
		domains := strings.Split(idp.Domains, ",")
		for _, d := range domains {
			if strings.TrimSpace(strings.ToLower(d)) == domain {
				matchedIdp = &idp
				break
			}
		}
		if matchedIdp != nil {
			break
		}
	}

	if matchedIdp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no identity provider configured for this domain"})
		return
	}

	redirectURL := h.callbackURL(c, matchedIdp.ID.String())
	var authURL string

	switch matchedIdp.Type {
	case "oidc":
		authURL = fmt.Sprintf("%s/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=openid email profile",
			strings.TrimRight(matchedIdp.OIDCIssuerURL, "/"), matchedIdp.OIDCClientID, redirectURL)
	case "saml":
		authURL = matchedIdp.SAMLSSOURL
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unsupported idp type"})
		return
	}

	c.JSON(http.StatusOK, SSOResponse{RedirectURL: authURL})
}

// Callback handles the OIDC/SAML response, performs JIT provisioning if enabled,
// and maps the IdP group to the System Role.
func (h *SSOHandler) Callback(c *gin.Context) {
	idpID := c.Param("id")
	code := c.Query("code")

	var idp models.IdentityProvider
	if err := h.db.First(&idp, "id = ?", idpID).Error; err != nil {
		h.redirectWithError(c, "Invalid Identity Provider")
		return
	}

	// For OIDC, exchange code for token -> get email
	var email, name, oauthID string
	var groups []string

	switch idp.Type {
	case "oidc":
		if code == "" {
			h.redirectWithError(c, "Missing authorization code")
			return
		}
		tokenResp, err := h.exchangeOIDCCode(c.Request.Context(), &idp, code, h.callbackURL(c, idp.ID.String()))
		if err != nil {
			h.logger.Error("OIDC token exchange failed", zap.Error(err))
			h.redirectWithError(c, "SSO Authentication failed")
			return
		}
		email = tokenResp.Email
		name = tokenResp.Name
		oauthID = tokenResp.Sub
		groups = append(groups, tokenResp.Groups...)
	case "saml":
        // SAML parsing is mock-only for this phase, typically uses CrewJam/saml
		h.redirectWithError(c, "SAML parsing not fully implemented in this demo")
		return
	}

	if email == "" {
		h.redirectWithError(c, "Identity provider did not return an email address")
		return
	}

	user, err := h.jitProvisionUser(&idp, email, name, oauthID, groups)
	if err != nil {
		h.logger.Error("JIT Provisioning failed", zap.Error(err))
		h.redirectWithError(c, "Account provisioning failed")
		return
	}

	token, err := h.generateJWT(user)
	if err != nil {
		h.logger.Error("JWT generation failed", zap.Error(err))
		h.redirectWithError(c, "Session creation failed")
		return
	}

	frontendURL := h.cfg.Frontend.URL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/oauth/callback?token=%s", frontendURL, token))
}

// ── Internal Helpers ────────────────────────────────────────────────────────

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type oidcClaims struct {
	Sub    string   `json:"sub"`
	Email  string   `json:"email"`
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

func (h *SSOHandler) exchangeOIDCCode(ctx context.Context, idp *models.IdentityProvider, code, redirectURI string) (*oidcClaims, error) {
	tokenURL := strings.TrimRight(idp.OIDCIssuerURL, "/") + "/token"
	body := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&grant_type=authorization_code",
		idp.OIDCClientID, idp.OIDCClientSecret, code, redirectURI)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var tokenResp oidcTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response")
	}

	if tokenResp.IDToken == "" {
		// Attempt UserInfo endpoint if IDToken parsing fails (mock)
		return nil, fmt.Errorf("OIDC provider did not return id_token")
	}

	// Extremely simplified JWT parser for extracting OIDC profile claims (No signature validation in this specific mock because we trust the TLS backchannel exchange).
	parts := strings.Split(tokenResp.IDToken, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid id_token format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims oidcClaims
	_ = json.Unmarshal(payload, &claims)

	return &claims, nil
}

func (h *SSOHandler) jitProvisionUser(idp *models.IdentityProvider, email, name, oauthID string, groups []string) (*models.User, error) {
	var user models.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err == nil {
		return &user, nil // user exists
	}

	if !idp.EnableJIT {
		return nil, fmt.Errorf("user does not exist and JIT provisioning is disabled")
	}

	// Determine Role based on IdP Groups mapping
	targetRole := idp.DefaultRole
	if idp.GroupRoleMapping != "" {
		var mapping map[string]string
		if err := json.Unmarshal([]byte(idp.GroupRoleMapping), &mapping); err == nil {
			for _, g := range groups {
				if r, exists := mapping[g]; exists {
					targetRole = r
					break
				}
			}
		}
	}

	user = models.User{
		Email:         email,
		Name:          name,
		Role:          "user",
		IsActive:      true,
		OAuthProvider: "sso_" + idp.Type,
		OAuthID:       oauthID,
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		// Create Org membership
		member := models.OrganizationMember{
			OrgID:  idp.OrgID,
			UserID: user.ID,
			Role:   targetRole,
		}
		return tx.Create(&member).Error
	})

	return &user, err
}

func (h *SSOHandler) callbackURL(c *gin.Context, idpID string) string {
	scheme := "https"
	if c.Request.TLS == nil && !strings.Contains(c.Request.Host, "localhost") {
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
	} else if c.Request.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/api/v1/sso/callback/%s", scheme, c.Request.Host, idpID)
}

func (h *SSOHandler) redirectWithError(c *gin.Context, errMsg string) {
	frontendURL := h.cfg.Frontend.URL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=%s", frontendURL, errMsg))
}

func (h *SSOHandler) generateJWT(u *models.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   u.ID.String(),
		"email": u.Email,
		"role":  u.Role,
		"exp":   time.Now().Add(h.cfg.JWT.ExpiresIn).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWT.Secret))
}
