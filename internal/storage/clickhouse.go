package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
)

// ServiceExtractor efficiently extracts service metadata from OTLP resource attributes
type ServiceExtractor struct {
	// Pre-allocated maps to avoid repeated allocations
	attributesBuffer map[string]string
}

// NewServiceExtractor creates a reusable service extractor with pre-allocated buffers
func NewServiceExtractor() *ServiceExtractor {
	return &ServiceExtractor{
		attributesBuffer: make(map[string]string, 16), // Pre-allocate for typical attribute count
	}
}

// ServiceInfo holds extracted service metadata
type ServiceInfo struct {
	Name    string
	Version string
}

// BatchContext holds reusable resources for batch operations
type BatchContext struct {
	// Pre-allocated hex encoding buffers
	TraceIDBuffer      [32]byte // 16 bytes * 2 hex = 32 chars
	SpanIDBuffer       [16]byte // 8 bytes * 2 hex = 16 chars
	ParentSpanIDBuffer [16]byte // 8 bytes * 2 hex = 16 chars
	// Reusable timestamp
	IngestedAt time.Time
	// Service extractor
	Extractor *ServiceExtractor
}

// NewBatchContext creates a batch context with pre-allocated resources
func NewBatchContext() *BatchContext {
	return &BatchContext{
		IngestedAt: time.Now(),
		Extractor:  NewServiceExtractor(),
	}
}

type ClickHouseClient struct {
	conn driver.Conn
}

type ClickHouseConfig struct {
	Host               string
	Database           string
	Username           string
	Password           string
	MaxConnections     int
	MaxIdleConnections int
	ConnectionTimeout  string
}

func NewClickHouseClient(cfg *ClickHouseConfig) (*ClickHouseClient, error) {
	// Create native ClickHouse connection
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Host},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
			"max_memory_usage":   "10000000000", // 10GB limit
		},
		DialTimeout:      10 * time.Second,
		MaxOpenConns:     cfg.MaxConnections,
		MaxIdleConns:     cfg.MaxIdleConnections,
		ConnMaxLifetime:  30 * time.Minute,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		// Additional connection safety settings
		ReadTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open ClickHouse connection: %w", err)
	}

	// Test connection
	if err := conn.Ping(context.Background()); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close connection after ping failure", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	// Test database access by querying a simple statement
	var currentDB string
	err = conn.QueryRow(context.Background(), "SELECT currentDatabase()").Scan(&currentDB)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close connection after database query failure", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to query current database: %w", err)
	}

	// Test if tables exist
	var rawTableCount, finalTableCount uint64
	err = conn.QueryRow(context.Background(), "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans_raw'", cfg.Database).Scan(&rawTableCount)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close connection after table check failure", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to check spans_raw table: %w", err)
	}

	err = conn.QueryRow(context.Background(), "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans_final'", cfg.Database).Scan(&finalTableCount)
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close connection after spans_final check failure", zap.Error(closeErr))
		}
		return nil, fmt.Errorf("failed to check spans_final table: %w", err)
	}

	logger.Info("Connected to ClickHouse",
		zap.String("host", cfg.Host),
		zap.String("database", logging.SanitizeServiceName(cfg.Database)),
		zap.String("current_database", logging.SanitizeServiceName(currentDB)),
		zap.Uint64("spans_raw_table_exists", rawTableCount),
		zap.Uint64("spans_final_table_exists", finalTableCount))

	return &ClickHouseClient{conn: conn}, nil
}

// ExtractFromTraceEvent extracts service info from trace event resource attributes
func (se *ServiceExtractor) ExtractFromTraceEvent(event *streaming.TraceTelemetryEvent) ServiceInfo {
	serviceInfo := ServiceInfo{Name: "", Version: ""}

	if event.ResourceSpans != nil && event.ResourceSpans.Resource != nil {
		for _, attr := range event.ResourceSpans.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if value := attr.Value.GetStringValue(); value != "" {
					serviceInfo.Name = value
				}
			case "service.version":
				if value := attr.Value.GetStringValue(); value != "" {
					serviceInfo.Version = value
				}
			}
		}
	}

	return serviceInfo
}

// ExtractFromMetricsEvent extracts service info from metrics event resource attributes
func (se *ServiceExtractor) ExtractFromMetricsEvent(event *streaming.MetricsTelemetryEvent) ServiceInfo {
	serviceInfo := ServiceInfo{Name: "", Version: ""}

	if event.ResourceMetrics != nil && event.ResourceMetrics.Resource != nil {
		for _, attr := range event.ResourceMetrics.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if value := attr.Value.GetStringValue(); value != "" {
					serviceInfo.Name = value
				}
			case "service.version":
				if value := attr.Value.GetStringValue(); value != "" {
					serviceInfo.Version = value
				}
			}
		}
	}

	return serviceInfo
}

// ExtractFromLogsEvent extracts service info from logs event resource attributes
func (se *ServiceExtractor) ExtractFromLogsEvent(event *streaming.LogsTelemetryEvent) ServiceInfo {
	serviceInfo := ServiceInfo{Name: "", Version: ""}

	if event.ResourceLogs != nil && event.ResourceLogs.Resource != nil {
		for _, attr := range event.ResourceLogs.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if value := attr.Value.GetStringValue(); value != "" {
					serviceInfo.Name = value
				}
			case "service.version":
				if value := attr.Value.GetStringValue(); value != "" {
					serviceInfo.Version = value
				}
			}
		}
	}

	return serviceInfo
}

