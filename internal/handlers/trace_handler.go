package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/validation"
	pkgerrors "github.com/jamesneb/playback-backend/pkg/errors"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	// idCounter provides atomic counter for ID generation
	idCounter int64
)

// TraceHandler handles HTTP/gRPC trace ingestion with high-performance processing
type TraceHandler struct {
	*BaseTelemetryHandler
	validator         *RequestValidator
	schemaValidator   *validation.SchemaValidator
	protobufValidator *validation.ProtobufValidator
	queryService      *TraceQueryService
	// Resilience components
	kinesisBuffer   *resilience.KinesisBuffer
	rateLimiter     *resilience.TenantRateLimiter
	deadLetterQueue *resilience.DeadLetterQueue
	// Pre-allocated buffers for hot path operations
	logFields []zap.Field
}

// TraceQueryService provides trace querying capabilities
type TraceQueryService struct {
	clickhouse *storage.ClickHouseClient
}

// TraceQueryResponse represents a trace query response
type TraceQueryResponse struct {
	TraceID   string        `json:"trace_id"`
	StartTime time.Time     `json:"start_time"`
	Duration  time.Duration `json:"duration"`
	SpanCount int           `json:"span_count"`
	Status    string        `json:"status"`
}

// GetTraceByID retrieves a trace by ID from ClickHouse
func (tqs *TraceQueryService) GetTraceByID(ctx context.Context, traceID string) (*TraceQueryResponse, error) {
	const query = `
		SELECT
			trace_id,
			min(start_time) as start_time,
			max(end_time) - min(start_time) as duration_ns,
			count(*) as span_count,
			any(status_code) as status_code
		FROM traces
		WHERE trace_id = ?
		GROUP BY trace_id
	`

	row := tqs.clickhouse.QueryRow(ctx, query, traceID)

	var result TraceQueryResponse
	var durationNs int64
	var statusCode int

	if err := row.Scan(
		&result.TraceID,
		&result.StartTime,
		&durationNs,
		&result.SpanCount,
		&statusCode,
	); err != nil {
		return nil, fmt.Errorf("trace not found or query failed: %w", err)
	}

	result.Duration = time.Duration(durationNs)
	result.Status = convertTraceStatusCode(statusCode)

	return &result, nil
}

func convertTraceStatusCode(code int) string {
	switch code {
	case 0:
		return "unset"
	case 1:
		return "success"
	case 2:
		return "error"
	default:
		return "unknown"
	}
}

// NewTraceHandler creates optimized trace handler with base consolidation
func NewTraceHandler(eventPublisher telemetry.EventPublisher, resilienceComponents *interfaces.ResilienceComponents) *TraceHandler {
	// Create high-performance base handler
	baseHandler := NewBaseTelemetryHandler(
		eventPublisher,
		&TraceMetadataExtractor{},
		NewStreamingTelemetryProcessor(eventPublisher, TelemetryTrace),
		TelemetryTrace,
	)

	handler := &TraceHandler{
		BaseTelemetryHandler: baseHandler,
		validator:            NewRequestValidator(),
		schemaValidator:      validation.NewSchemaValidator(false),
		protobufValidator:    validation.NewProtobufValidator(),
		logFields:            make([]zap.Field, 0, 8),
	}

	// Handle optional resilience components gracefully
	if resilienceComponents != nil {
		handler.kinesisBuffer = resilienceComponents.KinesisBuffer
		handler.rateLimiter = resilienceComponents.RateLimiter
		handler.deadLetterQueue = resilienceComponents.DeadLetterQueue
	}

	return handler
}

// NewTraceHandlerWithClickHouse creates a trace handler with ClickHouse query capabilities
func NewTraceHandlerWithClickHouse(eventPublisher telemetry.EventPublisher, resilienceComponents *interfaces.ResilienceComponents, clickhouse *storage.ClickHouseClient) *TraceHandler {
	handler := NewTraceHandler(eventPublisher, resilienceComponents)

	// Add ClickHouse query service for production trace queries
	if clickhouse != nil {
		handler.queryService = &TraceQueryService{
			clickhouse: clickhouse,
		}
	}

	return handler
}

// CreateTrace creates a new trace
// @Summary Create trace
// @Description Create a new distributed trace
// CreateTrace - consolidated high-performance trace ingestion
// @Summary Receive traces
// @Description Receive trace data from OpenTelemetry with optimized processing
// @Tags traces
// @Accept json
// @Produce json
// @Param traces body CreateTraceRequest true "Trace data"
// @Success 200 {object} TraceResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/traces [post]
func (h *TraceHandler) CreateTrace(c *gin.Context) {
	// Use consolidated base handler for HTTP ingestion
	h.HandleIngestion(c)
}

