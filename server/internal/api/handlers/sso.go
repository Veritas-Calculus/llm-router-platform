package handlers

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

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
// generates state/nonce/PKCE, and returns the OIDC/SAML authorize URL.
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

	redirectURL := h.callbackURL(matchedIdp.ID.String())
	var authURL string

	switch matchedIdp.Type {
	case "oidc":
		state, nonce, verifier, challenge, err := generateOIDCState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize SSO flow"})
			return
		}
		// Persist state/nonce/pkce-verifier + idp id in a short-lived,
		// HttpOnly+Secure cookie. 10 minutes is enough for any realistic
		// redirect round-trip.
		c.SetSameSite(http.SameSiteLaxMode)
		setOIDCStateCookie(c, state, nonce, verifier, matchedIdp.ID.String())

		q := url.Values{}
		q.Set("client_id", matchedIdp.OIDCClientID)
		q.Set("response_type", "code")
		q.Set("redirect_uri", redirectURL)
		q.Set("scope", "openid email profile")
		q.Set("state", state)
		q.Set("nonce", nonce)
		q.Set("code_challenge", challenge)
		q.Set("code_challenge_method", "S256")
		authURL = strings.TrimRight(matchedIdp.OIDCIssuerURL, "/") + "/authorize?" + q.Encode()
	case "saml":
		authURL = matchedIdp.SAMLSSOURL
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unsupported idp type"})
		return
	}

	c.JSON(http.StatusOK, SSOResponse{RedirectURL: authURL})
}

// ─── OIDC flow state (state / nonce / PKCE) ─────────────────────────────────

const ssoStateCookieName = "sso_oidc_state"

type ssoStatePayload struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	IDPID    string `json:"idp_id"`
}

