package resolvers

// This file contains auth domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"llm-router-platform/internal/api/handlers"
	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/errs"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/service/audit"
	"llm-router-platform/internal/service/user"
	"llm-router-platform/pkg/sanitize"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, input model.LoginInput) (*model.AuthPayload, error) {
	ip, ua := clientInfo(ctx)

	// ── Login failure rate limiting ──
	if err := r.LoginLimiter.Check(ctx, input.Email, ip); err != nil {
		r.AuditService.Log(ctx, audit.ActionLoginFailed, uuid.Nil, uuid.Nil, ip, ua, map[string]interface{}{"email": sanitize.LogValue(input.Email), "reason": "rate_limited"})
		return nil, err
	}

	// ── Turnstile CAPTCHA verification ──
	// Force a CAPTCHA solve once the per-email failure counter trips, even
	// if Turnstile is otherwise disabled. This is the IP-agnostic brake on
	// botnets rotating residential IPs (each new IP resets the per-(email,ip)
	// counter but never the per-email one).
	captchaToken := ""
	if input.CaptchaToken != nil {
		captchaToken = *input.CaptchaToken
	}
	if r.LoginLimiter.RequireCaptcha(ctx, input.Email) {
		if err := r.TurnstileSvc.VerifyForced(ctx, captchaToken, ip); err != nil {
			r.AuditService.Log(ctx, audit.ActionLoginFailed, uuid.Nil, uuid.Nil, ip, ua, map[string]interface{}{"email": sanitize.LogValue(input.Email), "reason": "captcha_required"})
			return nil, err
		}
	} else if err := r.TurnstileSvc.Verify(ctx, captchaToken, ip); err != nil {
		return nil, err
	}

	u, err := r.UserSvc.Authenticate(ctx, input.Email, input.Password)
	if err != nil {
		r.LoginLimiter.RecordFailure(ctx, input.Email, ip)
		r.AuditService.Log(ctx, audit.ActionLoginFailed, uuid.Nil, uuid.Nil, ip, ua, map[string]interface{}{"email": sanitize.LogValue(input.Email)})
		return nil, fmt.Errorf("invalid credentials")
	}

	r.LoginLimiter.ResetOnSuccess(ctx, input.Email, ip)
	r.AuditService.Log(ctx, audit.ActionLogin, u.ID, u.ID, ip, ua, nil)
	token, err := r.generateJWT(u)
	if err != nil {
		return nil, err
	}
	refresh, err := r.generateRefreshJWT(u)
	if err != nil {
		return nil, err
	}
	r.setRefreshTokenCookie(ctx, refresh)
	r.setAccessTokenCookie(ctx, token)
	return &model.AuthPayload{Token: token, User: userToGQL(u)}, nil
}

