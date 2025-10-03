package monitoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// DatabasePerformanceMonitor provides comprehensive database performance monitoring
type DatabasePerformanceMonitor struct {
	conn   driver.Conn
	logger *zap.Logger

	// Metrics
	queryDuration       *prometheus.HistogramVec
	queryErrorsTotal    *prometheus.CounterVec
	connectionPoolGauge *prometheus.GaugeVec
	slowQueryCounter    *prometheus.CounterVec
	batchOperations     *prometheus.HistogramVec
	tableMetrics        *prometheus.GaugeVec
	queryComplexity     *prometheus.HistogramVec
	memoryUsage         *prometheus.GaugeVec

	// Configuration
	slowQueryThreshold time.Duration
	metricsInterval    time.Duration
	enableQueryLogging bool
	enableSlowQueryLog bool
	maxQueryLogLength  int

	// State
	mu                sync.RWMutex
	connectionStats   ConnectionStats
	queryCache        map[string]*QueryStats
	currentQueries    map[string]*ActiveQuery
	tableSizeCache    map[string]TableSize
	lastMetricsUpdate time.Time

	// Background monitoring
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// ConnectionStats holds connection pool statistics
type ConnectionStats struct {
	OpenConnections   int32
	InUseConnections  int32
	IdleConnections   int32
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxLifetimeClosed int64
}

// QueryStats tracks performance metrics for specific queries
type QueryStats struct {
	QueryHash       string
	QueryTemplate   string
	ExecutionCount  int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	MinDuration     time.Duration
	MaxDuration     time.Duration
	ErrorCount      int64
	LastExecution   time.Time
	RowsAffected    int64
}

// ActiveQuery represents a currently running query
type ActiveQuery struct {
	QueryID   string
	Query     string
	StartTime time.Time
	Duration  time.Duration
	User      string
	Database  string
	ClientIP  string
	ThreadID  uint64
}

// TableSize holds table size information
type TableSize struct {
	Database          string
	Table             string
	Rows              uint64
	UncompressedBytes uint64
	CompressedBytes   uint64
	CompressionRatio  float64
	PartCount         uint32
	LastUpdated       time.Time
}

// Config holds database monitoring configuration
type Config struct {
	SlowQueryThreshold  time.Duration `json:"slow_query_threshold"`
	MetricsInterval     time.Duration `json:"metrics_interval"`
	EnableQueryLogging  bool          `json:"enable_query_logging"`
	EnableSlowQueryLog  bool          `json:"enable_slow_query_log"`
	MaxQueryLogLength   int           `json:"max_query_log_length"`
	EnableTableMetrics  bool          `json:"enable_table_metrics"`
	EnableActiveQueries bool          `json:"enable_active_queries"`
}

// NewDatabasePerformanceMonitor creates a new database performance monitor
func NewDatabasePerformanceMonitor(conn driver.Conn, logger *zap.Logger, cfg *Config) *DatabasePerformanceMonitor {
	return newDatabasePerformanceMonitor(conn, logger, cfg, "")
}

// NewDatabasePerformanceMonitorForTest creates a monitor for testing with unique metric names
func NewDatabasePerformanceMonitorForTest(conn driver.Conn, logger *zap.Logger, cfg *Config, testID string) *DatabasePerformanceMonitor {
	return newDatabasePerformanceMonitor(conn, logger, cfg, testID)
}

func newDatabasePerformanceMonitor(conn driver.Conn, logger *zap.Logger, cfg *Config, testSuffix string) *DatabasePerformanceMonitor {
	if cfg == nil {
		cfg = &Config{
			SlowQueryThreshold:  time.Second,
			MetricsInterval:     30 * time.Second,
			EnableQueryLogging:  true,
			EnableSlowQueryLog:  true,
			MaxQueryLogLength:   1000,
			EnableTableMetrics:  true,
			EnableActiveQueries: true,
		}
	}

	// Add test suffix to metric names if provided
	metricSuffix := ""
	if testSuffix != "" {
		metricSuffix = "_" + testSuffix
	}

	monitor := &DatabasePerformanceMonitor{
		conn:   conn,
		logger: logger,

		// Initialize metrics
		queryDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "clickhouse_query_duration_seconds" + metricSuffix,
			Help:    "ClickHouse query execution duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		}, []string{"operation", "table", "query_type", "success"}),

		queryErrorsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "clickhouse_query_errors_total" + metricSuffix,
			Help: "Total number of ClickHouse query errors",
		}, []string{"operation", "error_type"}),

		connectionPoolGauge: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "clickhouse_connection_pool" + metricSuffix,
			Help: "ClickHouse connection pool statistics",
		}, []string{"state"}),

		slowQueryCounter: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "clickhouse_slow_queries_total" + metricSuffix,
			Help: "Total number of slow ClickHouse queries",
		}, []string{"operation", "threshold"}),

		batchOperations: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "clickhouse_batch_operation_duration_seconds" + metricSuffix,
			Help:    "ClickHouse batch operation duration in seconds",
			Buckets: []float64{.01, .05, .1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"operation", "batch_size_range"}),

		tableMetrics: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "clickhouse_table_metrics" + metricSuffix,
			Help: "ClickHouse table-level metrics",
		}, []string{"database", "table", "metric"}),

		queryComplexity: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "clickhouse_query_complexity" + metricSuffix,
			Help:    "ClickHouse query complexity score",
			Buckets: []float64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000},
		}, []string{"operation"}),

		memoryUsage: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "clickhouse_memory_usage_bytes" + metricSuffix,
			Help: "ClickHouse memory usage in bytes",
		}, []string{"type"}),

		// Configuration
		slowQueryThreshold: cfg.SlowQueryThreshold,
		metricsInterval:    cfg.MetricsInterval,
		enableQueryLogging: cfg.EnableQueryLogging,
		enableSlowQueryLog: cfg.EnableSlowQueryLog,
		maxQueryLogLength:  cfg.MaxQueryLogLength,

		// Initialize state
		queryCache:     make(map[string]*QueryStats),
		currentQueries: make(map[string]*ActiveQuery),
		tableSizeCache: make(map[string]TableSize),
		stopCh:         make(chan struct{}),
	}

	// Start background monitoring
	monitor.startBackgroundMonitoring()

	return monitor
}

