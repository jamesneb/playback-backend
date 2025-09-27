package handlers

import "time"

// HTTP Request Validation Constants
const (
	// MaxPayloadSize defines the maximum allowed request payload size (10MB)
	MaxPayloadSize = 10 * 1024 * 1024

	// MaxServiceNameLength defines maximum length for service names
	MaxServiceNameLength = 255

	// MaxTraceIDLength defines maximum length for trace IDs (32 hex chars for 128-bit)
	MaxTraceIDLength = 32

	// MinTraceIDLength defines minimum length for trace IDs
	MinTraceIDLength = 16

	// RequestTimeout defines the default timeout for processing requests
	RequestTimeout = 10 * time.Second

	// ContentTypeJSON defines the expected content type for JSON requests
	ContentTypeJSON = "application/json"
)

// OTLP Structure Validation Constants
const (
	// MinOTLPDataSize defines minimum size for valid OTLP data
	MinOTLPDataSize = 10

	// MaxResourceSpans defines maximum number of resource spans to process
	MaxResourceSpans = 1000

	// MaxSpansPerResourceSpan defines maximum spans per resource span
	MaxSpansPerResourceSpan = 10000
)

// Default Values
const (
	// DefaultServiceName used when service name cannot be extracted
	DefaultServiceName = "unknown"

	// DefaultTenantID used when tenant ID cannot be determined
	DefaultTenantID = "default"
)

// Error Messages
const (
	ErrInvalidContentType     = "Content-Type must be application/json"
	ErrPayloadTooLarge       = "Request size exceeds maximum allowed size"
	ErrInvalidOTLPStructure  = "Data does not conform to OTLP trace format"
	ErrInvalidTraceData      = "Invalid OTLP trace data"
	ErrRateLimitExceeded     = "Too many requests for this tenant"
	ErrTelemetryUnavailable  = "Telemetry pipeline unavailable"
	ErrProcessingFailed      = "Failed to process trace data"
	ErrInternalServer        = "Internal server error"
)