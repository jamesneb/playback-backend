package telemetry

import (
	"context"
	"encoding/json"
)

// EventPublisher defines a generic interface for publishing telemetry events
// This abstraction allows swapping different streaming systems (Kinesis, Kafka, Pulsar, etc.)
type EventPublisher interface {
	// PublishTrace publishes trace telemetry data
	PublishTrace(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error

	// PublishMetrics publishes metrics telemetry data
	PublishMetrics(ctx context.Context, data json.RawMessage, serviceName, sourceIP, userAgent string) error

	// PublishLogs publishes logs telemetry data
	PublishLogs(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error

	// Close cleans up resources
	Close() error
}

// TelemetryStore defines a generic interface for persisting telemetry data
// This abstraction allows swapping different storage systems (ClickHouse, PostgreSQL, BigQuery, etc.)
// Uses a generic interface{} to avoid import cycles with specific implementations
type TelemetryStore interface {
	// InsertTrace stores trace telemetry event
	InsertTrace(ctx context.Context, event interface{}) error

	// InsertMetric stores metric telemetry event
	InsertMetric(ctx context.Context, event interface{}) error

	// InsertLog stores log telemetry event
	InsertLog(ctx context.Context, event interface{}) error

	// Close cleans up resources
	Close() error
}

// TelemetryEvent represents a generic telemetry event interface
// This allows different implementations (protobuf-based, JSON-based, etc.)
type TelemetryEvent interface {
	GetType() string
	GetServiceName() string
	GetTraceID() string
	GetMetadata() EventMetadata
	GetSerializedData() ([]byte, error)
	Validate() error
}

// EventMetadata contains common metadata for telemetry events
type EventMetadata struct {
	IngestedAt string `json:"ingested_at"`
	SourceIP   string `json:"source_ip"`
	TenantID   string `json:"tenant_id,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
}
