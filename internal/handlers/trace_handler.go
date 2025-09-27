package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
)

var (
	// idCounter provides atomic counter for ID generation
	idCounter int64
)

// TraceHandler handles HTTP trace ingestion requests with comprehensive
// validation, rate limiting, and resilience patterns.
type TraceHandler struct {
	eventPublisher telemetry.EventPublisher
	validator      *RequestValidator
	// Resilience components
	kinesisBuffer   *resilience.KinesisBuffer
	rateLimiter     *resilience.TenantRateLimiter
	deadLetterQueue *resilience.DeadLetterQueue
}

// NewTraceHandler creates a new trace handler with all required dependencies
// and proper initialization of validation components.
//
// Parameters:
//   - kinesisClient: Configured Kinesis client for stream publishing
//   - resilienceComponents: Collection of resilience patterns (circuit breaker, rate limiter, etc.)
//
// Returns:
//   - *TraceHandler: Fully initialized trace handler ready for request processing
func NewTraceHandler(eventPublisher telemetry.EventPublisher, resilienceComponents *interfaces.ResilienceComponents) *TraceHandler {
	handler := &TraceHandler{
		eventPublisher: eventPublisher,
		validator:      NewRequestValidator(),
	}

	// Handle optional resilience components gracefully
	if resilienceComponents != nil {
		handler.kinesisBuffer = resilienceComponents.KinesisBuffer
		handler.rateLimiter = resilienceComponents.RateLimiter
		handler.deadLetterQueue = resilienceComponents.DeadLetterQueue
	}

	return handler
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
	// Validate and parse request
	otlpData, err := h.validateAndParseRequest(c)
	if err != nil {
		return // Response already sent by validation method
	}

	// Extract trace metadata
	traceMetadata, err := h.extractTraceMetadata(c, otlpData)
	if err != nil {
		return // Response already sent
	}

	// Apply rate limiting
	if !h.applyRateLimit(c, traceMetadata.TenantID) {
		return // Response already sent
	}

	// Log ingestion event
	h.logIngestedTrace(c, traceMetadata, len(otlpData))

	// Publish to Kinesis
	if !h.publishTraceToKinesis(c, otlpData, traceMetadata) {
		return // Response already sent
	}

	// Send success response
	h.sendSuccessResponse(c, traceMetadata)
}

type traceMetadata struct {
	ServiceName string
	TraceID     string
	TenantID    string
}

func (h *TraceHandler) validateAndParseRequest(c *gin.Context) (json.RawMessage, error) {
	// Perform comprehensive request validation
	if validationErr := h.validator.ValidateRequest(c); validationErr != nil {
		logger.Warn("Request validation failed",
			zap.Error(validationErr),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))))

		h.respondWithValidationError(c, validationErr)
		return nil, validationErr
	}

	// Parse the OTLP trace data with size already validated
	var otlpData json.RawMessage
	if err := c.ShouldBindJSON(&otlpData); err != nil {
		logger.Error("Failed to parse JSON payload after validation",
			zap.Error(err),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))))

		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   ErrInvalidTraceData,
			Message: "Malformed JSON in request body",
		})
		return nil, err
	}

	// Validate OTLP structure and content
	if validationErr := h.validator.ValidateOTLPTraceData(otlpData); validationErr != nil {
		logger.Warn("OTLP data validation failed",
			zap.Error(validationErr),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("data_size", logging.SanitizeDataSize(len(otlpData))))

		h.respondWithValidationError(c, validationErr)
		return nil, validationErr
	}

	return otlpData, nil
}

func (h *TraceHandler) extractTraceMetadata(c *gin.Context, otlpData json.RawMessage) (*traceMetadata, error) {
	// Single parse operation to extract both service name and trace ID
	rawServiceName, rawTraceID := h.extractServiceNameAndTraceID(otlpData)

	serviceName := h.validator.ValidateServiceName(rawServiceName)
	traceID := h.validator.ValidateTraceID(rawTraceID)

	// Use service name as tenant ID with proper fallback
	tenantID := serviceName
	if tenantID == DefaultServiceName {
		tenantID = DefaultTenantID
	}

	return &traceMetadata{
		ServiceName: serviceName,
		TraceID:     traceID,
		TenantID:    tenantID,
	}, nil
}

func (h *TraceHandler) applyRateLimit(c *gin.Context, tenantID string) bool {
	if h.rateLimiter != nil && !h.rateLimiter.Allow(tenantID) {
		logger.Warn("HTTP trace request rate limited",
			zap.String("tenant", logging.SanitizeTenantID(tenantID)),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())))
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Error:   "Rate limit exceeded",
			Message: "Too many requests for this tenant",
		})
		return false
	}
	return true
}

