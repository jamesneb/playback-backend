package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

type TraceHandler struct{
	kinesisClient   *streaming.KinesisClient
	// Resilience components
	kinesisBuffer   *resilience.KinesisBuffer
	rateLimiter    *resilience.TenantRateLimiter
	deadLetterQueue *resilience.DeadLetterQueue
}

func NewTraceHandler(kinesisClient *streaming.KinesisClient, resilienceComponents *ResilienceComponents) *TraceHandler {
	return &TraceHandler{
		kinesisClient:   kinesisClient,
		kinesisBuffer:   resilienceComponents.KinesisBuffer,
		rateLimiter:    resilienceComponents.RateLimiter,
		deadLetterQueue: resilienceComponents.DeadLetterQueue,
	}
}

// ResilienceComponents groups resilience-related dependencies for HTTP handlers
type ResilienceComponents struct {
	KinesisBuffer   *resilience.KinesisBuffer
	RateLimiter     *resilience.TenantRateLimiter
	DeadLetterQueue *resilience.DeadLetterQueue
}

// CreateTrace creates a new trace
// @Summary Create trace
// @Description Create a new distributed trace
// @Tags traces
// @Accept json
// @Produce json
// @Param trace body CreateTraceRequest true "Trace data"
// @Success 201 {object} TraceResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/traces [post]
func (h *TraceHandler) CreateTrace(c *gin.Context) {
	// Parse the OTLP trace data (raw JSON)
	var otlpData json.RawMessage
	if err := c.ShouldBindJSON(&otlpData); err != nil {
		logger.Error("Failed to parse OTLP trace data",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.GetHeader("User-Agent")),
		)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid OTLP trace data",
			Message: err.Error(),
		})
		return
	}

	// Extract service name and trace ID for logging and partitioning
	serviceName := extractServiceName(otlpData)
	traceID := extractTraceID(otlpData)
	
	// Use service name as tenant ID (you might want a more sophisticated mapping)
	tenantID := serviceName
	if tenantID == "" {
		tenantID = "default"
	}

	// Apply tenant rate limiting
	if h.rateLimiter != nil && !h.rateLimiter.Allow(tenantID) {
		logger.Warn("HTTP trace request rate limited",
			zap.String("tenant", tenantID),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Error:   "Rate limit exceeded",
			Message: "Too many requests for this tenant",
		})
		return
	}

	// Log the ingestion event
	logger.Info("Received OTLP trace data",
		zap.String("service_name", serviceName),
		zap.String("trace_id", traceID),
		zap.String("tenant", tenantID),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.Int("data_size_bytes", len(otlpData)),
	)

	// Create legacy telemetry event for HTTP JSON data
	event := &streaming.LegacyTelemetryEvent{
		Type:        "traces",
		Data:        otlpData,
		ServiceName: serviceName,
		TraceID:     traceID,
		Metadata: streaming.LegacyTelemetryMetadata{
			IngestedAt: time.Now(),
			SourceIP:   c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// KINESIS-FIRST: Use resilient buffer if available
	if h.kinesisBuffer != nil {
		// Convert to proper telemetry event interface (this needs to be implemented)
		// For now, fallback to direct Kinesis
		err := h.kinesisClient.PublishTrace(
			ctx,
			otlpData,
			serviceName,
			traceID,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
		)
		if err != nil {
			// Send to DLQ if available
			if h.deadLetterQueue != nil {
				// Create a basic telemetry event for DLQ
				basicEvent := &streaming.TraceTelemetryEvent{
					BaseTelemetryEvent: streaming.BaseTelemetryEvent{
						Type:        streaming.TelemetryTypeTraces,
						ServiceName: serviceName,
						TraceID:     traceID,
						Metadata: streaming.TelemetryMetadata{
						IngestedAt: event.Metadata.IngestedAt,
						SourceIP:   event.Metadata.SourceIP,
					},
					},
					// Note: ResourceSpans would be nil for JSON data
				}
				
				if dlqErr := h.deadLetterQueue.SendToDLQ(ctx, basicEvent, err, tenantID, "http", "kinesis_publish_failed"); dlqErr != nil {
					logger.Error("Failed to send to DLQ", zap.Error(dlqErr))
				}
			}
			
			logger.Error("Failed to publish trace to Kinesis",
				zap.Error(err),
				zap.String("service_name", serviceName),
				zap.String("trace_id", traceID),
				zap.String("tenant", tenantID),
			)
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{
				Error:   "Telemetry pipeline unavailable",
				Message: "Please try again later",
			})
			return
		}
	} else {
		// Fallback to direct Kinesis (original behavior)
		err := h.kinesisClient.PublishTrace(
			ctx,
			otlpData,
			serviceName,
			traceID,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
		)
		if err != nil {
			logger.Error("Failed to publish trace to Kinesis",
				zap.Error(err),
				zap.String("service_name", serviceName),
				zap.String("trace_id", traceID),
			)
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Failed to process trace data",
				Message: "Internal server error",
			})
			return
		}
	}

	// Log successful ingestion
	logger.Info("Successfully processed HTTP trace via Kinesis-first approach",
		zap.String("service_name", serviceName),
		zap.String("trace_id", traceID),
		zap.String("tenant", tenantID),
	)

	// Return success response
	response := TraceResponse{
		ID:        generateID(),
		TraceID:   traceID,
		CreatedAt: time.Now(),
	}

	c.JSON(http.StatusAccepted, response)
}

