package validation

import (
	"testing"
	"time"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// BenchmarkProtobufValidator benchmarks the protobuf validator creation
func BenchmarkProtobufValidator(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		validator := NewProtobufValidator()
		_ = validator
	}
}

// BenchmarkMemoryPressureCheck benchmarks memory pressure checking
func BenchmarkMemoryPressureCheck(b *testing.B) {
	validator := NewProtobufValidator()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := validator.checkMemoryPressure()
		_ = err
	}
}

// BenchmarkTraceValidation benchmarks trace validation performance
func BenchmarkTraceValidation(b *testing.B) {
	validator := NewProtobufValidator()

	// Create test trace data
	testTrace := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("test-trace-id-12"),
								SpanId:            []byte("test-span"),
								Name:              "test-span",
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().UnixNano()),
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateTraceRequest(testTrace)
		_ = err
	}
}

// BenchmarkTraceValidationParallel benchmarks trace validation under concurrent load
func BenchmarkTraceValidationParallel(b *testing.B) {
	validator := NewProtobufValidator()

	testTrace := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("test-trace-id-12"),
								SpanId:            []byte("test-span"),
								Name:              "test-span",
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().UnixNano()),
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := validator.ValidateTraceRequest(testTrace)
			_ = err
		}
	})
}

// BenchmarkMetricsValidation benchmarks metrics validation performance
func BenchmarkMetricsValidation(b *testing.B) {
	validator := NewProtobufValidator()

	testMetrics := &metricspb.MetricsData{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name: "test-metric",
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateMetricsRequest(testMetrics)
		_ = err
	}
}

// BenchmarkLogsValidation benchmarks logs validation performance
func BenchmarkLogsValidation(b *testing.B) {
	validator := NewProtobufValidator()

	testLogs := &logspb.LogsData{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano: uint64(time.Now().UnixNano()),
								// Simplified for benchmark - focus on the validation performance
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateLogsRequest(testLogs)
		_ = err
	}
}

// BenchmarkProtobufSizeCalculation benchmarks protobuf size calculation
func BenchmarkProtobufSizeCalculation(b *testing.B) {
	testTrace := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("test-trace-id-12"),
								SpanId:            []byte("test-span"),
								Name:              "test-span",
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().UnixNano()),
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		size := proto.Size(testTrace)
		_ = size
	}
}

// BenchmarkValidateProtobufSize benchmarks the size validation method
func BenchmarkValidateProtobufSize(b *testing.B) {
	validator := NewProtobufValidator()

	testTrace := &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("test-trace-id-12"),
								SpanId:            []byte("test-span"),
								Name:              "test-span",
								StartTimeUnixNano: uint64(time.Now().UnixNano()),
								EndTimeUnixNano:   uint64(time.Now().UnixNano()),
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := validator.ValidateProtobufSize(testTrace, MaxProtobufTraceSize, "trace")
		_ = err
	}
}