// Register is the resolver for the register field.
//
// SECURITY (C-01): the $5 welcome credit is NOT granted at registration
// anymore. Credit is granted by VerifyEmail once the user proves they own
// the email address. Captcha is enforced for every mode, and is verified
// BEFORE we create the user record — a failed captcha must not leave a
// half-onboarded row behind.
func (r *mutationResolver) Register(ctx context.Context, input model.RegisterInput) (*model.AuthPayload, error) {
	// Registration mode enforcement
	mode := r.Config().Registration.Mode
	if mode == "" {
		mode = "closed"
	}

	if mode == "closed" {
		return nil, fmt.Errorf("registration is currently closed")
	}

	// Verify captcha first so failed captcha doesn't leave half-created rows.
	// The captcha service handles dev/turnstile/hcaptcha/disabled selection.
	if err := r.verifyCaptcha(ctx, input.CaptchaToken); err != nil {
		return nil, err
	}

	if mode == "invite" {
		if input.InviteCode == nil || *input.InviteCode == "" {
			return nil, errs.Validation("inviteCode", "invite code is required")
		}
		if err := r.consumeInviteCode(ctx, *input.InviteCode); err != nil {
			return nil, err
		}
	}

	u, err := r.UserSvc.Register(ctx, input.Email, input.Password, input.Name)
	if err != nil {
		// Translate user-service errors into typed validation errors so the
		// frontend can highlight the offending field. Pre-existing typed
		// errors (errs.Validation) propagate unchanged.
		var ve *errs.ValidationError
		if errors.As(err, &ve) {
			return nil, err
		}
		if isPasswordPolicyError(err) {
			return nil, errs.Validation("password", err.Error())
		}
		// "registration failed" is the user-service's deliberately vague
		// response to duplicate-email collisions — kept generic to prevent
		// user enumeration. We surface it as a generic email-field error
		// so the form can still attach the message to the email input
		// without leaking whether the email exists.
		if err.Error() == "registration failed" {
			return nil, errs.Validation("email", "registration failed")
		}
		return nil, err
	}

	// Onboard: create Org + Project + membership. NO welcome credit yet —
	// VerifyEmail grants it once email ownership is proven.
	if err := user.OnboardAccount(ctx, r.AdminSvc.DB(), u, user.OnboardAccountParams{
		GrantWelcomeCredit: false,
	}, r.Logger); err != nil {
		r.Logger.Error("failed to onboard new user", zap.Error(err), zap.String("user_id", u.ID.String()))
	}

	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionRegister, u.ID, u.ID, ip, ua, nil)

	// Create the verification token synchronously so we can report
	// emailVerificationSent accurately. The send is non-blocking with a
	// background context (the caller's ctx may be canceled when the
	// response is written).
	sendOK := true
	rawToken, tokenErr := r.EmailVerifySvc.CreateVerificationToken(ctx, u.ID)
	if tokenErr != nil {
		r.Logger.Error("failed to create verification token",
			zap.Error(tokenErr), zap.String("user_id", u.ID.String()))
		sendOK = false
	} else {
		uID := u.ID
		uEmail := u.Email
		uName := u.Name
		// Capture frontend URL for the dev-mode console log fallback below.
		feURL := r.Config().Frontend.URL
		go func() {
			sendErr := r.EmailService.SendEmailVerification(uEmail, uName, rawToken)
			if sendErr != nil {
				r.Logger.Error("failed to send verification email",
					zap.Error(sendErr), zap.String("user_id", uID.String()))
				return
			}
			// If SMTP is disabled (no host configured), the email service
			// silently returns nil. Surface the verification link in the
			// server log so local docker-compose users can still verify.
			if !r.Config().Email.Enabled {
				r.Logger.Info("email verification link (SMTP disabled)",
					zap.String("user_id", uID.String()),
					zap.String("url", fmt.Sprintf("%s/verify-email?token=%s", feURL, rawToken)))
			}
		}()
	}

	token, err := r.generateJWT(u)
	if err != nil {
		return nil, err
	}
	refresh, err := r.generateRefreshJWT(u)
	if err != nil {
		return nil, err
	}
	r.setRefreshTokenCookie(ctx, refresh)
	r.setAccessTokenCookie(ctx, token)
	return &model.AuthPayload{
		Token:                 token,
		User:                  userToGQL(u),
		EmailVerificationSent: &sendOK,
	}, nil
}

// RefreshToken is the resolver for the refreshToken field.
//
// This re-issues an access token from an already-authenticated session. The
// @auth directive ensures the incoming token is currently valid, and the REST
// middleware enforces the iat ≥ TokensInvalidatedAt invariant on the inbound
// token. We re-check that invariant here as defense in depth — without this,
// any future bypass of the middleware would let a stolen token be re-minted
// with a fresh iat that escapes future logout/password-change revocations.
func (r *mutationResolver) RefreshToken(ctx context.Context) (*model.AuthPayload, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, _ := uuid.Parse(uid)
	u, err := r.UserSvc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !u.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}
	if !u.TokensInvalidatedAt.IsZero() {
		if gc, gcErr := directives.GinContextFromContext(ctx); gcErr == nil {
			if v, ok := gc.Get("token_iat"); ok {
				if iat, ok := v.(time.Time); ok && iat.Before(u.TokensInvalidatedAt) {
					return nil, fmt.Errorf("token has been revoked")
				}
			}
		}
	}
	token, err := r.generateJWT(u)
	if err != nil {
		return nil, err
	}
	r.setAccessTokenCookie(ctx, token)
	return &model.AuthPayload{Token: token, User: userToGQL(u)}, nil
}

