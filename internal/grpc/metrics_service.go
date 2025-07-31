package grpc

import (
	"context"
	"encoding/json"
	"time"

	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
)

type MetricsService struct {
	metricscollectorpb.UnimplementedMetricsServiceServer
	streamHandler *streaming.KinesisHandler
	clickhouseHandler streaming.Handler
}

func NewMetricsService(streamHandler *streaming.KinesisHandler, clickhouseHandler streaming.Handler) *MetricsService {
	return &MetricsService{
		streamHandler:     streamHandler,
		clickhouseHandler: clickhouseHandler,
	}
}

func (s *MetricsService) Export(ctx context.Context, req *metricscollectorpb.ExportMetricsServiceRequest) (*metricscollectorpb.ExportMetricsServiceResponse, error) {
	logger.Info("Received gRPC metrics export request", 
		zap.Int("resource_metrics", len(req.ResourceMetrics)))

	// Extract client IP from gRPC context
	clientIP := ExtractClientIP(ctx)

	// Minimal processing: Convert OTLP protobuf to raw JSON for ClickHouse processing
	for _, resourceMetric := range req.ResourceMetrics {
		// Convert protobuf to JSON with minimal processing
		rawOTLP, err := protojson.Marshal(resourceMetric)
		if err != nil {
			logger.Error("Failed to marshal resource metric to JSON", zap.Error(err))
			continue
		}
		
		event := &streaming.TelemetryEvent{
			Type:        "metrics",
			ServiceName: extractServiceNameFromMetrics(resourceMetric),
			Data:        json.RawMessage(rawOTLP),
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   clientIP,
			},
		}

		// Dual path: Send to both real-time processor and durable Kinesis
		go func(e *streaming.TelemetryEvent) {
			// Real-time path - direct to ClickHouse (fast)
			if s.clickhouseHandler != nil {
				if err := s.clickhouseHandler.HandleTelemetryEvent(ctx, e); err != nil {
					logger.Error("Failed to handle metrics in real-time path", zap.Error(err))
				}
			}
		}(event)

		// Durable path - to Kinesis for fan-out and replay capability
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

