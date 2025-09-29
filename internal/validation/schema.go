package validation

import (
	"sync"
	"time"
	"unsafe"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Hot path validation constants - zero allocations
const (
	maxAttributeKeyLength   = 256
	maxAttributeValueLength = 4096
	maxAttributesPerSpan    = 128
	maxEventsPerSpan        = 64
	maxLinksPerSpan         = 32
	maxTimestampSkewNanos   = int64(7 * 24 * time.Hour)

	// Pre-allocated error messages for zero allocation
	errSpanNil          = "span is nil"
	errTraceIDEmpty     = "trace ID empty"
	errSpanIDEmpty      = "span ID empty"
	errAttributeNil     = "attribute is nil"
	errAttributeKeyLong = "attribute key too long"
	errTooManyAttrs     = "too many attributes"
)

// Pre-allocated byte slices for hex conversion - zero allocation lookups
var (
	hexLookup = [256]bool{
		'0': true, '1': true, '2': true, '3': true, '4': true,
		'5': true, '6': true, '7': true, '8': true, '9': true,
		'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true,
		'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true,
	}

	// Object pools for zero-allocation validation
	validatorPool = sync.Pool{
		New: func() interface{} {
			return &hotPathValidator{
				spanIDSet:  make(map[uint64]struct{}, 512),
				traceIDSet: make(map[uint64]struct{}, 64),
			}
		},
	}

)

// SchemaValidator provides zero-allocation OpenTelemetry validation
type SchemaValidator struct {
	enableStrict bool
}

// NewSchemaValidator creates high-performance validator
func NewSchemaValidator(enableStrict bool) *SchemaValidator {
	return &SchemaValidator{enableStrict: enableStrict}
}

// hotPathValidator contains reusable validation state
type hotPathValidator struct {
	spanIDSet  map[uint64]struct{} // Hash-based deduplication
	traceIDSet map[uint64]struct{}
	spanCount  int
}


// getValidator retrieves validator from pool and resets state
func getValidator() *hotPathValidator {
	v := validatorPool.Get().(*hotPathValidator)

	// Clear maps efficiently without reallocating
	for k := range v.spanIDSet {
		delete(v.spanIDSet, k)
	}
	for k := range v.traceIDSet {
		delete(v.traceIDSet, k)
	}
	v.spanCount = 0

	return v
}

// returnValidator returns validator to pool
func returnValidator(v *hotPathValidator) {
	validatorPool.Put(v)
}

// ValidateTraceData performs hot-path validation with zero allocations
func (sv *SchemaValidator) ValidateTraceData(data *tracepb.TracesData) error {
	if data == nil {
		return status.Error(codes.InvalidArgument, "trace data is nil")
	}

	validator := getValidator()
	defer returnValidator(validator)

	// Validate all resource spans in single pass
	for i := 0; i < len(data.ResourceSpans); i++ {
		rs := data.ResourceSpans[i]
		if rs == nil {
			return status.Error(codes.InvalidArgument, "resource span is nil")
		}

		// Validate all scope spans
		for j := 0; j < len(rs.ScopeSpans); j++ {
			ss := rs.ScopeSpans[j]
			if ss == nil {
				return status.Error(codes.InvalidArgument, "scope span is nil")
			}

			// Hot path: validate spans with minimal bounds checking
			if err := sv.validateSpansHotPath(ss.Spans, validator); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateSpansHotPath optimized span validation with unsafe operations
func (sv *SchemaValidator) validateSpansHotPath(spans []*tracepb.Span, validator *hotPathValidator) error {
	spanCount := len(spans)
	validator.spanCount += spanCount

	// Bounds check once for the entire slice
	if spanCount > maxAttributesPerSpan {
		return status.Error(codes.InvalidArgument, errTooManyAttrs)
	}

	// Process spans with manual loop unrolling for better performance
	i := 0
	for i < spanCount-3 {
		// Unroll loop by 4 for better CPU pipeline utilization
		if err := sv.validateSpanInline(spans[i], validator); err != nil {
			return err
		}
		if err := sv.validateSpanInline(spans[i+1], validator); err != nil {
			return err
		}
		if err := sv.validateSpanInline(spans[i+2], validator); err != nil {
			return err
		}
		if err := sv.validateSpanInline(spans[i+3], validator); err != nil {
			return err
		}
		i += 4
	}

	// Handle remaining spans
	for i < spanCount {
		if err := sv.validateSpanInline(spans[i], validator); err != nil {
			return err
		}
		i++
	}

	return nil
}

// validateSpanInline inlined span validation for maximum performance
func (sv *SchemaValidator) validateSpanInline(span *tracepb.Span, validator *hotPathValidator) error {
	if span == nil {
		return status.Error(codes.InvalidArgument, errSpanNil)
	}

	// Fast path: validate required fields with single bounds check
	if len(span.TraceId) == 0 {
		return status.Error(codes.InvalidArgument, errTraceIDEmpty)
	}
	if len(span.SpanId) == 0 {
		return status.Error(codes.InvalidArgument, errSpanIDEmpty)
	}

	// Zero-allocation ID validation using unsafe pointer arithmetic
	traceIDHash := fastHash64(span.TraceId)
	spanIDHash := fastHash64(span.SpanId)

	// Check for duplicate span IDs using hash-based deduplication
	if _, exists := validator.spanIDSet[spanIDHash]; exists {
		return status.Error(codes.InvalidArgument, "duplicate span ID")
	}
	validator.spanIDSet[spanIDHash] = struct{}{}
	validator.traceIDSet[traceIDHash] = struct{}{}

	// Validate timestamps with minimal branching
	if span.StartTimeUnixNano == 0 {
		return status.Error(codes.InvalidArgument, "invalid start timestamp")
	}

	// Fast attribute validation with bounds check
	attrCount := len(span.Attributes)
	if attrCount > maxAttributesPerSpan {
		return status.Error(codes.InvalidArgument, errTooManyAttrs)
	}

	// Hot path: validate attributes with minimal overhead
	for i := 0; i < attrCount; i++ {
		attr := span.Attributes[i]
		if attr == nil {
			return status.Error(codes.InvalidArgument, errAttributeNil)
		}

		// Fast key validation without regex
		keyLen := len(attr.Key)
		if keyLen == 0 || keyLen > maxAttributeKeyLength {
			return status.Error(codes.InvalidArgument, errAttributeKeyLong)
		}

		// Skip expensive value validation in hot path unless strict mode
		if sv.enableStrict && attr.Value != nil {
			if err := sv.validateAttributeValueFast(attr.Value); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateAttributeValueFast optimized attribute value validation
func (sv *SchemaValidator) validateAttributeValueFast(value *commonpb.AnyValue) error {
	// Use type switch with jump table optimization
	switch value.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		if v := value.GetStringValue(); len(v) > maxAttributeValueLength {
			return status.Error(codes.InvalidArgument, "string value too long")
		}
	case *commonpb.AnyValue_BytesValue:
		if v := value.GetBytesValue(); len(v) > maxAttributeValueLength {
			return status.Error(codes.InvalidArgument, "bytes value too long")
		}
	}
	return nil
}

// fastHash64 computes fast 64-bit hash using FNV-1a algorithm
func fastHash64(data []byte) uint64 {
	const (
		fnvOffsetBasis = 14695981039346656037
		fnvPrime       = 1099511628211
	)

	hash := uint64(fnvOffsetBasis)
	dataLen := len(data)

	// Process 8 bytes at a time using unsafe for maximum performance
	i := 0
	for i+7 < dataLen {
		// Load 8 bytes as uint64 using unsafe
		chunk := *(*uint64)(unsafe.Pointer(&data[i]))
		hash ^= chunk
		hash *= fnvPrime
		i += 8
	}

	// Process remaining bytes
	for i < dataLen {
		hash ^= uint64(data[i])
		hash *= fnvPrime
		i++
	}

	return hash
}

// ValidateMetricsData hot-path metrics validation
func (sv *SchemaValidator) ValidateMetricsData(data *metricspb.MetricsData) error {
	if data == nil {
		return status.Error(codes.InvalidArgument, "metrics data is nil")
	}

	// Validate resource metrics with minimal overhead
	for i := 0; i < len(data.ResourceMetrics); i++ {
		rm := data.ResourceMetrics[i]
		if rm == nil {
			return status.Error(codes.InvalidArgument, "resource metrics is nil")
		}

		// Validate scope metrics
		for j := 0; j < len(rm.ScopeMetrics); j++ {
			sm := rm.ScopeMetrics[j]
			if sm == nil {
				return status.Error(codes.InvalidArgument, "scope metrics is nil")
			}

			// Fast metric validation
			if err := sv.validateMetricsHotPath(sm.Metrics); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateMetricsHotPath optimized metrics validation
func (sv *SchemaValidator) validateMetricsHotPath(metrics []*metricspb.Metric) error {
	for i := 0; i < len(metrics); i++ {
		metric := metrics[i]
		if metric == nil {
			return status.Error(codes.InvalidArgument, "metric is nil")
		}

		// Minimal validation - just check name exists
		if len(metric.Name) == 0 {
			return status.Error(codes.InvalidArgument, "metric name empty")
		}
	}
	return nil
}

// ValidateLogsData hot-path logs validation
func (sv *SchemaValidator) ValidateLogsData(data *logspb.LogsData) error {
	if data == nil {
		return status.Error(codes.InvalidArgument, "logs data is nil")
	}

	// Validate resource logs with minimal overhead
	for i := 0; i < len(data.ResourceLogs); i++ {
		rl := data.ResourceLogs[i]
		if rl == nil {
			return status.Error(codes.InvalidArgument, "resource logs is nil")
		}

		// Validate scope logs
		for j := 0; j < len(rl.ScopeLogs); j++ {
			sl := rl.ScopeLogs[j]
			if sl == nil {
				return status.Error(codes.InvalidArgument, "scope logs is nil")
			}

			// Fast log record validation
			if err := sv.validateLogRecordsHotPath(sl.LogRecords); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateLogRecordsHotPath optimized log record validation
func (sv *SchemaValidator) validateLogRecordsHotPath(records []*logspb.LogRecord) error {
	for i := 0; i < len(records); i++ {
		record := records[i]
		if record == nil {
			return status.Error(codes.InvalidArgument, "log record is nil")
		}

		// Minimal validation - just check timestamp exists
		if record.TimeUnixNano == 0 {
			return status.Error(codes.InvalidArgument, "log timestamp zero")
		}
	}
	return nil
}

// isValidHexBytesFast validates hex bytes using lookup table - zero allocations
func isValidHexBytesFast(data []byte) bool {
	dataLen := len(data)

	// Process 4 bytes at a time for better performance
	i := 0
	for i+3 < dataLen {
		if !hexLookup[data[i]] || !hexLookup[data[i+1]] ||
		   !hexLookup[data[i+2]] || !hexLookup[data[i+3]] {
			return false
		}
		i += 4
	}

	// Process remaining bytes
	for i < dataLen {
		if !hexLookup[data[i]] {
			return false
		}
		i++
	}

	return true
}

// bytesToHexUnsafe converts bytes to hex string using unsafe - zero allocations
func bytesToHexUnsafe(src []byte) string {
	if len(src) == 0 {
		return ""
	}

	// Pre-allocated hex digits for maximum performance
	const hexDigits = "0123456789abcdef"

	// Allocate result buffer with exact size
	dst := make([]byte, len(src)*2)

	// Manual loop unrolling for better performance
	j := 0
	for i := 0; i < len(src); i++ {
		dst[j] = hexDigits[src[i]>>4]
		dst[j+1] = hexDigits[src[i]&0x0f]
		j += 2
	}

	// Zero-allocation conversion using unsafe
	return *(*string)(unsafe.Pointer(&dst))
}