// WrapQuery instruments a query execution with performance monitoring
func (m *DatabasePerformanceMonitor) WrapQuery(ctx context.Context, operation, query string, fn func() error) error {
	startTime := time.Now()
	queryID := generateQueryID()
	queryHash := hashQuery(query)

	// Register active query
	m.registerActiveQuery(queryID, query, operation)
	defer m.unregisterActiveQuery(queryID)

	// Execute the query
	err := fn()
	duration := time.Since(startTime)

	// Record metrics
	success := "true"
	if err != nil {
		success = "false"
		m.recordQueryError(operation, err)
	}

	// Extract query metadata
	table := extractTableName(query)
	queryType := classifyQuery(query)

	// Update metrics
	m.queryDuration.WithLabelValues(operation, table, queryType, success).Observe(duration.Seconds())

	// Check for slow queries
	if duration > m.slowQueryThreshold {
		m.recordSlowQuery(operation, query, duration)
	}

	// Update query statistics
	m.updateQueryStats(queryHash, query, duration, err)

	// Log query if enabled
	if m.enableQueryLogging {
		m.logQuery(operation, query, duration, err)
	}

	return err
}

// WrapBatchOperation instruments batch operations with specialized metrics
func (m *DatabasePerformanceMonitor) WrapBatchOperation(ctx context.Context, operation string, batchSize int, fn func() error) error {
	startTime := time.Now()

	err := fn()
	duration := time.Since(startTime)

	batchSizeRange := categorizeBatchSize(batchSize)
	m.batchOperations.WithLabelValues(operation, batchSizeRange).Observe(duration.Seconds())

	if err != nil {
		m.recordQueryError(operation, err)
	}

	m.logger.Debug("Batch operation completed",
		zap.String("operation", operation),
		zap.Int("batch_size", batchSize),
		zap.Duration("duration", duration),
		zap.Error(err))

	return err
}

