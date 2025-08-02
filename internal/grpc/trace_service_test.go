package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestNewTraceService(t *testing.T) {
	service := NewTraceService(nil, nil)

	assert.NotNil(t, service)
}

func TestTraceService_Export_Basic(t *testing.T) {
	tests := []struct {
		name              string
		request           *tracecollectorpb.ExportTraceServiceRequest
		expectedError     bool
		expectedSpanCount int
	}{
		{
			name: "successful export with single span",
			request: &tracecollectorpb.ExportTraceServiceRequest{
				ResourceSpans: []*tracepb.ResourceSpans{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeSpans: []*tracepb.ScopeSpans{
							{
								Spans: []*tracepb.Span{
									{
										TraceId:           []byte("test-trace-id-123456789012"),
										SpanId:            []byte("test-span-id"),
										Name:              "test-operation",
										Kind:              tracepb.Span_SPAN_KIND_SERVER,
										StartTimeUnixNano: 1640995200000000000,
										EndTimeUnixNano:   1640995201000000000,
										Status: &tracepb.Status{
											Code: tracepb.Status_STATUS_CODE_OK,
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError:     false,
			expectedSpanCount: 1,
		},
		{
			name: "multiple spans with different trace IDs",
			request: &tracecollectorpb.ExportTraceServiceRequest{
				ResourceSpans: []*tracepb.ResourceSpans{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "multi-trace-service"}},
								},
							},
						},
						ScopeSpans: []*tracepb.ScopeSpans{
							{
								Spans: []*tracepb.Span{
									{
										TraceId: []byte("trace-id-1-123456789012"),
										SpanId:  []byte("span-1"),
										Name:    "operation-1",
										Kind:    tracepb.Span_SPAN_KIND_CLIENT,
									},
									{
										TraceId: []byte("trace-id-2-123456789012"),
										SpanId:  []byte("span-2"),
										Name:    "operation-2",
										Kind:    tracepb.Span_SPAN_KIND_SERVER,
									},
									{
										TraceId: []byte("trace-id-1-123456789012"),
										SpanId:  []byte("span-3"),
										Name:    "operation-3",
										Kind:    tracepb.Span_SPAN_KIND_INTERNAL,
									},
								},
							},
						},
					},
				},
			},
			expectedError:     false,
			expectedSpanCount: 3,
		},
		{
			name: "empty request",
			request: &tracecollectorpb.ExportTraceServiceRequest{
				ResourceSpans: []*tracepb.ResourceSpans{},
			},
			expectedError:     false,
			expectedSpanCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewTraceService(nil, nil)

			response, err := service.Export(context.Background(), tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, int64(0), response.PartialSuccess.RejectedSpans)
			}
		})
	}
}

