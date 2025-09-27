package grpc

import (
	"context"
	"log"
	"time"

	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/internal/validation"
	"github.com/jamesneb/playback-backend/pkg/logger"
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TraceService struct {
	tracecollectorpb.UnimplementedTraceServiceServer
	streamHandler     *streaming.KinesisHandler
	clickhouseHandler streaming.Handler // Direct ClickHouse for real-time path
	validator         *validation.ProtobufValidator // Add protobuf validator

	// Resilience components
	kinesisBuffer     *resilience.KinesisBuffer
	rateLimiter      *resilience.TenantRateLimiter
	circuitBreaker   *resilience.CircuitBreaker
	deadLetterQueue  *resilience.DeadLetterQueue
}

func NewTraceService(streamHandler *streaming.KinesisHandler, clickhouseHandler streaming.Handler,
	resilienceComponents *interfaces.ResilienceComponents) *TraceService {
	log.Printf("🔥 TRACE SERVICE INIT: NewTraceService called - protobuf debugging enabled!")
	return &TraceService{
		streamHandler:     streamHandler,
		clickhouseHandler: clickhouseHandler,
		validator:         validation.NewProtobufValidator(),
		kinesisBuffer:     resilienceComponents.KinesisBuffer,
		rateLimiter:      resilienceComponents.RateLimiter,
		circuitBreaker:   resilienceComponents.CircuitBreaker,
		deadLetterQueue:  resilienceComponents.DeadLetterQueue,
	}
}


func (s *TraceService) Export(ctx context.Context, req *tracecollectorpb.ExportTraceServiceRequest) (*tracecollectorpb.ExportTraceServiceResponse, error) {
	log.Printf("🔥 GRPC EXPORT START: TraceService.Export called with %d resource spans", len(req.ResourceSpans))
	logger.Info("Received gRPC trace export request",
		zap.Int("resource_spans", len(req.ResourceSpans)))

	// Validate protobuf request first
	tracesData := &tracepb.TracesData{ResourceSpans: req.ResourceSpans}
	if err := s.validator.ValidateTraceRequest(tracesData); err != nil {
		logger.Warn("Invalid trace request", zap.Error(err))
		return nil, err
	}

	// Extract client IP from gRPC context
	clientIP := ExtractClientIP(ctx)

	// Extract tenant ID from service name (you might want to extract this differently)
	tenantID := "default" // TODO: Extract from headers or service metadata
	if len(req.ResourceSpans) > 0 {
		serviceName := streaming.ExtractServiceNameFromTraces(req.ResourceSpans[0])
		if serviceName != "" {
			tenantID = serviceName // Use service name as tenant ID for now
		}
	}

	// Apply global rate limiting first
	if s.rateLimiter != nil && !s.rateLimiter.Allow(tenantID) {
		return nil, status.Errorf(codes.ResourceExhausted, "tenant rate limit exceeded")
	}

	// Process with type-safe protobuf events - KINESIS-FIRST approach
	for _, resourceSpan := range req.ResourceSpans {
		// Create type-safe trace event with native protobuf data
		event := &streaming.TraceTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeTraces,
				ServiceName: streaming.ExtractServiceNameFromTraces(resourceSpan),
				TraceID:     streaming.ExtractTraceIDFromTraces(resourceSpan),
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   clientIP,
				},
			},
			ResourceSpans: resourceSpan, // Native protobuf - much more efficient!
		}
		
		log.Printf("🔥 GRPC: Created TraceTelemetryEvent for service=%s, traceID=%s, type=%T", 
			event.ServiceName, event.TraceID, event)

		// Validate the event
		if err := event.Validate(); err != nil {
			logger.Error("Invalid trace event", zap.Error(err))
			continue
		}

		// KINESIS-FIRST: Primary path through resilient buffer
		if s.kinesisBuffer != nil {
			if err := s.kinesisBuffer.BufferEvent(ctx, event, tenantID, "grpc"); err != nil {
				// If buffering fails, this is a serious issue - fail the request
				logger.Error("Failed to buffer trace event", 
					zap.String("tenant", tenantID),
					zap.Error(err))
				return nil, status.Errorf(codes.Unavailable, "telemetry pipeline overloaded")
			}
			log.Printf("🔥 GRPC: Successfully buffered trace event for tenant=%s", tenantID)
		} else {
			// Fallback to direct Kinesis (old behavior)
			if s.streamHandler != nil {
				log.Printf("🔥 GRPC: Fallback to direct Kinesis for %T", event)
				if err := s.streamHandler.HandleTelemetryEvent(ctx, event); err != nil {
					logger.Error("Failed to send trace to Kinesis", zap.Error(err))
					// Send to DLQ if available
					if s.deadLetterQueue != nil {
						if dlqErr := s.deadLetterQueue.SendToDLQ(ctx, event, err, tenantID, "grpc", "kinesis_direct_failed"); dlqErr != nil {
							logger.Error("Failed to send to DLQ", zap.Error(dlqErr))
						}
					}
					return nil, status.Errorf(codes.Unavailable, "telemetry pipeline unavailable")
				}
			}
		}

		// Real-time ClickHouse insertion removed - data flows through consumer only
		// This ensures proper data pipeline: gRPC -> Kinesis -> Consumer -> ClickHouse
	}

	logger.Info("Successfully processed gRPC trace export",
		zap.Int("spans_processed", countSpans(req.ResourceSpans)))

	return &tracecollectorpb.ExportTraceServiceResponse{
		PartialSuccess: &tracecollectorpb.ExportTracePartialSuccess{
			RejectedSpans: 0,
		},
	}, nil
}

// Helper function to count spans for logging

func countSpans(resourceSpans []*tracepb.ResourceSpans) int {
	count := 0
	for _, rs := range resourceSpans {
		for _, ss := range rs.ScopeSpans {
			count += len(ss.Spans)
		}
	}
	return count
}
