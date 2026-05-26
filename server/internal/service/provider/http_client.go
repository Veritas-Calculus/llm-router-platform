package provider

import (
	"net/http"
	"time"

	"llm-router-platform/pkg/sanitize"
)

const providerHTTPTimeout = 600 * time.Second

func defaultHTTPClient() *http.Client {
	return sanitize.SafeHTTPClient(true, providerHTTPTimeout)
}