// RotateRefreshToken is the resolver for the rotateRefreshToken field.
func (r *mutationResolver) RotateRefreshToken(ctx context.Context, refreshToken string) (*model.AuthPayload, error) {
	usingCookie := false
	if refreshToken == "" {
		var ok bool
		refreshToken, ok = refreshTokenFromCookie(ctx)
		if !ok {
			return nil, fmt.Errorf("invalid refresh token")
		}
		usingCookie = true
	}

	// Validate the provided refresh token before issuing new ones
	claims, err := r.validateRefreshJWT(refreshToken)
	if err != nil {
		if usingCookie {
			r.clearRefreshTokenCookie(ctx)
		}
		return nil, fmt.Errorf("invalid refresh token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		if usingCookie {
			r.clearRefreshTokenCookie(ctx)
		}
		return nil, fmt.Errorf("invalid refresh token")
	}
	u, err := r.UserSvc.GetByID(ctx, id)
	if err != nil {
		if usingCookie {
			r.clearRefreshTokenCookie(ctx)
		}
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Reject if user is deactivated
	if !u.IsActive {
		if usingCookie {
			r.clearRefreshTokenCookie(ctx)
		}
		return nil, fmt.Errorf("account is disabled")
	}

	// Reject if tokens were invalidated (logout, password change) after this refresh token was issued
	if !u.TokensInvalidatedAt.IsZero() {
		iat, _ := claims.GetIssuedAt()
		if iat != nil && iat.Before(u.TokensInvalidatedAt) {
			if usingCookie {
				r.clearRefreshTokenCookie(ctx)
			}
			return nil, fmt.Errorf("refresh token has been revoked")
		}
	}

	token, err := r.generateJWT(u)
	if err != nil {
		return nil, err
	}
	refresh, err := r.generateRefreshJWT(u)
	if err != nil {
		return nil, err
	}
	r.setRefreshTokenCookie(ctx, refresh)
	r.setAccessTokenCookie(ctx, token)
	if usingCookie {
		return &model.AuthPayload{Token: token, User: userToGQL(u)}, nil
	}
	return &model.AuthPayload{Token: token, RefreshToken: &refresh, User: userToGQL(u)}, nil
}

// ExchangeOAuthCode redeems the HttpOnly cookie that an OAuth/SSO callback
// set after successful authentication. The cookie holds a short opaque
// code; we look it up in Redis to recover the user_id, fetch the user, and
// mint a fresh access+refresh pair. The code is consumed (DEL) on a
// successful lookup so it can't be replayed.
func (r *mutationResolver) ExchangeOAuthCode(ctx context.Context) (*model.AuthPayload, error) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("invalid request")
	}
	code, err := gc.Cookie(handlers.OAuthExchangeCookieName)
	if err != nil || code == "" {
		return nil, fmt.Errorf("no exchange cookie")
	}
	// Clear the cookie regardless of outcome.
	gc.SetSameSite(http.SameSiteLaxMode)
	gc.SetCookie(handlers.OAuthExchangeCookieName, "", -1, "/", "", true, true)

	redisClient := r.RedisClient()
	if redisClient == nil {
		return nil, fmt.Errorf("auth backend unavailable")
	}
	key := handlers.OAuthExchangeRedisKey(code)
	uidStr, err := redisClient.GetDel(ctx, key).Result()
	if err != nil || uidStr == "" {
		return nil, fmt.Errorf("invalid or expired code")
	}
	uid, err := uuid.Parse(uidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid code payload")
	}
	u, err := r.UserSvc.GetByID(ctx, uid)
	if err != nil || u == nil {
		return nil, fmt.Errorf("user not found")
	}
	if !u.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	token, err := r.generateJWT(u)
	if err != nil {
		return nil, err
	}
	refresh, err := r.generateRefreshJWT(u)
	if err != nil {
		return nil, err
	}
	r.setRefreshTokenCookie(ctx, refresh)
	r.setAccessTokenCookie(ctx, token)
	return &model.AuthPayload{Token: token, User: userToGQL(u)}, nil
}

// Logout is the resolver for the logout field.
func (r *mutationResolver) Logout(ctx context.Context) (bool, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, _ := uuid.Parse(uid)
	_ = r.UserSvc.InvalidateTokens(ctx, id)
	r.clearRefreshTokenCookie(ctx)
	r.clearAccessTokenCookie(ctx)
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionLogout, id, id, ip, ua, nil)
	return true, nil
}

