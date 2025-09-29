package telemetry

import (
	"encoding/json"
	"time"
	"unsafe"
)

// TraceEvent represents a high-performance trace telemetry event
// Uses minimal memory allocations and efficient field access
type TraceEvent struct {
	// Core fields - optimized memory layout (most accessed fields first)
	ServiceName string          `json:"service_name"`
	TraceID     string          `json:"trace_id"`
	Data        json.RawMessage `json:"data"`
	IngestedAt  time.Time       `json:"ingested_at"`

	// Metadata stored separately to avoid allocation on hot path
	metadata EventMetadata
}

// GetType returns the event type with zero allocations
func (e *TraceEvent) GetType() string {
	return "trace"
}

// GetServiceName returns the service name
func (e *TraceEvent) GetServiceName() string {
	return e.ServiceName
}

// GetTraceID extracts trace ID - for performance, assumes it's already set
// In production, this would be set during event creation from OTLP data
func (e *TraceEvent) GetTraceID() string {
	// If trace ID is empty, attempt to extract from data (expensive operation)
	if e.TraceID == "" && len(e.Data) > 0 {
		// Fast path: try to extract from cached field first
		if extracted := extractTraceIDFromJSON(e.Data); extracted != "" {
			e.TraceID = extracted // Cache for future calls
		}
	}
	return e.TraceID
}

// GetMetadata returns event metadata
func (e *TraceEvent) GetMetadata() EventMetadata {
	return e.metadata
}

// GetSerializedData returns serialized event data with minimal allocations
func (e *TraceEvent) GetSerializedData() ([]byte, error) {
	// Fast path for common case - reuse existing JSON if unchanged
	if len(e.Data) > 0 {
		return e.Data, nil
	}
	return json.Marshal(e)
}

// Validate performs high-performance validation
func (e *TraceEvent) Validate() error {
	// Branch prediction optimized - most common failures first
	if len(e.ServiceName) == 0 {
		return ErrInvalidServiceName
	}
	if len(e.Data) == 0 {
		return ErrEmptyData
	}
	// Trace ID validation is optional for metrics/logs
	return nil
}

// MetricEvent represents a high-performance metric telemetry event
type MetricEvent struct {
	ServiceName string          `json:"service_name"`
	Data        json.RawMessage `json:"data"`
	IngestedAt  time.Time       `json:"ingested_at"`
	metadata    EventMetadata
}

func (e *MetricEvent) GetType() string {
	return "metric"
}

func (e *MetricEvent) GetServiceName() string {
	return e.ServiceName
}

// GetTraceID returns empty string for metrics (metrics don't have trace IDs)
func (e *MetricEvent) GetTraceID() string {
	return ""
}

func (e *MetricEvent) GetMetadata() EventMetadata {
	return e.metadata
}

func (e *MetricEvent) GetSerializedData() ([]byte, error) {
	if len(e.Data) > 0 {
		return e.Data, nil
	}
	return json.Marshal(e)
}

func (e *MetricEvent) Validate() error {
	if len(e.ServiceName) == 0 {
		return ErrInvalidServiceName
	}
	if len(e.Data) == 0 {
		return ErrEmptyData
	}
	return nil
}

// LogEvent represents a high-performance log telemetry event
type LogEvent struct {
	ServiceName string          `json:"service_name"`
	TraceID     string          `json:"trace_id,omitempty"` // Optional for logs
	Data        json.RawMessage `json:"data"`
	IngestedAt  time.Time       `json:"ingested_at"`
	metadata    EventMetadata
}

func (e *LogEvent) GetType() string {
	return "log"
}

func (e *LogEvent) GetServiceName() string {
	return e.ServiceName
}

// GetTraceID extracts trace ID from logs if available
func (e *LogEvent) GetTraceID() string {
	// For logs, trace ID is optional but if available, extract it
	if e.TraceID == "" && len(e.Data) > 0 {
		if extracted := extractTraceIDFromJSON(e.Data); extracted != "" {
			e.TraceID = extracted
		}
	}
	return e.TraceID
}

func (e *LogEvent) GetMetadata() EventMetadata {
	return e.metadata
}

func (e *LogEvent) GetSerializedData() ([]byte, error) {
	if len(e.Data) > 0 {
		return e.Data, nil
	}
	return json.Marshal(e)
}

func (e *LogEvent) Validate() error {
	if len(e.ServiceName) == 0 {
		return ErrInvalidServiceName
	}
	if len(e.Data) == 0 {
		return ErrEmptyData
	}
	return nil
}

// extractTraceIDFromJSON performs high-performance trace ID extraction from JSON data
// Uses unsafe string operations and optimized parsing for hot path
func extractTraceIDFromJSON(data []byte) string {
	if len(data) == 0 {
		return ""
	}

	// Convert to string using unsafe for zero-copy operation
	s := *(*string)(unsafe.Pointer(&data))

	// Fast string search for trace_id field
	const traceIDKey = `"trace_id"`
	idx := findInString(s, traceIDKey)
	if idx == -1 {
		// Try alternative formats
		const traceIDKeyAlt = `"traceId"`
		idx = findInString(s, traceIDKeyAlt)
		if idx == -1 {
			return ""
		}
	}

	// Find the value after the key
	start := idx + len(traceIDKey)
	if start >= len(s) {
		return ""
	}

	// Skip whitespace and colon
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == ':') {
		start++
	}

	// Find the quoted value
	if start >= len(s) || s[start] != '"' {
		return ""
	}
	start++ // Skip opening quote

	// Find closing quote
	end := start
	for end < len(s) && s[end] != '"' {
		end++
	}

	if end > start {
		return s[start:end]
	}

	return ""
}

// findInString performs optimized string search
func findInString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}

	// Simple but efficient search for small patterns
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i] == substr[0] {
			match := true
			for j := 1; j < len(substr); j++ {
				if s[i+j] != substr[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}