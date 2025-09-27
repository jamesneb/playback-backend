package validation

import (
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Type aliases for better readability and type safety
type (
	MessageSize  int64
)

// Size constants
const (
	MaxProtobufTraceSize   MessageSize = 8 * 1024 * 1024  // 8MB max for trace data
	MaxProtobufMetricsSize MessageSize = 4 * 1024 * 1024  // 4MB max for metrics data
	MaxProtobufLogsSize    MessageSize = 2 * 1024 * 1024  // 2MB max for logs data
	MaxProtobufSpansCount  int         = 10000            // Max spans per request
	MaxProtobufScopeCount  int         = 100              // Max scopes per resource
)

// Error constants
const (
	ErrProtobufSizeTooLarge   = "protobuf message size exceeds limit"
	ErrProtobufInvalidData    = "protobuf message contains invalid data"
	ErrProtobufSpanCount      = "too many spans in trace request"
	ErrProtobufScopeCount     = "too many scopes in resource"
	ErrProtobufMarshalFailed  = "failed to marshal protobuf data"
)

// ProtobufValidator provides validation for OTLP protobuf messages
type ProtobufValidator struct{}

// NewProtobufValidator creates a new protobuf validator
func NewProtobufValidator() *ProtobufValidator {
	return &ProtobufValidator{}
}

// ValidateTraceRequest validates an OTLP trace export request
func (v *ProtobufValidator) ValidateTraceRequest(req *tracepb.TracesData) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, ErrProtobufInvalidData+": request is nil")
	}

	// Check message size
	size := proto.Size(req)
	if MessageSize(size) > MaxProtobufTraceSize {
		return status.Errorf(codes.ResourceExhausted,
			ErrProtobufSizeTooLarge+": trace data size %d bytes exceeds limit %d bytes",
			size, MaxProtobufTraceSize)
	}

	// Validate resource spans
	totalSpans := 0
	for i, resourceSpan := range req.ResourceSpans {
		if resourceSpan == nil {
			return status.Errorf(codes.InvalidArgument,
				ErrProtobufInvalidData+": resource span %d is nil", i)
		}

		// Check scope count
		if len(resourceSpan.ScopeSpans) > MaxProtobufScopeCount {
			return status.Errorf(codes.InvalidArgument,
				ErrProtobufScopeCount+": resource span %d has %d scopes, max %d allowed",
				i, len(resourceSpan.ScopeSpans), MaxProtobufScopeCount)
		}

		// Count spans and validate
		for j, scopeSpan := range resourceSpan.ScopeSpans {
			if scopeSpan == nil {
				return status.Errorf(codes.InvalidArgument,
					ErrProtobufInvalidData+": scope span %d.%d is nil", i, j)
			}

			spanCount := len(scopeSpan.Spans)
			totalSpans += spanCount

			// Validate individual spans
			for k, span := range scopeSpan.Spans {
				if span == nil {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": span %d.%d.%d is nil", i, j, k)
				}

				// Basic span validation
				if len(span.TraceId) == 0 {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": span %d.%d.%d has empty trace ID", i, j, k)
				}

				if len(span.SpanId) == 0 {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": span %d.%d.%d has empty span ID", i, j, k)
				}
			}
		}
	}

	// Check total span count
	if totalSpans > MaxProtobufSpansCount {
		return status.Errorf(codes.InvalidArgument,
			ErrProtobufSpanCount+": request contains %d spans, max %d allowed",
			totalSpans, MaxProtobufSpansCount)
	}

	return nil
}