// GetTrace retrieves a trace by ID
// @Summary Get trace
// @Description Get a trace by its ID
// @Tags traces
// @Produce json
// @Param id path string true "Trace ID"
// @Success 200 {object} TraceResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/traces/{id} [get]
func (h *TraceHandler) GetTrace(c *gin.Context) {
	id := c.Param("id")

	response := TraceResponse{
		ID:        id,
		TraceID:   "sample-trace-" + id,
		CreatedAt: time.Now(),
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
	ID        string    `json:"id" example:"1"`
	TraceID   string    `json:"trace_id" example:"abc123"`
	CreatedAt time.Time `json:"created_at" example:"2023-01-01T00:00:00Z"`
}

type ErrorResponse struct {
	Error   string `json:"error" example:"Invalid request"`
	Message string `json:"message" example:"Field validation failed"`
}

// Helper functions for extracting metadata from OTLP data
func extractServiceName(data json.RawMessage) string {
	// Parse OTLP structure to extract service name
	var otlp struct {
		ResourceSpans []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
		} `json:"resourceSpans"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return "unknown"
	}

	for _, rs := range otlp.ResourceSpans {
		for _, attr := range rs.Resource.Attributes {
			if attr.Key == "service.name" && attr.Value.StringValue != "" {
				return attr.Value.StringValue
			}
		}
	}

	return "unknown"
}

func extractTraceID(data json.RawMessage) string {
	// Parse OTLP structure to extract trace ID - support both new and legacy formats
	var otlp struct {
		ResourceSpans []struct {
			ScopeSpans []struct {
				Spans []struct {
					TraceID string `json:"traceId"`
				} `json:"spans"`
			} `json:"scopeSpans"`
			// Legacy format support
			InstrumentationLibrarySpans []struct {
				Spans []struct {
					TraceID string `json:"traceId"`
				} `json:"spans"`
			} `json:"instrumentationLibrarySpans"`
		} `json:"resourceSpans"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return ""
	}

	for _, rs := range otlp.ResourceSpans {
		// Check modern scopeSpans format
		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				if span.TraceID != "" {
					return span.TraceID
				}
			}
		}
		// Check legacy instrumentationLibrarySpans format
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

func generateID() string {
	return strings.ReplaceAll(time.Now().Format("20060102150405.000000"), ".", "")
}