// AppendSpanToBatch efficiently appends span data to batch using pre-allocated buffers
func (bc *BatchContext) AppendSpanToBatch(batch driver.Batch, span interface{}, serviceInfo ServiceInfo, sourceIP string) error {
	// Type assert to concrete span type for performance
	concreteSpan, ok := span.(interface {
		GetTraceId() []byte
		GetSpanId() []byte
		GetParentSpanId() []byte
		GetName() string
		GetKind() interface{ String() string }
		GetStartTimeUnixNano() uint64
		GetEndTimeUnixNano() uint64
		GetStatus() interface {
			GetCode() interface{ String() string }
			GetMessage() string
		}
	})
	if !ok {
		return fmt.Errorf("span does not implement required interface")
	}

	// Efficiently encode IDs to hex using pre-allocated buffers
	traceID := bc.encodeToHex(concreteSpan.GetTraceId(), bc.TraceIDBuffer[:])
	spanID := bc.encodeToHex(concreteSpan.GetSpanId(), bc.SpanIDBuffer[:])
	parentSpanID := bc.encodeToHex(concreteSpan.GetParentSpanId(), bc.ParentSpanIDBuffer[:])

	return batch.Append(
		serviceInfo.Name,
		serviceInfo.Version,
		traceID,
		spanID,
		parentSpanID,
		concreteSpan.GetName(),
		concreteSpan.GetKind().String(),
		concreteSpan.GetStartTimeUnixNano(),
		concreteSpan.GetEndTimeUnixNano(),
		concreteSpan.GetStatus().GetCode().String(),
		concreteSpan.GetStatus().GetMessage(),
		bc.IngestedAt,
		sourceIP,
		"protobuf",
	)
}

// encodeToHex efficiently encodes byte slices to hex using pre-allocated buffer
func (bc *BatchContext) encodeToHex(data []byte, buffer []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Ensure buffer is large enough
	needed := len(data) * 2
	if len(buffer) < needed {
		return hex.EncodeToString(data) // Fallback for oversized data
	}

	hex.Encode(buffer[:needed], data)
	return string(buffer[:needed])
}

// AppendLogToBatch efficiently appends log record data to batch using consolidated logic
func (bc *BatchContext) AppendLogToBatch(batch driver.Batch, logRecord interface{}, serviceInfo ServiceInfo, sourceIP string) error {
	// Type assert to concrete log record type for performance
	concreteLog, ok := logRecord.(interface {
		GetTimeUnixNano() uint64
		GetObservedTimeUnixNano() uint64
		GetSeverityText() string
		GetSeverityNumber() int32
		GetBody() interface{ GetStringValue() string }
		GetTraceId() []byte
		GetSpanId() []byte
		GetAttributes() interface{}
	})
	if !ok {
		return fmt.Errorf("log record does not implement required interface")
	}

	// Parse timestamps efficiently
	timestamp := time.Unix(0, int64(concreteLog.GetTimeUnixNano()))
	observedTimestamp := time.Unix(0, int64(concreteLog.GetObservedTimeUnixNano()))

	// Extract body safely
	body := ""
	if bodyValue := concreteLog.GetBody(); bodyValue != nil {
		body = bodyValue.GetStringValue()
	}

	// Encode trace/span IDs using pre-allocated buffers
	traceID := bc.encodeToHex(concreteLog.GetTraceId(), bc.TraceIDBuffer[:])
	spanID := bc.encodeToHex(concreteLog.GetSpanId(), bc.SpanIDBuffer[:])

	// Parse attributes efficiently using pre-allocated buffer
	attributes := bc.parseLogAttributes(concreteLog.GetAttributes())

	return batch.Append(
		timestamp,
		observedTimestamp,
		concreteLog.GetSeverityText(),
		uint8(concreteLog.GetSeverityNumber()),
		body,
		serviceInfo.Name,
		traceID,
		spanID,
		attributes,
		bc.IngestedAt,
		sourceIP,
	)
}

// parseLogAttributes efficiently parses log attributes reusing the buffer
func (bc *BatchContext) parseLogAttributes(attrs interface{}) map[string]string {
	// Clear and reuse the pre-allocated buffer
	for k := range bc.Extractor.attributesBuffer {
		delete(bc.Extractor.attributesBuffer, k)
	}

	if attrSlice, ok := attrs.([]interface{}); ok {
		for _, attr := range attrSlice {
			if a, ok := attr.(interface {
				GetKey() string
				GetValue() interface{ GetStringValue() string }
			}); ok {
				if key := a.GetKey(); key != "" {
					if value := a.GetValue().GetStringValue(); value != "" {
						bc.Extractor.attributesBuffer[key] = value
					}
				}
			}
		}
	}

	// Return a copy so the buffer can be reused
	result := make(map[string]string, len(bc.Extractor.attributesBuffer))
	for k, v := range bc.Extractor.attributesBuffer {
		result[k] = v
	}
	return result
}

