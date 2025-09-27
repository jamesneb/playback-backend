package interfaces

import (
	"context"
	"encoding/json"

	"github.com/jamesneb/playback-backend/internal/streaming"
)

// StreamPublisher defines the interface for publishing telemetry data to streaming services
type StreamPublisher interface {
	// PublishTrace publishes trace data to the traces stream
	PublishTrace(ctx context.Context, data json.RawMessage, serviceName, traceID, clientIP, userAgent string) error

	// PublishMetric publishes metric data to the metrics stream
	PublishMetric(ctx context.Context, data json.RawMessage, serviceName string, metricCount int, clientIP, userAgent string) error

	// PublishLog publishes log data to the logs stream
	PublishLog(ctx context.Context, data json.RawMessage, serviceName, traceID string, logCount int, clientIP, userAgent string) error

	// Start starts the streaming client
	Start(ctx context.Context) error

	// Close closes the streaming client gracefully
	Close() error
}

// StreamConsumer defines the interface for consuming telemetry data from streaming services
type StreamConsumer interface {
	// Start starts consuming from all configured streams
	Start(ctx context.Context) error

	// Stop stops consuming and closes all connections
	Stop() error

	// RegisterHandler registers a handler for processing telemetry events
	RegisterHandler(eventType string, handler func(streaming.TelemetryEvent) error)
}

// TelemetryEventProcessor defines the interface for processing telemetry events
type TelemetryEventProcessor interface {
	// ProcessEvent processes a single telemetry event
	ProcessEvent(ctx context.Context, event streaming.TelemetryEvent) error

	// ProcessBatch processes a batch of telemetry events
	ProcessBatch(ctx context.Context, events []streaming.TelemetryEvent) error
}

// EventValidator defines the interface for validating telemetry events
type EventValidator interface {
	// ValidateTrace validates trace data structure and content
	ValidateTrace(data json.RawMessage) error

	// ValidateMetric validates metric data structure and content
	ValidateMetric(data json.RawMessage) error

	// ValidateLog validates log data structure and content
	ValidateLog(data json.RawMessage) error

	// ValidateServiceName validates and sanitizes service names
	ValidateServiceName(serviceName string) string

	// ValidateTraceID validates and sanitizes trace IDs
	ValidateTraceID(traceID string) string
}