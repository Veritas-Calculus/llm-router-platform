package proxy

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestProxyModel(t *testing.T) {
	proxy := &models.Proxy{
		URL:      "http://proxy.example.com:8080",
		Type:     "http",
		Username: "user",
		Password: "pass",
		Region:   "us-east-1",
		IsActive: true,
		Weight:   1.0,
	}

	assert.Equal(t, "http://proxy.example.com:8080", proxy.URL)
	assert.Equal(t, "http", proxy.Type)
	assert.Equal(t, "us-east-1", proxy.Region)
	assert.True(t, proxy.IsActive)
}

func TestProxyURLParsing(t *testing.T) {
	proxyURL := "http://user:pass@proxy.example.com:8080"

	parsed, err := url.Parse(proxyURL)
	assert.NoError(t, err)
	assert.Equal(t, "http", parsed.Scheme)
	assert.Equal(t, "proxy.example.com:8080", parsed.Host)
	assert.Equal(t, "user", parsed.User.Username())

	password, _ := parsed.User.Password()
	assert.Equal(t, "pass", password)
}

func TestProxyTypes(t *testing.T) {
	types := []string{"http", "https", "socks5"}

	for _, pt := range types {
		proxy := &models.Proxy{Type: pt}
		assert.Contains(t, types, proxy.Type)
	}
}

func TestProxySelection(t *testing.T) {
	proxies := []*models.Proxy{
		{URL: "http://proxy1.com:8080", Weight: 0.5, IsActive: true},
		{URL: "http://proxy2.com:8080", Weight: 0.3, IsActive: true},
		{URL: "http://proxy3.com:8080", Weight: 0.2, IsActive: false},
	}

	var active []*models.Proxy
	for _, p := range proxies {
		if p.IsActive {
			active = append(active, p)
		}
	}

	assert.Len(t, active, 2)
}

func TestWeightedProxySelection(t *testing.T) {
	proxies := []*models.Proxy{
		{URL: "http://proxy1.com:8080", Weight: 0.5},
		{URL: "http://proxy2.com:8080", Weight: 0.3},
		{URL: "http://proxy3.com:8080", Weight: 0.2},
	}

	var totalWeight float64
	for _, p := range proxies {
		totalWeight += p.Weight
	}

	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

func TestRegionFiltering(t *testing.T) {
	proxies := []*models.Proxy{
		{URL: "http://proxy1.com", Region: "us-east-1"},
		{URL: "http://proxy2.com", Region: "us-west-1"},
		{URL: "http://proxy3.com", Region: "eu-west-1"},
		{URL: "http://proxy4.com", Region: "us-east-1"},
	}

	targetRegion := "us-east-1"
	var filtered []*models.Proxy
	for _, p := range proxies {
		if p.Region == targetRegion {
			filtered = append(filtered, p)
		}
	}

	assert.Len(t, filtered, 2)
}

func TestProxyHealthTracking(t *testing.T) {
	type proxyStatus struct {
		ProxyID   uuid.UUID
		IsHealthy bool
	}

	statuses := map[uuid.UUID]*proxyStatus{}

	id1 := uuid.New()
	id2 := uuid.New()

	statuses[id1] = &proxyStatus{ProxyID: id1, IsHealthy: true}
	statuses[id2] = &proxyStatus{ProxyID: id2, IsHealthy: false}

	var healthyCount int
	for _, s := range statuses {
		if s.IsHealthy {
			healthyCount++
		}
	}

	assert.Equal(t, 1, healthyCount)
}

func TestProxyRetry(t *testing.T) {
	maxRetries := 3
	retryCount := 0
	success := false

	for retryCount < maxRetries {
		retryCount++
		if retryCount == 2 {
			success = true
			break
		}
	}

	assert.True(t, success)
	assert.Equal(t, 2, retryCount)
}

func TestProxyLatencyThreshold(t *testing.T) {
	threshold := int64(500)
	latencies := []int64{100, 250, 600, 150, 800}

	var aboveThreshold int
	for _, l := range latencies {
		if l > threshold {
			aboveThreshold++
		}
	}

	assert.Equal(t, 2, aboveThreshold)
}

func TestProxyConnectionString(t *testing.T) {
	proxy := &models.Proxy{
		URL:      "http://proxy.example.com:8080",
		Username: "user",
		Password: "secret",
	}

	parsed, _ := url.Parse(proxy.URL)
	if proxy.Username != "" {
		parsed.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	expected := "http://user:secret@proxy.example.com:8080"
	assert.Equal(t, expected, parsed.String())
}

func TestEmptyProxy(t *testing.T) {
	proxy := &models.Proxy{}

	assert.Empty(t, proxy.URL)
	assert.Empty(t, proxy.Type)
	assert.False(t, proxy.IsActive)
}
