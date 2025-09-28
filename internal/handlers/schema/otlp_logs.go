package schema

// OTLP Logs Protocol structures
// These structures are defined according to the OpenTelemetry Protocol specification
// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/logs/v1/logs.proto

// LogsRequest represents the top-level OTLP logs request
type LogsRequest struct {
	ResourceLogs []ResourceLog `json:"resourceLogs"`
}

// ResourceLog represents logs associated with a resource
type ResourceLog struct {
	Resource  Resource   `json:"resource"`
	ScopeLogs []ScopeLog `json:"scopeLogs"`
	SchemaURL string     `json:"schemaUrl,omitempty"`
}

// ScopeLog represents logs from a single instrumentation scope
type ScopeLog struct {
	Scope      Scope       `json:"scope"`
	LogRecords []LogRecord `json:"logRecords"`
}

// LogRecord represents a single log record
type LogRecord struct {
	TimeUnixNano           uint64        `json:"timeUnixNano"`
	ObservedTimeUnixNano   uint64        `json:"observedTimeUnixNano,omitempty"`
	SeverityNumber         int32         `json:"severityNumber,omitempty"`
	SeverityText           string        `json:"severityText,omitempty"`
	Body                   LogRecordBody `json:"body,omitempty"`
	Attributes             []Attribute   `json:"attributes,omitempty"`
	DroppedAttributesCount uint32        `json:"droppedAttributesCount,omitempty"`
	Flags                  uint32        `json:"flags,omitempty"`
	TraceID                string        `json:"traceId,omitempty"`
	SpanID                 string        `json:"spanId,omitempty"`
}

// LogRecordBody represents the body of a log record
type LogRecordBody struct {
	StringValue *string `json:"stringValue,omitempty"`
}

// Resource represents an OTLP resource
type Resource struct {
	Attributes             []Attribute `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
}

// Scope represents an instrumentation scope
type Scope struct {
	Name                   string      `json:"name,omitempty"`
	Version                string      `json:"version,omitempty"`
	Attributes             []Attribute `json:"attributes,omitempty"`
	DroppedAttributesCount uint32      `json:"droppedAttributesCount,omitempty"`
}

// Attribute represents an OTLP attribute key-value pair
type Attribute struct {
	Key   string         `json:"key"`
	Value AttributeValue `json:"value"`
}

// AttributeValue represents an OTLP attribute value
type AttributeValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
	IntValue    *int64   `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}