// GetConnectionStats returns current connection pool statistics
func (m *DatabasePerformanceMonitor) GetConnectionStats() ConnectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connectionStats
}

// GetQueryStats returns query performance statistics
func (m *DatabasePerformanceMonitor) GetQueryStats() map[string]*QueryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid concurrent access issues
	result := make(map[string]*QueryStats, len(m.queryCache))
	for k, v := range m.queryCache {
		stats := *v // Create a copy
		result[k] = &stats
	}
	return result
}

// GetActiveQueries returns currently running queries
func (m *DatabasePerformanceMonitor) GetActiveQueries() []*ActiveQuery {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queries := make([]*ActiveQuery, 0, len(m.currentQueries))
	for _, query := range m.currentQueries {
		// Update duration
		queryCopy := *query
		queryCopy.Duration = time.Since(query.StartTime)
		queries = append(queries, &queryCopy)
	}
	return queries
}

// GetTableSizes returns table size information
func (m *DatabasePerformanceMonitor) GetTableSizes() map[string]TableSize {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]TableSize, len(m.tableSizeCache))
	for k, v := range m.tableSizeCache {
		result[k] = v
	}
	return result
}

// startBackgroundMonitoring starts background metrics collection
func (m *DatabasePerformanceMonitor) startBackgroundMonitoring() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(m.metricsInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collectMetrics()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// collectMetrics collects various database metrics
func (m *DatabasePerformanceMonitor) collectMetrics() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Update connection stats
	m.updateConnectionStats()

	// Update table metrics
	m.updateTableMetrics(ctx)

	// Update memory usage
	m.updateMemoryUsage(ctx)

	// Update process list
	m.updateProcessList(ctx)

	m.lastMetricsUpdate = time.Now()
}

// updateConnectionStats updates connection pool statistics
func (m *DatabasePerformanceMonitor) updateConnectionStats() {
	// Note: ClickHouse Go driver doesn't expose detailed connection stats
	// This would need to be implemented based on the specific driver capabilities
	// For now, we'll set basic metrics

	m.mu.Lock()
	defer m.mu.Unlock()

	// These would be populated from actual connection pool stats
	m.connectionStats.OpenConnections = 10 // Example values
	m.connectionStats.InUseConnections = 2
	m.connectionStats.IdleConnections = 8

	// Update Prometheus metrics
	m.connectionPoolGauge.WithLabelValues("open").Set(float64(m.connectionStats.OpenConnections))
	m.connectionPoolGauge.WithLabelValues("in_use").Set(float64(m.connectionStats.InUseConnections))
	m.connectionPoolGauge.WithLabelValues("idle").Set(float64(m.connectionStats.IdleConnections))
}

// updateTableMetrics updates table-level metrics
func (m *DatabasePerformanceMonitor) updateTableMetrics(ctx context.Context) {
	query := `
		SELECT
			database,
			table,
			sum(rows) as total_rows,
			sum(data_uncompressed_bytes) as uncompressed_bytes,
			sum(data_compressed_bytes) as compressed_bytes,
			count() as part_count
		FROM system.parts
		WHERE database NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA')
			AND active = 1
		GROUP BY database, table
		ORDER BY total_rows DESC
		LIMIT 100
	`

	rows, err := m.conn.Query(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query table metrics", zap.Error(err))
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			m.logger.Error("Failed to close rows", zap.Error(err))
		}
	}()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing cache
	for k := range m.tableSizeCache {
		delete(m.tableSizeCache, k)
	}

	for rows.Next() {
		var database, table string
		var totalRows uint64
		var uncompressedBytes, compressedBytes uint64
		var partCount uint32

		if err := rows.Scan(&database, &table, &totalRows, &uncompressedBytes, &compressedBytes, &partCount); err != nil {
			m.logger.Error("Failed to scan table metrics row", zap.Error(err))
			continue
		}

		compressionRatio := float64(uncompressedBytes) / float64(compressedBytes)
		if compressedBytes == 0 {
			compressionRatio = 1.0
		}

		key := fmt.Sprintf("%s.%s", database, table)
		m.tableSizeCache[key] = TableSize{
			Database:          database,
			Table:             table,
			Rows:              totalRows,
			UncompressedBytes: uncompressedBytes,
			CompressedBytes:   compressedBytes,
			CompressionRatio:  compressionRatio,
			PartCount:         partCount,
			LastUpdated:       time.Now(),
		}

		// Update Prometheus metrics
		m.tableMetrics.WithLabelValues(database, table, "rows").Set(float64(totalRows))
		m.tableMetrics.WithLabelValues(database, table, "uncompressed_bytes").Set(float64(uncompressedBytes))
		m.tableMetrics.WithLabelValues(database, table, "compressed_bytes").Set(float64(compressedBytes))
		m.tableMetrics.WithLabelValues(database, table, "compression_ratio").Set(compressionRatio)
		m.tableMetrics.WithLabelValues(database, table, "part_count").Set(float64(partCount))
	}
}

