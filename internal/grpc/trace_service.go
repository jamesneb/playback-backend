package grpc

import (
	"context"
	"encoding/json"
	"time"

	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
)

type TraceService struct {
	tracecollectorpb.UnimplementedTraceServiceServer
	streamHandler *streaming.KinesisHandler
	clickhouseHandler streaming.Handler // Direct ClickHouse for real-time path
}

func NewTraceService(streamHandler *streaming.KinesisHandler, clickhouseHandler streaming.Handler) *TraceService {
	return &TraceService{
		streamHandler:     streamHandler,
		clickhouseHandler: clickhouseHandler,
	}
}

func (s *TraceService) Export(ctx context.Context, req *tracecollectorpb.ExportTraceServiceRequest) (*tracecollectorpb.ExportTraceServiceResponse, error) {
	logger.Info("Received gRPC trace export request", 
		zap.Int("resource_spans", len(req.ResourceSpans)))

	// Extract client IP from gRPC context
	clientIP := ExtractClientIP(ctx)

	// Minimal processing: Convert OTLP protobuf to raw JSON for ClickHouse processing
	for _, resourceSpan := range req.ResourceSpans {
		// Convert protobuf to JSON with minimal processing
		rawOTLP, err := protojson.Marshal(resourceSpan)
		if err != nil {
			logger.Error("Failed to marshal resource span to JSON", zap.Error(err))
			continue
		}
		
		event := &streaming.TelemetryEvent{
			Type:        "traces",
			TraceID:     extractTraceID(resourceSpan),    // Still need for Kinesis partitioning
			ServiceName: extractServiceName(resourceSpan), // Still need for ClickHouse partitioning
			Data:        json.RawMessage(rawOTLP),         // Raw JSON - no complex processing
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
					logger.Error("Failed to handle trace in real-time path", zap.Error(err))
				}
			}
		}(event)

		// Durable path - to Kinesis for fan-out and replay capability
		if s.streamHandler != nil {
			if err := s.streamHandler.HandleTelemetryEvent(ctx, event); err != nil {
				logger.Error("Failed to send trace to Kinesis", zap.Error(err))
				// Don't return error - we still have real-time path
			}
		}
	}

	logger.Info("Successfully processed gRPC trace export", 
		zap.Int("spans_processed", countSpans(req.ResourceSpans)))

	return &tracecollectorpb.ExportTraceServiceResponse{
		PartialSuccess: &tracecollectorpb.ExportTracePartialSuccess{
			RejectedSpans: 0,
		},
	}, nil
}

// Helper functions to extract data from OTLP protobuf
func extractTraceID(resourceSpan *tracepb.ResourceSpans) string {
	if len(resourceSpan.ScopeSpans) > 0 && len(resourceSpan.ScopeSpans[0].Spans) > 0 {
		return string(resourceSpan.ScopeSpans[0].Spans[0].TraceId)
	}
	return "unknown"
}

func extractServiceName(resourceSpan *tracepb.ResourceSpans) string {
	if resourceSpan.Resource != nil {
		for _, attr := range resourceSpan.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.GetStringValue() != "" {
				return attr.Value.GetStringValue()
			}
		}
	}
	return "unknown"
}

// Complex processing functions removed - all moved to ClickHouse materialized views

func countSpans(resourceSpans []*tracepb.ResourceSpans) int {
	count := 0
	for _, rs := range resourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	return count
}

