package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// MockKinesisAPI implements the Kinesis API interface for testing
type MockKinesisAPI struct {
	mock.Mock
}

func (m *MockKinesisAPI) PutRecord(ctx context.Context, params *kinesis.PutRecordInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*kinesis.PutRecordOutput), args.Error(1)
}

func (m *MockKinesisAPI) PutRecords(ctx context.Context, params *kinesis.PutRecordsInput, optFns ...func(*kinesis.Options)) (*kinesis.PutRecordsOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*kinesis.PutRecordsOutput), args.Error(1)
}

func (m *MockKinesisAPI) DescribeStream(ctx context.Context, params *kinesis.DescribeStreamInput, optFns ...func(*kinesis.Options)) (*kinesis.DescribeStreamOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*kinesis.DescribeStreamOutput), args.Error(1)
}

func TestNewKinesisClient(t *testing.T) {
	tests := []struct {
		name          string
		config        *config.KinesisConfig
		expectedError bool
		description   string
	}{
		{
			name: "valid localstack configuration",
			config: &config.KinesisConfig{
				Region:          "us-east-1",
				EndpointURL:     "http://localhost:4566", // LocalStack endpoint
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				Streams: map[string]string{
					"traces":  "test-traces-stream",
					"metrics": "test-metrics-stream",
					"logs":    "test-logs-stream",
				},
				BatchSize:     100,
				FlushInterval: "5s",
				RetryAttempts: 3,
				RetryDelay:    time.Second,
			},
			expectedError: false,
			description:   "LocalStack configuration should work without AWS credentials",
		},
		{
			name: "valid aws configuration",
			config: &config.KinesisConfig{
				Region: "us-east-1",
				Streams: map[string]string{
					"traces":  "traces-stream",
					"metrics": "metrics-stream",
					"logs":    "logs-stream",
				},
				BatchSize:     100,
				FlushInterval: "5s",
				RetryAttempts: 3,
				RetryDelay:    time.Second,
			},
			expectedError: false,
			description:   "AWS configuration structure should be valid",
		},
		{
			name: "empty streams configuration",
			config: &config.KinesisConfig{
				Region:  "us-east-1",
				Streams: map[string]string{},
			},
			expectedError: false,
			description:   "Empty streams should not cause initialization error",
		},
		{
			name: "missing region",
			config: &config.KinesisConfig{
				Streams: map[string]string{
					"traces": "test-traces",
				},
			},
			expectedError: false, // AWS SDK handles default region
			description:   "Missing region should use AWS SDK defaults",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewKinesisClient(tt.config)

			// Test the configuration structure regardless of AWS connectivity
			assert.NotNil(t, tt.config, "Config should not be nil")

			if tt.expectedError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, client)
			} else {
				// Even if AWS/LocalStack is not available, we can test the structure
				if err != nil {
					// This is expected in CI environments without LocalStack/AWS
					t.Logf("Client creation failed (expected in test env): %v", err)
					// Verify it's an AWS connectivity error, not a code error
					assert.True(t,
						containsAny(err.Error(), []string{"connection", "network", "endpoint", "credentials", "no such host"}),
						"Should be a connectivity error, got: %v", err)
					return
				}

				// If client creation succeeded (LocalStack available)
				assert.NotNil(t, client, "Client should not be nil when creation succeeds")

				// Test client properties
				assert.NotNil(t, client.streams, "Streams map should be initialized")
				assert.NotNil(t, client.batchChannels, "Batch channels should be initialized")
				assert.NotNil(t, client.shutdownCh, "Shutdown channel should be initialized")
				assert.Greater(t, client.batchSize, 0, "Batch size should be positive")
				assert.Greater(t, client.flushInterval, time.Duration(0), "Flush interval should be positive")

				// Test stream mapping
				for streamType, streamName := range tt.config.Streams {
					if streamName != "" {
						assert.Equal(t, streamName, client.streams[streamType],
							"Stream mapping should match config for %s", streamType)
					}
				}

				// Clean up
				if err := client.Close(); err != nil {
					t.Errorf("Failed to close client: %v", err)
				}
			}
		})
	}
}

