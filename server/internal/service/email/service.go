// Package email provides transactional email services.
package email

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/service/email/templates"
)

// Service handles sending transactional emails.
type Service struct {
	config config.EmailConfig
	feURL  string
	tmpl   *template.Template
}

// NewService creates a new email service.
func NewService(cfg config.EmailConfig, feURL string) *Service {
	// Parse embedded templates once
	tmpl, err := template.ParseFS(templates.FS, "*.html")
	if err != nil {
		// Log this or panic, depending on application initialization strategy
		// For now, we print to console during startup
		fmt.Printf("Error parsing email templates: %v\n", err)
	}

	return &Service{
		config: cfg,
		feURL:  feURL,
		tmpl:   tmpl,
	}
}

// render is a helper to render a template into a string.
func (s *Service) render(name string, data interface{}) (string, error) {
	if s.tmpl == nil {
		return "", fmt.Errorf("templates not loaded")
	}

	var buf bytes.Buffer
	if err := s.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// SendResetPasswordEmail sends a password reset link to the user.
func (s *Service) SendResetPasswordEmail(to, token string) error {
	if !s.config.Enabled {
		return nil // Email disabled, ignore or log
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.feURL, token)
	subject := "Reset Your Password - LLM Router"
	body, err := s.render("reset_password.html", map[string]string{
		"ResetURL": resetURL,
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.send(to, subject, body)
}

// SendWelcomeEmail sends a welcome email to newly registered users.
func (s *Service) SendWelcomeEmail(to, name string) error {
	if !s.config.Enabled {
		return nil
	}

	subject := "Welcome to LLM Router!"
	body, err := s.render("welcome.html", map[string]string{
		"Name":     name,
		"LoginURL": fmt.Sprintf("%s/login", s.feURL),
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	return s.send(to, subject, body)
}

// SendQuotaWarningEmail sends a warning email when balance is low.
func (s *Service) SendQuotaWarningEmail(to, name, balance, threshold string) error {
	if !s.config.Enabled {
		return nil
	}

	subject := "Action Required: Low Balance Warning - LLM Router"
	body, err := s.render("quota_warning.html", map[string]string{
		"Name":         name,
		"Balance":      balance,
		"Threshold":    threshold,
		"DashboardURL": fmt.Sprintf("%s/admin/billing", s.feURL),
	})
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

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
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
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

