package validation

import (
	"runtime"
	"sync/atomic"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// Type aliases for better readability and type safety
type (
	MessageSize int64
)

// Size constants
const (
	MaxProtobufTraceSize   MessageSize = 4 * 1024 * 1024 // 4MB max for trace data (reduced from 8MB)
	MaxProtobufMetricsSize MessageSize = 2 * 1024 * 1024 // 2MB max for metrics data (reduced from 4MB)
	MaxProtobufLogsSize    MessageSize = 1 * 1024 * 1024 // 1MB max for logs data (reduced from 2MB)
	MaxProtobufSpansCount  int         = 5000            // Max spans per request (reduced from 10000)
	MaxProtobufScopeCount  int         = 50              // Max scopes per resource (reduced from 100)

	// Memory pressure thresholds
	MemoryPressureThresholdBytes = 512 * 1024 * 1024 // 512MB heap threshold
	MaxConcurrentRequests        = 100               // Max concurrent protobuf validations
)

// Error constants
const (
	ErrProtobufSizeTooLarge  = "protobuf message size exceeds limit"
	ErrProtobufInvalidData   = "protobuf message contains invalid data"
	ErrProtobufSpanCount     = "too many spans in trace request"
	ErrProtobufScopeCount    = "too many scopes in resource"
	ErrProtobufMarshalFailed = "failed to marshal protobuf data"
	ErrMemoryPressure        = "system under memory pressure, rejecting request"
	ErrTooManyRequests       = "too many concurrent requests"
)

// ProtobufValidator provides validation for OTLP protobuf messages with memory monitoring
type ProtobufValidator struct {
	concurrentRequests int64 // Atomic counter for concurrent requests
}

// NewProtobufValidator creates a new protobuf validator
func NewProtobufValidator() *ProtobufValidator {
	return &ProtobufValidator{}
}

// checkMemoryPressure validates system memory state before processing
func (v *ProtobufValidator) checkMemoryPressure() error {
	// Check concurrent request limit
	current := atomic.LoadInt64(&v.concurrentRequests)
	if current >= MaxConcurrentRequests {
		return status.Errorf(codes.ResourceExhausted, ErrTooManyRequests+": %d concurrent requests", current)
	}

	// Check memory pressure
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	if memStats.HeapInuse > MemoryPressureThresholdBytes {
		return status.Errorf(codes.ResourceExhausted,
			ErrMemoryPressure+": heap usage %dMB exceeds threshold %dMB",
			memStats.HeapInuse/(1024*1024), MemoryPressureThresholdBytes/(1024*1024))
	}

	return nil
}

// ValidateTraceRequest validates an OTLP trace export request with memory monitoring
func (v *ProtobufValidator) ValidateTraceRequest(req *tracepb.TracesData) error {
	// Check memory pressure first
	if err := v.checkMemoryPressure(); err != nil {
		return err
	}

	// Increment concurrent request counter
	atomic.AddInt64(&v.concurrentRequests, 1)
	defer atomic.AddInt64(&v.concurrentRequests, -1)

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

// ValidateMetricsRequest validates an OTLP metrics export request with memory monitoring
func (v *ProtobufValidator) ValidateMetricsRequest(req *metricspb.MetricsData) error {
	// Check memory pressure first
	if err := v.checkMemoryPressure(); err != nil {
		return err
	}

	// Increment concurrent request counter
	atomic.AddInt64(&v.concurrentRequests, 1)
	defer atomic.AddInt64(&v.concurrentRequests, -1)

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

// ValidateLogsRequest validates an OTLP logs export request with memory monitoring
func (v *ProtobufValidator) ValidateLogsRequest(req *logspb.LogsData) error {
	// Check memory pressure first
	if err := v.checkMemoryPressure(); err != nil {
		return err
	}

	// Increment concurrent request counter
	atomic.AddInt64(&v.concurrentRequests, 1)
	defer atomic.AddInt64(&v.concurrentRequests, -1)

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
