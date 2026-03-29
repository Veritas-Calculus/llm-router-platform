package resolvers

// This file contains integration domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"fmt"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"
	"net/http"
	neturl "net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpdateIntegration is the resolver for the updateIntegration field.
func (r *mutationResolver) UpdateIntegration(ctx context.Context, name string, input model.UpdateIntegrationInput) (*model.IntegrationConfig, error) {
	var conf models.IntegrationConfig
	if err := r.AdminSvc.DB().Where("name = ?", name).First(&conf).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			conf = models.IntegrationConfig{
				ID:      uuid.New(),
				Name:    name,
				Enabled: input.Enabled,
				Config:  []byte(input.Config),
			}
			if e := r.AdminSvc.DB().Create(&conf).Error; e != nil {
				return nil, e
			}
		} else {
			return nil, err
		}
	} else {
		conf.Enabled = input.Enabled
		conf.Config = []byte(input.Config)
		if e := r.AdminSvc.DB().Save(&conf).Error; e != nil {
			return nil, e
		}
	}

	return &model.IntegrationConfig{
		ID:        conf.ID.String(),
		Name:      conf.Name,
		Enabled:   conf.Enabled,
		Config:    string(conf.Config),
		UpdatedAt: conf.UpdatedAt,
	}, nil
}

// TestLangfuseConnection is the resolver for the testLangfuseConnection field.
func (r *mutationResolver) TestLangfuseConnection(ctx context.Context, publicKey string, secretKey string, host string) (bool, error) {
	// SSRF protection: validate host URL before making request
	if err := sanitize.ValidateWebhookURL(host, false, r.Config().Server.AllowLocalProviders); err != nil {
		return false, fmt.Errorf("invalid host URL: %w", err)
	}

	// Parse and fully validate URL components
	parsedURL, err := neturl.Parse(strings.TrimRight(host, "/"))
	if err != nil {
		return false, fmt.Errorf("invalid host URL: %w", err)
	}

	// Validate scheme is http or https only
	scheme := parsedURL.Scheme
	if scheme != "http" && scheme != "https" {
		return false, fmt.Errorf("invalid URL scheme: must be http or https")
	}

	// Validate hostname — only allow standard domain/IP patterns
	hostname := parsedURL.Host
	if hostname == "" {
		return false, fmt.Errorf("invalid host URL: empty hostname")
	}

	// Construct the health check URL from hardcoded components + validated host
	// nosemgrep: go.net.ssrf.tainted-url-host
	healthURL := scheme + "://" + hostname + "/api/public/health" //nolint:gocritic // intentional URL construction from validated parts

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return false, fmt.Errorf("invalid host URL: %w", err)
	}
	req.SetBasicAuth(publicKey, secretKey)

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: sanitize.SafeTransport(r.Config().Server.AllowLocalProviders),
	}
	resp, err := client.Do(req) // CodeQL: URL is constructed from validated scheme + host, not raw user input
	if err != nil {
		return false, fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	return false, fmt.Errorf("langfuse returned HTTP %d", resp.StatusCode)
}