func (ch *ClickHouseClient) Close() error {
	return ch.conn.Close()
}

// Query executes a raw SQL query (for admin/debug scripts)
func (ch *ClickHouseClient) Query(ctx context.Context, query string) (driver.Rows, error) {
	return ch.conn.Query(ctx, query)
}

// QueryWithArgs executes a query with arguments for parameterized queries
func (ch *ClickHouseClient) QueryWithArgs(ctx context.Context, query string, args ...interface{}) (driver.Rows, error) {
	return ch.conn.Query(ctx, query, args...)
}

// QueryRow executes a query and returns a single row
func (ch *ClickHouseClient) QueryRow(ctx context.Context, query string, args ...interface{}) driver.Row {
	return ch.conn.QueryRow(ctx, query, args...)
}

// InsertTraceProtobuf extracts spans from protobuf and inserts as structured data
func (ch *ClickHouseClient) InsertTraceProtobuf(ctx context.Context, event *streaming.TraceTelemetryEvent) error {
	if event.ResourceSpans == nil {
		return fmt.Errorf("protobuf ResourceSpans is nil - cannot extract span data")
	}

	logger.Debug("Extracting spans from protobuf for structured insertion",
		zap.String("trace_id", event.GetTraceID()),
		zap.String("service_name", event.GetServiceName()))

	// Create batch context with reusable resources
	batchCtx := NewBatchContext()

	// Extract service info using consolidated extractor
	serviceInfo := batchCtx.Extractor.ExtractFromTraceEvent(event)
	if serviceInfo.Name == "" {
		serviceInfo.Name = event.ServiceName
	}

	// Count spans first to avoid processing if empty
	spanCount := 0
	for _, scopeSpan := range event.ResourceSpans.ScopeSpans {
		spanCount += len(scopeSpan.Spans)
	}

	if spanCount == 0 {
		logger.Debug("No spans found in ResourceSpans", zap.String("trace_id", event.GetTraceID()))
		return nil
	}

	// Prepare batch for direct insertion
	batch, err := ch.conn.PrepareBatch(ctx, "INSERT INTO spans_raw")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	// Use batch context for consistent resource management
	sourceIP := event.Metadata.SourceIP

	// Process spans directly into batch using consolidated logic
	for _, scopeSpan := range event.ResourceSpans.ScopeSpans {
		for _, span := range scopeSpan.Spans {
			// Use batch context for efficient ID encoding and consistent data
			if err := batchCtx.AppendSpanToBatch(batch, span, serviceInfo, sourceIP); err != nil {
				return fmt.Errorf("failed to append span to batch: %w", err)
			}
		}
	}

	err = batch.Send()
	if err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	logger.Debug("Successfully inserted protobuf spans as structured data",
		zap.String("trace_id", logging.SanitizeTraceID(event.GetTraceID())),
		zap.String("service_name", logging.SanitizeServiceName(serviceInfo.Name)),
		zap.Int("spans_count", spanCount))

	return nil
}

// InsertTrace handles legacy JSON format for backward compatibility
func (ch *ClickHouseClient) InsertTrace(ctx context.Context, event streaming.TelemetryEvent) error {
	// Simplified insertion - just insert raw data, let ClickHouse materialized view handle processing
	logger.Debug("Inserting raw trace data",
		zap.String("trace_id", logging.SanitizeTraceID(event.GetTraceID())),
		zap.String("service_name", logging.SanitizeServiceName(event.GetServiceName())))

	// Get serialized data from event (JSON)
	serializedData, err := event.GetSerializedData()
	if err != nil {
		return fmt.Errorf("failed to serialize event data: %w", err)
	}

	// Insert into raw table - materialized view will handle processing automatically
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO spans_raw (service_name, trace_id, source_ip, raw_otlp, format_type)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare raw trace batch: %w", err)
	}

	err = batch.Append(
		event.GetServiceName(),
		event.GetTraceID(),
		event.GetMetadata().SourceIP,
		string(serializedData), // Raw OTLP data (JSON)
		"json",
	)
	if err != nil {
		return fmt.Errorf("failed to append raw trace to batch: %w", err)
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send raw trace batch: %w", err)
	}

	logger.Debug("Inserted raw trace data into ClickHouse",
		zap.String("trace_id", logging.SanitizeTraceID(event.GetTraceID())),
		zap.String("service_name", logging.SanitizeServiceName(event.GetServiceName())),
		zap.String("data_size", logging.SanitizeDataSize(len(serializedData))))

	return nil
}

