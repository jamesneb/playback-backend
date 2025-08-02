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

func (h *ClickHouseHandler) HandleTelemetryEvent(ctx context.Context, event *streaming.TelemetryEvent) error {
	// Check if client is available
	if h.client == nil {
		logger.Warn("ClickHouse client not available, skipping telemetry event",
			zap.String("type", event.Type),
			zap.String("service", event.ServiceName))
		return nil // Don't fail - just skip the insertion
	}

	switch event.Type {
	case "traces":
		if err := h.client.InsertTrace(ctx, event); err != nil {
			logger.Error("Failed to insert trace to ClickHouse", 
				zap.String("trace_id", event.TraceID),
				zap.Error(err))
			return err
		}
		logger.Debug("Inserted trace to ClickHouse via real-time path", 
			zap.String("trace_id", event.TraceID))
		
	case "metrics":
		if err := h.client.InsertMetric(ctx, event); err != nil {
			logger.Error("Failed to insert metrics to ClickHouse", 
				zap.String("service", event.ServiceName),
				zap.Error(err))
			return err
		}
		logger.Debug("Inserted metrics to ClickHouse via real-time path", 
			zap.String("service", event.ServiceName))
		
	case "logs":
		if err := h.client.InsertLog(ctx, event); err != nil {
			logger.Error("Failed to insert logs to ClickHouse", 
				zap.String("service", event.ServiceName),
				zap.Error(err))
			return err
		}
		logger.Debug("Inserted logs to ClickHouse via real-time path", 
			zap.String("service", event.ServiceName))
	}
	
	return nil
}