package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
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
	var rawTableCount, finalTableCount uint64
	err = conn.QueryRow(context.Background(), "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans_raw'", cfg.Database).Scan(&rawTableCount)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to check spans_raw table: %w", err)
	}
	
	err = conn.QueryRow(context.Background(), "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans_final'", cfg.Database).Scan(&finalTableCount)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to check spans_final table: %w", err)
	}

	logger.Info("Connected to ClickHouse", 
		zap.String("host", cfg.Host),
		zap.String("database", cfg.Database),
		zap.String("current_database", currentDB),
		zap.Uint64("spans_raw_table_exists", rawTableCount),
		zap.Uint64("spans_final_table_exists", finalTableCount))

	return &ClickHouseClient{conn: conn}, nil
}

func (ch *ClickHouseClient) Close() error {
	return ch.conn.Close()
}

func (ch *ClickHouseClient) InsertTrace(ctx context.Context, event *streaming.TelemetryEvent) error {
	// Simplified insertion - just insert raw data, let ClickHouse materialized view handle processing
	logger.Debug("Inserting raw trace data", 
		zap.String("trace_id", event.TraceID),
		zap.String("service_name", event.ServiceName))
	
	// Data is already json.RawMessage
	rawJSON := event.Data

	// Insert into raw table - materialized view will handle processing automatically
	batch, err := ch.conn.PrepareBatch(ctx, `
		INSERT INTO spans_raw (service_name, trace_id, source_ip, raw_otlp)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare raw trace batch: %w", err)
	}

	err = batch.Append(
		event.ServiceName,
		event.TraceID,
		event.Metadata.SourceIP,
		string(rawJSON), // Raw OTLP JSON
	)
	if err != nil {
		return fmt.Errorf("failed to append raw trace to batch: %w", err)
	}

	if err := batch.Send(); err != nil {
		return fmt.Errorf("failed to send raw trace batch: %w", err)
	}

	logger.Info("Inserted raw trace data into ClickHouse", 
		zap.String("trace_id", event.TraceID),
		zap.String("service_name", event.ServiceName),
		zap.Int("raw_json_length", len(string(rawJSON))))

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

// parseTraceData is no longer needed - ClickHouse materialized views handle all processing

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
		if attr.Key == "service.name" {
			serviceName = attr.Value.StringValue
		} else if attr.Key == "service.version" {
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