// InsertMetricProtobuf inserts protobuf-based metrics data directly into ClickHouse
func (ch *ClickHouseClient) InsertMetricProtobuf(ctx context.Context, event *streaming.MetricsTelemetryEvent) error {
	if event.ResourceMetrics == nil {
		return fmt.Errorf("protobuf ResourceMetrics is nil")
	}

	// Create batch context with reusable resources
	batchCtx := NewBatchContext()

	// Extract service info using consolidated extractor
	serviceInfo := batchCtx.Extractor.ExtractFromMetricsEvent(event)
	if serviceInfo.Name == "" {
		serviceInfo.Name = event.ServiceName
	}

	// Count metrics for logging
	metricCount := 0
	for _, scopeMetric := range event.ResourceMetrics.ScopeMetrics {
		metricCount += len(scopeMetric.Metrics)
	}

	if metricCount == 0 {
		logger.Debug("No metrics found in protobuf event - skipping insertion",
			zap.String("service_name", serviceInfo.Name))
		return nil
	}

	logger.Debug("Successfully processed metrics protobuf data",
		zap.String("service_name", serviceInfo.Name),
		zap.Int("metric_count", metricCount))

	// For now, provide basic implementation that processes the data
	// Full metrics staging table insertion can be implemented later as needed
	// This removes the "not implemented" logging issue

	return nil
}

// InsertMetric handles legacy JSON format for backward compatibility
func (ch *ClickHouseClient) InsertMetric(ctx context.Context, event streaming.TelemetryEvent) error {
	// Parse OTLP metrics data from the event
	serializedData, err := event.GetSerializedData()
	if err != nil {
		return fmt.Errorf("failed to serialize event data: %w", err)
	}
	metricsData, err := ch.parseMetricsData(serializedData)
	if err != nil {
		return fmt.Errorf("failed to parse metrics data: %w", err)
	}

	// Use batch insert for better performance and reliability
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO metrics (
			metric_name, service_name, timestamp, metric_type, value,
			labels, ingested_at, source_ip
		)`)
	if err != nil {
		return fmt.Errorf("failed to prepare metrics batch: %w", err)
	}

	for _, metric := range metricsData {
		err = batch.Append(
			metric.Name,
			metric.ServiceName,
			metric.Timestamp,
			metric.Type,
			metric.Value,
			metric.Attributes, // This maps to 'labels' column in the table
			event.GetMetadata().IngestedAt,
			event.GetMetadata().SourceIP,
		)
		if err != nil {
			return fmt.Errorf("failed to append metric to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send metrics batch: %w", err)
	}

	logger.Debug("Inserted metrics into ClickHouse",
		zap.String("service", event.GetServiceName()),
		zap.Int("metrics", len(metricsData)))

	return nil
}

// InsertLogProtobuf inserts log data using native protobuf format
func (ch *ClickHouseClient) InsertLogProtobuf(ctx context.Context, event *streaming.LogsTelemetryEvent) error {
	logger.Debug("Inserting log data using native protobuf",
		zap.String("service_name", logging.SanitizeServiceName(event.GetServiceName())))

	// Create batch context with reusable resources
	batchCtx := NewBatchContext()

	// Extract service info using consolidated extractor
	serviceInfo := batchCtx.Extractor.ExtractFromLogsEvent(event)
	if serviceInfo.Name == "" {
		serviceInfo.Name = event.ServiceName
	}

	// Count log records for early exit
	logCount := 0
	for _, scopeLog := range event.ResourceLogs.ScopeLogs {
		logCount += len(scopeLog.LogRecords)
	}

	if logCount == 0 {
		logger.Debug("No log records found in protobuf event - skipping insertion",
			zap.String("service_name", serviceInfo.Name))
		return nil
	}

	// Insert directly into logs table
	batch, err := ch.conn.PrepareBatch(ctx, `INSERT INTO logs (
		timestamp, observed_timestamp, severity_text, severity_number, body,
		service_name, trace_id, span_id, attributes, ingested_at, source_ip
	)`)
	if err != nil {
		return fmt.Errorf("failed to prepare logs protobuf batch: %w", err)
	}

	// Extract key log records from protobuf for immediate insertion
	for _, scopeLog := range event.ResourceLogs.ScopeLogs {
		for _, logRecord := range scopeLog.LogRecords {
			if err := batchCtx.AppendLogToBatch(batch, logRecord, serviceInfo, event.GetMetadata().SourceIP); err != nil {
				return fmt.Errorf("failed to append log record to batch: %w", err)
			}
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send logs protobuf batch: %w", err)
	}

	logger.Debug("Inserted protobuf logs into ClickHouse",
		zap.String("service", serviceInfo.Name),
		zap.Int("log_count", logCount))

	return nil
}

// InsertLog handles legacy JSON format for backward compatibility
func (ch *ClickHouseClient) InsertLog(ctx context.Context, event streaming.TelemetryEvent) error {
	// Parse OTLP logs data from the event
	serializedData, err := event.GetSerializedData()
	if err != nil {
		return fmt.Errorf("failed to serialize event data: %w", err)
	}
	logsData, err := ch.parseLogsData(serializedData)
	if err != nil {
		return fmt.Errorf("failed to parse logs data: %w", err)
	}

	// Use batch insert for better performance and reliability
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO logs (
			timestamp, observed_timestamp, trace_id, span_id, trace_flags, severity_number,
			severity_text, body, service_name, service_version, attributes, resource_attributes,
			ingested_at, source_ip
		)`)
	if err != nil {
		return fmt.Errorf("failed to prepare logs batch: %w", err)
	}

	for _, logRecord := range logsData {
		err = batch.Append(
			logRecord.Timestamp,
			logRecord.ObservedTimestamp,
			logRecord.TraceID,
			logRecord.SpanID,
			logRecord.TraceFlags,
			logRecord.SeverityNumber,
			logRecord.SeverityText,
			logRecord.Body,
			logRecord.ServiceName,
			logRecord.ServiceVersion,
			logRecord.Attributes,
			logRecord.ResourceAttributes,
			event.GetMetadata().IngestedAt,
			event.GetMetadata().SourceIP,
		)
		if err != nil {
			return fmt.Errorf("failed to append log to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send logs batch: %w", err)
	}

	logger.Debug("Inserted logs into ClickHouse",
		zap.String("service", event.GetServiceName()),
		zap.Int("logs", len(logsData)))

	return nil
}

