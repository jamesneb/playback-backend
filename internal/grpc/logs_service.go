package grpc

import (
	"context"
	"time"

	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

type LogsService struct {
	logscollectorpb.UnimplementedLogsServiceServer
	streamHandler *streaming.KinesisHandler
	clickhouseHandler streaming.Handler
}

func NewLogsService(streamHandler *streaming.KinesisHandler, clickhouseHandler streaming.Handler) *LogsService {
	return &LogsService{
		streamHandler:     streamHandler,
		clickhouseHandler: clickhouseHandler,
	}
}

func (s *LogsService) Export(ctx context.Context, req *logscollectorpb.ExportLogsServiceRequest) (*logscollectorpb.ExportLogsServiceResponse, error) {
	logger.Info("Received gRPC logs export request", 
		zap.Int("resource_logs", len(req.ResourceLogs)))

	// Convert OTLP protobuf to our internal telemetry event format
	for _, resourceLog := range req.ResourceLogs {
		event := &streaming.TelemetryEvent{
			Type:        "logs",
			ServiceName: extractServiceNameFromLogs(resourceLog),
			Data:        convertResourceLogToMap(resourceLog),
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   "grpc-client",
			},
		}

		// Dual path: Send to both real-time processor and durable Kinesis
		go func(e *streaming.TelemetryEvent) {
			// Real-time path - direct to ClickHouse (fast)
			if s.clickhouseHandler != nil {
				if err := s.clickhouseHandler.HandleTelemetryEvent(ctx, e); err != nil {
					logger.Error("Failed to handle logs in real-time path", zap.Error(err))
				}
			}
		}(event)

		// Durable path - to Kinesis for fan-out and replay capability
		if s.streamHandler != nil {
			if err := s.streamHandler.HandleTelemetryEvent(ctx, event); err != nil {
				logger.Error("Failed to send logs to Kinesis", zap.Error(err))
			}
		}
	}

	logger.Info("Successfully processed gRPC logs export", 
		zap.Int("log_records_processed", countLogRecords(req.ResourceLogs)))

	return &logscollectorpb.ExportLogsServiceResponse{
		PartialSuccess: &logscollectorpb.ExportLogsPartialSuccess{
			RejectedLogRecords: 0,
		},
	}, nil
}

func extractServiceNameFromLogs(resourceLog *logspb.ResourceLogs) string {
	if resourceLog.Resource != nil {
		for _, attr := range resourceLog.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.GetStringValue() != "" {
				return attr.Value.GetStringValue()
			}
		}
	}
	return "unknown"
}

func convertResourceLogToMap(resourceLog *logspb.ResourceLogs) interface{} {
	return map[string]interface{}{
		"resourceLogs": []interface{}{resourceLog},
	}
}

func countLogRecords(resourceLogs []*logspb.ResourceLogs) int {
	count := 0
	for _, rl := range resourceLogs {
		for _, sl := range rl.ScopeLogs {
			count += len(sl.LogRecords)
		}
	}
	return count
}