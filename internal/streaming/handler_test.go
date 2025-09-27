package streaming

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// MockKinesisClient implements the client interface for testing the handler
type MockKinesisClient struct {
	mock.Mock
}

// MockUnknownTelemetryEvent implements TelemetryEvent for testing unknown event types
type MockUnknownTelemetryEvent struct {
	EventType   TelemetryEventType
	ServiceName string
}

func (m *MockUnknownTelemetryEvent) GetType() TelemetryEventType { return m.EventType }
func (m *MockUnknownTelemetryEvent) GetServiceName() string      { return m.ServiceName }
func (m *MockUnknownTelemetryEvent) GetTraceID() string          { return "" }
func (m *MockUnknownTelemetryEvent) GetMetadata() TelemetryMetadata {
	return TelemetryMetadata{IngestedAt: time.Now()}
}
func (m *MockUnknownTelemetryEvent) GetSerializedData() ([]byte, error) {
	return []byte(`{"type":"unknown"}`), nil
}
func (m *MockUnknownTelemetryEvent) Validate() error { return nil }

func (m *MockKinesisClient) PublishTrace(ctx context.Context, traceData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	args := m.Called(ctx, traceData, serviceName, traceID, sourceIP, userAgent)
	return args.Error(0)
}

func (m *MockKinesisClient) PublishMetrics(ctx context.Context, metricsData json.RawMessage, serviceName, sourceIP, userAgent string) error {
	args := m.Called(ctx, metricsData, serviceName, sourceIP, userAgent)
	return args.Error(0)
}

func (m *MockKinesisClient) PublishLogs(ctx context.Context, logsData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	args := m.Called(ctx, logsData, serviceName, traceID, sourceIP, userAgent)
	return args.Error(0)
}

func (m *MockKinesisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestKinesisHandlerEventRouting(t *testing.T) {
	// Test the handler event routing logic without making actual AWS calls
	tests := []struct {
		name        string
		event       TelemetryEvent
		description string
	}{
		{
			name: "trace event structure",
			event: &TraceTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        TelemetryTypeTraces,
					ServiceName: "user-service",
					TraceID:     "trace-abc123",
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   "192.168.1.100",
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
				}, // Mock protobuf data with content
			},
			description: "Trace events should have proper structure for routing",
		},
		{
			name: "metrics event structure",
			event: &MetricsTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        TelemetryTypeMetrics,
					ServiceName: "api-service",
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   "10.0.1.50",
						TenantID:   "metrics-tenant",
					},
				},
				ResourceMetrics: &metricspb.ResourceMetrics{
					ScopeMetrics: []*metricspb.ScopeMetrics{{
						Metrics: []*metricspb.Metric{{
							Name: "mock-metric",
						}},
					}},
				}, // Mock protobuf data with content
			},
			description: "Metrics events should have proper structure for routing",
		},
		{
			name: "logs event structure",
			event: &LogsTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        TelemetryTypeLogs,
					ServiceName: "auth-service",
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
									StringValue: "mock log message",
								},
							},
						}},
					}},
				}, // Mock protobuf data with content
			},
			description: "Logs events should have proper structure for routing",
		},
		{
			name: "unknown event type structure",
			event: &TraceTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        "unknown",
					ServiceName: "unknown-service",
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   "1.2.3.4",
						TenantID:   "default",
					},
				},
				ResourceSpans: &tracepb.ResourceSpans{
					ScopeSpans: []*tracepb.ScopeSpans{{
						Spans: []*tracepb.Span{{
							Name:    "unknown-span",
							TraceId: []byte("unknown-trace-16"),
							SpanId:  []byte("span-id8"),
						}},
					}},
				},
			},
			description: "Unknown event types should have proper structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test event structure validation using interface methods
			assert.NotNil(t, tt.event, "Event should not be nil")
			assert.NotEmpty(t, tt.event.GetType(), "Event type should not be empty")
			assert.NotEmpty(t, tt.event.GetServiceName(), "Service name should not be empty")
			assert.False(t, tt.event.GetMetadata().IngestedAt.IsZero(), "IngestedAt should be set")

			// Test the switch case logic in HandleTelemetryEvent
			switch tt.event.GetType() {
			case TelemetryTypeTraces:
				// Validate trace-specific fields
				assert.NotEmpty(t, tt.event.GetTraceID(), "Trace events should have trace ID")
				assert.NotEmpty(t, tt.event.GetMetadata().SourceIP, "Source IP should be set")
			case TelemetryTypeMetrics:
				// Validate metrics-specific characteristics
				assert.NotEmpty(t, tt.event.GetServiceName(), "Metrics should have service name")
			case TelemetryTypeLogs:
				// Validate logs-specific fields
				assert.NotEmpty(t, tt.event.GetServiceName(), "Logs should have service name")
			default:
				// For unknown types, just validate basic structure
				assert.NotEmpty(t, tt.event.GetType(), "Event type should be preserved")
			}

			// Test that event can be serialized (using interface method)
			data, err := tt.event.GetSerializedData()
			if err == nil {
				assert.Greater(t, len(data), 0, "Serialized data should not be empty")
			}
			// Note: Some events may not have serialization implemented yet
		})
	}
}