func TestExtractTraceID(t *testing.T) {
	tests := []struct {
		name     string
		resource *tracepb.ResourceSpans
		expected string
	}{
		{
			name: "trace ID present",
			resource: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId: []byte("my-trace-id-123456789012"),
								SpanId:  []byte("span-1"),
							},
						},
					},
				},
			},
			expected: "my-trace-id-123456789012",
		},
		{
			name: "no spans",
			resource: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{},
					},
				},
			},
			expected: "unknown",
		},
		{
			name: "no scope spans",
			resource: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{},
			},
			expected: "unknown",
		},
		{
			name:     "nil resource spans",
			resource: &tracepb.ResourceSpans{},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTraceID(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		name     string
		resource *tracepb.ResourceSpans
		expected string
	}{
		{
			name: "service name present",
			resource: &tracepb.ResourceSpans{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "my-trace-service"}},
						},
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "1.0.0"}},
						},
					},
				},
			},
			expected: "my-trace-service",
		},
		{
			name: "no service name attribute",
			resource: &tracepb.ResourceSpans{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "1.0.0"}},
						},
					},
				},
			},
			expected: "unknown",
		},
		{
			name: "empty service name",
			resource: &tracepb.ResourceSpans{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: ""}},
						},
					},
				},
			},
			expected: "unknown",
		},
		{
			name: "nil resource",
			resource: &tracepb.ResourceSpans{
				Resource: nil,
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractServiceName(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountSpans(t *testing.T) {
	tests := []struct {
		name          string
		resourceSpans []*tracepb.ResourceSpans
		expected      int
	}{
		{
			name: "single resource with multiple spans",
			resourceSpans: []*tracepb.ResourceSpans{
				{
					ScopeSpans: []*tracepb.ScopeSpans{
						{
							Spans: []*tracepb.Span{
								{TraceId: []byte("trace-1"), SpanId: []byte("span-1")},
								{TraceId: []byte("trace-1"), SpanId: []byte("span-2")},
								{TraceId: []byte("trace-1"), SpanId: []byte("span-3")},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "multiple resources with spans",
			resourceSpans: []*tracepb.ResourceSpans{
				{
					ScopeSpans: []*tracepb.ScopeSpans{
						{
							Spans: []*tracepb.Span{
								{TraceId: []byte("trace-1"), SpanId: []byte("span-1")},
								{TraceId: []byte("trace-1"), SpanId: []byte("span-2")},
							},
						},
					},
				},
				{
					ScopeSpans: []*tracepb.ScopeSpans{
						{
							Spans: []*tracepb.Span{
								{TraceId: []byte("trace-2"), SpanId: []byte("span-3")},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "empty resource spans",
			resourceSpans: []*tracepb.ResourceSpans{
				{
					ScopeSpans: []*tracepb.ScopeSpans{
						{
							Spans: []*tracepb.Span{},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name:          "nil resource spans",
			resourceSpans: nil,
			expected:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countSpans(tt.resourceSpans)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Integration test that validates the overall trace service structure
func TestTraceService_Integration(t *testing.T) {
	service := NewTraceService(nil, nil)

	// Create a realistic trace request with detailed span information
	request := &tracecollectorpb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "integration-trace-service"}},
						},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "integration-test",
							Version: "1.0.0",
						},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("integration-trace-123456789012"),
								SpanId:            []byte("integration-span-456"),
								Name:              "integration-operation",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: 1640995200000000000,
								EndTimeUnixNano:   1640995201000000000,
								Status: &tracepb.Status{
									Code:    tracepb.Status_STATUS_CODE_OK,
									Message: "Operation completed successfully",
								},
								Attributes: []*commonpb.KeyValue{
									{
										Key:   "http.method",
										Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}},
									},
									{
										Key:   "http.status_code",
										Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 200}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Execute the test
	response, err := service.Export(context.Background(), request)

	// Should succeed with nil handlers
	assert.NoError(t, err)
	assert.NotNil(t, response)
	if response.PartialSuccess != nil {
		assert.Equal(t, int64(0), response.PartialSuccess.RejectedSpans)
	}
}

func TestTraceService_SpanKinds(t *testing.T) {
	service := NewTraceService(nil, nil)

	// Test different span kinds
	spanKinds := []struct {
		kind tracepb.Span_SpanKind
		name string
	}{
		{tracepb.Span_SPAN_KIND_UNSPECIFIED, "UNSPECIFIED"},
		{tracepb.Span_SPAN_KIND_INTERNAL, "INTERNAL"},
		{tracepb.Span_SPAN_KIND_SERVER, "SERVER"},
		{tracepb.Span_SPAN_KIND_CLIENT, "CLIENT"},
		{tracepb.Span_SPAN_KIND_PRODUCER, "PRODUCER"},
		{tracepb.Span_SPAN_KIND_CONSUMER, "CONSUMER"},
	}

	for _, spanKind := range spanKinds {
		t.Run(spanKind.name, func(t *testing.T) {
			request := &tracecollectorpb.ExportTraceServiceRequest{
				ResourceSpans: []*tracepb.ResourceSpans{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeSpans: []*tracepb.ScopeSpans{
							{
								Spans: []*tracepb.Span{
									{
										TraceId:           []byte("test-trace-123456789012"),
										SpanId:            []byte("test-span"),
										Name:              spanKind.name + " span test",
										Kind:              spanKind.kind,
										StartTimeUnixNano: 1640995200000000000,
										EndTimeUnixNano:   1640995201000000000,
									},
								},
							},
						},
					},
				},
			}

			response, err := service.Export(context.Background(), request)
			assert.NoError(t, err)
			assert.NotNil(t, response)
		})
	}
}

func TestTraceService_SpanStatuses(t *testing.T) {
	service := NewTraceService(nil, nil)

	// Test different span status codes
	statusCodes := []struct {
		code tracepb.Status_StatusCode
		name string
	}{
		{tracepb.Status_STATUS_CODE_UNSET, "UNSET"},
		{tracepb.Status_STATUS_CODE_OK, "OK"},
		{tracepb.Status_STATUS_CODE_ERROR, "ERROR"},
	}

	for _, status := range statusCodes {
		t.Run(status.name, func(t *testing.T) {
			request := &tracecollectorpb.ExportTraceServiceRequest{
				ResourceSpans: []*tracepb.ResourceSpans{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeSpans: []*tracepb.ScopeSpans{
							{
								Spans: []*tracepb.Span{
									{
										TraceId:           []byte("test-trace-123456789012"),
										SpanId:            []byte("test-span"),
										Name:              status.name + " span test",
										StartTimeUnixNano: 1640995200000000000,
										EndTimeUnixNano:   1640995201000000000,
										Status: &tracepb.Status{
											Code:    status.code,
											Message: status.name + " status message",
										},
									},
								},
							},
						},
					},
				},
			}

			response, err := service.Export(context.Background(), request)
			assert.NoError(t, err)
			assert.NotNil(t, response)
		})
	}
}

// Benchmark test for trace service performance
func BenchmarkTraceService_Export(b *testing.B) {
	service := NewTraceService(nil, nil)
	
	request := &tracecollectorpb.ExportTraceServiceRequest{
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
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte("benchmark-trace-123456789012"),
								SpanId:            []byte("benchmark-span"),
								Name:              "benchmark-operation",
								StartTimeUnixNano: 1640995200000000000,
								EndTimeUnixNano:   1640995201000000000,
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Export(context.Background(), request)
		if err != nil {
			b.Fatal(err)
		}
	}
}