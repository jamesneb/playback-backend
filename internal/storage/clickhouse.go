package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
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
		},
		DialTimeout:      10 * time.Second,
		MaxOpenConns:     cfg.MaxConnections,
		MaxIdleConns:     cfg.MaxIdleConnections,
		ConnMaxLifetime:  30 * time.Minute,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open ClickHouse connection: %w", err)
	}

	// Test connection
	if err := conn.Ping(context.Background()); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping ClickHouse: %w", err)
	}

	// Test database access by querying a simple statement
	var currentDB string
	err = conn.QueryRow(context.Background(), "SELECT currentDatabase()").Scan(&currentDB)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to query current database: %w", err)
	}

	// Test if tables exist
	var tableCount uint64
	err = conn.QueryRow(context.Background(), "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans'", cfg.Database).Scan(&tableCount)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to check spans table: %w", err)
	}

	logger.Info("Connected to ClickHouse", 
		zap.String("host", cfg.Host),
		zap.String("database", cfg.Database),
		zap.String("current_database", currentDB),
		zap.Uint64("spans_table_exists", tableCount))

	return &ClickHouseClient{conn: conn}, nil
}

func (ch *ClickHouseClient) Close() error {
	return ch.conn.Close()
}

func (ch *ClickHouseClient) InsertTrace(ctx context.Context, event *streaming.TelemetryEvent) error {
	// Debug: Log entry into InsertTrace
	logger.Info("DEBUG: InsertTrace called", 
		zap.String("trace_id", event.TraceID),
		zap.String("service_name", event.ServiceName),
		zap.String("type", event.Type))
	
	// Parse OTLP trace data from the event
	traceData, err := ch.parseTraceData(event.Data)
	if err != nil {
		logger.Error("DEBUG: parseTraceData failed", zap.Error(err))
		return fmt.Errorf("failed to parse trace data: %w", err)
	}
	
	logger.Info("DEBUG: parseTraceData returned", zap.Int("trace_count", len(traceData)))

	// Use batch insert for better performance and reliability
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO spans (
			trace_id, span_id, parent_span_id, operation_name, service_name, service_version,
			start_time, end_time, duration_ns, status_code, status_message,
			resource_attributes, span_attributes, ingested_at, source_ip
		)`)
	if err != nil {
		return fmt.Errorf("failed to prepare trace batch: %w", err)
	}

	for _, trace := range traceData {
		err = batch.Append(
			trace.TraceID,
			trace.SpanID,
			trace.ParentSpanID,
			trace.OperationName,
			trace.ServiceName,
			trace.ServiceVersion,
			trace.StartTime,
			trace.EndTime,
			trace.DurationNs,
			trace.StatusCode,
			trace.StatusMessage,
			trace.ResourceAttributes,
			trace.SpanAttributes,
			event.Metadata.IngestedAt,
			event.Metadata.SourceIP,
		)
		if err != nil {
			return fmt.Errorf("failed to append trace to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send trace batch: %w", err)
	}

	logger.Info("Inserted spans into ClickHouse", 
		zap.String("trace_id", event.TraceID),
		zap.Int("spans", len(traceData)))

	return nil
}

func (ch *ClickHouseClient) InsertMetric(ctx context.Context, event *streaming.TelemetryEvent) error {
	// Parse OTLP metrics data from the event
	metricsData, err := ch.parseMetricsData(event.Data)
	if err != nil {
		return fmt.Errorf("failed to parse metrics data: %w", err)
	}

	// Use batch insert for better performance and reliability
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO metrics (
			metric_name, service_name, timestamp, metric_type, value,
			attributes, resource_attributes, ingested_at, source_ip
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
			metric.Attributes,
			metric.ResourceAttributes,
			event.Metadata.IngestedAt,
			event.Metadata.SourceIP,
		)
		if err != nil {
			return fmt.Errorf("failed to append metric to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send metrics batch: %w", err)
	}

	logger.Info("Inserted metrics into ClickHouse", 
		zap.String("service", event.ServiceName),
		zap.Int("metrics", len(metricsData)))

	return nil
}

func (ch *ClickHouseClient) InsertLog(ctx context.Context, event *streaming.TelemetryEvent) error {
	// Parse OTLP logs data from the event
	logsData, err := ch.parseLogsData(event.Data)
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
			event.Metadata.IngestedAt,
			event.Metadata.SourceIP,
		)
		if err != nil {
			return fmt.Errorf("failed to append log to batch: %w", err)
		}
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send logs batch: %w", err)
	}

	logger.Info("Inserted logs into ClickHouse", 
		zap.String("service", event.ServiceName),
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

func (ch *ClickHouseClient) parseTraceData(data interface{}) ([]TraceData, error) {
	// For now, return a simplified trace based on the event
	// In a full implementation, this would parse the complete OTLP structure
	
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	
	// Debug: Log the actual JSON structure
	logger.Info("DEBUG: parseTraceData JSON structure", zap.String("json", string(jsonData)))

	var otlp struct {
		ResourceSpans []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeSpans []struct {
				Spans []struct {
					TraceID           string `json:"traceId"`
					SpanID            string `json:"spanId"`
					ParentSpanID      string `json:"parentSpanId"`
					Name              string `json:"name"`
					StartTimeUnixNano int64  `json:"startTimeUnixNano"`
					EndTimeUnixNano   int64  `json:"endTimeUnixNano"`
				} `json:"spans"`
			} `json:"scopeSpans"`
			InstrumentationLibrarySpans []struct {
				Spans []struct {
					TraceID           string `json:"traceId"`
					SpanID            string `json:"spanId"`
					ParentSpanID      string `json:"parentSpanId"`
					Name              string `json:"name"`
					StartTimeUnixNano int64  `json:"startTimeUnixNano"`
					EndTimeUnixNano   int64  `json:"endTimeUnixNano"`
				} `json:"spans"`
			} `json:"instrumentationLibrarySpans"`
		} `json:"resourceSpans"`
	}

	if err := json.Unmarshal(jsonData, &otlp); err != nil {
		return nil, err
	}

	var traces []TraceData

	for _, rs := range otlp.ResourceSpans {
		// Extract service name from resource attributes
		serviceName := "unknown"
		resourceAttrs := make(map[string]string)
		for _, attr := range rs.Resource.Attributes {
			resourceAttrs[attr.Key] = attr.Value.StringValue
			if attr.Key == "service.name" {
				serviceName = attr.Value.StringValue
			}
		}

		// Process both scopeSpans and instrumentationLibrarySpans for compatibility
		allSpans := []struct {
			TraceID           string `json:"traceId"`
			SpanID            string `json:"spanId"`
			ParentSpanID      string `json:"parentSpanId"`
			Name              string `json:"name"`
			StartTimeUnixNano int64  `json:"startTimeUnixNano"`
			EndTimeUnixNano   int64  `json:"endTimeUnixNano"`
		}{}

		for _, ss := range rs.ScopeSpans {
			allSpans = append(allSpans, ss.Spans...)
		}
		for _, ils := range rs.InstrumentationLibrarySpans {
			allSpans = append(allSpans, ils.Spans...)
		}

		for _, span := range allSpans {
			startTime := time.Unix(0, span.StartTimeUnixNano)
			endTime := time.Unix(0, span.EndTimeUnixNano)
			duration := uint64(span.EndTimeUnixNano - span.StartTimeUnixNano)

			trace := TraceData{
				TraceID:            span.TraceID,
				SpanID:             span.SpanID,
				ParentSpanID:       span.ParentSpanID,
				OperationName:      span.Name,
				ServiceName:        serviceName,
				ServiceVersion:     resourceAttrs["service.version"],
				StartTime:          startTime,
				EndTime:            endTime,
				DurationNs:         duration,
				StatusCode:         "OK", // Default status
				StatusMessage:      "",
				ResourceAttributes: resourceAttrs,
				SpanAttributes:     make(map[string]string), // Simplified
			}
			traces = append(traces, trace)
		}
	}

	return traces, nil
}

func (ch *ClickHouseClient) parseMetricsData(data interface{}) ([]MetricData, error) {
	// Simplified metrics parsing - in production this would handle the full OTLP metrics structure
	return []MetricData{
		{
			Name:               "example_metric",
			ServiceName:        "unknown",
			Timestamp:          time.Now(),
			Type:               "gauge",
			Value:              1.0,
			Attributes:         make(map[string]string),
			ResourceAttributes: make(map[string]string),
		},
	}, nil
}

func (ch *ClickHouseClient) parseLogsData(data interface{}) ([]LogData, error) {
	// Simplified logs parsing - in production this would handle the full OTLP logs structure
	return []LogData{
		{
			Timestamp:          time.Now(),
			ObservedTimestamp:  time.Now(),
			TraceID:            "",
			SpanID:             "",
			TraceFlags:         0,
			SeverityNumber:     9, // INFO level
			SeverityText:       "INFO",
			Body:               "Example log message",
			ServiceName:        "unknown",
			ServiceVersion:     "",
			Attributes:         make(map[string]string),
			ResourceAttributes: make(map[string]string),
		},
	}, nil
}