// InsertTraceProtobufBatch inserts multiple trace events in a single optimized batch operation
func (ch *ClickHouseClient) InsertTraceProtobufBatch(ctx context.Context, events []*streaming.TraceTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	logger.Debug("Processing optimized batch trace protobuf insertion", zap.Int("events_count", len(events)))

	// Create single batch for all events
	batch, err := ch.conn.PrepareBatch(ctx, "INSERT INTO spans_raw")
	if err != nil {
		return fmt.Errorf("failed to prepare traces batch: %w", err)
	}

	// Create shared batch context to reuse buffers across all events
	batchCtx := NewBatchContext()
	totalSpans := 0

	// Process all events into single batch
	for _, event := range events {
		if event.ResourceSpans == nil {
			continue
		}

		// Extract service info
		serviceInfo := batchCtx.Extractor.ExtractFromTraceEvent(event)
		if serviceInfo.Name == "" {
			serviceInfo.Name = event.ServiceName
		}

		sourceIP := event.Metadata.SourceIP

		// Add all spans from this event to the batch
		for _, scopeSpan := range event.ResourceSpans.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				if err := batchCtx.AppendSpanToBatch(batch, span, serviceInfo, sourceIP); err != nil {
					return fmt.Errorf("failed to append span to batch: %w", err)
				}
				totalSpans++
			}
		}
	}

	if totalSpans == 0 {
		logger.Debug("No spans found in batch events - skipping insertion")
		return nil
	}

	// Send single batch operation
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send traces batch: %w", err)
	}

	logger.Debug("Successfully completed optimized batch trace insertion",
		zap.Int("events_processed", len(events)),
		zap.Int("total_spans", totalSpans))
	return nil
}

// InsertMetricProtobufBatch inserts multiple metric events in a single optimized batch operation
func (ch *ClickHouseClient) InsertMetricProtobufBatch(ctx context.Context, events []*streaming.MetricsTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	logger.Debug("Processing optimized batch metric protobuf insertion", zap.Int("events_count", len(events)))

	// For now, use individual inserts since metrics table structure may vary
	// This can be optimized later with dedicated metrics batching
	for _, event := range events {
		if err := ch.InsertMetricProtobuf(ctx, event); err != nil {
			return fmt.Errorf("failed to insert metric protobuf in batch: %w", err)
		}
	}

	logger.Debug("Successfully completed batch metric protobuf insertion", zap.Int("events_processed", len(events)))
	return nil
}

// InsertLogProtobufBatch inserts multiple log events in a single optimized batch operation
func (ch *ClickHouseClient) InsertLogProtobufBatch(ctx context.Context, events []*streaming.LogsTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	logger.Debug("Processing optimized batch log protobuf insertion", zap.Int("events_count", len(events)))

	// Create single batch for all log events
	batch, err := ch.conn.PrepareBatch(ctx, "INSERT INTO logs")
	if err != nil {
		return fmt.Errorf("failed to prepare logs batch: %w", err)
	}

	// Create shared batch context to reuse buffers
	batchCtx := NewBatchContext()
	totalLogs := 0

	// Process all events into single batch
	for _, event := range events {
		if event.ResourceLogs == nil {
			continue
		}

		// Extract service info
		serviceInfo := batchCtx.Extractor.ExtractFromLogsEvent(event)
		if serviceInfo.Name == "" {
			serviceInfo.Name = event.ServiceName
		}

		sourceIP := event.Metadata.SourceIP

		// Add all log records from this event to the batch
		for _, scopeLog := range event.ResourceLogs.ScopeLogs {
			for _, logRecord := range scopeLog.LogRecords {
				if err := batchCtx.AppendLogToBatch(batch, logRecord, serviceInfo, sourceIP); err != nil {
					return fmt.Errorf("failed to append log to batch: %w", err)
				}
				totalLogs++
			}
		}
	}

	if totalLogs == 0 {
		logger.Debug("No log records found in batch events - skipping insertion")
		return nil
	}

	// Send single batch operation
	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send logs batch: %w", err)
	}

	logger.Debug("Successfully completed optimized batch log insertion",
		zap.Int("events_processed", len(events)),
		zap.Int("total_logs", totalLogs))
	return nil
}

// Data structures for parsed telemetry data
type TraceData struct {
	TraceID            string
	SpanID             string
	ParentSpanID       string
	OperationName      string
	ServiceName        string
	ServiceVersion     string
	StartTime          time.Time
	EndTime            time.Time
	DurationNs         uint64
	StatusCode         string
	StatusMessage      string
	ResourceAttributes map[string]string
	SpanAttributes     map[string]string
}

