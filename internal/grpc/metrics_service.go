package grpc

import (
	"context"
	"time"

	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/internal/validation"
	"github.com/jamesneb/playback-backend/pkg/logger"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"go.uber.org/zap"
)

type MetricsService struct {
	metricscollectorpb.UnimplementedMetricsServiceServer
	streamHandler     *streaming.KinesisHandler
	clickhouseHandler streaming.Handler
	validator         *validation.ProtobufValidator
}

func NewMetricsService(streamHandler *streaming.KinesisHandler, clickhouseHandler streaming.Handler) *MetricsService {
	return &MetricsService{
		streamHandler:     streamHandler,
		clickhouseHandler: clickhouseHandler,
		validator:         validation.NewProtobufValidator(),
	}
}

func (s *MetricsService) Export(ctx context.Context, req *metricscollectorpb.ExportMetricsServiceRequest) (*metricscollectorpb.ExportMetricsServiceResponse, error) {
	logger.Info("Received gRPC metrics export request",
		zap.Int("resource_metrics", len(req.ResourceMetrics)))

	// Validate protobuf request first
	metricsData := &metricspb.MetricsData{ResourceMetrics: req.ResourceMetrics}
	if err := s.validator.ValidateMetricsRequest(metricsData); err != nil {
		logger.Warn("Invalid metrics request", zap.Error(err))
		return nil, err
	}

	// Extract client IP from gRPC context
	clientIP := ExtractClientIP(ctx)

	// Minimal processing: Convert OTLP protobuf to raw JSON for ClickHouse processing
	for _, resourceMetric := range req.ResourceMetrics {
		// Use native protobuf - no JSON conversion needed
		
		event := &streaming.MetricsTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeMetrics,
				ServiceName: extractServiceNameFromMetrics(resourceMetric),
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   clientIP,
				},
			},
			ResourceMetrics: resourceMetric,
		}

		// Single path: Send to Kinesis only - consumer handles ClickHouse insertion
		if s.streamHandler != nil {
			if err := s.streamHandler.HandleTelemetryEvent(ctx, event); err != nil {
				logger.Error("Failed to send metrics to Kinesis", zap.Error(err))
			}
		}
	}

	logger.Info("Successfully processed gRPC metrics export", 
		zap.Int("metrics_processed", countMetrics(req.ResourceMetrics)))

	return &metricscollectorpb.ExportMetricsServiceResponse{
		PartialSuccess: &metricscollectorpb.ExportMetricsPartialSuccess{
			RejectedDataPoints: 0,
		},
	}, nil
}

func extractServiceNameFromMetrics(resourceMetric *metricspb.ResourceMetrics) string {
	if resourceMetric.Resource != nil {
		for _, attr := range resourceMetric.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.GetStringValue() != "" {
				return attr.Value.GetStringValue()
			}
		}
	}
	return "unknown"
}

func convertResourceMetricToMap(resourceMetric *metricspb.ResourceMetrics) interface{} {
	return map[string]interface{}{
		"resourceMetrics": []interface{}{resourceMetric},
	}
}

func countMetrics(resourceMetrics []*metricspb.ResourceMetrics) int {
	count := 0
	for _, rm := range resourceMetrics {
		for _, sm := range rm.ScopeMetrics {
			count += len(sm.Metrics)
		}
	}
	return count
}