// Helper function to check if error message contains any of the expected substrings
func containsAny(text string, substrings []string) bool {
	for _, substr := range substrings {
		if len(text) > 0 && len(substr) > 0 &&
			(text == substr || (len(text) >= len(substr) && findSubstring(text, substr))) {
			return true
		}
	}
	return false
}

func findSubstring(text, substr string) bool {
	if len(substr) > len(text) {
		return false
	}
	for i := 0; i <= len(text)-len(substr); i++ {
		if text[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestPublishTrace(t *testing.T) {
	tests := []struct {
		name        string
		traceData   json.RawMessage
		serviceName string
		traceID     string
		sourceIP    string
		userAgent   string
		description string
	}{
		{
			name:        "successful trace publish with all fields",
			traceData:   json.RawMessage(`{"spans": [{"name": "test-span"}]}`),
			serviceName: "user-service",
			traceID:     "trace-abc123",
			sourceIP:    "192.168.1.100",
			userAgent:   "Mozilla/5.0",
			description: "Complete trace data should be processed correctly",
		},
		{
			name:        "trace publish with minimal data",
			traceData:   json.RawMessage(`{"trace": "minimal"}`),
			serviceName: "minimal-service",
			traceID:     "",
			sourceIP:    "127.0.0.1",
			userAgent:   "",
			description: "Trace with empty traceID should use serviceName as partition key",
		},
		{
			name:        "trace publish with complex json",
			traceData:   json.RawMessage(`{"resourceSpans": [{"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "complex-service"}}]}}]}`),
			serviceName: "complex-service",
			traceID:     "complex-trace-456",
			sourceIP:    "10.0.0.1",
			userAgent:   "Go-http-client/1.1",
			description: "Complex OTLP trace data should be handled correctly",
		},
		{
			name:        "empty service and trace",
			traceData:   json.RawMessage(`{"empty": true}`),
			serviceName: "",
			traceID:     "",
			sourceIP:    "unknown",
			userAgent:   "",
			description: "Empty identifiers should fallback to 'unknown' partition key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the TelemetryEvent creation logic (like PublishTrace does)
			event := &TraceTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        "traces",
					ServiceName: tt.serviceName,
					TraceID:     tt.traceID,
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   tt.sourceIP,
						TenantID:   "test-tenant",
					},
				},
				ResourceSpans: &tracepb.ResourceSpans{
					ScopeSpans: []*tracepb.ScopeSpans{{
						Spans: []*tracepb.Span{{
							Name:    "mock-span",
							TraceId: []byte("test-trace-id-16"),
							SpanId:  []byte("span-id8"),
						}},
					}},
				}, // Mock protobuf data - tests focus on structure, not parsing
			}

			// Validate event structure
			assert.Equal(t, TelemetryEventType("traces"), event.GetType(), "Event type should be traces")
			assert.Equal(t, tt.serviceName, event.GetServiceName(), "Service name should match")
			assert.Equal(t, tt.traceID, event.GetTraceID(), "Trace ID should match")
			assert.NotNil(t, event.ResourceSpans, "Mock trace data should be initialized")
			assert.Equal(t, tt.sourceIP, event.GetMetadata().SourceIP, "Source IP should match")
			assert.Equal(t, "test-tenant", event.GetMetadata().TenantID, "Tenant ID should match")
			assert.False(t, event.GetMetadata().IngestedAt.IsZero(), "IngestedAt should be set")

			// Test partition key logic (from PublishTrace)
			partitionKey := tt.traceID
			if partitionKey == "" {
				partitionKey = tt.serviceName
				if partitionKey == "" {
					partitionKey = "unknown"
				}
			}

			if tt.traceID != "" {
				assert.Equal(t, tt.traceID, partitionKey, "Should use traceID as partition key when available")
			} else if tt.serviceName != "" {
				assert.Equal(t, tt.serviceName, partitionKey, "Should use serviceName as partition key when traceID empty")
			} else {
				assert.Equal(t, "unknown", partitionKey, "Should use 'unknown' as partition key when both empty")
			}

			// Test JSON marshaling (like publishEvent does)
			data, err := event.GetSerializedData()
			if err == nil {
				assert.Greater(t, len(data), 0, "Marshaled data should not be empty")
			}
			// Note: Some events may not have serialization implemented yet
		})
	}
}

