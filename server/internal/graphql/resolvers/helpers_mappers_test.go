package resolvers

import (
	"testing"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
)

func TestProxyToGQLMapsOperationalFields(t *testing.T) {
	checkedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	proxy := &models.Proxy{
		BaseModel:    models.BaseModel{ID: uuid.New(), CreatedAt: checkedAt},
		URL:          "http://proxy.example:8080",
		Type:         "http",
		Region:       "us",
		IsActive:     true,
		Weight:       2.5,
		Username:     "user",
		Password:     "encrypted",
		SuccessCount: 11,
		FailureCount: 3,
		AvgLatency:   42.5,
		LastChecked:  checkedAt,
	}

	got := proxyToGQL(proxy)

	if got.Weight != proxy.Weight || got.SuccessCount != 11 || got.FailureCount != 3 {
		t.Fatalf("proxy counters not mapped: %+v", got)
	}
	if got.AvgLatency != proxy.AvgLatency {
		t.Fatalf("avg latency = %v, want %v", got.AvgLatency, proxy.AvgLatency)
	}
	if got.LastChecked == nil || !got.LastChecked.Equal(checkedAt) {
		t.Fatalf("last checked = %v, want %v", got.LastChecked, checkedAt)
	}
	if !got.HasAuth {
		t.Fatal("hasAuth = false, want true")
	}
}

func TestMcpServerToGQLMapsStatusAndTools(t *testing.T) {
	serverID := uuid.New()
	toolID := uuid.New()
	checkedAt := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	server := &models.MCPServer{
		BaseModel:     models.BaseModel{ID: serverID, CreatedAt: checkedAt},
		Name:          "files",
		Type:          "stdio",
		Command:       "mcp-files",
		IsActive:      true,
		Status:        "connected",
		LastCheckedAt: checkedAt,
		Tools: []models.MCPTool{
			{
				BaseModel:   models.BaseModel{ID: toolID},
				ServerID:    serverID,
				Name:        "read_file",
				Description: "Read a file",
				IsActive:    true,
			},
		},
	}

	got := mcpServerToGQL(server)

	if got.Status != "connected" {
		t.Fatalf("status = %q, want connected", got.Status)
	}
	if got.LastCheckedAt == nil || !got.LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("last checked = %v, want %v", got.LastCheckedAt, checkedAt)
	}
	if len(got.Tools) != 1 || got.Tools[0].ID != toolID.String() || !got.Tools[0].IsActive {
		t.Fatalf("tools not mapped: %+v", got.Tools)
	}
}
