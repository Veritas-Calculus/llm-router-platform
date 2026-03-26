// Package handlers implements HTTP handlers for the API.
// oauth2.go implements OAuth2 social login (GitHub, Google).
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	configService "llm-router-platform/internal/service/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// OAuth2Handler handles OAuth2 social login flows.
type OAuth2Handler struct {
	cfg       *config.Config
	configSvc *configService.Service
	db        *gorm.DB
	logger    *zap.Logger
}

// NewOAuth2Handler creates a new OAuth2 handler.
func NewOAuth2Handler(cfg *config.Config, configSvc *configService.Service, db *gorm.DB, logger *zap.Logger) *OAuth2Handler {
	return &OAuth2Handler{cfg: cfg, configSvc: configSvc, db: db, logger: logger}
}

// resolveConfig reads OAuth2 config from DB (with env fallback) at request time.
func (h *OAuth2Handler) resolveConfig(ctx context.Context) config.OAuth2Config {
	if h.configSvc != nil {
		return h.configSvc.GetOAuth2Config(ctx, h.cfg.OAuth2)
	}
	return h.cfg.OAuth2
}

// providerConfig returns the OAuth2 provider config for given provider name.
func (h *OAuth2Handler) providerConfig(ctx context.Context, provider string) (*config.OAuth2ProviderConfig, error) {
	resolved := h.resolveConfig(ctx)
	switch provider {
	case "github":
		if resolved.GitHub.ClientID == "" {
			return nil, fmt.Errorf("GitHub OAuth2 not configured")
		}
		return &resolved.GitHub, nil
	case "google":
		if resolved.Google.ClientID == "" {
			return nil, fmt.Errorf("google OAuth2 not configured")
		}
		return &resolved.Google, nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// Redirect initiates the OAuth2 flow by redirecting to the provider's auth page.
func (h *OAuth2Handler) Redirect(c *gin.Context) {
	provider := c.Param("provider")
	pcfg, err := h.providerConfig(c.Request.Context(), provider)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "requested OAuth2 provider is not available"})
		return
	}

	// Generate state parameter (CSRF protection)
	state := uuid.New().String()
	c.SetCookie("oauth2_state", state, 300, "/", "", true, true) // 5 min, secure, httpOnly

	redirectURL := h.callbackURL(c, provider)

	var authURL string
	switch provider {
	case "github":
		authURL = fmt.Sprintf(
			"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&state=%s&scope=read:user user:email",
			pcfg.ClientID, redirectURL, state,
		)
	case "google":
		authURL = fmt.Sprintf(
			"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&state=%s&response_type=code&scope=openid email profile",
			pcfg.ClientID, redirectURL, state,
		)
	}

	c.Redirect(http.StatusTemporaryRedirect, authURL)
}

// Callback handles the OAuth2 callback from the provider.
func (h *OAuth2Handler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	// Validate state
	savedState, _ := c.Cookie("oauth2_state")
	if state == "" || state != savedState {
		h.redirectWithError(c, "Invalid state parameter")
		return
	}

	pcfg, err := h.providerConfig(c.Request.Context(), provider)
	if err != nil {
		h.redirectWithError(c, err.Error())
		return
	}

	// Exchange code for access token
	accessToken, err := h.exchangeCode(c.Request.Context(), provider, pcfg, code, h.callbackURL(c, provider))
	if err != nil {
		h.logger.Error("OAuth2 token exchange failed", zap.Error(err))
		h.redirectWithError(c, "Authentication failed")
		return
	}

	// Get user info from provider
	email, name, oauthID, err := h.getUserInfo(c.Request.Context(), provider, accessToken)
	if err != nil {
		h.logger.Error("OAuth2 user info fetch failed", zap.Error(err))
		h.redirectWithError(c, "Failed to retrieve user information")
		return
	}

	// Find or create user
	user, err := h.findOrCreateUser(email, name, provider, oauthID)
	if err != nil {
		h.logger.Error("OAuth2 user creation failed", zap.Error(err))
		h.redirectWithError(c, "Account creation failed")
		return
	}

	// Generate JWT
	token, err := h.generateJWT(user)
	if err != nil {
		h.logger.Error("OAuth2 JWT generation failed", zap.Error(err))
		h.redirectWithError(c, "Token generation failed")
		return
	}

	// Redirect to frontend with token
	frontendURL := h.cfg.Frontend.URL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/oauth/callback?token=%s", frontendURL, token))
}

