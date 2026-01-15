// Package proxy provides proxy pool management.
package proxy

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
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
func (s *Service) Create(ctx context.Context, proxyURL, proxyType, region, username, password string, upstreamProxyID *uuid.UUID) (*models.Proxy, error) {
	proxy := &models.Proxy{
		URL:             proxyURL,
		Type:            proxyType,
		Region:          region,
		Username:        username,
		Password:        password,
		UpstreamProxyID: upstreamProxyID,
		IsActive:        true,
		Weight:          1.0,
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
func (s *Service) Update(ctx context.Context, id uuid.UUID, proxyURL, proxyType, region string, isActive bool, username, password string, upstreamProxyID *uuid.UUID) (*models.Proxy, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	proxy.URL = proxyURL
	proxy.Type = proxyType
	proxy.Region = region
	proxy.IsActive = isActive
	proxy.Username = username
	proxy.UpstreamProxyID = upstreamProxyID
	if password != "" {
		proxy.Password = password
	}

	if err := s.proxyRepo.Update(ctx, proxy); err != nil {
		return nil, err
	}
	return proxy, nil
}

// Delete removes a proxy.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.proxyRepo.Delete(ctx, id)
}

// Toggle enables or disables a proxy.
func (s *Service) Toggle(ctx context.Context, id uuid.UUID) (*models.Proxy, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	proxy.IsActive = !proxy.IsActive
	if err := s.proxyRepo.Update(ctx, proxy); err != nil {
		return nil, err
	}
	return proxy, nil
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

// normalizeProxyURL ensures the proxy URL has a proper scheme.
func (s *Service) normalizeProxyURL(proxy *models.Proxy) string {
	proxyURLStr := proxy.URL
	if !strings.Contains(proxyURLStr, "://") {
		if proxy.Type == "socks5" {
			proxyURLStr = "socks5://" + proxyURLStr
		} else if proxy.Type == "https" {
			proxyURLStr = "https://" + proxyURLStr
		} else {
			proxyURLStr = "http://" + proxyURLStr
		}
	}
	return proxyURLStr
}

// buildProxyTransport creates an http.Transport with proxy chain support.
func (s *Service) buildProxyTransport(ctx context.Context, proxy *models.Proxy) (*http.Transport, error) {
	proxyURLStr := s.normalizeProxyURL(proxy)
	proxyURL, err := url.Parse(proxyURLStr)
	if err != nil {
		return nil, err
	}

	if proxy.Username != "" && proxy.Password != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	// Check if this proxy has an upstream proxy
	if proxy.UpstreamProxyID != nil {
		upstreamProxy, err := s.proxyRepo.GetByID(ctx, *proxy.UpstreamProxyID)
		if err != nil {
			s.logger.Warn("failed to get upstream proxy, using direct connection",
				zap.String("proxy_id", proxy.ID.String()),
				zap.String("upstream_id", proxy.UpstreamProxyID.String()),
				zap.Error(err))
		} else {
			// Build chained transport
			return s.buildChainedTransport(ctx, proxyURL, upstreamProxy)
		}
	}

	// Simple single-proxy transport
	return &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
	}, nil
}

// buildChainedTransport creates a transport that connects through an upstream proxy first.
func (s *Service) buildChainedTransport(ctx context.Context, targetProxyURL *url.URL, upstreamProxy *models.Proxy) (*http.Transport, error) {
	upstreamURLStr := s.normalizeProxyURL(upstreamProxy)
	upstreamURL, err := url.Parse(upstreamURLStr)
	if err != nil {
		return nil, err
	}

	if upstreamProxy.Username != "" && upstreamProxy.Password != "" {
		upstreamURL.User = url.UserPassword(upstreamProxy.Username, upstreamProxy.Password)
	}

	// Create a custom dialer that connects through the upstream proxy
	transport := &http.Transport{
		Proxy: http.ProxyURL(upstreamURL),
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			// First connect to the upstream proxy
			dialer := &net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}

			// Parse upstream host
			upstreamHost := upstreamURL.Host
			conn, err := dialer.DialContext(dialCtx, "tcp", upstreamHost)
			if err != nil {
				return nil, fmt.Errorf("failed to connect to upstream proxy: %w", err)
			}

			// Send CONNECT request to upstream proxy for the target proxy
			targetHost := targetProxyURL.Host
			connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetHost, targetHost)

			// Add upstream proxy auth if needed
			if upstreamURL.User != nil {
				username := upstreamURL.User.Username()
				password, _ := upstreamURL.User.Password()
				auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
			}
			connectReq += "\r\n"

			if _, err := conn.Write([]byte(connectReq)); err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to send CONNECT request: %w", err)
			}

			// Read response
			reader := bufio.NewReader(conn)
			resp, err := http.ReadResponse(reader, nil)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				conn.Close()
				return nil, fmt.Errorf("upstream CONNECT failed: %s", resp.Status)
			}

			// Now we have a tunnel through the upstream proxy to the target proxy
			return conn, nil
		},
	}

	// Override the Proxy function to send requests through the target proxy
	transport.Proxy = http.ProxyURL(targetProxyURL)

	return transport, nil
}

// checkProxyHealth tests proxy connectivity.
func (s *Service) checkProxyHealth(ctx context.Context, proxy *models.Proxy) (bool, time.Duration, error) {
	start := time.Now()

	transport, err := s.buildProxyTransport(ctx, proxy)
	if err != nil {
		return false, 0, err
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	// Use ip.plz.ac to test proxy connectivity - it returns the IP address
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ip.plz.ac", nil)
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

	// Check response is valid (status 200 and non-empty body means proxy works)
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