// updateMemoryUsage updates memory usage metrics
func (m *DatabasePerformanceMonitor) updateMemoryUsage(ctx context.Context) {
	query := `
		SELECT
			metric,
			value
		FROM system.asynchronous_metrics
		WHERE metric LIKE '%Memory%' OR metric LIKE '%Cache%'
	`

	rows, err := m.conn.Query(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query memory metrics", zap.Error(err))
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			m.logger.Error("Failed to close rows", zap.Error(err))
		}
	}()

	for rows.Next() {
		var metric string
		var value float64

		if err := rows.Scan(&metric, &value); err != nil {
			m.logger.Error("Failed to scan memory metrics row", zap.Error(err))
			continue
		}

		m.memoryUsage.WithLabelValues(metric).Set(value)
	}
}

// updateProcessList updates active queries information
func (m *DatabasePerformanceMonitor) updateProcessList(ctx context.Context) {
	query := `
		SELECT
			query_id,
			query,
			user,
			database,
			elapsed,
			read_rows,
			read_bytes,
			memory_usage,
			thread_ids
		FROM system.processes
		WHERE query_id != ''
		ORDER BY elapsed DESC
	`

	rows, err := m.conn.Query(ctx, query)
	if err != nil {
		m.logger.Error("Failed to query process list", zap.Error(err))
		return
	}
	defer func() {
		if err := rows.Close(); err != nil {
			m.logger.Error("Failed to close rows", zap.Error(err))
		}
	}()

	activeQueries := make(map[string]*ActiveQuery)

	for rows.Next() {
		var queryID, query, user, database string
		var elapsed float64
		var readRows, readBytes, memoryUsage uint64
		var threadIDs []uint64

		if err := rows.Scan(&queryID, &query, &user, &database, &elapsed, &readRows, &readBytes, &memoryUsage, &threadIDs); err != nil {
			m.logger.Error("Failed to scan process list row", zap.Error(err))
			continue
		}

		duration := time.Duration(elapsed * float64(time.Second))
		var threadID uint64
		if len(threadIDs) > 0 {
			threadID = threadIDs[0]
		}

		activeQueries[queryID] = &ActiveQuery{
			QueryID:   queryID,
			Query:     query,
			StartTime: time.Now().Add(-duration),
			Duration:  duration,
			User:      user,
			Database:  database,
			ThreadID:  threadID,
		}
	}

	m.mu.Lock()
	// Merge with our tracked queries
	for id, query := range m.currentQueries {
		if _, exists := activeQueries[id]; !exists {
			activeQueries[id] = query
		}
	}
	m.currentQueries = activeQueries
	m.mu.Unlock()
}

// Helper methods

func (m *DatabasePerformanceMonitor) registerActiveQuery(queryID, query, operation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentQueries[queryID] = &ActiveQuery{
		QueryID:   queryID,
		Query:     truncateQuery(query, m.maxQueryLogLength),
		StartTime: time.Now(),
		User:      operation, // Use operation as user for our tracking
		Database:  "telemetry",
	}
}

func (m *DatabasePerformanceMonitor) unregisterActiveQuery(queryID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.currentQueries, queryID)
}