// CreateTraceProtobuf handles gRPC protobuf trace ingestion without JSON
func (h *TraceHandler) CreateTraceProtobuf(ctx context.Context, req *tracecollectorpb.ExportTraceServiceRequest) (*tracecollectorpb.ExportTraceServiceResponse, error) {
	// Extract metadata directly from protobuf
	metadata := OTLPMetadata{
		ServiceName: extractServiceNameFromTraceRequest(req),
		TraceID:     extractTraceIDFromTraceRequest(req),
		Count:       int32(countSpansInRequest(req)),
		DataSize:    int32(proto.Size(req)),
	}

	// Process without JSON conversion for maximum performance
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal protobuf: %w", err)
	}

	// Use streaming processor directly
	processor := NewStreamingTelemetryProcessor(h.eventPublisher, TelemetryTrace)
	if err := processor.ProcessTelemetryData(ctx, data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to process trace data: %w", err)
	}

	return &tracecollectorpb.ExportTraceServiceResponse{}, nil
}

// GetTrace retrieves a trace by ID using ClickHouse
// @Summary Get trace
// @Description Get a trace by its ID
// @Tags traces
// @Produce json
// @Param id path string true "Trace ID"
// @Success 200 {object} TraceResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/traces/{id} [get]
func (h *TraceHandler) GetTrace(c *gin.Context) {
	traceID := c.Param("id")

	if traceID == "" {
		pkgerrors.AbortBadRequest(c, "Trace ID parameter is required")
		return
	}

	// If no ClickHouse client available, return error
	if h.queryService == nil || h.queryService.clickhouse == nil {
		logger.Error("ClickHouse not available for trace queries",
			zap.String("trace_id", traceID))
		pkgerrors.AbortServiceUnavailable(c, "trace-query-service", 120)
		return
	}

	// Query trace from ClickHouse
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	trace, err := h.queryService.GetTraceByID(ctx, traceID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			pkgerrors.AbortNotFound(c, "trace")
		} else {
			logger.Error("Failed to query trace",
				zap.String("trace_id", traceID),
				zap.Error(err))
			pkgerrors.AbortDatabaseError(c, "trace_query", err)
		}
		return
	}

	// Convert to response format
	response := TraceResponse{
		ID:        trace.TraceID,
		TraceID:   trace.TraceID,
		CreatedAt: trace.StartTime,
	}

	c.JSON(http.StatusOK, response)
}

type CreateTraceRequest struct {
	TraceID   string            `json:"trace_id" example:"abc123" binding:"required"`
	SpanID    string            `json:"span_id" example:"def456" binding:"required"`
	Timestamp time.Time         `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Tags      map[string]string `json:"tags" example:"service:api,version:1.0"`
}

type TraceResponse struct {
	ID          string    `json:"id" example:"1"`
	TraceID     string    `json:"trace_id" example:"abc123"`
	CreatedAt   time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
	Status      string    `json:"status,omitempty" example:"accepted"`
	Message     string    `json:"message,omitempty" example:"Successfully processed"`
	ServiceName string    `json:"service_name,omitempty" example:"my-service"`
}

type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request"`
	Message string `json:"message" example:"Field validation failed"`
}

// extractServiceNameAndTraceID performs a single parse to extract both
// service name and trace ID from OTLP trace data, eliminating repeated unmarshalling.
func (h *TraceHandler) extractServiceNameAndTraceID(data json.RawMessage) (string, string) {
	var otlpStructure struct {
		ResourceSpans []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeSpans []struct {
				Spans []struct {
					TraceID string `json:"traceId"`
				} `json:"spans"`
			} `json:"scopeSpans"`
			// Legacy format support for backward compatibility
			InstrumentationLibrarySpans []struct {
				Spans []struct {
					TraceID string `json:"traceId"`
				} `json:"spans"`
			} `json:"instrumentationLibrarySpans"`
		} `json:"resourceSpans"`
	}

	if err := json.Unmarshal(data, &otlpStructure); err != nil {
		logger.Debug("Failed to parse OTLP for metadata extraction",
			zap.Error(err))
		return DefaultServiceName, ""
	}

	// Extract service name
	serviceName := h.findServiceNameInResourceSpans(otlpStructure.ResourceSpans)
	if serviceName == "" {
		serviceName = DefaultServiceName
	}

	// Extract trace ID using the same parsed structure
	traceID := h.findTraceIDInResourceSpansOptimized(otlpStructure.ResourceSpans)

	return serviceName, traceID
}