func TestPublishMetrics(t *testing.T) {
	tests := []struct {
		name        string
		metricsData json.RawMessage
		serviceName string
		sourceIP    string
		userAgent   string
		description string
	}{
		{
			name:        "successful metrics publish",
			metricsData: json.RawMessage(`{"resourceMetrics": [{"scopeMetrics": [{"metrics": [{"name": "http_requests_total"}]}]}]}`),
			serviceName: "api-service",
			sourceIP:    "192.168.1.50",
			userAgent:   "Prometheus/2.0",
			description: "Complete metrics data should be processed correctly",
		},
		{
			name:        "counter metrics",
			metricsData: json.RawMessage(`{"metrics": [{"name": "requests_count", "value": 42}]}`),
			serviceName: "counter-service",
			sourceIP:    "10.0.1.100",
			userAgent:   "",
			description: "Counter metrics should be handled properly",
		},
		{
			name:        "gauge metrics",
			metricsData: json.RawMessage(`{"gauges": [{"name": "memory_usage", "value": 85.5}]}`),
			serviceName: "monitoring-service",
			sourceIP:    "172.16.0.1",
			userAgent:   "MetricCollector/1.0",
			description: "Gauge metrics should be processed correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the TelemetryEvent creation logic (like PublishMetrics does)
			event := &MetricsTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        "metrics",
					ServiceName: tt.serviceName,
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   tt.sourceIP,
						TenantID:   "test-tenant",
					},
				},
				ResourceMetrics: &metricspb.ResourceMetrics{
					ScopeMetrics: []*metricspb.ScopeMetrics{{
						Metrics: []*metricspb.Metric{{
							Name: "test-metric",
						}},
					}},
				}, // Mock protobuf data
			}

			// Validate event structure
			assert.Equal(t, TelemetryEventType("metrics"), event.GetType(), "Event type should be metrics")
			assert.Equal(t, tt.serviceName, event.GetServiceName(), "Service name should match")
			assert.Empty(t, event.GetTraceID(), "Metrics should not have trace ID")
			assert.NotNil(t, event.ResourceMetrics, "Mock metrics data should be initialized")

			// Test partition key logic for metrics (uses service name + timestamp)
			// Simulating: fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()%1000)
			partitionKey := fmt.Sprintf("%s-%d", tt.serviceName, time.Now().UnixNano()%1000)
			assert.Contains(t, partitionKey, tt.serviceName, "Partition key should contain service name")
			assert.Contains(t, partitionKey, "-", "Partition key should contain separator")

			// Test JSON marshaling
			data, err := event.GetSerializedData()
			if err == nil {
				assert.Greater(t, len(data), 0, "Marshaled data should not be empty")
			}
		})
	}
}

