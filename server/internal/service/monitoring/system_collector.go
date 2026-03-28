// Package monitoring provides system-level monitoring and metrics collection.
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BuildInfo holds version/build metadata injected at compile time.
type BuildInfo struct {
	Version   string
	GitCommit string
	BuildTime string
}

// ServiceInfo describes the running service.
type ServiceInfo struct {
	Version    string `json:"version"`
	GitCommit  string `json:"gitCommit"`
	BuildTime  string `json:"buildTime"`
	Uptime     string `json:"uptime"`
	ConfigMode string `json:"configMode"`
}

// RuntimeInfo contains Go runtime statistics.
type RuntimeInfo struct {
	Goroutines  int     `json:"goroutines"`
	HeapAllocMB float64 `json:"heapAllocMB"`
	HeapSysMB   float64 `json:"heapSysMB"`
	GCPauseMs   float64 `json:"gcPauseMs"`
	NumGC       int     `json:"numGC"`
	CPUCores    int     `json:"cpuCores"`
}

// DependencyStatus describes the health of a single dependency.
type DependencyStatus struct {
	Name      string  `json:"name"`
	Status    string  `json:"status"` // healthy | unhealthy | unknown
	LatencyMs float64 `json:"latencyMs"`
	Version   string  `json:"version,omitempty"`
	Details   string  `json:"details,omitempty"` // JSON blob
}

// SystemStatus is the aggregate system status response.
type SystemStatus struct {
	Service       ServiceInfo        `json:"service"`
	Runtime       RuntimeInfo        `json:"runtime"`
	Dependencies  []DependencyStatus `json:"dependencies"`
	OverallStatus string             `json:"overallStatus"` // healthy | degraded | critical
}

// ── Load Monitoring Types ───────────────────────────────────────────

// ServiceLoad contains real-time service performance metrics.
type ServiceLoad struct {
	RequestsInFlight  int     `json:"requestsInFlight"`
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	AvgLatencyMs      float64 `json:"avgLatencyMs"`
	P95LatencyMs      float64 `json:"p95LatencyMs"`
	ErrorRate         float64 `json:"errorRate"`
}

// DatabaseLoad contains PostgreSQL performance metrics.
type DatabaseLoad struct {
	ActiveConnections     int     `json:"activeConnections"`
	MaxConnections        int     `json:"maxConnections"`
	PoolIdle              int     `json:"poolIdle"`
	PoolInUse             int     `json:"poolInUse"`
	TransactionsPerSecond float64 `json:"transactionsPerSecond"`
	CacheHitRate          float64 `json:"cacheHitRate"`
	Deadlocks             int     `json:"deadlocks"`
}

// RedisLoad contains Redis performance metrics.
type RedisLoad struct {
	ConnectedClients int     `json:"connectedClients"`
	UsedMemoryMB     float64 `json:"usedMemoryMB"`
	MaxMemoryMB      float64 `json:"maxMemoryMB"`
	OpsPerSecond     float64 `json:"opsPerSecond"`
	HitRate          float64 `json:"hitRate"`
	KeyCount         int     `json:"keyCount"`
}

// SystemLoad is the aggregate load response.
type SystemLoad struct {
	Service  ServiceLoad  `json:"service"`
	Database DatabaseLoad `json:"database"`
	Redis    RedisLoad    `json:"redis"`
}

// ── Collector ───────────────────────────────────────────────────────

// Collector gathers system-level metrics.
type Collector struct {
	db          *gorm.DB
	redisClient *redis.Client
	buildInfo   BuildInfo
	configMode  string
	startTime   time.Time
	logger      *zap.Logger
}

// NewCollector creates a new system metrics collector.
func NewCollector(db *gorm.DB, redisClient *redis.Client, buildInfo BuildInfo, configMode string, logger *zap.Logger) *Collector {
	return &Collector{
		db:          db,
		redisClient: redisClient,
		buildInfo:   buildInfo,
		configMode:  configMode,
		startTime:   time.Now(),
		logger:      logger,
	}
}

