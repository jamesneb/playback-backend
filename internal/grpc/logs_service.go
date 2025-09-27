package grpc

import (
	"context"
	"time"

	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/internal/validation"
	"github.com/jamesneb/playback-backend/pkg/logger"
	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"go.uber.org/zap"
)

type LogsService struct {
	logscollectorpb.UnimplementedLogsServiceServer
	streamHandler     streaming.Handler // Use interface to support mocks
	clickhouseHandler streaming.Handler
	validator         *validation.ProtobufValidator
}

func NewLogsService(streamHandler streaming.Handler, clickhouseHandler streaming.Handler) *LogsService {
	return &LogsService{
		streamHandler:     streamHandler,
		clickhouseHandler: clickhouseHandler,
		validator:         validation.NewProtobufValidator(),
	}
}

func (s *LogsService) Export(ctx context.Context, req *logscollectorpb.ExportLogsServiceRequest) (*logscollectorpb.ExportLogsServiceResponse, error) {
	logger.Info("Received gRPC logs export request",
		zap.Int("resource_logs", len(req.ResourceLogs)))

	// Validate protobuf request first
	logsData := &logspb.LogsData{ResourceLogs: req.ResourceLogs}
	if err := s.validator.ValidateLogsRequest(logsData); err != nil {
		logger.Warn("Invalid logs request", zap.Error(err))
		return nil, err
	}

	// Extract client IP from gRPC context
	clientIP := ExtractClientIP(ctx)

	// Minimal processing: Convert OTLP protobuf to raw JSON for ClickHouse processing
	for _, resourceLog := range req.ResourceLogs {
		// Use native protobuf - no JSON conversion needed
		
		event := &streaming.LogsTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeLogs,
				ServiceName: extractServiceNameFromLogs(resourceLog),
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   clientIP,
				},
			},
			ResourceLogs: resourceLog,
		}

		// Single path: Send to Kinesis only - consumer handles ClickHouse insertion
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