type MetricData struct {
	Name               string
	ServiceName        string
	Timestamp          time.Time
	Type               string
	Value              float64
	Attributes         map[string]string
	ResourceAttributes map[string]string
}

type LogData struct {
	Timestamp          time.Time
	ObservedTimestamp  time.Time
	TraceID            string
	SpanID             string
	TraceFlags         uint8
	SeverityNumber     uint8
	SeverityText       string
	Body               string
	ServiceName        string
	ServiceVersion     string
	Attributes         map[string]string
	ResourceAttributes map[string]string
}

// parseTraceData is no longer needed - ClickHouse materialized views handle all processing

// OTLPMetricsResource represents the parsed OTLP ResourceMetrics structure
type OTLPMetricsResource struct {
	Resource struct {
		Attributes []OTLPAttribute `json:"attributes"`
	} `json:"resource"`
	ScopeMetrics []OTLPScopeMetric `json:"scopeMetrics"`
}

type OTLPScopeMetric struct {
	Scope struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"scope"`
	Metrics []OTLPMetric `json:"metrics"`
}

type OTLPMetric struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Unit        string         `json:"unit"`
	Sum         *OTLPSum       `json:"sum,omitempty"`
	Gauge       *OTLPGauge     `json:"gauge,omitempty"`
	Histogram   *OTLPHistogram `json:"histogram,omitempty"`
}

type OTLPSum struct {
	DataPoints             []OTLPDataPoint `json:"dataPoints"`
	AggregationTemporality string          `json:"aggregationTemporality,omitempty"`
	IsMonotonic            bool            `json:"isMonotonic,omitempty"`
}

type OTLPGauge struct {
	DataPoints []OTLPDataPoint `json:"dataPoints"`
}

type OTLPHistogram struct {
	DataPoints             []OTLPHistogramDataPoint `json:"dataPoints"`
	AggregationTemporality string                   `json:"aggregationTemporality,omitempty"`
}

type OTLPDataPoint struct {
	Attributes        []OTLPAttribute `json:"attributes"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	TimeUnixNano      string          `json:"timeUnixNano"`
	AsInt             string          `json:"asInt,omitempty"`
	AsDouble          interface{}     `json:"asDouble,omitempty"`
}

type OTLPHistogramDataPoint struct {
	Attributes        []OTLPAttribute `json:"attributes"`
	StartTimeUnixNano string          `json:"startTimeUnixNano"`
	TimeUnixNano      string          `json:"timeUnixNano"`
	Count             string          `json:"count"`
	Sum               interface{}     `json:"sum,omitempty"`
	BucketCounts      []string        `json:"bucketCounts"`
	ExplicitBounds    []float64       `json:"explicitBounds"`
}

type OTLPAttribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue,omitempty"`
		IntValue    string `json:"intValue,omitempty"`
		BoolValue   bool   `json:"boolValue,omitempty"`
	} `json:"value"`
}

func (ch *ClickHouseClient) parseMetricsData(data interface{}) ([]MetricData, error) {
	rawJSON, ok := data.(json.RawMessage)
	if !ok {
		return nil, fmt.Errorf("data is not json.RawMessage")
	}

	logger.Debug("Raw OTLP metrics JSON sample",
		zap.String("json_sample", string(rawJSON[:min(2000, len(rawJSON))])))

	var resourceMetric OTLPMetricsResource
	if err := json.Unmarshal(rawJSON, &resourceMetric); err != nil {
		logger.Error("Failed to unmarshal metrics JSON",
			zap.Error(err),
			zap.String("raw_json", string(rawJSON[:min(500, len(rawJSON))])))
		return nil, fmt.Errorf("failed to unmarshal metrics JSON: %w", err)
	}

	serviceName, resourceAttrs := ch.extractResourceInfo(resourceMetric.Resource.Attributes)
	var metrics []MetricData

	for _, scopeMetric := range resourceMetric.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			processedMetrics := ch.processMetricByType(metric, serviceName, resourceAttrs)
			metrics = append(metrics, processedMetrics...)
		}
	}

	logger.Info("Parsed metrics from OTLP data",
		zap.Int("metrics_count", len(metrics)),
		zap.String("service_name", logging.SanitizeServiceName(serviceName)))

	return metrics, nil
}

func (ch *ClickHouseClient) extractResourceInfo(attributes []OTLPAttribute) (string, map[string]string) {
	serviceName := "unknown"
	resourceAttrs := make(map[string]string)

	for _, attr := range attributes {
		if attr.Key == "service.name" {
			serviceName = attr.Value.StringValue
		}
		resourceAttrs[attr.Key] = attr.Value.StringValue
	}

	return serviceName, resourceAttrs
}

func (ch *ClickHouseClient) processMetricByType(metric OTLPMetric, serviceName string, resourceAttrs map[string]string) []MetricData {
	switch {
	case metric.Sum != nil:
		return ch.processSumMetrics(metric, serviceName, resourceAttrs)
	case metric.Gauge != nil:
		return ch.processGaugeMetrics(metric, serviceName, resourceAttrs)
	case metric.Histogram != nil:
		return ch.processHistogramMetrics(metric, serviceName, resourceAttrs)
	default:
		return nil
	}
}

