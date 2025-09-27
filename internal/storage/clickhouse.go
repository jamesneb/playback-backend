package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

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

func (ch *ClickHouseClient) Close() error {
	return ch.conn.Close()
}

// Query executes a raw SQL query (for admin/debug scripts)
func (ch *ClickHouseClient) Query(ctx context.Context, query string) (driver.Rows, error) {
	return ch.conn.Query(ctx, query)
}

// InsertTraceProtobuf extracts spans from protobuf and inserts as structured data
func (ch *ClickHouseClient) InsertTraceProtobuf(ctx context.Context, event *streaming.TraceTelemetryEvent) error {
	if event.ResourceSpans == nil {
		return fmt.Errorf("protobuf ResourceSpans is nil - cannot extract span data")
	}

	logger.Debug("Extracting spans from protobuf for structured insertion",
		zap.String("trace_id", event.GetTraceID()),
		zap.String("service_name", event.GetServiceName()))

	// Extract spans from the protobuf ResourceSpans
	spans := make([]map[string]interface{}, 0)

	// Get service name from resource attributes
	serviceName := ""
	serviceVersion := ""
	if event.ResourceSpans.Resource != nil {
		for _, attr := range event.ResourceSpans.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if attr.Value.GetStringValue() != "" {
					serviceName = attr.Value.GetStringValue()
				}
			case "service.version":
				if attr.Value.GetStringValue() != "" {
					serviceVersion = attr.Value.GetStringValue()
				}
			}
		}
	}

	// Extract individual spans
	for _, scopeSpan := range event.ResourceSpans.ScopeSpans {
		for _, span := range scopeSpan.Spans {
			spanData := map[string]interface{}{
				"service_name":         serviceName,
				"service_version":      serviceVersion,
				"trace_id":             fmt.Sprintf("%x", span.TraceId),
				"span_id":              fmt.Sprintf("%x", span.SpanId),
				"parent_span_id":       fmt.Sprintf("%x", span.ParentSpanId),
				"name":                 span.Name,
				"kind":                 int32(span.Kind),
				"start_time_unix_nano": span.StartTimeUnixNano,
				"end_time_unix_nano":   span.EndTimeUnixNano,
				"status_code":          int32(span.Status.GetCode()),
				"status_message":       span.Status.GetMessage(),
				"ingested_at":          event.Metadata.IngestedAt,
				"source_ip":            event.Metadata.SourceIP,
				"format_type":          "native",
			}
			spans = append(spans, spanData)
		}
	}

	if len(spans) == 0 {
		logger.Debug("No spans found in ResourceSpans", zap.String("trace_id", event.GetTraceID()))
		return nil
	}

	// Batch insert all spans
	batch, err := ch.conn.PrepareBatch(ctx, "INSERT INTO spans_raw")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, spanData := range spans {
		err := batch.Append(
			spanData["service_name"],
			spanData["service_version"],
			spanData["trace_id"],
			spanData["span_id"],
			spanData["parent_span_id"],
			spanData["name"],
			spanData["kind"],
			spanData["start_time_unix_nano"],
			spanData["end_time_unix_nano"],
			spanData["status_code"],
			spanData["status_message"],
			spanData["ingested_at"],
			spanData["source_ip"],
			spanData["format_type"],
		)
		if err != nil {
			return fmt.Errorf("failed to append span to batch: %w", err)
		}
	}

	err = batch.Send()
	if err != nil {
		return fmt.Errorf("failed to send batch: %w", err)
	}

	logger.Info("Successfully inserted protobuf spans as structured data",
		zap.String("trace_id", logging.SanitizeTraceID(event.GetTraceID())),
		zap.String("service_name", logging.SanitizeServiceName(serviceName)),
		zap.Int("spans_count", len(spans)))

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

	logger.Info("Inserted raw trace data into ClickHouse",
		zap.String("trace_id", logging.SanitizeTraceID(event.GetTraceID())),
		zap.String("service_name", logging.SanitizeServiceName(event.GetServiceName())),
		zap.String("data_size", logging.SanitizeDataSize(len(serializedData))))

	return nil
}

// InsertMetricProtobuf - metrics protobuf insertion not implemented yet
func (ch *ClickHouseClient) InsertMetricProtobuf(ctx context.Context, event *streaming.MetricsTelemetryEvent) error {
	logger.Debug("Metrics protobuf insertion not implemented - skipping",
		zap.String("service_name", event.GetServiceName()))
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

	logger.Info("Inserted metrics into ClickHouse",
		zap.String("service", event.GetServiceName()),
		zap.Int("metrics", len(metricsData)))

	return nil
}

