package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestNewLogsService(t *testing.T) {
	service := NewLogsService(nil, nil)

	assert.NotNil(t, service)
}

func TestLogsService_Export_Basic(t *testing.T) {
	tests := []struct {
		name              string
		request           *logscollectorpb.ExportLogsServiceRequest
		expectedError     bool
		expectedLogCount  int
	}{
		{
			name: "successful export with single log record",
			request: &logscollectorpb.ExportLogsServiceRequest{
				ResourceLogs: []*logspb.ResourceLogs{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeLogs: []*logspb.ScopeLogs{
							{
								LogRecords: []*logspb.LogRecord{
									{
										TimeUnixNano:     1640995200000000000,
										SeverityNumber:   logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
										SeverityText:     "INFO",
										Body:             &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Test log message"}},
										TraceId:          []byte("test-trace-id-123"),
										SpanId:           []byte("test-span-id"),
									},
								},
							},
						},
					},
				},
			},
			expectedError:    false,
			expectedLogCount: 1,
		},
		{
			name: "multiple log records",
			request: &logscollectorpb.ExportLogsServiceRequest{
				ResourceLogs: []*logspb.ResourceLogs{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "multi-log-service"}},
								},
							},
						},
						ScopeLogs: []*logspb.ScopeLogs{
							{
								LogRecords: []*logspb.LogRecord{
									{
										TimeUnixNano:   1640995200000000000,
										SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
										Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Log message 1"}},
									},
									{
										TimeUnixNano:   1640995201000000000,
										SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
										Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Log message 2"}},
									},
									{
										TimeUnixNano:   1640995202000000000,
										SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_WARN,
										Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Log message 3"}},
									},
								},
							},
						},
					},
				},
			},
			expectedError:    false,
			expectedLogCount: 3,
		},
		{
			name: "empty request",
			request: &logscollectorpb.ExportLogsServiceRequest{
				ResourceLogs: []*logspb.ResourceLogs{},
			},
			expectedError:    false,
			expectedLogCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewLogsService(nil, nil)

			response, err := service.Export(context.Background(), tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, int64(0), response.PartialSuccess.RejectedLogRecords)
			}
		})
	}
}

func TestExtractServiceNameFromLogs(t *testing.T) {
	tests := []struct {
		name     string
		resource *logspb.ResourceLogs
		expected string
	}{
		{
			name: "service name present",
			resource: &logspb.ResourceLogs{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "my-logs-service"}},
						},
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "1.0.0"}},
						},
					},
				},
			},
			expected: "my-logs-service",
		},
		{
			name: "no service name attribute",
			resource: &logspb.ResourceLogs{
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
			resource: &logspb.ResourceLogs{
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
			resource: &logspb.ResourceLogs{
				Resource: nil,
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractServiceNameFromLogs(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountLogRecords(t *testing.T) {
	tests := []struct {
		name         string
		resourceLogs []*logspb.ResourceLogs
		expected     int
	}{
		{
			name: "single resource with multiple log records",
			resourceLogs: []*logspb.ResourceLogs{
				{
					ScopeLogs: []*logspb.ScopeLogs{
						{
							LogRecords: []*logspb.LogRecord{
								{Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "log1"}}},
								{Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "log2"}}},
								{Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "log3"}}},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "multiple resources with log records",
			resourceLogs: []*logspb.ResourceLogs{
				{
					ScopeLogs: []*logspb.ScopeLogs{
						{
							LogRecords: []*logspb.LogRecord{
								{Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "log1"}}},
								{Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "log2"}}},
							},
						},
					},
				},
				{
					ScopeLogs: []*logspb.ScopeLogs{
						{
							LogRecords: []*logspb.LogRecord{
								{Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "log3"}}},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "empty resource logs",
			resourceLogs: []*logspb.ResourceLogs{
				{
					ScopeLogs: []*logspb.ScopeLogs{
						{
							LogRecords: []*logspb.LogRecord{},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name:         "nil resource logs",
			resourceLogs: nil,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countLogRecords(tt.resourceLogs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertResourceLogToMap(t *testing.T) {
	resourceLog := &logspb.ResourceLogs{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
				},
			},
		},
		ScopeLogs: []*logspb.ScopeLogs{
			{
				LogRecords: []*logspb.LogRecord{
					{
						Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test log"}},
					},
				},
			},
		},
	}

	result := convertResourceLogToMap(resourceLog)

	// Verify the structure
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")
	
	resourceLogs, exists := resultMap["resourceLogs"]
	assert.True(t, exists, "Should contain resourceLogs key")
	
	resourceLogsSlice, ok := resourceLogs.([]interface{})
	assert.True(t, ok, "resourceLogs should be a slice")
	assert.Len(t, resourceLogsSlice, 1, "Should contain one resource log")
}

// Integration test that validates the overall logs service structure
func TestLogsService_Integration(t *testing.T) {
	service := NewLogsService(nil, nil)

	// Create a realistic logs request
	request := &logscollectorpb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "integration-logs-service"}},
						},
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "integration-test",
							Version: "1.0.0",
						},
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:     1640995200000000000,
								SeverityNumber:   logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
								SeverityText:     "INFO",
								Body:             &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Integration test log message"}},
								TraceId:          []byte("integration-trace-123"),
								SpanId:           []byte("integration-span-456"),
								Attributes: []*commonpb.KeyValue{
									{
										Key:   "test.type",
										Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "integration"}},
									},
								},
							},
							{
								TimeUnixNano:     1640995201000000000,
								SeverityNumber:   logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
								SeverityText:     "ERROR",
								Body:             &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Integration test error message"}},
								TraceId:          []byte("integration-trace-123"),
								SpanId:           []byte("integration-span-789"),
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
		assert.Equal(t, int64(0), response.PartialSuccess.RejectedLogRecords)
	}
}

func TestLogsService_LogLevels(t *testing.T) {
	service := NewLogsService(nil, nil)

	// Test different log severity levels
	severityLevels := []struct {
		level logspb.SeverityNumber
		text  string
	}{
		{logspb.SeverityNumber_SEVERITY_NUMBER_DEBUG, "DEBUG"},
		{logspb.SeverityNumber_SEVERITY_NUMBER_INFO, "INFO"},
		{logspb.SeverityNumber_SEVERITY_NUMBER_WARN, "WARN"},
		{logspb.SeverityNumber_SEVERITY_NUMBER_ERROR, "ERROR"},
		{logspb.SeverityNumber_SEVERITY_NUMBER_FATAL, "FATAL"},
	}

	for _, severity := range severityLevels {
		t.Run(severity.text, func(t *testing.T) {
			request := &logscollectorpb.ExportLogsServiceRequest{
				ResourceLogs: []*logspb.ResourceLogs{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeLogs: []*logspb.ScopeLogs{
							{
								LogRecords: []*logspb.LogRecord{
									{
										TimeUnixNano:   1640995200000000000,
										SeverityNumber: severity.level,
										SeverityText:   severity.text,
										Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: severity.text + " level log message"}},
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

// Benchmark test for logs service performance
func BenchmarkLogsService_Export(b *testing.B) {
	service := NewLogsService(nil, nil)
	
	request := &logscollectorpb.ExportLogsServiceRequest{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "benchmark-service"}},
						},
					},
				},
				ScopeLogs: []*logspb.ScopeLogs{
					{
						LogRecords: []*logspb.LogRecord{
							{
								TimeUnixNano:   1640995200000000000,
								SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
								Body:           &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "Benchmark log message"}},
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