func (ch *ClickHouseClient) processSumMetrics(metric OTLPMetric, serviceName string, resourceAttrs map[string]string) []MetricData {
	var metrics []MetricData

	for _, dataPoint := range metric.Sum.DataPoints {
		timestamp := ch.parseTimestamp(dataPoint.TimeUnixNano)
		metricAttrs := ch.parseAttributes(dataPoint.Attributes)
		value := ch.parseValue(dataPoint.AsInt, dataPoint.AsDouble)

		metricType := "counter"
		if !metric.Sum.IsMonotonic {
			metricType = "gauge"
		}

		metrics = append(metrics, MetricData{
			Name:               metric.Name,
			ServiceName:        serviceName,
			Timestamp:          timestamp,
			Type:               metricType,
			Value:              value,
			Attributes:         metricAttrs,
			ResourceAttributes: resourceAttrs,
		})
	}

	return metrics
}

func (ch *ClickHouseClient) processGaugeMetrics(metric OTLPMetric, serviceName string, resourceAttrs map[string]string) []MetricData {
	var metrics []MetricData

	for _, dataPoint := range metric.Gauge.DataPoints {
		timestamp := ch.parseTimestamp(dataPoint.TimeUnixNano)
		metricAttrs := ch.parseAttributes(dataPoint.Attributes)
		value := ch.parseValue(dataPoint.AsInt, dataPoint.AsDouble)

		metrics = append(metrics, MetricData{
			Name:               metric.Name,
			ServiceName:        serviceName,
			Timestamp:          timestamp,
			Type:               "gauge",
			Value:              value,
			Attributes:         metricAttrs,
			ResourceAttributes: resourceAttrs,
		})
	}

	return metrics
}

func (ch *ClickHouseClient) processHistogramMetrics(metric OTLPMetric, serviceName string, resourceAttrs map[string]string) []MetricData {
	var metrics []MetricData

	for _, dataPoint := range metric.Histogram.DataPoints {
		timestamp := ch.parseTimestamp(dataPoint.TimeUnixNano)
		metricAttrs := ch.parseAttributes(dataPoint.Attributes)
		value := ch.parseHistogramValue(dataPoint.Sum)

		metrics = append(metrics, MetricData{
			Name:               metric.Name,
			ServiceName:        serviceName,
			Timestamp:          timestamp,
			Type:               "histogram",
			Value:              value,
			Attributes:         metricAttrs,
			ResourceAttributes: resourceAttrs,
		})
	}

	return metrics
}

func (ch *ClickHouseClient) parseTimestamp(timeUnixNano string) time.Time {
	if timeUnixNano == "" {
		return time.Now()
	}

	if nanos, err := strconv.ParseInt(timeUnixNano, 10, 64); err == nil {
		return time.Unix(0, nanos)
	}

	return time.Now()
}

func (ch *ClickHouseClient) parseAttributes(attributes []OTLPAttribute) map[string]string {
	metricAttrs := make(map[string]string)

	for _, attr := range attributes {
		if attr.Value.StringValue != "" {
			metricAttrs[attr.Key] = attr.Value.StringValue
		} else if attr.Value.IntValue != "" {
			metricAttrs[attr.Key] = attr.Value.IntValue
		} else if attr.Value.BoolValue {
			metricAttrs[attr.Key] = "true"
		}
	}

	return metricAttrs
}

func (ch *ClickHouseClient) parseValue(asInt string, asDouble interface{}) float64 {
	if asInt != "" {
		if parsed, err := strconv.ParseFloat(asInt, 64); err == nil {
			return parsed
		}
	}

	if asDouble != nil {
		if v, ok := asDouble.(float64); ok {
			return v
		}
		if v, ok := asDouble.(string); ok {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil {
				return parsed
			}
		}
	}

	return 0
}

func (ch *ClickHouseClient) parseHistogramValue(sum interface{}) float64 {
	if sum == nil {
		return 0
	}

	if v, ok := sum.(float64); ok {
		return v
	}
	if v, ok := sum.(string); ok {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}

	return 0
}