// findTraceIDInResourceSpansOptimized searches for the first valid trace ID using the combined structure.
func (h *TraceHandler) findTraceIDInResourceSpansOptimized(resourceSpans []struct {
	Resource struct {
		Attributes []struct {
			Key   string `json:"key"`
			Value struct {
				StringValue string `json:"stringValue"`
			} `json:"value"`
		} `json:"attributes"`
	} `json:"resource"`
	ScopeSpans []struct {
		Spans []struct {
			TraceID string `json:"traceId"`
		} `json:"spans"`
	} `json:"scopeSpans"`
	InstrumentationLibrarySpans []struct {
		Spans []struct {
			TraceID string `json:"traceId"`
		} `json:"spans"`
	} `json:"instrumentationLibrarySpans"`
}) string {
	for _, rs := range resourceSpans {
		// Check modern scopeSpans format first
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				if span.TraceID != "" {
					return span.TraceID
				}
			}
		}

		// Fall back to legacy instrumentationLibrarySpans format
		for _, ils := range rs.InstrumentationLibrarySpans {
			for _, span := range ils.Spans {
				if span.TraceID != "" {
					return span.TraceID
				}
			}
		}
	}
	return ""
}

// findServiceNameInResourceSpans searches for service.name attribute in resource spans.
func (h *TraceHandler) findServiceNameInResourceSpans(resourceSpans []struct {
	Resource struct {
		Attributes []struct {
			Key   string `json:"key"`
			Value struct {
				StringValue string `json:"stringValue"`
			} `json:"value"`
		} `json:"attributes"`
	} `json:"resource"`
	ScopeSpans []struct {
		Spans []struct {
			TraceID string `json:"traceId"`
		} `json:"spans"`
	} `json:"scopeSpans"`
	InstrumentationLibrarySpans []struct {
		Spans []struct {
			TraceID string `json:"traceId"`
		} `json:"spans"`
	} `json:"instrumentationLibrarySpans"`
}) string {
	const serviceNameKey = "service.name"

	for _, rs := range resourceSpans {
		for _, attr := range rs.Resource.Attributes {
			if attr.Key == serviceNameKey && attr.Value.StringValue != "" {
				return attr.Value.StringValue
			}
		}
	}

	return ""
}

// generateID creates a unique identifier based on current timestamp
// with nanosecond precision and atomic counter for guaranteed uniqueness.
// Optimized to avoid fmt.Sprintf allocation overhead on hot path.
func generateID() string {
	// Add atomic counter to nanosecond timestamp for guaranteed uniqueness
	timestamp := time.Now().UnixNano()
	counter := atomic.AddInt64(&idCounter, 1)

	// Use strconv for better performance than fmt.Sprintf
	timestampStr := strconv.FormatInt(timestamp, 10)
	counterStr := strconv.FormatInt(counter%1000, 10)

	// Pre-pad counter to 3 digits
	switch len(counterStr) {
	case 1:
		return timestampStr + "00" + counterStr
	case 2:
		return timestampStr + "0" + counterStr
	default:
		return timestampStr + counterStr
	}
}

// Helper functions for gRPC protobuf handling

// extractServiceNameFromTraceRequest extracts service name from protobuf request
func extractServiceNameFromTraceRequest(req *tracecollectorpb.ExportTraceServiceRequest) string {
	for _, resourceSpan := range req.ResourceSpans {
		if resourceSpan.Resource != nil {
			for _, attr := range resourceSpan.Resource.Attributes {
				if attr.Key == "service.name" {
					if stringValue := attr.Value.GetStringValue(); stringValue != "" {
						return stringValue
					}
				}
			}
		}
	}
	return "unknown"
}

// extractTraceIDFromTraceRequest extracts the first trace ID from protobuf request
func extractTraceIDFromTraceRequest(req *tracecollectorpb.ExportTraceServiceRequest) string {
	for _, resourceSpan := range req.ResourceSpans {
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			for _, span := range scopeSpan.Spans {
				if len(span.TraceId) > 0 {
					return fmt.Sprintf("%x", span.TraceId)
				}
			}
		}
	}
	return ""
}

// countSpansInRequest counts total spans in protobuf request
func countSpansInRequest(req *tracecollectorpb.ExportTraceServiceRequest) int {
	count := 0
	for _, resourceSpan := range req.ResourceSpans {
		for _, scopeSpan := range resourceSpan.ScopeSpans {
			count += len(scopeSpan.Spans)
		}
	}
	return count
}
