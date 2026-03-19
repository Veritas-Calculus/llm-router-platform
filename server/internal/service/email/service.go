// Package email provides transactional email services.
package email

import (
	"fmt"
	"net/smtp"
	"strings"

	"llm-router-platform/internal/config"
)

// Service handles sending transactional emails.
type Service struct {
	config config.EmailConfig
	feURL  string
}

// NewService creates a new email service.
func NewService(cfg config.EmailConfig, feURL string) *Service {
	return &Service{
		config: cfg,
		feURL:  feURL,
	}
}

// SendResetPasswordEmail sends a password reset link to the user.
func (s *Service) SendResetPasswordEmail(to, token string) error {
	if !s.config.Enabled {
		return nil // Email disabled, ignore or log
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.feURL, token)
	subject := "Reset Your Password - LLM Router"
	body := fmt.Sprintf(
		"Hello,\n\nYou requested a password reset for your LLM Router account. "+
			"Please click the link below to set a new password:\n\n%s\n\n"+
			"This link will expire in 1 hour. If you did not request this, please ignore this email.\n\n"+
			"Best regards,\nThe LLM Router Team",
		resetURL,
	)

	return s.send(to, subject, body)
}

// SendWelcomeEmail sends a welcome email to newly registered users.
func (s *Service) SendWelcomeEmail(to, name string) error {
	if !s.config.Enabled {
		return nil
	}

	subject := "Welcome to LLM Router!"
	body := fmt.Sprintf(
		"Hello %s,\n\nWelcome to LLM Router! Your account has been successfully created.\n\n"+
			"You can now log in and start using our intelligent routing services.\n\n"+
			"Best regards,\nThe LLM Router Team",
		name,
	)

	return s.send(to, subject, body)
}

// validateEmailHeader rejects strings containing CR or LF to prevent
// email header injection (CWE-93 / SMTP header injection).
func validateEmailHeader(s string) error {
	if strings.ContainsAny(s, "\r\n") {
		return fmt.Errorf("email header injection attempt detected")
	}
	return nil
}

// send is a helper to perform the actual SMTP delivery.
func (s *Service) send(to, subject, body string) error {
	// Guard against email header injection
	if err := validateEmailHeader(to); err != nil {
		return err
	}
	if err := validateEmailHeader(subject); err != nil {
		return err
	}

	fromAddr := s.config.From
	if s.config.FromName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		fromAddr,
		to,
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}

	return smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(msg))
}