// ValidateMetricsRequest validates an OTLP metrics export request
func (v *ProtobufValidator) ValidateMetricsRequest(req *metricspb.MetricsData) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, ErrProtobufInvalidData+": request is nil")
	}

	// Check message size
	size := proto.Size(req)
	if MessageSize(size) > MaxProtobufMetricsSize {
		return status.Errorf(codes.ResourceExhausted,
			ErrProtobufSizeTooLarge+": metrics data size %d bytes exceeds limit %d bytes",
			size, MaxProtobufMetricsSize)
	}

	// Validate resource metrics
	for i, resourceMetric := range req.ResourceMetrics {
		if resourceMetric == nil {
			return status.Errorf(codes.InvalidArgument,
				ErrProtobufInvalidData+": resource metric %d is nil", i)
		}

		// Check scope count
		if len(resourceMetric.ScopeMetrics) > MaxProtobufScopeCount {
			return status.Errorf(codes.InvalidArgument,
				ErrProtobufScopeCount+": resource metric %d has %d scopes, max %d allowed",
				i, len(resourceMetric.ScopeMetrics), MaxProtobufScopeCount)
		}

		// Validate scopes and metrics
		for j, scopeMetric := range resourceMetric.ScopeMetrics {
			if scopeMetric == nil {
				return status.Errorf(codes.InvalidArgument,
					ErrProtobufInvalidData+": scope metric %d.%d is nil", i, j)
			}

			// Validate individual metrics
			for k, metric := range scopeMetric.Metrics {
				if metric == nil {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": metric %d.%d.%d is nil", i, j, k)
				}

				if metric.Name == "" {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": metric %d.%d.%d has empty name", i, j, k)
				}
			}
		}
	}

	return nil
}

// ValidateLogsRequest validates an OTLP logs export request
func (v *ProtobufValidator) ValidateLogsRequest(req *logspb.LogsData) error {
	if req == nil {
		return status.Errorf(codes.InvalidArgument, ErrProtobufInvalidData+": request is nil")
	}

	// Check message size
	size := proto.Size(req)
	if MessageSize(size) > MaxProtobufLogsSize {
		return status.Errorf(codes.ResourceExhausted,
			ErrProtobufSizeTooLarge+": logs data size %d bytes exceeds limit %d bytes",
			size, MaxProtobufLogsSize)
	}

	// Validate resource logs
	for i, resourceLog := range req.ResourceLogs {
		if resourceLog == nil {
			return status.Errorf(codes.InvalidArgument,
				ErrProtobufInvalidData+": resource log %d is nil", i)
		}

		// Check scope count
		if len(resourceLog.ScopeLogs) > MaxProtobufScopeCount {
			return status.Errorf(codes.InvalidArgument,
				ErrProtobufScopeCount+": resource log %d has %d scopes, max %d allowed",
				i, len(resourceLog.ScopeLogs), MaxProtobufScopeCount)
		}

		// Validate scopes and log records
		for j, scopeLog := range resourceLog.ScopeLogs {
			if scopeLog == nil {
				return status.Errorf(codes.InvalidArgument,
					ErrProtobufInvalidData+": scope log %d.%d is nil", i, j)
			}

			// Validate individual log records
			for k, logRecord := range scopeLog.LogRecords {
				if logRecord == nil {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": log record %d.%d.%d is nil", i, j, k)
				}

				// Basic log record validation
				if logRecord.TimeUnixNano == 0 {
					return status.Errorf(codes.InvalidArgument,
						ErrProtobufInvalidData+": log record %d.%d.%d has invalid timestamp", i, j, k)
				}
			}
		}
	}

	return nil
}

// ValidateProtobufSize checks if a protobuf message size is within limits
func (v *ProtobufValidator) ValidateProtobufSize(message proto.Message, maxSize MessageSize, messageType string) error {
	if message == nil {
		return status.Errorf(codes.InvalidArgument, ErrProtobufInvalidData+": %s message is nil", messageType)
	}

	size := proto.Size(message)
	if MessageSize(size) > maxSize {
		return status.Errorf(codes.ResourceExhausted,
			ErrProtobufSizeTooLarge+": %s size %d bytes exceeds limit %d bytes",
			messageType, size, maxSize)
	}

	return nil
}

// MarshalWithSizeCheck marshals a protobuf message with size validation
func (v *ProtobufValidator) MarshalWithSizeCheck(message proto.Message, maxSize MessageSize, messageType string) ([]byte, error) {
	// Check size before marshaling
	if err := v.ValidateProtobufSize(message, maxSize, messageType); err != nil {
		return nil, err
	}

	// Marshal the message
	data, err := proto.Marshal(message)
	if err != nil {
		return nil, status.Errorf(codes.Internal,
			ErrProtobufMarshalFailed+": failed to marshal %s: %v", messageType, err)
	}

	// Double-check marshaled size (should be same as proto.Size but ensures consistency)
	if MessageSize(len(data)) > maxSize {
		return nil, status.Errorf(codes.ResourceExhausted,
			ErrProtobufSizeTooLarge+": marshaled %s size %d bytes exceeds limit %d bytes",
			messageType, len(data), maxSize)
	}

	return data, nil
}