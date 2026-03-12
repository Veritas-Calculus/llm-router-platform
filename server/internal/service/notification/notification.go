// Package notification provides multi-channel alert delivery.
// Supports Webhook, Email (SMTP), DingTalk, and Feishu notification channels.
package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ChannelType defines the notification channel type.
type ChannelType string

const (
	ChannelWebhook  ChannelType = "webhook"
	ChannelEmail    ChannelType = "email"
	ChannelDingTalk ChannelType = "dingtalk"
	ChannelFeishu   ChannelType = "feishu"
)

// AlertPayload represents a standardized alert message.
type AlertPayload struct {
	TargetType string                 `json:"target_type"`
	TargetID   string                 `json:"target_id"`
	AlertType  string                 `json:"alert_type"`
	Message    string                 `json:"message"`
	Timestamp  string                 `json:"timestamp"`
	Severity   string                 `json:"severity"` // info, warning, critical
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// Channel is the interface for notification delivery.
type Channel interface {
	Send(ctx context.Context, payload *AlertPayload) error
	Type() ChannelType
}

// ─── Webhook Channel ────────────────────────────────────────

// WebhookChannel sends alerts via HTTP POST.
type WebhookChannel struct {
	url    string
	client *http.Client
}

// NewWebhookChannel creates a new webhook channel.
func NewWebhookChannel(url string) *WebhookChannel {
	return &WebhookChannel{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send sends a webhook notification.
func (c *WebhookChannel) Send(ctx context.Context, payload *AlertPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// Type returns the channel type.
func (c *WebhookChannel) Type() ChannelType { return ChannelWebhook }

// ─── Email Channel ──────────────────────────────────────────

// EmailConfig holds SMTP configuration.
type EmailConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

// EmailChannel sends alerts via SMTP email.
type EmailChannel struct {
	config     EmailConfig
	recipients []string
}

// NewEmailChannel creates a new email channel.
func NewEmailChannel(config EmailConfig, recipients []string) *EmailChannel {
	return &EmailChannel{
		config:     config,
		recipients: recipients,
	}
}

// Send sends an email notification.
func (c *EmailChannel) Send(_ context.Context, payload *AlertPayload) error {
	subject := fmt.Sprintf("[LLM Router %s] %s - %s", strings.ToUpper(payload.Severity), payload.AlertType, payload.TargetType)

	body := fmt.Sprintf(
		"Alert: %s\nTarget: %s (%s)\nSeverity: %s\nTime: %s\n\nMessage:\n%s",
		payload.AlertType,
		payload.TargetID,
		payload.TargetType,
		payload.Severity,
		payload.Timestamp,
		payload.Message,
	)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		c.config.From,
		strings.Join(c.recipients, ", "),
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	var auth smtp.Auth
	if c.config.Username != "" {
		auth = smtp.PlainAuth("", c.config.Username, c.config.Password, c.config.Host)
	}

	if err := smtp.SendMail(addr, auth, c.config.From, c.recipients, []byte(msg)); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

// Type returns the channel type.
func (c *EmailChannel) Type() ChannelType { return ChannelEmail }

// ─── DingTalk Channel ───────────────────────────────────────

// DingTalkChannel sends alerts to DingTalk robot webhook.
type DingTalkChannel struct {
	webhookURL string
	secret     string // for sign verification
	client     *http.Client
}

// NewDingTalkChannel creates a new DingTalk channel.
func NewDingTalkChannel(webhookURL, secret string) *DingTalkChannel {
	return &DingTalkChannel{
		webhookURL: webhookURL,
		secret:     secret,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send sends a DingTalk notification.
func (c *DingTalkChannel) Send(ctx context.Context, payload *AlertPayload) error {
	title := fmt.Sprintf("🚨 [%s] %s", strings.ToUpper(payload.Severity), payload.AlertType)
	text := fmt.Sprintf(
		"### %s\n\n- **目标**: %s (%s)\n- **级别**: %s\n- **时间**: %s\n\n> %s",
		title,
		payload.TargetID, payload.TargetType,
		payload.Severity,
		payload.Timestamp,
		payload.Message,
	)

	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal dingtalk message: %w", err)
	}

	url := c.webhookURL
	if c.secret != "" {
		url = c.signURL(url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create dingtalk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send dingtalk: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("dingtalk returned status %d", resp.StatusCode)
	}
	return nil
}

// signURL adds DingTalk signature to webhook URL.
func (c *DingTalkChannel) signURL(webhookURL string) string {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	stringToSign := ts + "\n" + c.secret

	h := hmac.New(sha256.New, []byte(c.secret))
	h.Write([]byte(stringToSign))
	sign := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%s&timestamp=%s&sign=%s", webhookURL, ts, sign)
}

// Type returns the channel type.
func (c *DingTalkChannel) Type() ChannelType { return ChannelDingTalk }

// ─── Feishu (Lark) Channel ──────────────────────────────────

// FeishuChannel sends alerts to Feishu (Lark) robot webhook.
type FeishuChannel struct {
	webhookURL string
	secret     string
	client     *http.Client
}

// NewFeishuChannel creates a new Feishu channel.
func NewFeishuChannel(webhookURL, secret string) *FeishuChannel {
	return &FeishuChannel{
		webhookURL: webhookURL,
		secret:     secret,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send sends a Feishu notification.
func (c *FeishuChannel) Send(ctx context.Context, payload *AlertPayload) error {
	title := fmt.Sprintf("🚨 [%s] %s", strings.ToUpper(payload.Severity), payload.AlertType)
	content := fmt.Sprintf(
		"目标: %s (%s)\n级别: %s\n时间: %s\n\n%s",
		payload.TargetID, payload.TargetType,
		payload.Severity,
		payload.Timestamp,
		payload.Message,
	)

	msg := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"content": title,
					"tag":     "plain_text",
				},
				"template": c.severityColor(payload.Severity),
			},
			"elements": []interface{}{
				map[string]interface{}{
					"tag": "div",
					"text": map[string]string{
						"content": content,
						"tag":     "lark_md",
					},
				},
			},
		},
	}

	// Add timestamp signature if secret is configured
	if c.secret != "" {
		ts := fmt.Sprintf("%d", time.Now().Unix())
		stringToSign := ts + "\n" + c.secret
		h := hmac.New(sha256.New, []byte(stringToSign))
		h.Write([]byte{})
		sign := base64.StdEncoding.EncodeToString(h.Sum(nil))
		msg["timestamp"] = ts
		msg["sign"] = sign
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal feishu message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create feishu request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("feishu returned status %d", resp.StatusCode)
	}
	return nil
}

// severityColor returns Feishu card header color by severity.
func (c *FeishuChannel) severityColor(severity string) string {
	switch severity {
	case "critical":
		return "red"
	case "warning":
		return "orange"
	default:
		return "blue"
	}
}

// Type returns the channel type.
func (c *FeishuChannel) Type() ChannelType { return ChannelFeishu }

// ─── Dispatcher ─────────────────────────────────────────────

// Dispatcher manages multiple notification channels and dispatches alerts.
type Dispatcher struct {
	channels []Channel
	logger   *zap.Logger
}

// NewDispatcher creates a new notification dispatcher.
func NewDispatcher(logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		channels: make([]Channel, 0),
		logger:   logger,
	}
}

// AddChannel registers a notification channel.
func (d *Dispatcher) AddChannel(ch Channel) {
	d.channels = append(d.channels, ch)
	d.logger.Info("notification channel registered", zap.String("type", string(ch.Type())))
}

// Dispatch sends an alert to all registered channels (best-effort, logs errors).
func (d *Dispatcher) Dispatch(ctx context.Context, payload *AlertPayload) {
	if payload.Timestamp == "" {
		payload.Timestamp = time.Now().Format(time.RFC3339)
	}
	if payload.Severity == "" {
		payload.Severity = "warning"
	}

	for _, ch := range d.channels {
		if err := ch.Send(ctx, payload); err != nil {
			d.logger.Error("notification delivery failed",
				zap.String("channel", string(ch.Type())),
				zap.Error(err),
			)
		} else {
			d.logger.Info("notification delivered",
				zap.String("channel", string(ch.Type())),
				zap.String("alert_type", payload.AlertType),
			)
		}
	}
}
