package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Legacy structures for HTTP JSON API compatibility
// These are used when data comes in via REST endpoints as JSON

// LegacyTelemetryEvent represents the old JSON-based telemetry event structure
// This is ONLY used for HTTP REST API compatibility
type LegacyTelemetryEvent struct {
	Type        string                  `json:"type"`         // "traces", "metrics", "logs"
	ServiceName string                  `json:"service_name"` // for partitioning
	TraceID     string                  `json:"trace_id,omitempty"`
	Data        json.RawMessage         `json:"data"` // Raw JSON data from HTTP API
	Metadata    LegacyTelemetryMetadata `json:"metadata"`
}

// LegacyTelemetryMetadata contains metadata for JSON-based events
type LegacyTelemetryMetadata struct {
	IngestedAt time.Time `json:"ingested_at"`
	SourceIP   string    `json:"source_ip"`
	UserAgent  string    `json:"user_agent,omitempty"`
	Version    string    `json:"version,omitempty"`
}

// LegacyHandler interface for backward compatibility with JSON-based events
type LegacyHandler interface {
	HandleLegacyTelemetryEvent(ctx context.Context, event *LegacyTelemetryEvent) error
}

// Implement TelemetryEvent interface for LegacyTelemetryEvent to work with KinesisBuffer
func (e *LegacyTelemetryEvent) GetType() TelemetryEventType {
	return TelemetryEventType(e.Type)
}

func (e *LegacyTelemetryEvent) GetServiceName() string {
	return e.ServiceName
}

func (e *LegacyTelemetryEvent) GetTraceID() string {
	return e.TraceID
}

func (e *LegacyTelemetryEvent) GetMetadata() TelemetryMetadata {
	// Convert legacy metadata to new format
	return TelemetryMetadata{
		IngestedAt: e.Metadata.IngestedAt,
		SourceIP:   e.Metadata.SourceIP,
	}
}

func (e *LegacyTelemetryEvent) GetSerializedData() ([]byte, error) {
	// For JSON data, return the raw JSON
	return e.Data, nil
}

func (e *LegacyTelemetryEvent) Validate() error {
	if e.Type == "" {
		return fmt.Errorf("telemetry event type cannot be empty")
	}
	if e.ServiceName == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if len(e.Data) == 0 {
		return fmt.Errorf("telemetry data cannot be empty")
	}
	return nil
}