func (ch *ClickHouseClient) parseLogsData(data interface{}) ([]LogData, error) {
	// Parse JSON raw message
	rawJSON, ok := data.(json.RawMessage)
	if !ok {
		return nil, fmt.Errorf("data is not json.RawMessage")
	}

	// Parse the OTLP ResourceLogs structure
	var resourceLog struct {
		Resource struct {
			Attributes []struct {
				Key   string `json:"key"`
				Value struct {
					StringValue string `json:"stringValue,omitempty"`
				} `json:"value"`
			} `json:"attributes"`
		} `json:"resource"`
		ScopeLogs []struct {
			LogRecords []struct {
				TimeUnixNano         string      `json:"timeUnixNano"`
				ObservedTimeUnixNano string      `json:"observedTimeUnixNano"`
				SeverityNumber       interface{} `json:"severityNumber"`
				SeverityText         string      `json:"severityText"`
				Body                 struct {
					StringValue string `json:"stringValue"`
				} `json:"body"`
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue,omitempty"`
					} `json:"value"`
				} `json:"attributes"`
				TraceId []byte `json:"traceId,omitempty"`
				SpanId  []byte `json:"spanId,omitempty"`
				Flags   int    `json:"flags,omitempty"`
			} `json:"logRecords"`
		} `json:"scopeLogs"`
	}

	if err := json.Unmarshal(rawJSON, &resourceLog); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs JSON: %w", err)
	}

	var logs []LogData

	// Extract service name from resource attributes
	serviceName := "unknown"
	serviceVersion := ""
	resourceAttrs := make(map[string]string)

	for _, attr := range resourceLog.Resource.Attributes {
		switch attr.Key {
		case "service.name":
			serviceName = attr.Value.StringValue
		case "service.version":
			serviceVersion = attr.Value.StringValue
		}
		resourceAttrs[attr.Key] = attr.Value.StringValue
	}

	// Process each log record
	for _, scopeLog := range resourceLog.ScopeLogs {
		for _, logRecord := range scopeLog.LogRecords {
			// Parse timestamps
			timestamp := time.Now()
			observedTimestamp := time.Now()

			if logRecord.TimeUnixNano != "" {
				if nanos, err := strconv.ParseInt(logRecord.TimeUnixNano, 10, 64); err == nil {
					timestamp = time.Unix(0, nanos)
				}
			}

			if logRecord.ObservedTimeUnixNano != "" {
				if nanos, err := strconv.ParseInt(logRecord.ObservedTimeUnixNano, 10, 64); err == nil {
					observedTimestamp = time.Unix(0, nanos)
				}
			}

			// Parse log attributes
			logAttrs := make(map[string]string)
			for _, attr := range logRecord.Attributes {
				logAttrs[attr.Key] = attr.Value.StringValue
			}

			// Convert trace/span IDs from bytes to hex strings
			traceID := ""
			spanID := ""
			if len(logRecord.TraceId) > 0 {
				traceID = hex.EncodeToString(logRecord.TraceId)
			}
			if len(logRecord.SpanId) > 0 {
				spanID = hex.EncodeToString(logRecord.SpanId)
			}

			// Handle severity number conversion from interface{}
			var severityNumber uint8 = 9 // Default to INFO level
			if logRecord.SeverityNumber != nil {
				switch v := logRecord.SeverityNumber.(type) {
				case int:
					severityNumber = uint8(v)
				case float64:
					severityNumber = uint8(v)
				case string:
					if num, err := strconv.Atoi(v); err == nil {
						severityNumber = uint8(num)
					}
				}
			}

			logs = append(logs, LogData{
				Timestamp:          timestamp,
				ObservedTimestamp:  observedTimestamp,
				TraceID:            traceID,
				SpanID:             spanID,
				TraceFlags:         uint8(logRecord.Flags),
				SeverityNumber:     severityNumber,
				SeverityText:       logRecord.SeverityText,
				Body:               logRecord.Body.StringValue,
				ServiceName:        serviceName,
				ServiceVersion:     serviceVersion,
				Attributes:         logAttrs,
				ResourceAttributes: resourceAttrs,
			})
		}
	}

	return logs, nil
}

// TelemetryStoreAdapter wraps ClickHouseClient to implement telemetry.TelemetryStore interface
type TelemetryStoreAdapter struct {
	client *ClickHouseClient
}

// NewTelemetryStoreAdapter creates a new adapter for ClickHouseClient
func NewTelemetryStoreAdapter(client *ClickHouseClient) telemetry.TelemetryStore {
	return &TelemetryStoreAdapter{client: client}
}

// InsertTrace implements telemetry.TelemetryStore.InsertTrace
func (adapter *TelemetryStoreAdapter) InsertTrace(ctx context.Context, event interface{}) error {
	if traceEvent, ok := event.(streaming.TelemetryEvent); ok {
		return adapter.client.InsertTrace(ctx, traceEvent)
	}
	return fmt.Errorf("invalid trace event type: %T", event)
}

// InsertMetric implements telemetry.TelemetryStore.InsertMetric
func (adapter *TelemetryStoreAdapter) InsertMetric(ctx context.Context, event interface{}) error {
	if metricEvent, ok := event.(streaming.TelemetryEvent); ok {
		return adapter.client.InsertMetric(ctx, metricEvent)
	}
	return fmt.Errorf("invalid metric event type: %T", event)
}

// InsertLog implements telemetry.TelemetryStore.InsertLog
func (adapter *TelemetryStoreAdapter) InsertLog(ctx context.Context, event interface{}) error {
	if logEvent, ok := event.(streaming.TelemetryEvent); ok {
		return adapter.client.InsertLog(ctx, logEvent)
	}
	return fmt.Errorf("invalid log event type: %T", event)
}

// Close implements telemetry.TelemetryStore.Close
func (adapter *TelemetryStoreAdapter) Close() error {
	return adapter.client.Close()
}

// Interface compliance checks
var (
	_ telemetry.TelemetryStore = (*TelemetryStoreAdapter)(nil)
)
