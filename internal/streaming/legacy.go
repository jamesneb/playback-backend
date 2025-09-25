package streaming

import (
	"context"
	"encoding/json"
	"time"
)

// Legacy structures for HTTP JSON API compatibility
// These are used when data comes in via REST endpoints as JSON

// LegacyTelemetryEvent represents the old JSON-based telemetry event structure
// This is ONLY used for HTTP REST API compatibility
type LegacyTelemetryEvent struct {
	Type        string                   `json:"type"`         // "traces", "metrics", "logs"
	ServiceName string                   `json:"service_name"` // for partitioning
	TraceID     string                   `json:"trace_id,omitempty"`
	Data        json.RawMessage          `json:"data"`         // Raw JSON data from HTTP API
	Metadata    LegacyTelemetryMetadata  `json:"metadata"`
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