// Providers returns the list of available OAuth2 providers (public endpoint).
func (h *OAuth2Handler) Providers(c *gin.Context) {
	resolved := h.resolveConfig(c.Request.Context())
	providers := make([]gin.H, 0)
	if resolved.GitHub.ClientID != "" {
		providers = append(providers, gin.H{"id": "github", "name": "GitHub"})
	}
	if resolved.Google.ClientID != "" {
		providers = append(providers, gin.H{"id": "google", "name": "Google"})
	}
	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

// ── Internal helpers ───────────────────────────────────────────────

func (h *OAuth2Handler) callbackURL(c *gin.Context, provider string) string {
	scheme := "https"
	if c.Request.TLS == nil && !strings.Contains(c.Request.Host, "localhost") {
		// Check X-Forwarded-Proto
		if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
	} else if c.Request.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/auth/oauth2/%s/callback", scheme, c.Request.Host, provider)
}

func (h *OAuth2Handler) redirectWithError(c *gin.Context, errMsg string) {
	frontendURL := h.cfg.Frontend.URL
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/login?error=%s", frontendURL, errMsg))
}

func (h *OAuth2Handler) exchangeCode(ctx context.Context, provider string, pcfg *config.OAuth2ProviderConfig, code, redirectURI string) (string, error) {
	var tokenURL string
	switch provider {
	case "github":
		tokenURL = "https://github.com/login/oauth/access_token" // #nosec G101 -- OAuth2 endpoint URL, not a credential
	case "google":
		tokenURL = "https://oauth2.googleapis.com/token" // #nosec G101 -- OAuth2 endpoint URL, not a credential
	}

	body := fmt.Sprintf("client_id=%s&client_secret=%s&code=%s&redirect_uri=%s&grant_type=authorization_code",
		pcfg.ClientID, pcfg.ClientSecret, code, redirectURI)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	token, ok := result["access_token"].(string)
	if !ok || token == "" {
		return "", fmt.Errorf("no access_token in response")
	}
	return token, nil
}

func (h *OAuth2Handler) getUserInfo(ctx context.Context, provider, accessToken string) (email, name, oauthID string, err error) {
	switch provider {
	case "github":
		return h.getGitHubUser(ctx, accessToken)
	case "google":
		return h.getGoogleUser(ctx, accessToken)
	default:
		return "", "", "", fmt.Errorf("unsupported provider")
	}
}

func (h *OAuth2Handler) getGitHubUser(ctx context.Context, token string) (email, name, oauthID string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Get user profile
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var user struct {
		ID    int    `json:"id"`
		Login string `json:"login"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", "", "", err
	}

	oauthID = fmt.Sprintf("%d", user.ID)
	name = user.Name
	if name == "" {
		name = user.Login
	}
	email = user.Email

	// If email is private, fetch from /user/emails
	if email == "" {
		req2, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user/emails", nil)
		req2.Header.Set("Authorization", "Bearer "+token)
		req2.Header.Set("Accept", "application/json")
		resp2, err := client.Do(req2)
		if err == nil {
			defer func() { _ = resp2.Body.Close() }()
			var emails []struct {
				Email    string `json:"email"`
				Primary  bool   `json:"primary"`
				Verified bool   `json:"verified"`
			}
			if json.NewDecoder(resp2.Body).Decode(&emails) == nil {
				for _, e := range emails {
					if e.Primary && e.Verified {
						email = e.Email
						break
					}
				}
			}
		}
	}

	if email == "" {
		return "", "", "", fmt.Errorf("could not retrieve email from GitHub")
	}
	return email, name, oauthID, nil
}

func (h *OAuth2Handler) getGoogleUser(ctx context.Context, token string) (email, name, oauthID string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	var user struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", "", "", err
	}

	return user.Email, user.Name, user.ID, nil
}

func (h *OAuth2Handler) findOrCreateUser(email, name, provider, oauthID string) (*models.User, error) {
	var user models.User

	// First try: find by OAuth ID + provider
	if err := h.db.Where("oauth_provider = ? AND oauth_id = ?", provider, oauthID).First(&user).Error; err == nil {
		return &user, nil
	}

	// Second try: find by email (link accounts)
	if err := h.db.Where("email = ?", email).First(&user).Error; err == nil {
		// Link OAuth to existing account (only if no OAuth provider set yet)
		updates := map[string]interface{}{}
		if user.OAuthProvider == "" {
			updates["oauth_provider"] = provider
			updates["oauth_id"] = oauthID
		}
		// OAuth provider has verified the email, mark as verified
		if !user.EmailVerified {
			now := time.Now()
			updates["email_verified"] = true
			updates["email_verified_at"] = now
		}
		if len(updates) > 0 {
			h.db.Model(&user).Updates(updates)
		}
		return &user, nil
	}

	// Create new user (no password for OAuth users)
	now := time.Now()
	user = models.User{
		Email:           email,
		PasswordHash:    "", // OAuth users have no password
		Name:            name,
		Role:            "user",
		IsActive:        true,
		OAuthProvider:   provider,
		OAuthID:         oauthID,
		EmailVerified:   true, // OAuth provider verified the email
		EmailVerifiedAt: &now,
	}
	if err := h.db.Create(&user).Error; err != nil {
		return nil, err
	}

	// Auto-create Org + Project + welcome credit (same as register)
	org := models.Organization{Name: name + "'s Org", OwnerID: user.ID}
	if err := h.db.Create(&org).Error; err == nil {
		member := models.OrganizationMember{OrgID: org.ID, UserID: user.ID, Role: "OWNER"}
		h.db.Create(&member)
		project := models.Project{OrgID: org.ID, Name: "Default", Description: "Auto-created project"}
		h.db.Create(&project)
		user.Balance = 5.0
		h.db.Model(&user).UpdateColumn("balance", 5.0)
		tx := models.Transaction{OrgID: org.ID, UserID: user.ID, Type: "recharge", Amount: 5.0, Balance: 5.0, Description: "Welcome credit", Currency: "USD"}
		h.db.Create(&tx)
	}

	return &user, nil
}

func (h *OAuth2Handler) generateJWT(u *models.User) (string, error) {
	ttl := h.cfg.JWT.ExpiresIn
	if ttl <= 0 {
		ttl = time.Hour // Default: 1 hour (consistent with resolver)
	}
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"role": u.Role,
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWT.Secret))
}