// ForgotPassword is the resolver for the forgotPassword field.
func (r *mutationResolver) ForgotPassword(ctx context.Context, email string) (bool, error) {
	u, err := r.UserSvc.GetByEmail(ctx, email)
	if err != nil {
		return true, nil // don't reveal if email exists
	}
	token, err := r.PasswordResetSvc.CreateResetToken(ctx, u.ID)
	if err != nil {
		// Log internally but don't reveal to user
		r.Logger.Error("failed to create reset token", zap.Error(err))
		return true, nil
	}
	_ = r.EmailService.SendResetPasswordEmail(u.Email, token)
	return true, nil
}

// ResetPassword is the resolver for the resetPassword field.
func (r *mutationResolver) ResetPassword(ctx context.Context, input model.ResetPasswordInput) (bool, error) {
	userID, err := r.PasswordResetSvc.ValidateAndConsumeToken(ctx, input.Token)
	if err != nil {
		return false, fmt.Errorf("invalid or expired reset token")
	}
	if err := r.UserSvc.ResetPassword(ctx, userID, input.NewPassword); err != nil {
		return false, err
	}
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionPasswordChange, userID, userID, ip, ua, map[string]interface{}{"method": "reset_token"})
	return true, nil
}

// ChangePassword is the resolver for the changePassword field.
func (r *mutationResolver) ChangePassword(ctx context.Context, input model.ChangePasswordInput) (bool, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, _ := uuid.Parse(uid)
	if err := r.UserSvc.ChangePassword(ctx, id, input.OldPassword, input.NewPassword); err != nil {
		return false, err
	}
	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionPasswordChange, id, id, ip, ua, nil)
	return true, nil
}

// UpdateProfile is the resolver for the updateProfile field.
func (r *mutationResolver) UpdateProfile(ctx context.Context, input model.UpdateProfileInput) (*model.User, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, _ := uuid.Parse(uid)
	name := ""
	if input.Name != nil {
		name = *input.Name
	}
	if err := r.UserSvc.UpdateProfile(ctx, id, name); err != nil {
		return nil, err
	}
	u, _ := r.UserSvc.GetByID(ctx, id)
	return userToGQL(u), nil
}

// VerifyEmail is the resolver for the verifyEmail field.
//
// C-01: Once email ownership is proven, this also grants the $5 welcome
// credit — but only once per account. Re-verification (idempotency) and
// pre-existing users with a non-NULL welcome_credit_granted_at are no-ops
// on the credit step.
func (r *mutationResolver) VerifyEmail(ctx context.Context, token string) (bool, error) {
	userID, err := r.EmailVerifySvc.VerifyEmail(ctx, token)
	if err != nil {
		return false, err
	}

	// Award welcome credit if (a) we haven't credited this user before, and
	// (b) the IP-based anti-abuse throttle in checkWelcomeCreditEligibility
	// permits it. Both conditions must hold; both failure modes leave the
	// user verified but uncredited.
	u, getErr := r.UserSvc.GetByID(ctx, userID)
	if getErr != nil {
		r.Logger.Error("VerifyEmail: failed to load user", zap.Error(getErr), zap.String("user_id", userID.String()))
		return true, nil
	}

	if u.WelcomeCreditGrantedAt != nil {
		// Already credited; treat as no-op success.
		return true, nil
	}

	if !r.checkWelcomeCreditEligibility(ctx) {
		r.Logger.Warn("VerifyEmail: welcome credit denied by IP throttle",
			zap.String("user_id", userID.String()))
		return true, nil
	}

	if err := r.Balance.GrantWelcomeCredit(ctx, u); err != nil {
		r.Logger.Error("VerifyEmail: failed to grant welcome credit", zap.Error(err), zap.String("user_id", userID.String()))
		// Don't fail the verification — the user IS verified, just uncredited.
	}
	return true, nil
}

// ResendVerificationEmail is the resolver for the resendVerificationEmail field.
func (r *mutationResolver) ResendVerificationEmail(ctx context.Context) (bool, error) {
	uidStr, _ := directives.UserIDFromContext(ctx)
	userID, err := uuid.Parse(uidStr)
	if err != nil {
		return false, fmt.Errorf("invalid user")
	}

	u, err := r.UserSvc.GetByID(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("user not found")
	}

	if u.EmailVerified {
		return false, fmt.Errorf("email is already verified")
	}

	rawToken, err := r.EmailVerifySvc.CreateVerificationToken(ctx, u.ID)
	if err != nil {
		return false, fmt.Errorf("failed to create verification token")
	}

	if sendErr := r.EmailService.SendEmailVerification(u.Email, u.Name, rawToken); sendErr != nil {
		r.Logger.Error("failed to send verification email", zap.Error(sendErr))
		// Don't fail — token was created, user can retry
	}

	return true, nil
}

