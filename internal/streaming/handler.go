package streaming

import (
	"context"
	"fmt"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Handler interface for telemetry data processing with type-safe events
type Handler interface {
	HandleTelemetryEvent(ctx context.Context, event TelemetryEvent) error
}

// TypedHandler interface for handling specific event types
type TypedHandler interface {
	HandleTraceEvent(ctx context.Context, event *TraceTelemetryEvent) error
	HandleMetricsEvent(ctx context.Context, event *MetricsTelemetryEvent) error
	HandleLogsEvent(ctx context.Context, event *LogsTelemetryEvent) error
}

// KinesisHandler implements Handler interface for Kinesis streaming
type KinesisHandler struct {
	client *KinesisClient
}

func NewKinesisHandler(client *KinesisClient) *KinesisHandler {
	return &KinesisHandler{
		client: client,
	}
}

// Implement LegacyHandler interface for backward compatibility with JSON HTTP API
func (h *KinesisHandler) HandleLegacyTelemetryEvent(ctx context.Context, event *LegacyTelemetryEvent) error {
	switch event.Type {
	case "traces":
		return h.client.PublishTrace(ctx, event.Data, event.ServiceName, event.TraceID,
			event.Metadata.SourceIP, event.Metadata.UserAgent)
	case "metrics":
		return h.client.PublishMetrics(ctx, event.Data, event.ServiceName,
			event.Metadata.SourceIP, event.Metadata.UserAgent)
	case "logs":
		return h.client.PublishLogs(ctx, event.Data, event.ServiceName, event.TraceID,
			event.Metadata.SourceIP, event.Metadata.UserAgent)
	default:
		return ErrUnsupportedEventType
	}
}

func (h *KinesisHandler) HandleTelemetryEvent(ctx context.Context, event TelemetryEvent) error {
	// Validate the event first
	if err := event.Validate(); err != nil {
		return err
	}

	switch e := event.(type) {
	case *TraceTelemetryEvent:
		logger.Debug("Using protobuf path for trace telemetry event")
		return h.HandleTraceEvent(ctx, e)
	case *MetricsTelemetryEvent:
		logger.Debug("Using protobuf path for metrics telemetry event")
		return h.HandleMetricsEvent(ctx, e)
	case *LogsTelemetryEvent:
		logger.Debug("Using protobuf path for logs telemetry event")
		return h.HandleLogsEvent(ctx, e)
	default:
		logger.Warn("Unknown event type, using legacy path", zap.String("event_type", fmt.Sprintf("%T", event)))
		return ErrUnsupportedEventType
	}
}

// Implement TypedHandler interface
func (h *KinesisHandler) HandleTraceEvent(ctx context.Context, event *TraceTelemetryEvent) error {
	// Use native protobuf publishing for gRPC events (no JSON conversion!)
	return h.client.PublishTraceProtobuf(ctx, event.ResourceSpans, event.ServiceName, event.TraceID,
		event.Metadata.SourceIP)
}

func (h *KinesisHandler) HandleMetricsEvent(ctx context.Context, event *MetricsTelemetryEvent) error {
	// Use native protobuf publishing for gRPC events (no JSON conversion!)
	return h.client.PublishMetricsProtobuf(ctx, event.ResourceMetrics, event.ServiceName,
		event.Metadata.SourceIP)
}

func (h *KinesisHandler) HandleLogsEvent(ctx context.Context, event *LogsTelemetryEvent) error {
	// Use native protobuf publishing for gRPC events (no JSON conversion!)
	return h.client.PublishLogsProtobuf(ctx, event.ResourceLogs, event.ServiceName, event.TraceID,
		event.Metadata.SourceIP)
}