// CollectStatus gathers a full system status snapshot.
func (c *Collector) CollectStatus(ctx context.Context) *SystemStatus {
	ss := &SystemStatus{
		Service: c.serviceInfo(),
		Runtime: c.runtimeInfo(),
	}
	ss.Dependencies = append(ss.Dependencies, c.checkPostgres(ctx))
	ss.Dependencies = append(ss.Dependencies, c.checkRedis(ctx))
	ss.OverallStatus = c.deriveOverall(ss.Dependencies)
	return ss
}

// CollectLoad gathers system load metrics from service, database, and Redis.
func (c *Collector) CollectLoad(ctx context.Context) *SystemLoad {
	return &SystemLoad{
		Service:  c.collectServiceLoad(),
		Database: c.collectDatabaseLoad(ctx),
		Redis:    c.collectRedisLoad(ctx),
	}
}

// ── Service Load (from Prometheus) ──────────────────────────────────

func (c *Collector) collectServiceLoad() ServiceLoad {
	sl := ServiceLoad{}

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		c.logger.Debug("failed to gather prometheus metrics", zap.Error(err))
		return sl
	}

	var totalRequests, totalErrors float64
	var totalDurationSum, totalDurationCount float64

	for _, mf := range families {
		switch mf.GetName() {
		case "llm_router_http_requests_in_flight":
			parseRequestsInFlight(mf, &sl)
		case "llm_router_http_requests_total":
			parseRequestsTotal(mf, &totalRequests, &totalErrors)
		case "llm_router_http_request_duration_seconds":
			parseRequestDuration(mf, &totalDurationSum, &totalDurationCount, &sl)
		}
	}

	uptimeSeconds := time.Since(c.startTime).Seconds()
	if uptimeSeconds > 0 {
		sl.RequestsPerSecond = totalRequests / uptimeSeconds
	}
	if totalDurationCount > 0 {
		sl.AvgLatencyMs = (totalDurationSum / totalDurationCount) * 1000
	}
	if totalRequests > 0 {
		sl.ErrorRate = totalErrors / totalRequests * 100
	}

	return sl
}

func parseRequestsInFlight(mf *dto.MetricFamily, sl *ServiceLoad) {
	for _, m := range mf.GetMetric() {
		if m.GetGauge() != nil {
			sl.RequestsInFlight = int(m.GetGauge().GetValue())
		}
	}
}

func parseRequestsTotal(mf *dto.MetricFamily, totalRequests, totalErrors *float64) {
	for _, m := range mf.GetMetric() {
		if m.GetCounter() == nil {
			continue
		}
		val := m.GetCounter().GetValue()
		*totalRequests += val
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "status" && len(lp.GetValue()) > 0 && lp.GetValue()[0] == '5' {
				*totalErrors += val
			}
		}
	}
}

func parseRequestDuration(mf *dto.MetricFamily, totalDurationSum, totalDurationCount *float64, sl *ServiceLoad) {
	for _, m := range mf.GetMetric() {
		if m.GetHistogram() == nil {
			continue
		}
		h := m.GetHistogram()
		*totalDurationSum += h.GetSampleSum()
		*totalDurationCount += float64(h.GetSampleCount())
		sl.P95LatencyMs = estimateQuantile(h, 0.95) * 1000
	}
}

// estimateQuantile approximates a quantile from histogram buckets.
func estimateQuantile(h *dto.Histogram, q float64) float64 {
	total := float64(h.GetSampleCount())
	if total == 0 {
		return 0
	}
	target := q * total
	var prev float64
	for _, b := range h.GetBucket() {
		if float64(b.GetCumulativeCount()) >= target {
			bucketCount := float64(b.GetCumulativeCount()) - prev
			if bucketCount > 0 {
				fraction := (target - prev) / bucketCount
				return b.GetUpperBound() * fraction
			}
			return b.GetUpperBound()
		}
		prev = float64(b.GetCumulativeCount())
	}
	if total > 0 {
		return h.GetSampleSum() / total
	}
	return 0
}

// ── Database Load ───────────────────────────────────────────────────