// InsertLogProtobuf inserts log data using native protobuf format
func (ch *ClickHouseClient) InsertLogProtobuf(ctx context.Context, event *streaming.LogsTelemetryEvent) error {
	logger.Debug("Inserting log data using native protobuf",
		zap.String("service_name", logging.SanitizeServiceName(event.GetServiceName())))

	// Serialize the native protobuf data
	protobufData, err := proto.Marshal(event.ResourceLogs)
	if err != nil {
		return fmt.Errorf("failed to marshal logs protobuf: %w", err)
	}

	// Insert directly into logs table
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO logs (
			timestamp, observed_timestamp, severity_text, severity_number, body,
			service_name, trace_id, span_id, attributes, ingested_at, source_ip
		)`)
	if err != nil {
		return fmt.Errorf("failed to prepare logs protobuf batch: %w", err)
	}

	// Extract key log records from protobuf for immediate insertion
	for _, scopeLog := range event.ResourceLogs.ScopeLogs {
		for _, logRecord := range scopeLog.LogRecords {
			timestamp := time.Unix(0, int64(logRecord.TimeUnixNano))
			observedTimestamp := time.Unix(0, int64(logRecord.ObservedTimeUnixNano))

			attributes := make(map[string]string)
			for _, attr := range logRecord.Attributes {
				attributes[attr.Key] = attr.Value.GetStringValue()
			}

			traceID := fmt.Sprintf("%x", logRecord.TraceId)
			spanID := fmt.Sprintf("%x", logRecord.SpanId)
			body := logRecord.Body.GetStringValue()

			err = batch.Append(
				timestamp,
				observedTimestamp,
				logRecord.SeverityText,
				uint8(logRecord.SeverityNumber),
				body,
				event.GetServiceName(),
				traceID,
				spanID,
				attributes,
				event.GetMetadata().IngestedAt,
				event.GetMetadata().SourceIP,
			)
			if err != nil {
				return fmt.Errorf("failed to append log record to batch: %w", err)
			}
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send logs protobuf batch: %w", err)
	}

	logger.Info("Inserted protobuf logs into ClickHouse",
		zap.String("service", event.GetServiceName()),
		zap.Int("protobuf_data_length", len(protobufData)))

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

	logger.Info("Inserted logs into ClickHouse",
		zap.String("service", event.GetServiceName()),
		zap.Int("logs", len(logsData)))

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

	logger.Info("Raw OTLP metrics JSON sample",
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
				traceID = fmt.Sprintf("%x", logRecord.TraceId)
			}
			if len(logRecord.SpanId) > 0 {
				spanID = fmt.Sprintf("%x", logRecord.SpanId)
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

// Batch insert methods for improved performance

// InsertTraceProtobufBatch inserts multiple protobuf trace events in a single batch operation
func (ch *ClickHouseClient) InsertTraceProtobufBatch(ctx context.Context, events []*streaming.TraceTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Prepare batch
	batch, err := ch.conn.PrepareBatch(ctx, "INSERT INTO spans_raw")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	// Process all events into batch
	for _, event := range events {
		// Use the existing individual insert logic but append to batch instead of sending
		if err := ch.appendTraceProtobufToBatch(batch, event); err != nil {
			return fmt.Errorf("failed to append protobuf trace to batch: %w", err)
		}
	}


	// Send batch
	return batch.Send()
}

// Helper methods to append individual events to batches

// appendTraceProtobufToBatch appends a single trace event to an existing batch
func (ch *ClickHouseClient) appendTraceProtobufToBatch(batch driver.Batch, event *streaming.TraceTelemetryEvent) error {
	if event.ResourceSpans == nil {
		return fmt.Errorf("protobuf ResourceSpans is nil - cannot extract span data")
	}

	// Extract spans from the protobuf ResourceSpans (same logic as InsertTraceProtobuf)
	// Get service name from resource attributes
	serviceName := ""
	serviceVersion := ""
	if event.ResourceSpans.Resource != nil {
		for _, attr := range event.ResourceSpans.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if attr.Value.GetStringValue() != "" {
					serviceName = attr.Value.GetStringValue()
				}
			case "service.version":
				if attr.Value.GetStringValue() != "" {
					serviceVersion = attr.Value.GetStringValue()
				}
			}
		}
	}

	// Use service name from event metadata if not found in resource attributes
	if serviceName == "" {
		serviceName = event.ServiceName
	}

	ingestedAt := time.Now()
	sourceIP := event.Metadata.SourceIP

	// Process each span
	for _, scopeSpan := range event.ResourceSpans.ScopeSpans {
		for _, span := range scopeSpan.Spans {
			// Extract span data similar to InsertTraceProtobuf
			traceID := fmt.Sprintf("%x", span.TraceId)
			spanID := fmt.Sprintf("%x", span.SpanId)
			parentSpanID := ""
			if len(span.ParentSpanId) > 0 {
				parentSpanID = fmt.Sprintf("%x", span.ParentSpanId)
			}

			if err := batch.Append(
				serviceName,
				serviceVersion,
				traceID,
				spanID,
				parentSpanID,
				span.Name,
				span.Kind.String(),
				span.StartTimeUnixNano,
				span.EndTimeUnixNano,
				span.Status.GetCode().String(),
				span.Status.GetMessage(),
				ingestedAt,
				sourceIP,
				"protobuf",
			); err != nil {
				return fmt.Errorf("failed to append span to batch: %w", err)
			}
		}
	}

	return nil
}

// appendMetricProtobufToBatch appends a single metric event to an existing batch
func (ch *ClickHouseClient) appendMetricProtobufToBatch(batch driver.Batch, event *streaming.MetricsTelemetryEvent) error {
	if event.ResourceMetrics == nil {
		return fmt.Errorf("protobuf ResourceMetrics is nil")
	}

	// Extract service info similar to InsertMetricProtobuf
	serviceName := ""
	serviceVersion := ""
	if event.ResourceMetrics.Resource != nil {
		for _, attr := range event.ResourceMetrics.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if attr.Value.GetStringValue() != "" {
					serviceName = attr.Value.GetStringValue()
				}
			case "service.version":
				if attr.Value.GetStringValue() != "" {
					serviceVersion = attr.Value.GetStringValue()
				}
			}
		}
	}

	if serviceName == "" {
		serviceName = event.ServiceName
	}

	ingestedAt := time.Now()
	sourceIP := event.Metadata.SourceIP

	// Process each metric (simplified - focusing on batch performance)
	for _, scopeMetric := range event.ResourceMetrics.ScopeMetrics {
		for _, metric := range scopeMetric.Metrics {
			if err := batch.Append(
				serviceName,
				serviceVersion,
				metric.Name,
				"gauge", // Simplified - would need proper type detection in production
				0.0,     // Simplified - would need proper value extraction
				"{}",    // Simplified - would need proper labels
				"{}",    // Simplified - would need proper resource attributes
				uint64(time.Now().UnixNano()),
				ingestedAt,
				sourceIP,
				"protobuf",
			); err != nil {
				return fmt.Errorf("failed to append metric to batch: %w", err)
			}
		}
	}

	return nil
}

// appendLogProtobufToBatch appends a single log event to an existing batch
func (ch *ClickHouseClient) appendLogProtobufToBatch(batch driver.Batch, event *streaming.LogsTelemetryEvent) error {
	if event.ResourceLogs == nil {
		return fmt.Errorf("protobuf ResourceLogs is nil")
	}

	// Extract service info similar to InsertLogProtobuf
	serviceName := ""
	serviceVersion := ""
	if event.ResourceLogs.Resource != nil {
		for _, attr := range event.ResourceLogs.Resource.Attributes {
			switch attr.Key {
			case "service.name":
				if attr.Value.GetStringValue() != "" {
					serviceName = attr.Value.GetStringValue()
				}
			case "service.version":
				if attr.Value.GetStringValue() != "" {
					serviceVersion = attr.Value.GetStringValue()
				}
			}
		}
	}

	if serviceName == "" {
		serviceName = event.ServiceName
	}

	ingestedAt := time.Now()
	sourceIP := event.Metadata.SourceIP

	// Process each log record
	for _, scopeLog := range event.ResourceLogs.ScopeLogs {
		for _, logRecord := range scopeLog.LogRecords {
			traceID := ""
			spanID := ""
			if len(logRecord.TraceId) > 0 {
				traceID = fmt.Sprintf("%x", logRecord.TraceId)
			}
			if len(logRecord.SpanId) > 0 {
				spanID = fmt.Sprintf("%x", logRecord.SpanId)
			}

			if err := batch.Append(
				serviceName,
				serviceVersion,
				traceID,
				spanID,
				logRecord.TimeUnixNano,
				logRecord.SeverityNumber,
				logRecord.SeverityText,
				logRecord.Body.GetStringValue(),
				"{}", // Simplified - would need proper attributes
				"{}", // Simplified - would need proper resource attributes
				ingestedAt,
				sourceIP,
				"protobuf",
			); err != nil {
				return fmt.Errorf("failed to append log to batch: %w", err)
			}
		}
	}

	return nil
}

// InsertMetricProtobufBatch inserts multiple protobuf metric events in a single batch operation
func (ch *ClickHouseClient) InsertMetricProtobufBatch(ctx context.Context, events []*streaming.MetricsTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Prepare batch
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO metrics_raw (
			service_name, service_version, metric_name, metric_type, metric_value,
			labels, resource_attributes, timestamp_unix_nano, ingested_at,
			source_ip, format_type
		)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	// Process all events into batch
	for _, event := range events {
		if err := ch.appendMetricProtobufToBatch(batch, event); err != nil {
			return fmt.Errorf("failed to append protobuf metric to batch: %w", err)
		}
	}


	// Send batch
	return batch.Send()
}

// InsertLogProtobufBatch inserts multiple protobuf log events in a single batch operation
func (ch *ClickHouseClient) InsertLogProtobufBatch(ctx context.Context, events []*streaming.LogsTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Prepare batch
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO logs_raw (
			service_name, service_version, trace_id, span_id, timestamp_unix_nano,
			severity_number, severity_text, body, attributes,
			resource_attributes, ingested_at, source_ip, format_type
		)`)
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	// Process all events into batch
	for _, event := range events {
		if err := ch.appendLogProtobufToBatch(batch, event); err != nil {
			return fmt.Errorf("failed to append protobuf log to batch: %w", err)
		}
	}


	// Send batch
	return batch.Send()
}