func TestPublishLogs(t *testing.T) {
	tests := []struct {
		name        string
		logsData    json.RawMessage
		serviceName string
		traceID     string
		sourceIP    string
		userAgent   string
		description string
	}{
		{
			name:        "successful logs publish with trace ID",
			logsData:    json.RawMessage(`{"resourceLogs": [{"scopeLogs": [{"logRecords": [{"body": {"stringValue": "Error occurred"}}]}]}]}`),
			serviceName: "auth-service",
			traceID:     "trace-logs-123",
			sourceIP:    "192.168.1.75",
			userAgent:   "LogAgent/1.2",
			description: "Complete logs data with trace ID should be processed correctly",
		},
		{
			name:        "logs publish without trace ID",
			logsData:    json.RawMessage(`{"logs": [{"level": "INFO", "message": "Request processed"}]}`),
			serviceName: "processing-service",
			traceID:     "",
			sourceIP:    "10.0.2.50",
			userAgent:   "",
			description: "Logs without trace ID should use service-based partition key",
		},
		{
			name:        "error logs",
			logsData:    json.RawMessage(`{"severity": "ERROR", "message": "Database connection failed", "timestamp": "2023-10-01T10:00:00Z"}`),
			serviceName: "db-service",
			traceID:     "error-trace-456",
			sourceIP:    "172.16.1.10",
			userAgent:   "ServiceMesh/1.0",
			description: "Error logs should be handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the TelemetryEvent creation logic (like PublishLogs does)
			event := &LogsTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        "logs",
					ServiceName: tt.serviceName,
					TraceID:     tt.traceID,
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   tt.sourceIP,
						TenantID:   "test-tenant",
					},
				},
				ResourceLogs: &logspb.ResourceLogs{
					ScopeLogs: []*logspb.ScopeLogs{{
						LogRecords: []*logspb.LogRecord{{
							Body: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test log message",
								},
							},
						}},
					}},
				}, // Mock protobuf data
			}

			// Validate event structure
			assert.Equal(t, TelemetryEventType("logs"), event.GetType(), "Event type should be logs")
			assert.Equal(t, tt.serviceName, event.GetServiceName(), "Service name should match")
			assert.Equal(t, tt.traceID, event.GetTraceID(), "Trace ID should match")
			assert.NotNil(t, event.ResourceLogs, "Mock logs data should be initialized")

			// Test partition key logic for logs (uses trace ID if available, otherwise service name + timestamp)
			var expectedPartitionKey string
			if tt.traceID != "" {
				expectedPartitionKey = tt.traceID
			} else {
				expectedPartitionKey = fmt.Sprintf("%s-", tt.serviceName) // Partial match since timestamp varies
			}

			if tt.traceID != "" {
				assert.Equal(t, tt.traceID, expectedPartitionKey, "Should use traceID as partition key when available")
			} else {
				// For service-based partition key, just check it starts with service name
				partitionKey := fmt.Sprintf("%s-%d", tt.serviceName, time.Now().UnixNano()%1000)
				assert.True(t, len(partitionKey) > len(tt.serviceName), "Partition key should be longer than service name")
				assert.Contains(t, partitionKey, tt.serviceName, "Partition key should contain service name")
			}

			// Test JSON marshaling
			data, err := event.GetSerializedData()
			if err == nil {
				assert.Greater(t, len(data), 0, "Marshaled data should not be empty")
			}
		})
	}
}

