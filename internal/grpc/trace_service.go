package grpc

import (
	"context"
	"fmt"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
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

	// Convert OTLP protobuf to our internal telemetry event format
	for _, resourceSpan := range req.ResourceSpans {
		event := &streaming.TelemetryEvent{
			Type:      "traces",
			TraceID:   extractTraceID(resourceSpan),
			ServiceName: extractServiceName(resourceSpan),
			Data:      convertResourceSpanToMap(resourceSpan),
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

func convertResourceSpanToMap(resourceSpan *tracepb.ResourceSpans) interface{} {
	// Convert protobuf to the JSON structure expected by parseTraceData
	resource := make(map[string]interface{})
	
	// Convert resource attributes
	if resourceSpan.Resource != nil {
		attributes := make([]map[string]interface{}, 0, len(resourceSpan.Resource.Attributes))
		for _, attr := range resourceSpan.Resource.Attributes {
			// Simplified attribute structure that parseTraceData expects
			attrMap := map[string]interface{}{
				"key": attr.Key,
				"value": map[string]interface{}{
					"stringValue": "", // Default empty string
				},
			}
			
			// Extract string value (simplified - just string values for now)
			if attr.Value != nil {
				switch v := attr.Value.Value.(type) {
				case *commonpb.AnyValue_StringValue:
					attrMap["value"].(map[string]interface{})["stringValue"] = v.StringValue
				case *commonpb.AnyValue_IntValue:
					attrMap["value"].(map[string]interface{})["stringValue"] = fmt.Sprintf("%d", v.IntValue)
				case *commonpb.AnyValue_DoubleValue:
					attrMap["value"].(map[string]interface{})["stringValue"] = fmt.Sprintf("%f", v.DoubleValue)
				case *commonpb.AnyValue_BoolValue:
					attrMap["value"].(map[string]interface{})["stringValue"] = fmt.Sprintf("%t", v.BoolValue)
				}
			}
			attributes = append(attributes, attrMap)
		}
		
		resource["attributes"] = attributes
	}
	
	// Convert scope spans to the structure parseTraceData expects
	scopeSpans := make([]map[string]interface{}, 0, len(resourceSpan.ScopeSpans))
	for _, ss := range resourceSpan.ScopeSpans {
		spans := make([]map[string]interface{}, 0, len(ss.Spans))
		for _, span := range ss.Spans {
			spanMap := map[string]interface{}{
				"traceId":           bytesToHex(span.TraceId),
				"spanId":            bytesToHex(span.SpanId),
				"parentSpanId":      bytesToHex(span.ParentSpanId),
				"name":              span.Name,
				"startTimeUnixNano": span.StartTimeUnixNano,
				"endTimeUnixNano":   span.EndTimeUnixNano,
			}
			spans = append(spans, spanMap)
		}
		
		scopeSpanMap := map[string]interface{}{
			"spans": spans,
		}
		scopeSpans = append(scopeSpans, scopeSpanMap)
	}
	
	// Return the structure that parseTraceData expects
	return map[string]interface{}{
		"resourceSpans": []map[string]interface{}{
			{
				"resource":   resource,
				"scopeSpans": scopeSpans,
			},
		},
	}
}

// Helper function to convert byte arrays to hex strings
func bytesToHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return fmt.Sprintf("%x", b)
}

func countSpans(resourceSpans []*tracepb.ResourceSpans) int {
	count := 0
	for _, rs := range resourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	return count
}