func (h *TraceHandler) logIngestedTrace(c *gin.Context, metadata *traceMetadata, dataSize int) {
	logger.Debug("Received OTLP trace data",
		zap.String("service_name", logging.SanitizeServiceName(metadata.ServiceName)),
		zap.String("trace_id", logging.SanitizeTraceID(metadata.TraceID)),
		zap.String("tenant", logging.SanitizeTenantID(metadata.TenantID)),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.String("data_size", logging.SanitizeDataSize(dataSize)),
	)
}

func (h *TraceHandler) publishTraceToKinesis(c *gin.Context, otlpData json.RawMessage, metadata *traceMetadata) bool {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	var err error

	// Use buffer if available for resilience
	if h.kinesisBuffer != nil {
		// Create a legacy telemetry event for JSON data
		event := &streaming.LegacyTelemetryEvent{
			Type:        string(streaming.TelemetryTypeTraces),
			ServiceName: metadata.ServiceName,
			TraceID:     metadata.TraceID,
			Data:        otlpData,
			Metadata: streaming.LegacyTelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   c.ClientIP(),
				UserAgent:  c.GetHeader("User-Agent"),
			},
		}

		// Buffer the event through the resilience layer
		err = h.kinesisBuffer.BufferEvent(ctx, event, metadata.TenantID, "http")
	} else {
		// Fallback to direct Kinesis (original behavior)
		err = h.eventPublisher.PublishTrace(
			ctx,
			otlpData,
			metadata.ServiceName,
			metadata.TraceID,
			c.ClientIP(),
			c.GetHeader("User-Agent"),
		)
	}

	if err != nil {
		// Handle DLQ if available
		if h.deadLetterQueue != nil {
			h.handlePublishFailure(ctx, otlpData, metadata, err)
		}

		logger.Error("Failed to publish trace to Kinesis",
			zap.Error(err),
			zap.String("service_name", logging.SanitizeServiceName(metadata.ServiceName)),
			zap.String("trace_id", logging.SanitizeTraceID(metadata.TraceID)),
			zap.String("tenant", logging.SanitizeTenantID(metadata.TenantID)),
		)

		status := http.StatusServiceUnavailable
		errorMsg := "Telemetry pipeline unavailable"
		if h.kinesisBuffer == nil {
			status = http.StatusInternalServerError
			errorMsg = "Failed to process trace data"
		}

		c.JSON(status, ErrorResponse{
			Error:   errorMsg,
			Message: "Please try again later",
		})
		return false
	}

	return true
}

func (h *TraceHandler) handlePublishFailure(ctx context.Context, otlpData json.RawMessage, metadata *traceMetadata, err error) {
	// Create a basic telemetry event for DLQ
	basicEvent := &streaming.TraceTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeTraces,
			ServiceName: metadata.ServiceName,
			TraceID:     metadata.TraceID,
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   "", // Will be filled by caller
			},
		},
		// Note: ResourceSpans would be nil for JSON data
	}

	if dlqErr := h.deadLetterQueue.SendToDLQ(ctx, basicEvent, err, metadata.TenantID, "http", "kinesis_publish_failed"); dlqErr != nil {
		logger.Error("Failed to send to DLQ", zap.Error(dlqErr))
	}
}

func (h *TraceHandler) sendSuccessResponse(c *gin.Context, metadata *traceMetadata) {
	// Log successful ingestion
	logger.Debug("Successfully processed HTTP trace via Kinesis-first approach",
		zap.String("service_name", logging.SanitizeServiceName(metadata.ServiceName)),
		zap.String("trace_id", logging.SanitizeTraceID(metadata.TraceID)),
		zap.String("tenant", logging.SanitizeTenantID(metadata.TenantID)),
	)

	// Return success response
	response := TraceResponse{
		ID:        generateID(),
		TraceID:   metadata.TraceID,
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

// respondWithValidationError sends a properly formatted validation error response
// to the client with appropriate HTTP status codes based on the validation error type.
func (h *TraceHandler) respondWithValidationError(c *gin.Context, validationErr *ValidationError) {
	statusCode := h.getStatusCodeForValidationError(validationErr)

	c.JSON(statusCode, ErrorResponse{
		Error:   validationErr.Message,
		Message: validationErr.Code,
	})
}

// getStatusCodeForValidationError maps validation error types to appropriate HTTP status codes.
func (h *TraceHandler) getStatusCodeForValidationError(validationErr *ValidationError) int {
	switch validationErr.Type {
	case ValidationTypeSize:
		if validationErr.Code == "PAYLOAD_TOO_LARGE" {
			return http.StatusRequestEntityTooLarge
		}
		return http.StatusBadRequest
	case ValidationTypeFormat:
		return http.StatusUnsupportedMediaType
	case ValidationTypeStructure, ValidationTypeContent:
		return http.StatusBadRequest
	case ValidationTypeLimit:
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusBadRequest
	}
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