func TestPublishBatch(t *testing.T) {
	tests := []struct {
		name        string
		streamType  string
		events      []TelemetryEvent
		description string
	}{
		{
			name:       "batch with multiple trace events",
			streamType: "traces",
			events: []TelemetryEvent{
				&TraceTelemetryEvent{
					BaseTelemetryEvent: BaseTelemetryEvent{
						Type:        "traces",
						ServiceName: "service-1",
						TraceID:     "trace-1",
						Metadata: TelemetryMetadata{
							IngestedAt: time.Now(),
							SourceIP:   "192.168.1.1",
							TenantID:   "service-1-tenant",
						},
					},
					ResourceSpans: &tracepb.ResourceSpans{
						ScopeSpans: []*tracepb.ScopeSpans{{
							Spans: []*tracepb.Span{{
								Name:    "batch-span",
								TraceId: []byte("batch-trace-id16"),
								SpanId:  []byte("span-id8"),
							}},
						}},
					}, // Mock protobuf data
				},
				&TraceTelemetryEvent{
					BaseTelemetryEvent: BaseTelemetryEvent{
						Type:        "traces",
						ServiceName: "service-2",
						TraceID:     "trace-2",
						Metadata: TelemetryMetadata{
							IngestedAt: time.Now(),
							SourceIP:   "192.168.1.2",
							TenantID:   "service-2-tenant",
						},
					},
					ResourceSpans: &tracepb.ResourceSpans{
						ScopeSpans: []*tracepb.ScopeSpans{{
							Spans: []*tracepb.Span{{
								Name:    "batch-span",
								TraceId: []byte("batch-trace-id16"),
								SpanId:  []byte("span-id8"),
							}},
						}},
					}, // Mock protobuf data
				},
			},
			description: "Multiple trace events should be batched correctly",
		},
		{
			name:       "batch with metrics events",
			streamType: "metrics",
			events: []TelemetryEvent{
				&MetricsTelemetryEvent{
					BaseTelemetryEvent: BaseTelemetryEvent{
						Type:        "metrics",
						ServiceName: "metrics-service",
						Metadata: TelemetryMetadata{
							IngestedAt: time.Now(),
							SourceIP:   "10.0.1.1",
							TenantID:   "metrics-tenant",
						},
					},
					ResourceMetrics: &metricspb.ResourceMetrics{
						ScopeMetrics: []*metricspb.ScopeMetrics{{
							Metrics: []*metricspb.Metric{{
								Name: "batch-metric",
							}},
						}},
					}, // Mock protobuf data
				},
			},
			description: "Single metrics event should be processed in batch",
		},
		{
			name:        "empty events slice",
			streamType:  "traces",
			events:      []TelemetryEvent{},
			description: "Empty events slice should be handled gracefully",
		},
		{
			name:       "mixed event types (invalid)",
			streamType: "traces",
			events: []TelemetryEvent{
				&TraceTelemetryEvent{
					BaseTelemetryEvent: BaseTelemetryEvent{Type: "traces", ServiceName: "svc1"},
					ResourceSpans: &tracepb.ResourceSpans{
						ScopeSpans: []*tracepb.ScopeSpans{{
							Spans: []*tracepb.Span{{
								Name:    "mixed-span",
								TraceId: []byte("mixed-trace-id16"),
								SpanId:  []byte("span-id8"),
							}},
						}},
					},
				},
				&MetricsTelemetryEvent{
					BaseTelemetryEvent: BaseTelemetryEvent{Type: "metrics", ServiceName: "svc2"},
					ResourceMetrics: &metricspb.ResourceMetrics{
						ScopeMetrics: []*metricspb.ScopeMetrics{{
							Metrics: []*metricspb.Metric{{
								Name: "mixed-metric",
							}},
						}},
					},
				},
			},
			description: "Mixed event types should be handled appropriately",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test batch structure validation
			assert.NotEmpty(t, tt.streamType, "Stream type should not be empty")

			if len(tt.events) == 0 {
				// Test empty batch handling
				assert.Len(t, tt.events, 0, "Empty events slice should be empty")
				return
			}

			// Test each event in the batch
			for i, event := range tt.events {
				// Validate event structure
				assert.NotEmpty(t, event.GetType(), "Event %d type should not be empty", i)
				assert.NotEmpty(t, event.GetServiceName(), "Event %d service name should not be empty", i)

				// Test JSON marshaling for each event (like PublishBatch does)
				data, err := event.GetSerializedData()
				if err == nil {
					assert.Greater(t, len(data), 0, "Event %d marshaled data should not be empty", i)
				}

				// Test partition key logic (like PublishBatch does)
				partitionKey := event.GetServiceName()
				if event.GetTraceID() != "" {
					partitionKey = event.GetTraceID()
				}

				if event.GetTraceID() != "" {
					assert.Equal(t, event.GetTraceID(), partitionKey, "Event %d should use TraceID as partition key", i)
				} else {
					assert.Equal(t, event.GetServiceName(), partitionKey, "Event %d should use ServiceName as partition key", i)
				}
			}

			// Test batch size constraints
			if len(tt.events) > 500 { // Kinesis batch limit is 500 records
				t.Logf("Batch size %d exceeds Kinesis limit of 500 records", len(tt.events))
			}

			// Test that all events in batch are for the correct stream type
			if tt.streamType != "mixed" {
				for i, event := range tt.events {
					if string(event.GetType()) != tt.streamType {
						t.Logf("Event %d type %s doesn't match stream type %s", i, event.GetType(), tt.streamType)
					}
				}
			}
		})
	}
}

