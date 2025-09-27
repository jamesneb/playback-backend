package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/stretchr/testify/assert"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestNewClickHouseHandler(t *testing.T) {
	client := &storage.ClickHouseClient{}
	handler := NewClickHouseHandler(client)

	assert.NotNil(t, handler)
	assert.Equal(t, client, handler.client)
}

func TestClickHouseHandler_HandleTelemetryEvent_NilClient(t *testing.T) {
	handler := &ClickHouseHandler{client: nil}
	ctx := context.Background()

	traceEvent := &streaming.TraceTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeTraces,
			ServiceName: "test-service",
			TraceID:     "test-trace-id",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
			},
		},
		ResourceSpans: &tracepb.ResourceSpans{},
	}

	// Should not fail when client is nil - just skips insertion
	err := handler.HandleTelemetryEvent(ctx, traceEvent)
	assert.NoError(t, err)
}

func TestClickHouseHandler_HandleTelemetryEvent_TraceEvent(t *testing.T) {
	// Test with nil client since we can't easily mock the ClickHouse connection
	handler := &ClickHouseHandler{client: nil}
	ctx := context.Background()

	traceEvent := &streaming.TraceTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeTraces,
			ServiceName: "test-service",
			TraceID:     "test-trace-id",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
			},
		},
		ResourceSpans: &tracepb.ResourceSpans{
			Resource: &resourcepb.Resource{},
			ScopeSpans: []*tracepb.ScopeSpans{
				{
					Scope: &commonpb.InstrumentationScope{
						Name:    "test-scope",
						Version: "1.0.0",
					},
					Spans: []*tracepb.Span{
						{
							TraceId:           []byte("test-trace-id-bytes"),
							SpanId:            []byte("test-span-id-bytes"),
							Name:              "test-span",
							Kind:              tracepb.Span_SPAN_KIND_INTERNAL,
							StartTimeUnixNano: uint64(time.Now().UnixNano()),
							EndTimeUnixNano:   uint64(time.Now().UnixNano()),
						},
					},
				},
			},
		},
	}

	// Should handle trace event gracefully
	err := handler.HandleTelemetryEvent(ctx, traceEvent)
	assert.NoError(t, err)
}

func TestClickHouseHandler_HandleTelemetryEvent_MetricsEvent(t *testing.T) {
	handler := &ClickHouseHandler{client: nil}
	ctx := context.Background()

	metricsEvent := &streaming.MetricsTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeMetrics,
			ServiceName: "test-service",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
			},
		},
		ResourceMetrics: &metricspb.ResourceMetrics{
			Resource: &resourcepb.Resource{},
			ScopeMetrics: []*metricspb.ScopeMetrics{
				{
					Scope: &commonpb.InstrumentationScope{
						Name:    "test-scope",
						Version: "1.0.0",
					},
					Metrics: []*metricspb.Metric{
						{
							Name:        "test-metric",
							Description: "Test metric description",
							Unit:        "count",
						},
					},
				},
			},
		},
	}

	// Should handle metrics event gracefully
	err := handler.HandleTelemetryEvent(ctx, metricsEvent)
	assert.NoError(t, err)
}

func TestClickHouseHandler_HandleTelemetryEvent_LogsEvent(t *testing.T) {
	handler := &ClickHouseHandler{client: nil}
	ctx := context.Background()

	logsEvent := &streaming.LogsTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeLogs,
			ServiceName: "test-service",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
			},
		},
		ResourceLogs: &logspb.ResourceLogs{
			Resource: &resourcepb.Resource{},
			ScopeLogs: []*logspb.ScopeLogs{
				{
					Scope: &commonpb.InstrumentationScope{
						Name:    "test-scope",
						Version: "1.0.0",
					},
					LogRecords: []*logspb.LogRecord{
						{
							TimeUnixNano:         uint64(time.Now().UnixNano()),
							ObservedTimeUnixNano: uint64(time.Now().UnixNano()),
							SeverityNumber:       logspb.SeverityNumber_SEVERITY_NUMBER_INFO,
							SeverityText:         "INFO",
						},
					},
				},
			},
		},
	}

	// Should handle logs event gracefully
	err := handler.HandleTelemetryEvent(ctx, logsEvent)
	assert.NoError(t, err)
}

func TestClickHouseHandler_HandleTelemetryEvent_UnknownEventType(t *testing.T) {
	handler := &ClickHouseHandler{client: nil}
	ctx := context.Background()

	// Create a custom event type that implements the interface but isn't one of the known types
	unknownEvent := &streaming.TraceTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        "unknown", // This should be ignored
			ServiceName: "test-service",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
			},
		},
		ResourceSpans: &tracepb.ResourceSpans{},
	}

	// Should handle unknown event type gracefully
	err := handler.HandleTelemetryEvent(ctx, unknownEvent)
	assert.NoError(t, err)
}

func TestClickHouseHandler_HandleTelemetryEvent_EventTypeMatching(t *testing.T) {
	handler := &ClickHouseHandler{client: nil}
	ctx := context.Background()

	// Test that the switch statement works based on GetType() method result
	tests := []struct {
		name  string
		event streaming.TelemetryEvent
	}{
		{
			name: "trace_event",
			event: &streaming.TraceTelemetryEvent{
				BaseTelemetryEvent: streaming.BaseTelemetryEvent{
					Type:        streaming.TelemetryTypeTraces,
					ServiceName: "test-service",
					TraceID:     "test-trace-id",
				},
				ResourceSpans: &tracepb.ResourceSpans{},
			},
		},
		{
			name: "metrics_event",
			event: &streaming.MetricsTelemetryEvent{
				BaseTelemetryEvent: streaming.BaseTelemetryEvent{
					Type:        streaming.TelemetryTypeMetrics,
					ServiceName: "test-service",
				},
				ResourceMetrics: &metricspb.ResourceMetrics{},
			},
		},
		{
			name: "logs_event",
			event: &streaming.LogsTelemetryEvent{
				BaseTelemetryEvent: streaming.BaseTelemetryEvent{
					Type:        streaming.TelemetryTypeLogs,
					ServiceName: "test-service",
				},
				ResourceLogs: &logspb.ResourceLogs{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.HandleTelemetryEvent(ctx, tt.event)
			assert.NoError(t, err)
		})
	}
}
