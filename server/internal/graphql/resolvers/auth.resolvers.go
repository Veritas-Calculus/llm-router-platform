package resolvers

// This file contains auth domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"encoding/json"
	"fmt"
	"llm-router-platform/internal/graphql/directives"
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
	captchaToken := ""
	if input.CaptchaToken != nil {
		captchaToken = *input.CaptchaToken
	}
	if err := r.TurnstileSvc.Verify(ctx, captchaToken, ip); err != nil {
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
	return &model.AuthPayload{Token: token, RefreshToken: &refresh, User: userToGQL(u)}, nil
}

// Register is the resolver for the register field.
func (r *mutationResolver) Register(ctx context.Context, input model.RegisterInput) (*model.AuthPayload, error) {
	// Registration mode enforcement
	mode := r.Config().Registration.Mode
	if mode == "" {
		mode = "closed"
	}

	switch mode {
	case "closed":
		return nil, fmt.Errorf("registration is currently closed")
	case "invite":
		if input.InviteCode == nil || *input.InviteCode == "" {
			return nil, fmt.Errorf("invite code is required")
		}
		if err := r.verifyCaptcha(ctx, input.CaptchaToken); err != nil {
			return nil, err
		}
		if err := r.consumeInviteCode(ctx, *input.InviteCode); err != nil {
			return nil, err
		}
	case "open":
		if err := r.verifyCaptcha(ctx, input.CaptchaToken); err != nil {
			return nil, err
		}
	}

	u, err := r.UserSvc.Register(ctx, input.Email, input.Password, input.Name)
	if err != nil {
		return nil, err
	}

	// Onboard: create Org + Project + optional Welcome Credit
	grantCredit := r.checkWelcomeCreditEligibility(ctx)

	if err := user.OnboardAccount(ctx, r.AdminSvc.DB(), u, user.OnboardAccountParams{
		GrantWelcomeCredit: grantCredit,
	}, r.Logger); err != nil {
		r.Logger.Error("failed to onboard new user", zap.Error(err), zap.String("user_id", u.ID.String()))
	}

	ip, ua := clientInfo(ctx)
	r.AuditService.Log(ctx, audit.ActionRegister, u.ID, u.ID, ip, ua, nil)

	// Send email verification (non-blocking)
	go func() {
		rawToken, tokenErr := r.EmailVerifySvc.CreateVerificationToken(ctx, u.ID)
		if tokenErr != nil {
			r.Logger.Error("failed to create verification token", zap.Error(tokenErr), zap.String("user_id", u.ID.String()))
			return
		}
		if sendErr := r.EmailService.SendEmailVerification(u.Email, u.Name, rawToken); sendErr != nil {
			r.Logger.Error("failed to send verification email", zap.Error(sendErr), zap.String("user_id", u.ID.String()))
		}
	}()

	token, err := r.generateJWT(u)
	if err != nil {
		return nil, err
	}
	refresh, err := r.generateRefreshJWT(u)
	if err != nil {
		return nil, err
	}
	return &model.AuthPayload{Token: token, RefreshToken: &refresh, User: userToGQL(u)}, nil
}

// RefreshToken is the resolver for the refreshToken field.
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
	token, _ := r.generateJWT(u)
	return &model.AuthPayload{Token: token, User: userToGQL(u)}, nil
}

// RotateRefreshToken is the resolver for the rotateRefreshToken field.
func (r *mutationResolver) RotateRefreshToken(ctx context.Context, refreshToken string) (*model.AuthPayload, error) {
	// Validate the provided refresh token before issuing new ones
	claims, err := r.validateRefreshJWT(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	u, err := r.UserSvc.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Reject if user is deactivated
	if !u.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	// Reject if tokens were invalidated (logout, password change) after this refresh token was issued
	if !u.TokensInvalidatedAt.IsZero() {
		iat, _ := claims.GetIssuedAt()
		if iat != nil && iat.Before(u.TokensInvalidatedAt) {
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
	return &model.AuthPayload{Token: token, RefreshToken: &refresh, User: userToGQL(u)}, nil
}

// Logout is the resolver for the logout field.
func (r *mutationResolver) Logout(ctx context.Context) (bool, error) {
	uid, err := directives.UserIDFromContext(ctx)
	if err != nil {
		return false, err
	}
	id, _ := uuid.Parse(uid)
	_ = r.UserSvc.InvalidateTokens(ctx, id)
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
func (r *mutationResolver) VerifyEmail(ctx context.Context, token string) (bool, error) {
	_, err := r.EmailVerifySvc.VerifyEmail(ctx, token)
	if err != nil {
		return false, err
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