func (m *DatabasePerformanceMonitor) recordQueryError(operation string, err error) {
	errorType := classifyError(err)
	m.queryErrorsTotal.WithLabelValues(operation, errorType).Inc()

	m.logger.Warn("Database query error",
		zap.String("operation", operation),
		zap.String("error_type", errorType),
		zap.Error(err))
}

func (m *DatabasePerformanceMonitor) recordSlowQuery(operation, query string, duration time.Duration) {
	threshold := "default"
	if duration > 10*time.Second {
		threshold = "very_slow"
	} else if duration > 5*time.Second {
		threshold = "slow"
	}

	m.slowQueryCounter.WithLabelValues(operation, threshold).Inc()

	if m.enableSlowQueryLog {
		m.logger.Warn("Slow query detected",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
			zap.String("threshold", threshold),
			zap.String("query", truncateQuery(query, m.maxQueryLogLength)))
	}
}

func (m *DatabasePerformanceMonitor) updateQueryStats(queryHash, query string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats, exists := m.queryCache[queryHash]
	if !exists {
		stats = &QueryStats{
			QueryHash:     queryHash,
			QueryTemplate: truncateQuery(query, m.maxQueryLogLength),
			MinDuration:   duration,
			MaxDuration:   duration,
		}
		m.queryCache[queryHash] = stats
	}

	stats.ExecutionCount++
	stats.TotalDuration += duration
	stats.AverageDuration = time.Duration(int64(stats.TotalDuration) / stats.ExecutionCount)
	stats.LastExecution = time.Now()

	if duration < stats.MinDuration {
		stats.MinDuration = duration
	}
	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}

	if err != nil {
		stats.ErrorCount++
	}
}

func (m *DatabasePerformanceMonitor) logQuery(operation, query string, duration time.Duration, err error) {
	fields := []zap.Field{
		zap.String("operation", operation),
		zap.Duration("duration", duration),
		zap.String("query", truncateQuery(query, m.maxQueryLogLength)),
	}

	if err != nil {
		fields = append(fields, zap.Error(err))
		m.logger.Error("Database query failed", fields...)
	} else {
		m.logger.Debug("Database query executed", fields...)
	}
}

// Stop stops the background monitoring
func (m *DatabasePerformanceMonitor) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// Utility functions

func generateQueryID() string {
	return fmt.Sprintf("query_%d", time.Now().UnixNano())
}

func hashQuery(query string) string {
	// Simple hash based on query structure
	normalized := normalizeQuery(query)
	return fmt.Sprintf("%x", []byte(normalized))
}

func normalizeQuery(query string) string {
	// Remove extra whitespace and normalize for consistent hashing
	// This is a simplified implementation
	return query
}

func extractTableName(query string) string {
	// Extract table name from query - simplified implementation
	// In production, this would be more sophisticated
	return "unknown"
}

func classifyQuery(query string) string {
	// Classify query type - simplified implementation
	if containsIgnoreCase(query, "INSERT") {
		return "insert"
	} else if containsIgnoreCase(query, "SELECT") {
		return "select"
	} else if containsIgnoreCase(query, "UPDATE") {
		return "update"
	} else if containsIgnoreCase(query, "DELETE") {
		return "delete"
	}
	return "other"
}

func classifyError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := err.Error()
	if containsIgnoreCase(errStr, "timeout") {
		return "timeout"
	} else if containsIgnoreCase(errStr, "connection") {
		return "connection"
	} else if containsIgnoreCase(errStr, "syntax") {
		return "syntax"
	}
	return "unknown"
}

func categorizeBatchSize(size int) string {
	switch {
	case size < 10:
		return "small"
	case size < 100:
		return "medium"
	case size < 1000:
		return "large"
	default:
		return "very_large"
	}
}

func truncateQuery(query string, maxLength int) string {
	if len(query) <= maxLength {
		return query
	}
	return query[:maxLength] + "..."
}

func containsIgnoreCase(str, substr string) bool {
	if substr == "" {
		return true
	}
	if str == "" {
		return false
	}
	// Use strings.Contains with strings.ToLower for proper case-insensitive comparison
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}
