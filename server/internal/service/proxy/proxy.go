// Package proxy provides proxy pool management.
package proxy

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/crypto"
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
	// Encrypt password if provided
	encryptedPassword := password
	if password != "" {
		if encrypted, err := crypto.Encrypt(password); err == nil {
			encryptedPassword = encrypted
		}
	}

	proxy := &models.Proxy{
		URL:             proxyURL,
		Type:            proxyType,
		Region:          region,
		Username:        username,
		Password:        encryptedPassword,
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
		// Encrypt password before storing
		if encrypted, err := crypto.Encrypt(password); err == nil {
			proxy.Password = encrypted
		} else {
			proxy.Password = password
		}
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
		switch proxy.Type {
		case "socks5":
			proxyURLStr = "socks5://" + proxyURLStr
		case "https":
			proxyURLStr = "https://" + proxyURLStr
		default:
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
		// Decrypt password before using
		password, _ := crypto.Decrypt(proxy.Password)
		proxyURL.User = url.UserPassword(proxy.Username, password)
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
// The flow is: client -> upstream proxy (CONNECT) -> [TLS if HTTPS proxy] -> target proxy -> destination
func (s *Service) buildChainedTransport(ctx context.Context, targetProxyURL *url.URL, upstreamProxy *models.Proxy) (*http.Transport, error) {
	upstreamURLStr := s.normalizeProxyURL(upstreamProxy)
	upstreamURL, err := url.Parse(upstreamURLStr)
	if err != nil {
		return nil, err
	}

	if upstreamProxy.Username != "" && upstreamProxy.Password != "" {
		upstreamURL.User = url.UserPassword(upstreamProxy.Username, upstreamProxy.Password)
	}

	// Check if target proxy requires TLS (HTTPS proxy)
	targetRequiresTLS := targetProxyURL.Scheme == "https"

	s.logger.Debug("building chained transport",
		zap.String("upstream", upstreamURL.Host),
		zap.String("target", targetProxyURL.Host),
		zap.String("target_scheme", targetProxyURL.Scheme),
		zap.Bool("target_requires_tls", targetRequiresTLS))

	// Capture logger for use in closure
	logger := s.logger

	// Create a custom dialer that:
	// 1. Connects to upstream proxy
	// 2. Sends CONNECT to establish tunnel to target proxy
	// 3. If target is HTTPS proxy, perform TLS handshake
	// 4. Through this tunnel, sends another CONNECT to target proxy for the final destination
	transport := &http.Transport{
		DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
			logger.Info("chained proxy dial started",
				zap.String("network", network),
				zap.String("target_addr", addr),
				zap.String("upstream_host", upstreamURL.Host),
				zap.String("target_proxy", targetProxyURL.Host))

			// First connect to the upstream proxy
			dialer := &net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}

			upstreamHost := upstreamURL.Host
			logger.Info("step 1: connecting to upstream proxy", zap.String("upstream", upstreamHost))
			conn, err := dialer.DialContext(dialCtx, "tcp", upstreamHost)
			if err != nil {
				logger.Error("failed to connect to upstream proxy", zap.String("upstream", upstreamHost), zap.Error(err))
				return nil, fmt.Errorf("failed to connect to upstream proxy %s: %w", upstreamHost, err)
			}
			logger.Info("step 1: connected to upstream proxy", zap.String("upstream", upstreamHost))

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

			logger.Info("step 2: sending CONNECT to upstream for target proxy",
				zap.String("target_host", targetHost),
				zap.String("request", strings.TrimSpace(connectReq)))

			if _, err := conn.Write([]byte(connectReq)); err != nil {
				logger.Error("failed to send CONNECT to upstream", zap.Error(err))
				_ = conn.Close()
				return nil, fmt.Errorf("failed to send CONNECT to upstream proxy: %w", err)
			}

			// Read response from upstream proxy
			reader := bufio.NewReader(conn)
			resp, err := http.ReadResponse(reader, nil)
			if err != nil {
				logger.Error("failed to read CONNECT response from upstream", zap.Error(err))
				_ = conn.Close()
				return nil, fmt.Errorf("failed to read CONNECT response from upstream: %w", err)
			}
			_ = resp.Body.Close()

			logger.Info("step 2: upstream CONNECT response", zap.Int("status_code", resp.StatusCode), zap.String("status", resp.Status))

			if resp.StatusCode != http.StatusOK {
				_ = conn.Close()
				return nil, fmt.Errorf("upstream CONNECT to target proxy failed: %s", resp.Status)
			}

			// Now we have a tunnel to the target proxy
			// If target proxy is HTTPS, we need to perform TLS handshake
			var targetConn net.Conn
			targetConn = conn
			if targetRequiresTLS {
				// Extract hostname without port for TLS ServerName
				targetHostname := targetHost
				if h, _, err := net.SplitHostPort(targetHost); err == nil {
					targetHostname = h
				}

				logger.Info("step 3: performing TLS handshake with target proxy", zap.String("server_name", targetHostname))

				tlsConn := tls.Client(conn, &tls.Config{
					ServerName: targetHostname,
					MinVersion: tls.VersionTLS12,
				})
				if err := tlsConn.HandshakeContext(dialCtx); err != nil {
					logger.Error("TLS handshake with target proxy failed", zap.Error(err))
					_ = conn.Close()
					return nil, fmt.Errorf("TLS handshake with target proxy failed: %w", err)
				}
				logger.Info("step 3: TLS handshake successful",
					zap.Uint16("tls_version", tlsConn.ConnectionState().Version),
					zap.String("cipher_suite", tls.CipherSuiteName(tlsConn.ConnectionState().CipherSuite)))
				targetConn = tlsConn
				// Create new reader for TLS connection
				reader = bufio.NewReader(tlsConn)
			} else {
				logger.Info("step 3: skipping TLS (target is HTTP proxy)")
			}

			// Send CONNECT request through the tunnel to target proxy for the final destination (addr)
			connectReq2 := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", addr, addr)

			// Add target proxy auth if needed
			if targetProxyURL.User != nil {
				username := targetProxyURL.User.Username()
				password, _ := targetProxyURL.User.Password()
				auth := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
				connectReq2 += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth)
				logger.Info("step 4: adding auth for target proxy", zap.String("username", username))
			}
			connectReq2 += "\r\n"

			logger.Info("step 4: sending CONNECT to target proxy for final destination",
				zap.String("destination", addr))

			if _, err := targetConn.Write([]byte(connectReq2)); err != nil {
				logger.Error("failed to send CONNECT to target proxy", zap.Error(err))
				_ = targetConn.Close()
				return nil, fmt.Errorf("failed to send CONNECT to target proxy: %w", err)
			}

			// Read response from target proxy
			resp2, err := http.ReadResponse(reader, nil)
			if err != nil {
				logger.Error("failed to read CONNECT response from target proxy", zap.Error(err))
				_ = targetConn.Close()
				return nil, fmt.Errorf("failed to read CONNECT response from target proxy: %w", err)
			}
			_ = resp2.Body.Close()

			logger.Info("step 4: target proxy CONNECT response",
				zap.Int("status_code", resp2.StatusCode),
				zap.String("status", resp2.Status),
				zap.String("destination", addr))

			if resp2.StatusCode != http.StatusOK {
				_ = targetConn.Close()
				return nil, fmt.Errorf("target proxy CONNECT to %s failed: %s", addr, resp2.Status)
			}

			logger.Info("chained proxy tunnel established successfully",
				zap.String("upstream", upstreamHost),
				zap.String("target_proxy", targetHost),
				zap.String("destination", addr))

			// Now we have a tunnel through both proxies to the final destination
			return targetConn, nil
		},
	}

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

	// Return detailed error if not healthy
	if !healthy {
		return false, latency, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, resp.Status)
	}

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
		// Decrypt password before using
		password, _ := crypto.Decrypt(proxy.Password)
		proxyURL.User = url.UserPassword(proxy.Username, password)
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