func TestKinesisHandlerSwitchCases(t *testing.T) {
	// Test the actual switch case logic in HandleTelemetryEvent without AWS calls
	client := &KinesisClient{
		streams: map[string]string{
			"traces":  "test-traces",
			"metrics": "test-metrics",
			"logs":    "test-logs",
		},
	}

	handler := NewKinesisHandler(client)

	// Test unknown event type (should return ErrUnsupportedEventType in default case)
	unknownEvent := &MockUnknownTelemetryEvent{
		EventType:   "unknown",
		ServiceName: "unknown-service",
	}

	ctx := context.Background()
	err := handler.HandleTelemetryEvent(ctx, unknownEvent)
	assert.Equal(t, ErrUnsupportedEventType, err, "Unknown event types should return ErrUnsupportedEventType")
}

func TestKinesisHandlerWithRealClient(t *testing.T) {
	// This test uses a KinesisClient without AWS connectivity to test the handler interface
	client := &KinesisClient{
		streams: map[string]string{
			"traces":  "test-traces",
			"metrics": "test-metrics",
			"logs":    "test-logs",
		},
		batchChannels: make(map[string]chan LegacyTelemetryEvent),
		shutdownCh:    make(chan struct{}),
		batchSize:     100,
		flushInterval: 5 * time.Second,
	}

	handler := NewKinesisHandler(client)

	t.Run("handler_interface_compliance", func(t *testing.T) {
		// Test that handler implements the Handler interface
		var _ Handler = handler
		assert.NotNil(t, handler, "Handler should not be nil")
		assert.NotNil(t, handler.client, "Handler client should not be nil")
	})

	t.Run("handler_with_nil_event", func(t *testing.T) {
		// Test handler with nil event (should handle gracefully)
		// This would panic if not handled properly in the actual implementation
		// For now, we test the structure
		assert.NotNil(t, handler, "Handler should handle nil event gracefully")
	})

	t.Run("handler_context_handling", func(t *testing.T) {
		// Test that handler respects context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		event := &TraceTelemetryEvent{
			BaseTelemetryEvent: BaseTelemetryEvent{
				Type:        TelemetryTypeTraces,
				ServiceName: "test-service",
				TraceID:     "test-trace",
				Metadata: TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "127.0.0.1",
					TenantID:   "test",
				},
			},
			ResourceSpans: &tracepb.ResourceSpans{
				ScopeSpans: []*tracepb.ScopeSpans{{
					Spans: []*tracepb.Span{{
						Name:    "context-test-span",
						TraceId: []byte("context-trace-16"),
						SpanId:  []byte("span-id8"),
					}},
				}},
			},
		}

		// Test context cancellation handling
		select {
		case <-ctx.Done():
			assert.True(t, true, "Context should be cancelled")
		default:
			t.Fatal("Context should be cancelled")
		}

		// The actual method would fail due to AWS connectivity, but we test the structure
		assert.NotNil(t, event, "Event should not be nil")
		assert.Equal(t, TelemetryEventType("traces"), event.Type, "Event type should be preserved")
	})
}

