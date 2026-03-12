// Package notification provides multi-channel alert delivery tests.
package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestWebhookChannelSend(t *testing.T) {
	var received bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected application/json, got %s", ct)
		}
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ch := NewWebhookChannel(ts.URL)
	payload := &AlertPayload{
		TargetType: "provider",
		TargetID:   "test-id",
		AlertType:  "health_check_failed",
		Message:    "provider is down",
		Severity:   "critical",
	}

	err := ch.Send(context.Background(), payload)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	if !received {
		t.Error("webhook not received")
	}
}

func TestWebhookChannelSendFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ch := NewWebhookChannel(ts.URL)
	payload := &AlertPayload{
		TargetType: "provider",
		TargetID:   "test-id",
		AlertType:  "test",
		Message:    "test",
	}

	err := ch.Send(context.Background(), payload)
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestDispatcherMultipleChannels(t *testing.T) {
	var count int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := zap.NewNop()
	d := NewDispatcher(logger)
	d.AddChannel(NewWebhookChannel(ts.URL))
	d.AddChannel(NewWebhookChannel(ts.URL))

	payload := &AlertPayload{
		TargetType: "provider",
		TargetID:   "test-id",
		AlertType:  "test",
		Message:    "test",
	}

	d.Dispatch(context.Background(), payload)

	if count != 2 {
		t.Errorf("expected 2 deliveries, got %d", count)
	}
}

func TestDispatcherSetsDefaults(t *testing.T) {
	var receivedSeverity string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		receivedSeverity = "ok"
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	logger := zap.NewNop()
	d := NewDispatcher(logger)
	d.AddChannel(NewWebhookChannel(ts.URL))

	payload := &AlertPayload{
		TargetType: "test",
		TargetID:   "test",
		AlertType:  "test",
		Message:    "test",
		// Severity and Timestamp empty
	}

	d.Dispatch(context.Background(), payload)

	if receivedSeverity == "" {
		t.Error("webhook was not called")
	}
	if payload.Severity != "warning" {
		t.Errorf("expected default severity 'warning', got %q", payload.Severity)
	}
	if payload.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
}

func TestChannelTypes(t *testing.T) {
	wh := NewWebhookChannel("http://example.com")
	if wh.Type() != ChannelWebhook {
		t.Errorf("expected %s, got %s", ChannelWebhook, wh.Type())
	}

	em := NewEmailChannel(EmailConfig{}, nil)
	if em.Type() != ChannelEmail {
		t.Errorf("expected %s, got %s", ChannelEmail, em.Type())
	}

	dt := NewDingTalkChannel("http://example.com", "secret")
	if dt.Type() != ChannelDingTalk {
		t.Errorf("expected %s, got %s", ChannelDingTalk, dt.Type())
	}

	fs := NewFeishuChannel("http://example.com", "secret")
	if fs.Type() != ChannelFeishu {
		t.Errorf("expected %s, got %s", ChannelFeishu, fs.Type())
	}
}

func TestDingTalkSignURL(t *testing.T) {
	dt := NewDingTalkChannel("https://oapi.dingtalk.com/robot/send?access_token=xxx", "test-secret")
	signed := dt.signURL(dt.webhookURL)

	if signed == dt.webhookURL {
		t.Error("signed URL should differ from original")
	}
	if len(signed) <= len(dt.webhookURL) {
		t.Error("signed URL should be longer")
	}
}

func TestFeishuSeverityColor(t *testing.T) {
	fs := NewFeishuChannel("http://example.com", "")

	tests := []struct {
		severity string
		want     string
	}{
		{"critical", "red"},
		{"warning", "orange"},
		{"info", "blue"},
		{"", "blue"},
	}

	for _, tt := range tests {
		got := fs.severityColor(tt.severity)
		if got != tt.want {
			t.Errorf("severity %q: got %q, want %q", tt.severity, got, tt.want)
		}
	}
}