// GenerateMfaSecret is the resolver for the generateMfaSecret field.
func (r *mutationResolver) GenerateMfaSecret(ctx context.Context) (*model.MfaSecretInfo, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, err := uuid.Parse(uid)
	if err != nil {
		return nil, fmt.Errorf("unauthorized")
	}

	u, err := r.UserSvc.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	info, err := r.UserSvc.GenerateMfaSecret(ctx, id, u.Email)
	if err != nil {
		return nil, err
	}

	return &model.MfaSecretInfo{
		Secret:      info.Secret,
		QRCodeURL:   info.QrCodeUrl,
		BackupCodes: info.BackupCodes,
	}, nil
}

// VerifyAndEnableMfa is the resolver for the verifyAndEnableMfa field.
func (r *mutationResolver) VerifyAndEnableMfa(ctx context.Context, code string) (bool, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, err := uuid.Parse(uid)
	if err != nil {
		return false, fmt.Errorf("unauthorized")
	}

	return r.UserSvc.VerifyAndEnableMfa(ctx, id, code)
}

// DisableMfa is the resolver for the disableMfa field.
func (r *mutationResolver) DisableMfa(ctx context.Context, code string) (bool, error) {
	uid, _ := directives.UserIDFromContext(ctx)
	id, err := uuid.Parse(uid)
	if err != nil {
		return false, fmt.Errorf("unauthorized")
	}

	return r.UserSvc.DisableMfa(ctx, id, code)
}

// Me is the resolver for the me field.
func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id, _ := uuid.Parse(uid)
	u, err := r.UserSvc.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return userToGQL(u), nil
}

// PasswordPolicy returns the server-enforced password policy. The frontend
// reads this on the signup form so its inline hint always matches what the
// server actually validates (M-09: avoid the "Minimum 6 characters" lie
// shipped while the server required 8). Public so unauthenticated pages
// can read it.
//
// Keep this in sync with server/internal/service/user/user.go::ValidatePassword
// and web/src/lib/auth/passwordPolicy.ts. Post-audit P0-2 added the
// blockCommonPasswords flag so the client-side validator can pre-empt the
// server's "password is too common" rejection instead of advertising fewer
// rules than the server enforces.
func (r *queryResolver) PasswordPolicy(_ context.Context) (*model.PasswordPolicy, error) {
	return &model.PasswordPolicy{
		MinLength:            8,
		RequireLetter:        true,
		RequireDigit:         true,
		RequireUpper:         true,
		RequireLower:         true,
		BlockCommonPasswords: true,
	}, nil
}

// RegistrationMode is the resolver for the registrationMode field.
// This is a public query (no @auth directive) so the login page can adapt.
func (r *queryResolver) RegistrationMode(ctx context.Context) (*model.RegistrationMode, error) {
	mode := r.Config().Registration.Mode
	if mode == "" {
		mode = "closed"
	}
	return &model.RegistrationMode{
		Mode:               mode,
		InviteCodeRequired: mode == "invite",
	}, nil
}

// SiteConfig is the resolver for the siteConfig field.
func (r *queryResolver) SiteConfig(ctx context.Context) (*model.SiteConfig, error) {
	result := &model.SiteConfig{
		SiteName:   "Router",
		Subtitle:   "",
		LogoURL:    "",
		FaviconURL: "",
	}

	all, err := r.SystemConfig.GetAllSettings(ctx)
	if err != nil {
		return result, nil
	}

	siteJSON, ok := all["site"]
	if !ok {
		return result, nil
	}

	var parsed struct {
		SiteName   string `json:"siteName"`
		Subtitle   string `json:"subtitle"`
		LogoUrl    string `json:"logoUrl"`
		FaviconUrl string `json:"faviconUrl"`
	}
	if json.Unmarshal([]byte(siteJSON), &parsed) == nil {
		if parsed.SiteName != "" {
			result.SiteName = parsed.SiteName
		}
		result.Subtitle = parsed.Subtitle
		result.LogoURL = parsed.LogoUrl
		result.FaviconURL = parsed.FaviconUrl
	}

	return result, nil
}
