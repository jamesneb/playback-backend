package streaming

import (
	"time"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// TelemetryMetadata contains metadata about the telemetry ingestion
type TelemetryMetadata struct {
	IngestedAt time.Time `json:"ingested_at"`
	SourceIP   string    `json:"source_ip"`
	TenantID   string    `json:"tenant_id,omitempty"`
}

// TelemetryEventType represents the type of telemetry data
type TelemetryEventType string

const (
	TelemetryTypeTraces  TelemetryEventType = "traces"
	TelemetryTypeMetrics TelemetryEventType = "metrics"
	TelemetryTypeLogs    TelemetryEventType = "logs"
)

// BaseTelemetryEvent contains common fields for all telemetry events
type BaseTelemetryEvent struct {
	Type        TelemetryEventType `json:"type"`
	ServiceName string             `json:"service_name"` // For partitioning
	TraceID     string             `json:"trace_id,omitempty"`
	Metadata    TelemetryMetadata  `json:"metadata"`
}

// TraceTelemetryEvent represents a trace event with native protobuf data
type TraceTelemetryEvent struct {
	BaseTelemetryEvent
	ResourceSpans *tracepb.ResourceSpans `json:"-"` // Native protobuf, not serialized to JSON
}

// MetricsTelemetryEvent represents a metrics event with native protobuf data
type MetricsTelemetryEvent struct {
	BaseTelemetryEvent
	ResourceMetrics *metricspb.ResourceMetrics `json:"-"` // Native protobuf
}

// LogsTelemetryEvent represents a logs event with native protobuf data
type LogsTelemetryEvent struct {
	BaseTelemetryEvent
	ResourceLogs *logspb.ResourceLogs `json:"-"` // Native protobuf
}

// TelemetryEvent interface for type-safe handling
type TelemetryEvent interface {
	GetType() TelemetryEventType
	GetServiceName() string
	GetTraceID() string
	GetMetadata() TelemetryMetadata
	GetSerializedData() ([]byte, error) // For ClickHouse storage
	Validate() error
}

// Implement TelemetryEvent interface for TraceTelemetryEvent
func (e *TraceTelemetryEvent) GetType() TelemetryEventType {
	return e.Type
}

func (e *TraceTelemetryEvent) GetServiceName() string {
	return e.ServiceName
}

func (e *TraceTelemetryEvent) GetTraceID() string {
	return e.TraceID
}

func (e *TraceTelemetryEvent) GetMetadata() TelemetryMetadata {
	return e.Metadata
}

func (e *TraceTelemetryEvent) Validate() error {
	if e.ResourceSpans == nil {
		return ErrInvalidTraceData
	}
	if e.ServiceName == "" {
		return ErrMissingServiceName
	}
	if len(e.ResourceSpans.ScopeSpans) == 0 {
		return ErrEmptySpanData
	}
	return nil
}

func (e *TraceTelemetryEvent) GetSerializedData() ([]byte, error) {
	return protojson.Marshal(e.ResourceSpans)
}

// Implement TelemetryEvent interface for MetricsTelemetryEvent
func (e *MetricsTelemetryEvent) GetType() TelemetryEventType {
	return e.Type
}

func (e *MetricsTelemetryEvent) GetServiceName() string {
	return e.ServiceName
}

func (e *MetricsTelemetryEvent) GetTraceID() string {
	return e.TraceID
}

func (e *MetricsTelemetryEvent) GetMetadata() TelemetryMetadata {
	return e.Metadata
}

func (e *MetricsTelemetryEvent) Validate() error {
	if e.ResourceMetrics == nil {
		return ErrInvalidMetricsData
	}
	if e.ServiceName == "" {
		return ErrMissingServiceName
	}
	if len(e.ResourceMetrics.ScopeMetrics) == 0 {
		return ErrEmptyMetricsData
	}
	return nil
}

func (e *MetricsTelemetryEvent) GetSerializedData() ([]byte, error) {
	return protojson.Marshal(e.ResourceMetrics)
}

// Implement TelemetryEvent interface for LogsTelemetryEvent
func (e *LogsTelemetryEvent) GetType() TelemetryEventType {
	return e.Type
}

func (e *LogsTelemetryEvent) GetServiceName() string {
	return e.ServiceName
}

func (e *LogsTelemetryEvent) GetTraceID() string {
	return e.TraceID
}

func (e *LogsTelemetryEvent) GetMetadata() TelemetryMetadata {
	return e.Metadata
}

func (e *LogsTelemetryEvent) Validate() error {
	if e.ResourceLogs == nil {
		return ErrInvalidLogsData
	}
	if e.ServiceName == "" {
		return ErrMissingServiceName
	}
	if len(e.ResourceLogs.ScopeLogs) == 0 {
		return ErrEmptyLogsData
	}
	return nil
}

func (e *LogsTelemetryEvent) GetSerializedData() ([]byte, error) {
	return protojson.Marshal(e.ResourceLogs)
}

// ProtobufTelemetryEvent is used for transporting native protobuf events via Kinesis
// This is sent as raw protobuf bytes - no JSON wrapping needed since Kinesis is content-agnostic
type ProtobufTelemetryEvent struct {
	Type        string            `protobuf:"bytes,1,opt,name=type" json:"type"`
	ServiceName string            `protobuf:"bytes,2,opt,name=service_name" json:"service_name"`
	TraceID     string            `protobuf:"bytes,3,opt,name=trace_id" json:"trace_id,omitempty"`
	Data        []byte            `protobuf:"bytes,4,opt,name=data" json:"data"`         // Raw OTLP protobuf bytes
	Format      string            `protobuf:"bytes,5,opt,name=format" json:"format"`     // "protobuf" identifier
	Metadata    TelemetryMetadata `protobuf:"bytes,6,opt,name=metadata" json:"metadata"`
}

// Helper functions to extract data from protobuf
func ExtractServiceNameFromTraces(resourceSpans *tracepb.ResourceSpans) string {
	if resourceSpans.Resource != nil {
		for _, attr := range resourceSpans.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.GetStringValue() != "" {
				return attr.Value.GetStringValue()
			}
		}
	}
	return "unknown"
}

func ExtractTraceIDFromTraces(resourceSpans *tracepb.ResourceSpans) string {
	if len(resourceSpans.ScopeSpans) > 0 && len(resourceSpans.ScopeSpans[0].Spans) > 0 {
		return string(resourceSpans.ScopeSpans[0].Spans[0].TraceId)
	}
	return ""
}

func ExtractServiceNameFromMetrics(resourceMetrics *metricspb.ResourceMetrics) string {
	if resourceMetrics.Resource != nil {
		for _, attr := range resourceMetrics.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.GetStringValue() != "" {
				return attr.Value.GetStringValue()
			}
		}
	}
	return "unknown"
}

func ExtractServiceNameFromLogs(resourceLogs *logspb.ResourceLogs) string {
	if resourceLogs.Resource != nil {
		for _, attr := range resourceLogs.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.GetStringValue() != "" {
				return attr.Value.GetStringValue()
			}
		}
	}
	return "unknown"
}

func ExtractTraceIDFromLogs(resourceLogs *logspb.ResourceLogs) string {
	if len(resourceLogs.ScopeLogs) > 0 && len(resourceLogs.ScopeLogs[0].LogRecords) > 0 {
		return string(resourceLogs.ScopeLogs[0].LogRecords[0].TraceId)
	}
	return ""
}