func (c *Collector) collectDatabaseLoad(ctx context.Context) DatabaseLoad {
	dl := DatabaseLoad{}
	if c.db == nil {
		return dl
	}

	sqlDB, err := c.db.DB()
	if err != nil {
		return dl
	}

	stats := sqlDB.Stats()
	dl.ActiveConnections = stats.InUse
	dl.MaxConnections = stats.MaxOpenConnections
	dl.PoolIdle = stats.Idle
	dl.PoolInUse = stats.InUse

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// TPS from pg_stat_database
	var xactCommit, xactRollback float64
	row := sqlDB.QueryRowContext(checkCtx,
		"SELECT COALESCE(SUM(xact_commit), 0), COALESCE(SUM(xact_rollback), 0) FROM pg_stat_database WHERE datname = current_database()")
	if err := row.Scan(&xactCommit, &xactRollback); err == nil {
		uptimeSec := time.Since(c.startTime).Seconds()
		if uptimeSec > 0 {
			dl.TransactionsPerSecond = (xactCommit + xactRollback) / uptimeSec
		}
	}

	// Cache hit rate
	var blksHit, blksRead float64
	row = sqlDB.QueryRowContext(checkCtx,
		"SELECT COALESCE(SUM(blks_hit), 0), COALESCE(SUM(blks_read), 0) FROM pg_stat_database WHERE datname = current_database()")
	if err := row.Scan(&blksHit, &blksRead); err == nil {
		total := blksHit + blksRead
		if total > 0 {
			dl.CacheHitRate = blksHit / total * 100
		}
	}

	// Deadlocks
	var deadlocks int
	row = sqlDB.QueryRowContext(checkCtx,
		"SELECT COALESCE(SUM(deadlocks), 0) FROM pg_stat_database WHERE datname = current_database()")
	if err := row.Scan(&deadlocks); err == nil {
		dl.Deadlocks = deadlocks
	}

	return dl
}

// ── Redis Load ──────────────────────────────────────────────────────

func (c *Collector) collectRedisLoad(ctx context.Context) RedisLoad {
	rl := RedisLoad{}
	if c.redisClient == nil {
		return rl
	}

	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	info, err := c.redisClient.Info(checkCtx, "server", "memory", "clients", "stats", "keyspace").Result()
	if err != nil {
		return rl
	}

	parsed := parseRedisInfo(info)
	rl.ConnectedClients = atoi(parsed["connected_clients"])
	rl.UsedMemoryMB = float64(atoi(parsed["used_memory"])) / (1024 * 1024)
	rl.MaxMemoryMB = float64(atoi(parsed["maxmemory"])) / (1024 * 1024)
	rl.OpsPerSecond = float64(atoi(parsed["instantaneous_ops_per_sec"]))

	hits := float64(atoi(parsed["keyspace_hits"]))
	misses := float64(atoi(parsed["keyspace_misses"]))
	if hits+misses > 0 {
		rl.HitRate = hits / (hits + misses) * 100
	}

	for k, v := range parsed {
		if strings.HasPrefix(k, "db") {
			parts := strings.Split(v, ",")
			for _, p := range parts {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) == 2 && kv[0] == "keys" {
					rl.KeyCount += atoi(kv[1])
				}
			}
		}
	}

	return rl
}

func atoi(s string) int {
	s = strings.TrimSpace(s)
	n, _ := strconv.Atoi(s)
	return n
}

// ── Status helpers ──────────────────────────────────────────────────

func (c *Collector) serviceInfo() ServiceInfo {
	uptime := time.Since(c.startTime)
	var uptimeStr string
	if uptime.Hours() >= 24 {
		days := int(uptime.Hours() / 24)
		hours := int(uptime.Hours()) % 24
		uptimeStr = fmt.Sprintf("%dd %dh", days, hours)
	} else if uptime.Hours() >= 1 {
		uptimeStr = fmt.Sprintf("%dh %dm", int(uptime.Hours()), int(uptime.Minutes())%60)
	} else {
		uptimeStr = fmt.Sprintf("%dm %ds", int(uptime.Minutes()), int(uptime.Seconds())%60)
	}
	return ServiceInfo{
		Version:    c.buildInfo.Version,
		GitCommit:  c.buildInfo.GitCommit,
		BuildTime:  c.buildInfo.BuildTime,
		Uptime:     uptimeStr,
		ConfigMode: c.configMode,
	}
}

