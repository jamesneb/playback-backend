package validation

import (
	"testing"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// createBenchmarkTraceData creates realistic trace data for benchmarking
func createBenchmarkTraceData(spanCount int) *tracepb.TracesData {
	spans := make([]*tracepb.Span, spanCount)

	// Pre-allocate common data to avoid allocation overhead in benchmark
	traceID := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}

	for i := 0; i < spanCount; i++ {
		spanID := []byte{byte(i + 1), 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, byte(i + 1)}

		spans[i] = &tracepb.Span{
			TraceId:           traceID,
			SpanId:            spanID,
			Name:              "benchmark-span",
			Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
			StartTimeUnixNano: 1640995200000000000,
			EndTimeUnixNano:   1640995200100000000,
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "benchmark-service"}},
				},
				{
					Key:   "http.method",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}},
				},
				{
					Key:   "http.status_code",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 200}},
				},
			},
		}
	}

	return &tracepb.TracesData{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "benchmark-service"}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: spans,
					},
				},
			},
		},
	}
}

// createBenchmarkMetricsData creates realistic metrics data for benchmarking
func createBenchmarkMetricsData(metricCount int) *metricspb.MetricsData {
	metrics := make([]*metricspb.Metric, metricCount)

	for i := 0; i < metricCount; i++ {
		metrics[i] = &metricspb.Metric{
			Name: "benchmark_metric",
			Unit: "count",
			Data: &metricspb.Metric_Sum{
				Sum: &metricspb.Sum{
					AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
					IsMonotonic:            true,
					DataPoints: []*metricspb.NumberDataPoint{
						{
							TimeUnixNano: 1640995200000000000,
							Value: &metricspb.NumberDataPoint_AsInt{
								AsInt: int64(i + 1),
							},
						},
					},
				},
			},
		}
	}

	return &metricspb.MetricsData{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: metrics,
					},
				},
			},
		},
	}
}

// createBenchmarkLogsData creates realistic logs data for benchmarking
func createBenchmarkLogsData(logCount int) *logspb.LogsData {
	logRecords := make([]*logspb.LogRecord, logCount)

	for i := 0; i < logCount; i++ {
		logRecords[i] = &logspb.LogRecord{
			TimeUnixNano: 1640995200000000000,
			Body: &commonpb.AnyValue{
				Value: &commonpb.AnyValue_StringValue{
					StringValue: "benchmark log message",
				},
			},
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "log.level",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "INFO"}},
				},
			},
		}
	}

	return &logspb.LogsData{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: logRecords,
					},
				},
			},
		},
	}
}

// Benchmark small trace validation (typical microservice span count)
func BenchmarkSchemaValidator_TraceData_Small(b *testing.B) {
	validator := NewSchemaValidator(false)
	traceData := createBenchmarkTraceData(10)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.ValidateTraceData(traceData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark medium trace validation (typical batch size)
func BenchmarkSchemaValidator_TraceData_Medium(b *testing.B) {
	validator := NewSchemaValidator(false)
	traceData := createBenchmarkTraceData(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.ValidateTraceData(traceData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark large trace validation (maximum realistic batch)
func BenchmarkSchemaValidator_TraceData_Large(b *testing.B) {
	validator := NewSchemaValidator(false)
	traceData := createBenchmarkTraceData(1000)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.ValidateTraceData(traceData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark trace validation with strict mode enabled
func BenchmarkSchemaValidator_TraceData_Strict(b *testing.B) {
	validator := NewSchemaValidator(true)
	traceData := createBenchmarkTraceData(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.ValidateTraceData(traceData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark metrics validation
func BenchmarkSchemaValidator_MetricsData(b *testing.B) {
	validator := NewSchemaValidator(false)
	metricsData := createBenchmarkMetricsData(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.ValidateMetricsData(metricsData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark logs validation
func BenchmarkSchemaValidator_LogsData(b *testing.B) {
	validator := NewSchemaValidator(false)
	logsData := createBenchmarkLogsData(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.ValidateLogsData(logsData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark hash function performance
func BenchmarkFastHash64(b *testing.B) {
	data := []byte("0123456789abcdef0123456789abcdef") // 32-byte trace ID

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		fastHash64(data)
	}
}

// Benchmark hex validation performance
func BenchmarkIsValidHexBytesFast(b *testing.B) {
	data := []byte("0123456789abcdef0123456789abcdef")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		isValidHexBytesFast(data)
	}
}

// Benchmark unsafe hex conversion
func BenchmarkBytesToHexUnsafe(b *testing.B) {
	data := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		bytesToHexUnsafe(data)
	}
}

// Benchmark validator pool efficiency
func BenchmarkValidatorPool(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		validator := getValidator()
		validator.spanCount = 100 // Simulate some work
		returnValidator(validator)
	}
}

// Benchmark memory pressure with concurrent validation
func BenchmarkSchemaValidator_Concurrent(b *testing.B) {
	validator := NewSchemaValidator(false)
	traceData := createBenchmarkTraceData(50)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := validator.ValidateTraceData(traceData); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark comparison with original protobuf validator
func BenchmarkProtobufValidator_Comparison(b *testing.B) {
	protobufValidator := NewProtobufValidator()
	traceData := createBenchmarkTraceData(100)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := protobufValidator.ValidateTraceRequest(traceData); err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark attribute validation hot path
func BenchmarkAttributeValidation_HotPath(b *testing.B) {
	validator := NewSchemaValidator(false)

	attr := &commonpb.KeyValue{
		Key: "test.attribute",
		Value: &commonpb.AnyValue{
			Value: &commonpb.AnyValue_StringValue{
				StringValue: "benchmark value",
			},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Actually validate the attribute value using the validator
		if err := validator.validateAttributeValueFast(attr.Value); err != nil {
			b.Fatal("validation failed:", err)
		}
	}
}

// Benchmark span validation inline performance
func BenchmarkSpanValidation_Inline(b *testing.B) {
	validator := NewSchemaValidator(false)
	hotValidator := getValidator()
	defer returnValidator(hotValidator)

	span := &tracepb.Span{
		TraceId:           []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanId:            []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		Name:              "benchmark-span",
		StartTimeUnixNano: 1640995200000000000,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if err := validator.validateSpanInline(span, hotValidator); err != nil {
			b.Fatal(err)
		}
	}
}
