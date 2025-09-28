package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/handlers/dto"
	"github.com/jamesneb/playback-backend/internal/handlers/services"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
)

type LogsHandler struct {
	eventPublisher telemetry.EventPublisher
	queryService   services.LogsQueryService
}

func NewLogsHandler(eventPublisher telemetry.EventPublisher) *LogsHandler {
	return &LogsHandler{
		eventPublisher: eventPublisher,
		queryService:   services.NewDefaultLogsQueryService(),
	}
}

// CreateLogs receives log data
// @Summary Receive logs
// @Description Receive log data from OpenTelemetry
// @Tags logs
// @Accept json
// @Produce json
// @Param logs body LogsRequest true "Log data"
// @Success 200 {object} LogsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/logs [post]
func (h *LogsHandler) CreateLogs(c *gin.Context) {
	// Validate content type
	contentType := c.GetHeader("Content-Type")
	if !strings.Contains(contentType, ContentTypeJSON) {
		logger.Warn("Invalid content type received",
			zap.String("content_type", contentType),
			zap.String("expected", ContentTypeJSON),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid content type",
			Message: "Content-Type must be application/json",
		})
		return
	}

	// Parse the OTLP logs data (raw JSON)
	var otlpData json.RawMessage
	if err := c.ShouldBindJSON(&otlpData); err != nil {
		logger.Error("Failed to parse OTLP logs data",
			zap.Error(err),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid OTLP logs data",
			Message: err.Error(),
		})
		return
	}

	// Extract service name, trace ID, and logs count with single parse operation
	serviceName, traceID, logsCount := extractLogsMetadata(otlpData)

	// Log the ingestion event
	logger.Debug("Received OTLP logs data",
		zap.String("service_name", logging.SanitizeServiceName(serviceName)),
		zap.String("trace_id", logging.SanitizeTraceID(traceID)),
		zap.Int("logs_count", logsCount),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.String("data_size", logging.SanitizeDataSize(len(otlpData))),
	)

	// Publish to Kinesis
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	err := h.eventPublisher.PublishLogs(
		ctx,
		otlpData,
		serviceName,
		traceID,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		logger.Error("Failed to publish logs to Kinesis",
			zap.Error(err),
			zap.String("service_name", serviceName),
			zap.String("trace_id", traceID),
			zap.Int("logs_count", logsCount),
		)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to process logs data",
			Message: "Internal server error",
		})
		return
	}

	// Log successful ingestion
	logger.Debug("Successfully published logs to Kinesis",
		zap.String("service_name", serviceName),
		zap.String("trace_id", traceID),
		zap.Int("logs_count", logsCount),
	)

	// Return success response
	response := dto.LogsResponse{
		Received:  logsCount,
		Timestamp: time.Now(),
		Status:    "accepted",
	}

	c.JSON(http.StatusAccepted, response)
}

// GetLogs retrieves logs (placeholder for querying)
// @Summary Get logs
// @Description Get log data for analysis
// @Tags logs
// @Produce json
// @Param service query string false "Service name"
// @Param level query string false "Log level"
// @Param from query string false "Start time (RFC3339)"
// @Param to query string false "End time (RFC3339)"
// @Param q query string false "Search query"
// @Success 200 {object} LogsQueryResponse
// @Router /api/v1/logs [get]
func (h *LogsHandler) GetLogs(c *gin.Context) {
	params := services.LogsQueryParams{
		Service: c.Query("service"),
		Level:   c.Query("level"),
		From:    c.Query("from"),
		To:      c.Query("to"),
		Query:   c.Query("q"),
		Limit:   100, // Default limit
		Offset:  0,   // Default offset
	}

	response, err := h.queryService.QueryLogs(params)
	if err != nil {
		logger.Error("Failed to query logs",
			zap.Error(err),
			zap.String("service", params.Service),
			zap.String("level", params.Level),
		)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to query logs",
			Message: "Internal server error",
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// Helper functions for extracting metadata from OTLP logs data
func extractLogsMetadata(data json.RawMessage) (string, string, int) {
	// Parse OTLP logs structure once to extract service name, trace ID, and count
	var otlp struct {
		ResourceLogs []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeLogs []struct {
				LogRecords []struct {
					TraceID string `json:"traceId"`
					Body    struct {
						StringValue string `json:"stringValue"`
					} `json:"body"`
				} `json:"logRecords"`
			} `json:"scopeLogs"`
		} `json:"resourceLogs"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return "unknown", "", 0
	}

	serviceName := "unknown"
	traceID := ""
	logsCount := 0

	for _, rl := range otlp.ResourceLogs {
		// Extract service name from first ResourceLog with service.name attribute
		if serviceName == "unknown" {
			for _, attr := range rl.Resource.Attributes {
				if attr.Key == "service.name" && attr.Value.StringValue != "" {
					serviceName = attr.Value.StringValue
					break
				}
			}
		}

		// Extract trace ID from first log record and count all log records
		for _, sl := range rl.ScopeLogs {
			for _, logRecord := range sl.LogRecords {
				if traceID == "" && logRecord.TraceID != "" {
					traceID = logRecord.TraceID
				}
			}
			logsCount += len(sl.LogRecords)
		}
	}

	return serviceName, traceID, logsCount
}