func (c *Collector) runtimeInfo() RuntimeInfo {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	var gcPause float64
	if ms.NumGC > 0 {
		gcPause = float64(ms.PauseNs[(ms.NumGC+255)%256]) / 1e6
	}
	return RuntimeInfo{
		Goroutines:  runtime.NumGoroutine(),
		HeapAllocMB: float64(ms.HeapAlloc) / (1024 * 1024),
		HeapSysMB:   float64(ms.HeapSys) / (1024 * 1024),
		GCPauseMs:   gcPause,
		NumGC:       int(ms.NumGC),
		CPUCores:    runtime.NumCPU(),
	}
}

func (c *Collector) checkPostgres(ctx context.Context) DependencyStatus {
	ds := DependencyStatus{Name: "postgres", Status: "unknown"}
	if c.db == nil {
		ds.Status = "unhealthy"
		return ds
	}
	sqlDB, err := c.db.DB()
	if err != nil {
		ds.Status = "unhealthy"
		ds.Details = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		return ds
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	if err := sqlDB.PingContext(checkCtx); err != nil {
		ds.Status = "unhealthy"
		ds.LatencyMs = float64(time.Since(start).Milliseconds())
		ds.Details = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		return ds
	}
	ds.LatencyMs = float64(time.Since(start).Milliseconds())
	ds.Status = "healthy"

	var version string
	if err := c.db.WithContext(checkCtx).Raw("SELECT version()").Scan(&version).Error; err == nil {
		ds.Version = version
	}

	stats := sqlDB.Stats()
	poolDetails, _ := json.Marshal(map[string]int{
		"maxOpen":   stats.MaxOpenConnections,
		"open":      stats.OpenConnections,
		"inUse":     stats.InUse,
		"idle":      stats.Idle,
		"waitCount": int(stats.WaitCount),
	})
	ds.Details = string(poolDetails)
	return ds
}

func (c *Collector) checkRedis(ctx context.Context) DependencyStatus {
	ds := DependencyStatus{Name: "redis", Status: "unknown"}
	if c.redisClient == nil {
		ds.Status = "unhealthy"
		return ds
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	start := time.Now()
	pong, err := c.redisClient.Ping(checkCtx).Result()
	ds.LatencyMs = float64(time.Since(start).Milliseconds())

	if err != nil || pong != "PONG" {
		ds.Status = "unhealthy"
		if err != nil {
			ds.Details = fmt.Sprintf(`{"error": "%s"}`, err.Error())
		}
		return ds
	}
	ds.Status = "healthy"

	info, err := c.redisClient.Info(checkCtx, "server", "memory", "clients", "stats", "keyspace").Result()
	if err == nil {
		parsed := parseRedisInfo(info)
		ds.Version = parsed["redis_version"]
		details, _ := json.Marshal(map[string]string{
			"usedMemory":       parsed["used_memory_human"],
			"maxMemory":        parsed["maxmemory_human"],
			"connectedClients": parsed["connected_clients"],
			"opsPerSec":        parsed["instantaneous_ops_per_sec"],
		})
		ds.Details = string(details)
	}
	return ds
}

func (c *Collector) deriveOverall(deps []DependencyStatus) string {
	unhealthyCount := 0
	for _, d := range deps {
		if d.Status == "unhealthy" {
			unhealthyCount++
		}
	}
	switch {
	case unhealthyCount == len(deps):
		return "critical"
	case unhealthyCount > 0:
		return "degraded"
	default:
		return "healthy"
	}
}

// ── Utility functions ───────────────────────────────────────────────

func parseRedisInfo(info string) map[string]string {
	result := make(map[string]string)
	lines := splitLines(info)
	for _, line := range lines {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		idx := indexOf(line, ':')
		if idx > 0 {
			result[line[:idx]] = line[idx+1:]
		}
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func indexOf(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
