package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/zap"
)

// LogEntry is the service-layer representation of a parsed log line
type LogEntry struct {
	Timestamp  string
	Level      string
	Message    string
	RequestID  *string
	Caller     *string
	Error      *string
	Method     *string
	Path       *string
	StatusCode *int
	Latency    *float64
	ClientIP   *string
	UserAgent  *string
	RawJSON    *string
}

// LokiResponse represents Loki query_range response JSON structure
type LokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [timestampNs, logLine]
		} `json:"result"`
	} `json:"data"`
}

// GetRequestLogs queries Grafana Loki for logs associated with a specific request ID and/or level.
// requestID and level are both optional filters. startTime/endTime should be RFC3339 strings.
// limit caps the number of log lines returned. Defaults to 500 if not specified.
func (s *Service) GetRequestLogs(ctx context.Context, requestID *string, level *string, startTime *string, endTime *string, limit *int) ([]*LogEntry, error) {
	lokiURL := s.config.Observability.LokiURL
	if lokiURL == "" {
		return nil, fmt.Errorf("loki is not configured or disabled via environment variables")
	}

	// Calculate time range
	now := time.Now()
	var startNs, endNs int64

	if endTime != nil && *endTime != "" {
		t, err := time.Parse(time.RFC3339, *endTime)
		if err != nil {
			return nil, fmt.Errorf("invalid endTime format, expected RFC3339: %w", err)
		}
		endNs = t.UnixNano()
	} else {
		endNs = now.UnixNano()
	}

	if startTime != nil && *startTime != "" {
		t, err := time.Parse(time.RFC3339, *startTime)
		if err != nil {
			return nil, fmt.Errorf("invalid startTime format, expected RFC3339: %w", err)
		}
		startNs = t.UnixNano()
	} else {
		// Default to last 30 minutes
		startNs = now.Add(-30 * time.Minute).UnixNano()
	}

	// Default limit
	queryLimit := 500
	if limit != nil && *limit > 0 {
		queryLimit = *limit
		if queryLimit > 5000 {
			queryLimit = 5000 // Safety cap
		}
	}

	// Build LogQL query with optional filters
	query := `{container="llm-router-server"}`
	if requestID != nil && *requestID != "" {
		query += fmt.Sprintf(` |= "%s"`, *requestID)
	}
	if level != nil && *level != "" {
		query += fmt.Sprintf(` |= "\"level\":\"%s\""`, *level)
	}
	query += " | json"

	reqURL, err := url.Parse(lokiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid loki URL: %w", err)
	}
	reqURL.Path = "/loki/api/v1/query_range"

	q := reqURL.Query()
	q.Set("query", query)
	q.Set("start", fmt.Sprintf("%d", startNs))
	q.Set("end", fmt.Sprintf("%d", endNs))
	q.Set("limit", fmt.Sprintf("%d", queryLimit))
	q.Set("direction", "forward") // Return chronological order
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create loki request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Error("failed to query loki", zap.Error(err), zap.Stringp("request_id", requestID))
		return nil, fmt.Errorf("failed to query log storage")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error("loki returned non-200 status", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("log query failed with status %d", resp.StatusCode)
	}

	var lokiResp LokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lokiResp); err != nil {
		return nil, fmt.Errorf("failed to decode loki response: %w", err)
	}

	var entries []*LogEntry
	for _, res := range lokiResp.Data.Result {
		for _, val := range res.Values {
			if len(val) < 2 {
				continue
			}

			tsStr := val[0]
			line := val[1]

			entry := &LogEntry{
				Level:   "info",
				Message: line,
			}

			// Safely parse JSON log line produced by Zap
			var logLine map[string]interface{}
			if err := json.Unmarshal([]byte(line), &logLine); err == nil {
				// Store raw JSON for detailed view
				rawCopy := line
				entry.RawJSON = &rawCopy

				// Core fields
				if msg, ok := logLine["msg"].(string); ok {
					entry.Message = msg
				}
				if l, ok := logLine["level"].(string); ok {
					entry.Level = l
				}
				if c, ok := logLine["caller"].(string); ok {
					entry.Caller = &c
				}
				if e, ok := logLine["error"].(string); ok {
					entry.Error = &e
				}

				// Request ID
				if rId, ok := logLine["request_id"].(string); ok {
					entry.RequestID = &rId
				} else if rId, ok := logLine["trace_id"].(string); ok {
					entry.RequestID = &rId
				}

				// HTTP request details
				if m, ok := logLine["method"].(string); ok {
					entry.Method = &m
				}
				if p, ok := logLine["path"].(string); ok {
					entry.Path = &p
				}
				if s, ok := logLine["status"].(float64); ok {
					sc := int(s)
					entry.StatusCode = &sc
				}
				if lat, ok := logLine["latency"].(float64); ok {
					entry.Latency = &lat
				}
				if ip, ok := logLine["client_ip"].(string); ok {
					entry.ClientIP = &ip
				}
				if ua, ok := logLine["user_agent"].(string); ok {
					entry.UserAgent = &ua
				}
			}

			// Parse timestamp
			var tsNs int64
			if _, err := fmt.Sscanf(tsStr, "%d", &tsNs); err == nil {
				entry.Timestamp = time.Unix(0, tsNs).Format(time.RFC3339Nano)
			} else {
				entry.Timestamp = tsStr
			}

			entries = append(entries, entry)
		}
	}

	return entries, nil
}
