// Package email provides transactional email services.
package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
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
		return nil
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

// SendEmailVerification sends a verification link to confirm an email address.
func (s *Service) SendEmailVerification(to, name, token string) error {
	if !s.config.Enabled {
		return nil
	}

	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.feURL, token)
	subject := "Verify Your Email - LLM Router"
	body, err := s.render("email_verification.html", map[string]string{
		"Name":      name,
		"VerifyURL": verifyURL,
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
// When TLS is enabled, it uses STARTTLS (ports 25/587) or implicit TLS (port 465)
// with proper certificate verification.
func (s *Service) send(to, subject, body string) error {
	// Guard against email header injection
	if err := validateEmailHeader(to); err != nil {
		return err
	}
	if err := validateEmailHeader(subject); err != nil {
		return err
	}

	// Sanitize: strip any CRLF from user-controlled values (defense-in-depth)
	safeTo := strings.ReplaceAll(strings.ReplaceAll(to, "\n", ""), "\r", "")
	safeSubject := strings.ReplaceAll(strings.ReplaceAll(subject, "\n", ""), "\r", "")

	fromAddr := s.config.From
	if s.config.FromName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", s.config.FromName, s.config.From)
	}

	// Build message from sanitized values only — no raw user input flows to SMTP
	headers := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		fromAddr,
		safeTo,
		safeSubject,
	)
	safeMsg := headers + body

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	if s.config.TLS {
		return s.sendTLS(addr, safeTo, safeMsg)
	}

	// Non-TLS fallback (local development only)
	var auth smtp.Auth
	if s.config.Username != "" {
		auth = smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
	}
	envelopeFrom := s.config.From
	envelopeTo := []string{safeTo}
	return smtp.SendMail(addr, auth, envelopeFrom, envelopeTo, []byte(safeMsg))
}

// sendTLS establishes a TLS connection for SMTP delivery.
// Port 465: implicit TLS (SMTPS). Ports 25/587: STARTTLS upgrade.
func (s *Service) sendTLS(addr, to, msg string) error {
	tlsConfig := &tls.Config{
		ServerName: s.config.Host,
		MinVersion: tls.VersionTLS12,
	}

	var conn net.Conn
	var err error

	if s.config.Port == 465 {
		// Implicit TLS (SMTPS)
		conn, err = tls.Dial("tcp", addr, tlsConfig)
	} else {
		// STARTTLS: connect plain first, then upgrade
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial failed: %w", err)
	}

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("smtp client creation failed: %w", err)
	}
	defer func() { _ = client.Close() }()

	// STARTTLS upgrade for non-465 ports
	if s.config.Port != 465 {
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp STARTTLS failed: %w", err)
		}
	}

	// Authenticate
	if s.config.Username != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	// Send
	if err := client.Mail(s.config.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO failed: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close failed: %w", err)
	}

	return client.Quit()
}