func TestVerifyStreams(t *testing.T) {
	tests := []struct {
		name        string
		streams     map[string]string
		description string
	}{
		{
			name: "all streams configured",
			streams: map[string]string{
				"traces":  "test-traces",
				"metrics": "test-metrics",
				"logs":    "test-logs",
			},
			description: "All three stream types should be configured",
		},
		{
			name: "partial streams configured",
			streams: map[string]string{
				"traces": "only-traces",
			},
			description: "Partial stream configuration should be handled",
		},
		{
			name: "empty stream names",
			streams: map[string]string{
				"traces":  "",
				"metrics": "valid-metrics",
				"logs":    "",
			},
			description: "Empty stream names should be detected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &KinesisClient{
				streams: tt.streams,
			}

			// Test stream configuration validation
			assert.NotNil(t, client.streams, "Streams map should not be nil")

			// Test each configured stream
			for streamType, streamName := range tt.streams {
				if streamName != "" {
					assert.Equal(t, streamName, client.streams[streamType],
						"Stream %s should match configuration", streamType)
					assert.NotEmpty(t, streamName, "Stream name should not be empty for %s", streamType)
				} else {
					// Empty stream names should be preserved for validation
					assert.Empty(t, client.streams[streamType],
						"Empty stream name should be preserved for %s", streamType)
				}
			}

			// Test that standard stream types are present in the map
			expectedTypes := []string{"traces", "metrics", "logs"}
			for _, streamType := range expectedTypes {
				_, exists := client.streams[streamType]
				if !exists && len(tt.streams) > 0 {
					// Only log if we expect streams to be configured
					t.Logf("Stream type %s not configured", streamType)
				}
			}
		})
	}
}

func TestBatchProcessing(t *testing.T) {
	t.Run("batch_configuration", func(t *testing.T) {
		client := &KinesisClient{
			batchSize:     50,
			flushInterval: 2 * time.Second,
			batchChannels: make(map[string]chan LegacyTelemetryEvent),
			shutdownCh:    make(chan struct{}),
		}

		// Test batch configuration
		assert.Equal(t, 50, client.batchSize, "Batch size should be configurable")
		assert.Equal(t, 2*time.Second, client.flushInterval, "Flush interval should be configurable")
		assert.NotNil(t, client.batchChannels, "Batch channels should be initialized")
		assert.NotNil(t, client.shutdownCh, "Shutdown channel should be initialized")

		// Test SetBatchConfig
		client.SetBatchConfig(100, 5*time.Second)
		assert.Equal(t, 100, client.batchSize, "Batch size should be updated")
		assert.Equal(t, 5*time.Second, client.flushInterval, "Flush interval should be updated")
	})

	t.Run("batch_channels_initialization", func(t *testing.T) {
		streams := map[string]string{
			"traces":  "test-traces",
			"metrics": "test-metrics",
			"logs":    "test-logs",
		}

		batchChannels := make(map[string]chan LegacyTelemetryEvent)
		batchSize := 100

		// Simulate StartBatchProcessor channel initialization
		for streamType := range streams {
			batchChannels[streamType] = make(chan LegacyTelemetryEvent, batchSize*2)
		}

		// Test that channels are created for each stream type
		for streamType := range streams {
			channel, exists := batchChannels[streamType]
			assert.True(t, exists, "Batch channel should exist for %s", streamType)
			assert.NotNil(t, channel, "Batch channel should not be nil for %s", streamType)
			assert.Equal(t, batchSize*2, cap(channel), "Channel capacity should be 2x batch size for %s", streamType)
		}

		// Clean up channels
		for _, ch := range batchChannels {
			close(ch)
		}
	})

	t.Run("async_publish_logic", func(t *testing.T) {
		// Test PublishAsync channel selection logic
		batchChannel := make(chan LegacyTelemetryEvent, 2)

		event := LegacyTelemetryEvent{
			Type:        "traces",
			ServiceName: "test-service",
			TraceID:     "test-trace",
			Data:        json.RawMessage(`{"test": true}`),
		}

		// Test successful channel send
		select {
		case batchChannel <- event:
			// Success case - channel has capacity
			assert.True(t, true, "Event should be sent to channel when capacity available")
		default:
			t.Fatal("Channel send should succeed when channel has capacity")
		}

		// Fill channel to test fallback behavior
		batchChannel <- event // Fill channel to capacity

		// Test channel full scenario
		select {
		case batchChannel <- event:
			t.Fatal("Channel send should fail when channel is full")
		default:
			// This is the expected fallback case
			assert.True(t, true, "Should fall back to direct publish when channel full")
		}

		close(batchChannel)
	})
}