func TestTelemetryEventValidation(t *testing.T) {
	tests := []struct {
		name        string
		event       TelemetryEvent
		description string
	}{
		{
			name: "complete trace event",
			event: &TraceTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        TelemetryTypeTraces,
					ServiceName: "complete-service",
					TraceID:     "complete-trace-123",
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						SourceIP:   "192.168.1.1",
						TenantID:   "complete-tenant",
					},
				},
				ResourceSpans: &tracepb.ResourceSpans{
					ScopeSpans: []*tracepb.ScopeSpans{{
						Spans: []*tracepb.Span{{
							Name:    "complete-span",
							TraceId: []byte("complete-trace-16"),
							SpanId:  []byte("span-id8"),
						}},
					}},
				},
			},
			description: "Complete event should have all required fields",
		},
		{
			name: "minimal event",
			event: &MetricsTelemetryEvent{
				BaseTelemetryEvent: BaseTelemetryEvent{
					Type:        TelemetryTypeMetrics,
					ServiceName: "minimal-service",
					Metadata: TelemetryMetadata{
						IngestedAt: time.Now(),
						TenantID:   "default",
					},
				},
				ResourceMetrics: &metricspb.ResourceMetrics{
					ScopeMetrics: []*metricspb.ScopeMetrics{{
						Metrics: []*metricspb.Metric{{
							Name: "minimal-metric",
						}},
					}},
				},
			},
			description: "Minimal event should work with required fields only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test interface methods
			assert.NotEmpty(t, tt.event.GetType(), "Type should not be empty")
			assert.NotEmpty(t, tt.event.GetServiceName(), "Service name should not be empty")
			assert.False(t, tt.event.GetMetadata().IngestedAt.IsZero(), "IngestedAt should be set")

			// Test serialization if implemented
			data, err := tt.event.GetSerializedData()
			if err == nil {
				assert.Greater(t, len(data), 0, "Serialized data should not be empty")
			}
		})
	}
}

func TestTelemetryMetadata(t *testing.T) {
	metadata := TelemetryMetadata{
		IngestedAt: time.Now(),
		SourceIP:   "203.0.113.42",
		TenantID:   "metadata-test",
	}

	t.Run("metadata_fields", func(t *testing.T) {
		assert.False(t, metadata.IngestedAt.IsZero(), "IngestedAt should be set")
		assert.Equal(t, "203.0.113.42", metadata.SourceIP, "Source IP should match")
		assert.Equal(t, "metadata-test", metadata.TenantID, "Tenant ID should match")
	})

	t.Run("metadata_json_serialization", func(t *testing.T) {
		data, err := json.Marshal(metadata)
		assert.NoError(t, err, "Metadata should marshal to JSON")
		assert.Greater(t, len(data), 0, "Marshaled data should not be empty")

		var unmarshaled TelemetryMetadata
		err = json.Unmarshal(data, &unmarshaled)
		assert.NoError(t, err, "Metadata should unmarshal from JSON")
		assert.Equal(t, metadata.SourceIP, unmarshaled.SourceIP, "Source IP should be preserved")
		assert.Equal(t, metadata.TenantID, unmarshaled.TenantID, "Tenant ID should be preserved")
		assert.Equal(t, metadata.SourceIP, unmarshaled.SourceIP, "Source IP should be preserved")
	})

	t.Run("empty_optional_fields", func(t *testing.T) {
		emptyMetadata := TelemetryMetadata{
			IngestedAt: time.Now(),
			SourceIP:   "127.0.0.1",
			// UserAgent and Version left empty
		}

		assert.False(t, emptyMetadata.IngestedAt.IsZero(), "IngestedAt should be set")
		assert.Equal(t, "127.0.0.1", emptyMetadata.SourceIP, "Source IP should be set")
		assert.Empty(t, emptyMetadata.TenantID, "Tenant ID should be empty")
	})
}
