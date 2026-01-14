// Package proxy provides proxy pool management.
package proxy

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"net/http"
	"net/url"
	"sync"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service handles proxy pool management.
type Service struct {
	proxyRepo  *repository.ProxyRepository
	httpClient *http.Client
	mu         sync.RWMutex
	logger     *zap.Logger
}

// NewService creates a new proxy service.
func NewService(proxyRepo *repository.ProxyRepository, logger *zap.Logger) *Service {
	return &Service{
		proxyRepo: proxyRepo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// Create adds a new proxy.
func (s *Service) Create(ctx context.Context, proxyURL, proxyType, region, username, password string) (*models.Proxy, error) {
	proxy := &models.Proxy{
		URL:      proxyURL,
		Type:     proxyType,
		Region:   region,
		Username: username,
		Password: password,
		IsActive: true,
		Weight:   1.0,
	}

	if err := s.proxyRepo.Create(ctx, proxy); err != nil {
		return nil, err
	}

	return proxy, nil
}

// GetAll returns all proxies.
func (s *Service) GetAll(ctx context.Context) ([]models.Proxy, error) {
	return s.proxyRepo.GetAll(ctx)
}

// GetActive returns all active proxies.
func (s *Service) GetActive(ctx context.Context) ([]models.Proxy, error) {
	return s.proxyRepo.GetActive(ctx)
}

// GetByID retrieves a proxy by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Proxy, error) {
	return s.proxyRepo.GetByID(ctx, id)
}

// Update updates a proxy.
func (s *Service) Update(ctx context.Context, id uuid.UUID, proxyURL, proxyType, region string, isActive bool) error {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	proxy.URL = proxyURL
	proxy.Type = proxyType
	proxy.Region = region
	proxy.IsActive = isActive

	return s.proxyRepo.Update(ctx, proxy)
}

// Delete removes a proxy.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.proxyRepo.Delete(ctx, id)
}

// Toggle enables or disables a proxy.
func (s *Service) Toggle(ctx context.Context, id uuid.UUID) error {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	proxy.IsActive = !proxy.IsActive
	return s.proxyRepo.Update(ctx, proxy)
}

// SelectProxy selects a proxy based on weights.
func (s *Service) SelectProxy(ctx context.Context) (*models.Proxy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	proxies, err := s.proxyRepo.GetActive(ctx)
	if err != nil {
		return nil, err
	}

	if len(proxies) == 0 {
		return nil, nil
	}

	var totalWeight float64
	for _, p := range proxies {
		totalWeight += p.Weight
	}

	if totalWeight == 0 {
		return &proxies[secureRandomInt(len(proxies))], nil
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range proxies {
		cumulative += proxies[i].Weight
		if random <= cumulative {
			return &proxies[i], nil
		}
	}

	return &proxies[len(proxies)-1], nil
}

// CheckHealth verifies a proxy is accessible.
func (s *Service) CheckHealth(ctx context.Context, id uuid.UUID) (bool, time.Duration, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return false, 0, err
	}

	return s.checkProxyHealth(ctx, proxy)
}

// checkProxyHealth tests proxy connectivity.
func (s *Service) checkProxyHealth(ctx context.Context, proxy *models.Proxy) (bool, time.Duration, error) {
	start := time.Now()

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		return false, 0, err
	}

	if proxy.Username != "" && proxy.Password != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/ip", nil)
	if err != nil {
		return false, 0, err
	}

	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		s.updateProxyStats(ctx, proxy.ID, false, latency)
		return false, latency, err
	}
	defer func() { _ = resp.Body.Close() }()

	healthy := resp.StatusCode == http.StatusOK
	s.updateProxyStats(ctx, proxy.ID, healthy, latency)

	return healthy, latency, nil
}

// updateProxyStats updates proxy statistics.
func (s *Service) updateProxyStats(ctx context.Context, id uuid.UUID, success bool, latency time.Duration) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return
	}

	if success {
		proxy.SuccessCount++
	} else {
		proxy.FailureCount++
	}

	total := proxy.SuccessCount + proxy.FailureCount
	if total > 0 {
		proxy.AvgLatency = (proxy.AvgLatency*float64(total-1) + float64(latency.Milliseconds())) / float64(total)
	}

	proxy.LastChecked = time.Now()
	_ = s.proxyRepo.Update(ctx, proxy)
}

// GetHTTPClient returns an HTTP client configured with a proxy.
func (s *Service) GetHTTPClient(ctx context.Context) (*http.Client, error) {
	proxy, err := s.SelectProxy(ctx)
	if err != nil || proxy == nil {
		return s.httpClient, nil
	}

	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		return s.httpClient, nil
	}

	if proxy.Username != "" && proxy.Password != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}

	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}, nil
}

// secureRandomInt returns a cryptographically secure random int in [0, n).
func secureRandomInt(n int) int {
	if n <= 0 {
		return 0
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	// #nosec G115 - n is guaranteed to be positive and within bounds for array indexing
	return int(binary.LittleEndian.Uint32(b[:]) % uint32(n))
}

// secureRandomFloat64 returns a cryptographically secure random float64 in [0, 1).
func secureRandomFloat64() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return float64(binary.LittleEndian.Uint64(b[:])>>11) / (1 << 53)
}
