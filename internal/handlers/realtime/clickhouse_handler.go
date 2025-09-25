package realtime

import (
	"context"

	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// ClickHouseHandler implements streaming.Handler interface for direct ClickHouse insertion
type ClickHouseHandler struct {
	client *storage.ClickHouseClient
}

func NewClickHouseHandler(client *storage.ClickHouseClient) *ClickHouseHandler {
	return &ClickHouseHandler{
		client: client,
	}
}

func (h *ClickHouseHandler) HandleTelemetryEvent(ctx context.Context, event streaming.TelemetryEvent) error {
	// Check if client is available
	if h.client == nil {
		logger.Warn("ClickHouse client not available, skipping telemetry event",
			zap.String("type", string(event.GetType())),
			zap.String("service", event.GetServiceName()))
		return nil // Don't fail - just skip the insertion
	}

	switch event.GetType() {
	case streaming.TelemetryTypeTraces:
		// Check if it's a protobuf trace event
		if traceEvent, ok := event.(*streaming.TraceTelemetryEvent); ok {
			// Use native protobuf insertion
			if err := h.client.InsertTraceProtobuf(ctx, traceEvent); err != nil {
				logger.Error("Failed to insert protobuf trace to ClickHouse", 
					zap.String("trace_id", event.GetTraceID()),
					zap.Error(err))
				return err
			}
			logger.Debug("Inserted protobuf trace to ClickHouse via real-time path", 
				zap.String("trace_id", event.GetTraceID()))
		} else {
			// Use legacy JSON insertion
			if err := h.client.InsertTrace(ctx, event); err != nil {
				logger.Error("Failed to insert JSON trace to ClickHouse", 
					zap.String("trace_id", event.GetTraceID()),
					zap.Error(err))
				return err
			}
			logger.Debug("Inserted JSON trace to ClickHouse via real-time path", 
				zap.String("trace_id", event.GetTraceID()))
		}
		
	case streaming.TelemetryTypeMetrics:
		// Check if it's a protobuf metric event
		if metricEvent, ok := event.(*streaming.MetricsTelemetryEvent); ok {
			// Use native protobuf insertion
			if err := h.client.InsertMetricProtobuf(ctx, metricEvent); err != nil {
				logger.Error("Failed to insert protobuf metrics to ClickHouse", 
					zap.String("service", event.GetServiceName()),
					zap.Error(err))
				return err
			}
			logger.Debug("Inserted protobuf metrics to ClickHouse via real-time path", 
				zap.String("service", event.GetServiceName()))
		} else {
			// Use legacy JSON insertion
			if err := h.client.InsertMetric(ctx, event); err != nil {
				logger.Error("Failed to insert JSON metrics to ClickHouse", 
					zap.String("service", event.GetServiceName()),
					zap.Error(err))
				return err
			}
			logger.Debug("Inserted JSON metrics to ClickHouse via real-time path", 
				zap.String("service", event.GetServiceName()))
		}
		
	case streaming.TelemetryTypeLogs:
		// Check if it's a protobuf log event
		if logEvent, ok := event.(*streaming.LogsTelemetryEvent); ok {
			// Use native protobuf insertion
			if err := h.client.InsertLogProtobuf(ctx, logEvent); err != nil {
				logger.Error("Failed to insert protobuf logs to ClickHouse", 
					zap.String("service", event.GetServiceName()),
					zap.Error(err))
				return err
			}
			logger.Debug("Inserted protobuf logs to ClickHouse via real-time path", 
				zap.String("service", event.GetServiceName()))
		} else {
			// Use legacy JSON insertion
			if err := h.client.InsertLog(ctx, event); err != nil {
				logger.Error("Failed to insert JSON logs to ClickHouse", 
					zap.String("service", event.GetServiceName()),
					zap.Error(err))
				return err
			}
			logger.Debug("Inserted JSON logs to ClickHouse via real-time path", 
				zap.String("service", event.GetServiceName()))
		}
	}
	
	return nil
}