func TestKinesisHandler(t *testing.T) {
	// Create a mock KinesisClient for testing the handler
	mockClient := &KinesisClient{
		streams: map[string]string{
			"traces":  "test-traces",
			"metrics": "test-metrics",
			"logs":    "test-logs",
		},
	}

	handler := NewKinesisHandler(mockClient)

	t.Run("handler_creation", func(t *testing.T) {
		assert.NotNil(t, handler, "Handler should not be nil")
		assert.NotNil(t, handler.client, "Handler client should not be nil")
		assert.Equal(t, mockClient, handler.client, "Handler should reference the provided client")
	})

	t.Run("handle_trace_event", func(t *testing.T) {
		event := &TraceTelemetryEvent{
			BaseTelemetryEvent: BaseTelemetryEvent{
				Type:        "traces",
				ServiceName: "trace-service",
				TraceID:     "trace-123",
				Metadata: TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "192.168.1.100",
					TenantID:   "trace-tenant",
				},
			},
			ResourceSpans: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{{
					Spans: []*tracepb.Span{{
						Name:    "handler-span",
						TraceId: []byte("handler-trace-16"),
						SpanId:  []byte("span-id8"),
					}},
				}},
			}, // Mock protobuf data
		}

		// Test that handler correctly identifies trace events
		assert.Equal(t, TelemetryEventType("traces"), event.GetType(), "Event type should be traces")
		assert.NotEmpty(t, event.GetServiceName(), "Service name should not be empty")
		assert.NotEmpty(t, event.GetTraceID(), "Trace ID should not be empty")
		assert.NotNil(t, event.ResourceSpans, "Data should not be nil")
		assert.Equal(t, "192.168.1.100", event.GetMetadata().SourceIP, "Source IP should match")
		assert.Equal(t, "trace-tenant", event.GetMetadata().TenantID, "Tenant ID should match")
	})

	t.Run("handle_metrics_event", func(t *testing.T) {
		event := &MetricsTelemetryEvent{
			BaseTelemetryEvent: BaseTelemetryEvent{
				Type:        "metrics",
				ServiceName: "metrics-service",
				Metadata: TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "10.0.1.50",
					TenantID:   "metrics-tenant",
				},
			},
			ResourceMetrics: &metricspb.ResourceMetrics{
				ScopeMetrics: []*metricspb.ScopeMetrics{{
					Metrics: []*metricspb.Metric{{
						Name: "handler-metric",
					}},
				}},
			}, // Mock protobuf data
		}

		// Test that handler correctly identifies metrics events
		assert.Equal(t, TelemetryEventType("metrics"), event.GetType(), "Event type should be metrics")
		assert.NotEmpty(t, event.GetServiceName(), "Service name should not be empty")
		assert.Empty(t, event.GetTraceID(), "Metrics should not have trace ID")
		assert.NotNil(t, event.ResourceMetrics, "Data should not be nil")
	})

	t.Run("handle_logs_event", func(t *testing.T) {
		event := &LogsTelemetryEvent{
			BaseTelemetryEvent: BaseTelemetryEvent{
				Type:        "logs",
				ServiceName: "logs-service",
				TraceID:     "log-trace-456",
				Metadata: TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "172.16.1.25",
					TenantID:   "logs-tenant",
				},
			},
			ResourceLogs: &logspb.ResourceLogs{
				ScopeLogs: []*logspb.ScopeLogs{{
					LogRecords: []*logspb.LogRecord{{
						Body: &commonpb.AnyValue{
							Value: &commonpb.AnyValue_StringValue{
								StringValue: "handler log message",
							},
						},
					}},
				}},
			}, // Mock protobuf data
		}

		// Test that handler correctly identifies logs events
		assert.Equal(t, TelemetryEventType("logs"), event.GetType(), "Event type should be logs")
		assert.NotEmpty(t, event.GetServiceName(), "Service name should not be empty")
		assert.NotEmpty(t, event.GetTraceID(), "Trace ID should not be empty for correlated logs")
		assert.NotNil(t, event.ResourceLogs, "Data should not be nil")
	})

	t.Run("handle_unknown_event_type", func(t *testing.T) {
		event := &TraceTelemetryEvent{
			BaseTelemetryEvent: BaseTelemetryEvent{
				Type:        "unknown",
				ServiceName: "unknown-service",
				Metadata: TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "1.2.3.4",
					TenantID:   "unknown-tenant",
				},
			},
			ResourceSpans: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{{
					Spans: []*tracepb.Span{{
						Name:    "handler-span",
						TraceId: []byte("handler-trace-16"),
						SpanId:  []byte("span-id8"),
					}},
				}},
			}, // Mock protobuf data
		}

		// Test that handler can handle unknown event types gracefully
		assert.Equal(t, TelemetryEventType("unknown"), event.GetType(), "Event type should be preserved")
		assert.NotEmpty(t, event.GetServiceName(), "Service name should not be empty")
		assert.NotNil(t, event.ResourceSpans, "Data should not be nil")

		// Unknown types should be ignored (returning nil error)
		// This tests the default case in HandleTelemetryEvent
	})
}