// generateOIDCState returns a (state, nonce, pkce_verifier, pkce_challenge)
// tuple. Values are drawn from crypto/rand and encoded using base64url so they
// are safe in URLs.
func generateOIDCState() (state, nonce, verifier, challenge string, err error) {
	state, err = randomURLString(32)
	if err != nil {
		return "", "", "", "", err
	}
	nonce, err = randomURLString(32)
	if err != nil {
		return "", "", "", "", err
	}
	verifier, err = randomURLString(64) // PKCE requires 43–128 chars
	if err != nil {
		return "", "", "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return state, nonce, verifier, challenge, nil
}

func randomURLString(n int) (string, error) {
	// crypto/rand via rsa package is overkill; we just want random bytes.
	buf := make([]byte, n)
	if _, err := readRandom(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// readRandom is a tiny indirection so tests can stub it.
var readRandom = func(b []byte) (int, error) {
	return cryptorand.Read(b)
}

func setOIDCStateCookie(c *gin.Context, state, nonce, verifier, idpID string) {
	payload, _ := json.Marshal(ssoStatePayload{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		IDPID:    idpID,
	})
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	c.SetCookie(ssoStateCookieName, encoded, 600, "/", "", true, true)
}

func readAndClearOIDCStateCookie(c *gin.Context) (*ssoStatePayload, error) {
	raw, err := c.Cookie(ssoStateCookieName)
	if err != nil || raw == "" {
		return nil, errors.New("missing SSO state cookie")
	}
	// Clear immediately (one-time use).
	c.SetCookie(ssoStateCookieName, "", -1, "/", "", true, true)

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid SSO state cookie: %w", err)
	}
	var s ssoStatePayload
	if err := json.Unmarshal(decoded, &s); err != nil {
		return nil, fmt.Errorf("invalid SSO state cookie: %w", err)
	}
	if s.State == "" || s.Nonce == "" || s.Verifier == "" || s.IDPID == "" {
		return nil, errors.New("SSO state cookie missing fields")
	}
	return &s, nil
}

// Callback handles the OIDC/SAML response, performs JIT provisioning if enabled,
// and maps the IdP group to the System Role.
func (h *SSOHandler) Callback(c *gin.Context) {
	idpID := c.Param("id")
	code := c.Query("code")
	state := c.Query("state")

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

		// Load and clear the one-time state cookie. Any failure to verify
		// state/nonce/idp_id aborts the flow.
		stateCookie, err := readAndClearOIDCStateCookie(c)
		if err != nil {
			h.logger.Warn("SSO state cookie verification failed", zap.Error(err))
			h.redirectWithError(c, "SSO session expired or invalid")
			return
		}
		if subtle.ConstantTimeCompare([]byte(stateCookie.State), []byte(state)) != 1 {
			h.logger.Warn("SSO state mismatch")
			h.redirectWithError(c, "SSO state verification failed")
			return
		}
		if stateCookie.IDPID != idpID {
			h.logger.Warn("SSO idp mismatch against state cookie")
			h.redirectWithError(c, "SSO state verification failed")
			return
		}

		tokenResp, err := h.exchangeOIDCCode(
			c.Request.Context(),
			&idp,
			code,
			h.callbackURL(idp.ID.String()),
			stateCookie.Verifier,
			stateCookie.Nonce,
		)
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

func (h *SSOHandler) exchangeOIDCCode(ctx context.Context, idp *models.IdentityProvider, code, redirectURI, codeVerifier, expectedNonce string) (*oidcClaims, error) {
	tokenURL := strings.TrimRight(idp.OIDCIssuerURL, "/") + "/token"

	form := url.Values{}
	form.Set("client_id", idp.OIDCClientID)
	form.Set("client_secret", idp.OIDCClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", codeVerifier)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := sanitize.SafeHTTPClient(h.cfg.Server.AllowLocalProviders, 10*time.Second)
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
		return nil, fmt.Errorf("OIDC provider did not return id_token")
	}

	// SECURITY: verify id_token signature against the IdP's JWKS before
	// trusting any claim. Also enforce iss/aud/exp/nonce.
	claims, err := h.verifyIDToken(ctx, idp, tokenResp.IDToken, expectedNonce)
	if err != nil {
		return nil, fmt.Errorf("id_token verification failed: %w", err)
	}
	return claims, nil
}

func (h *SSOHandler) jitProvisionUser(idp *models.IdentityProvider, email, name, oauthID string, groups []string) (*models.User, error) {
	var user models.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err == nil {
		// SECURITY: require the pre-existing account to already be linked to
		// this IdP before letting the IdP log in as it. Blocks an IdP admin
		// (or a compromised IdP account) from impersonating a pre-existing
		// password user who happens to share their email.
		if user.PasswordHash != "" && user.OAuthProvider == "" {
			h.logger.Warn("refusing SSO to pre-existing password account without explicit link",
				zap.String("idp_id", idp.ID.String()))
			return nil, fmt.Errorf("an account with this email already exists — link SSO from account settings first")
		}
		if user.OAuthProvider != "" && user.OAuthProvider != "sso_"+idp.Type {
			h.logger.Warn("refusing SSO to account linked to a different provider",
				zap.String("idp_id", idp.ID.String()),
				zap.String("existing_provider", user.OAuthProvider))
			return nil, fmt.Errorf("account is linked to a different identity provider")
		}
		return &user, nil
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

// callbackURL builds the OIDC redirect_uri from the server-side configured
// public URL. It deliberately does NOT read Host or X-Forwarded-Proto from
// the request so a header-injecting attacker cannot steer OAuth providers to
// attacker-controlled hosts.
func (h *SSOHandler) callbackURL(idpID string) string {
	base := strings.TrimRight(h.publicBackendURL(), "/")
	return fmt.Sprintf("%s/api/v1/sso/callback/%s", base, idpID)
}

// publicBackendURL returns the externally-visible backend URL used to build
// OAuth/SSO callback URLs. Prefers PUBLIC_BACKEND_URL, then FRONTEND_URL.
func (h *SSOHandler) publicBackendURL() string {
	if h.cfg.Frontend.PublicBackendURL != "" {
		return h.cfg.Frontend.PublicBackendURL
	}
	if h.cfg.Frontend.URL != "" {
		return h.cfg.Frontend.URL
	}
	return "http://localhost:8080"
}

func (h *SSOHandler) redirectWithError(c *gin.Context, errMsg string) {
	frontendURL := h.cfg.Frontend.URL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=%s", frontendURL, errMsg))
}

// ─── OIDC id_token signature verification (RS256 + JWKS) ────────────────────

// verifyIDToken validates an OIDC id_token:
//  1. discover JWKS via the issuer's /.well-known/openid-configuration
//  2. verify RS256 signature using the matching JWK (by kid)
//  3. verify iss matches the IdP issuer
//  4. verify aud contains the IdP client_id
//  5. verify exp is in the future
//  6. verify nonce matches the one we generated at Discover time
func (h *SSOHandler) verifyIDToken(ctx context.Context, idp *models.IdentityProvider, rawToken, expectedNonce string) (*oidcClaims, error) {
	keyFunc := func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing alg %q (only RS256 allowed)", t.Method.Alg())
		}
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("id_token missing kid header")
		}
		return h.resolveJWK(ctx, idp, kid)
	}

	parsed, err := jwt.Parse(rawToken, keyFunc, jwt.WithValidMethods([]string{"RS256"}))
	if err != nil {
		return nil, err
	}
	mapClaims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid id_token claims")
	}

	// iss
	iss, _ := mapClaims["iss"].(string)
	if iss == "" || !sameIssuer(iss, idp.OIDCIssuerURL) {
		return nil, fmt.Errorf("id_token issuer %q does not match configured issuer", iss)
	}

	// aud (string or []string)
	if !audienceContains(mapClaims["aud"], idp.OIDCClientID) {
		return nil, errors.New("id_token aud does not contain configured client_id")
	}

	// exp — jwt library already checks this via parsed.Valid, but be explicit
	if exp, ok := mapClaims["exp"].(float64); ok {
		if time.Now().Unix() >= int64(exp) {
			return nil, errors.New("id_token expired")
		}
	}

	// nonce
	gotNonce, _ := mapClaims["nonce"].(string)
	if subtle.ConstantTimeCompare([]byte(gotNonce), []byte(expectedNonce)) != 1 {
		return nil, errors.New("id_token nonce mismatch")
	}

	out := &oidcClaims{}
	out.Sub, _ = mapClaims["sub"].(string)
	out.Email, _ = mapClaims["email"].(string)
	out.Name, _ = mapClaims["name"].(string)
	// email_verified: some IdPs gate email with this claim. We require it if present.
	if ev, ok := mapClaims["email_verified"].(bool); ok && !ev {
		return nil, errors.New("id_token email is not verified at IdP")
	}
	if rawGroups, ok := mapClaims["groups"].([]interface{}); ok {
		for _, g := range rawGroups {
			if s, ok := g.(string); ok {
				out.Groups = append(out.Groups, s)
			}
		}
	}
	return out, nil
}

func sameIssuer(a, b string) bool {
	// Issuer comparison allows an optional trailing slash on either side.
	return strings.TrimRight(a, "/") == strings.TrimRight(b, "/")
}

func audienceContains(raw interface{}, clientID string) bool {
	switch v := raw.(type) {
	case string:
		return v == clientID
	case []interface{}:
		for _, x := range v {
			if s, ok := x.(string); ok && s == clientID {
				return true
			}
		}
	}
	return false
}

// ─── JWKS cache ─────────────────────────────────────────────────────────────

type jwkSet struct {
	Keys []jwkEntry `json:"keys"`
}

type jwkEntry struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksCacheEntry struct {
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
}

var (
	jwksCache   = map[string]jwksCacheEntry{}
	jwksCacheMu sync.Mutex
	// jwksTTL caps how long a single JWKS snapshot is considered fresh. After
	// expiry the next request will re-fetch. Shorter TTLs increase network
	// load; longer TTLs delay noticing IdP key rotation.
	jwksTTL = 5 * time.Minute
)

// resolveJWK fetches (and caches) the IdP's JWKS, then returns the matching
// RSA public key for the given kid.
func (h *SSOHandler) resolveJWK(ctx context.Context, idp *models.IdentityProvider, kid string) (*rsa.PublicKey, error) {
	cacheKey := idp.ID.String()

	jwksCacheMu.Lock()
	entry, ok := jwksCache[cacheKey]
	jwksCacheMu.Unlock()

	if ok && time.Now().Before(entry.expiresAt) {
		if key := entry.keys[kid]; key != nil {
			return key, nil
		}
		// Fall through: kid miss triggers a refresh to handle rotation.
	}

	keys, err := h.fetchJWKS(ctx, idp)
	if err != nil {
		return nil, err
	}
	jwksCacheMu.Lock()
	jwksCache[cacheKey] = jwksCacheEntry{keys: keys, expiresAt: time.Now().Add(jwksTTL)}
	jwksCacheMu.Unlock()

	if key := keys[kid]; key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("no JWK found for kid %s", kid)
}

func (h *SSOHandler) fetchJWKS(ctx context.Context, idp *models.IdentityProvider) (map[string]*rsa.PublicKey, error) {
	client := sanitize.SafeHTTPClient(h.cfg.Server.AllowLocalProviders, 10*time.Second)

	// 1. Discovery
	discoveryURL := strings.TrimRight(idp.OIDCIssuerURL, "/") + "/.well-known/openid-configuration"
	discReq, _ := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	discReq.Header.Set("Accept", "application/json")
	discResp, err := client.Do(discReq)
	if err != nil {
		return nil, fmt.Errorf("OIDC discovery request failed: %w", err)
	}
	defer func() { _ = discResp.Body.Close() }()
	if discResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OIDC discovery returned %d", discResp.StatusCode)
	}
	var disc struct {
		JWKSURI string `json:"jwks_uri"`
		Issuer  string `json:"issuer"`
	}
	if err := json.NewDecoder(discResp.Body).Decode(&disc); err != nil {
		return nil, fmt.Errorf("OIDC discovery decode: %w", err)
	}
	if disc.JWKSURI == "" {
		return nil, errors.New("OIDC discovery did not return jwks_uri")
	}
	if disc.Issuer != "" && !sameIssuer(disc.Issuer, idp.OIDCIssuerURL) {
		return nil, fmt.Errorf("OIDC discovery issuer %q does not match configured issuer", disc.Issuer)
	}

	// 2. JWKS fetch
	jwksReq, _ := http.NewRequestWithContext(ctx, "GET", disc.JWKSURI, nil)
	jwksResp, err := client.Do(jwksReq)
	if err != nil {
		return nil, fmt.Errorf("JWKS request failed: %w", err)
	}
	defer func() { _ = jwksResp.Body.Close() }()
	if jwksResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS returned %d", jwksResp.StatusCode)
	}
	var set jwkSet
	if err := json.NewDecoder(jwksResp.Body).Decode(&set); err != nil {
		return nil, fmt.Errorf("JWKS decode: %w", err)
	}

	// 3. Build kid -> *rsa.PublicKey map
	out := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" {
			continue
		}
		pub, err := rsaKeyFromJWK(k.N, k.E)
		if err != nil {
			h.logger.Warn("skipping malformed JWK", zap.String("kid", k.Kid), zap.Error(err))
			continue
		}
		out[k.Kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("JWKS contained no usable RSA keys")
	}
	return out, nil
}

func rsaKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, fmt.Errorf("bad n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, fmt.Errorf("bad e: %w", err)
	}
	// e is a big-endian unsigned integer; pad into an int.
	var e int
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("exponent is zero")
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
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