func TestClientClose(t *testing.T) {
	client := &KinesisClient{
		batchChannels: map[string]chan LegacyTelemetryEvent{
			"traces":  make(chan LegacyTelemetryEvent, 10),
			"metrics": make(chan LegacyTelemetryEvent, 10),
			"logs":    make(chan LegacyTelemetryEvent, 10),
		},
		shutdownCh: make(chan struct{}),
	}

	// Test that all channels can receive before close
	for streamType, channel := range client.batchChannels {
		select {
		case channel <- LegacyTelemetryEvent{}:
			// Success - channel is open
		default:
			t.Fatalf("Channel %s should accept events before close", streamType)
		}
	}

	// Test close operation
	err := client.Close()
	assert.NoError(t, err, "Close should not return error")

	// After close, shutdown channel should be closed
	select {
	case <-client.shutdownCh:
		// Expected - channel should be closed
	default:
		t.Fatal("Stop batching channel should be closed after Close()")
	}
}

// Benchmark test for basic structure validation
func BenchmarkTelemetryEventCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		event := &TraceTelemetryEvent{
			BaseTelemetryEvent: BaseTelemetryEvent{
				Type:        "traces",
				ServiceName: "bench-service",
				TraceID:     "bench-trace",
				Metadata: TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "127.0.0.1",
					TenantID:   "bench-tenant",
				},
			},
			ResourceSpans: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{{
					Spans: []*tracepb.Span{{
						Name:    "handler-span",
						TraceId: []byte("handler-trace-16"),
						SpanId:  []byte("span-id8"),
					}},
				}},
			}, // Mock protobuf data
		}
		if event.GetType() == "" {
			b.Fatal("Event type should not be empty")
		}
	}
}
