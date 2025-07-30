package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

type LogsHandler struct{
	kinesisClient *streaming.KinesisClient
}

func NewLogsHandler(kinesisClient *streaming.KinesisClient) *LogsHandler {
	return &LogsHandler{
		kinesisClient: kinesisClient,
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
	// Parse the OTLP logs data (raw JSON)
	var otlpData json.RawMessage
	if err := c.ShouldBindJSON(&otlpData); err != nil {
		logger.Error("Failed to parse OTLP logs data",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.GetHeader("User-Agent")),
		)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid OTLP logs data",
			Message: err.Error(),
		})
		return
	}

	// Extract service name and trace ID for logging and partitioning
	serviceName := extractLogsServiceName(otlpData)
	traceID := extractLogsTraceID(otlpData)
	logsCount := countLogs(otlpData)

	// Log the ingestion event
	logger.Info("Received OTLP logs data",
		zap.String("service_name", serviceName),
		zap.String("trace_id", traceID),
		zap.Int("logs_count", logsCount),
		zap.String("client_ip", c.ClientIP()),
		zap.String("user_agent", c.GetHeader("User-Agent")),
		zap.Int("data_size_bytes", len(otlpData)),
	)

	// Publish to Kinesis
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := h.kinesisClient.PublishLogs(
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
	logger.Info("Successfully published logs to Kinesis",
		zap.String("service_name", serviceName),
		zap.String("trace_id", traceID),
		zap.Int("logs_count", logsCount),
	)

	// Return success response
	response := LogsResponse{
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
	service := c.Query("service")
	level := c.Query("level")
	from := c.Query("from")
	to := c.Query("to")
	query := c.Query("q")

	// Mock response for now
	response := LogsQueryResponse{
		Service:   service,
		Level:     level,
		TimeRange: TimeRange{From: from, To: to},
		Query:     query,
		Logs: []LogEntry{
			{
				Timestamp: time.Now().Add(-time.Minute * 5),
				Level:     "INFO",
				Message:   "Order creation started",
				Service:   "order-service",
				TraceID:   "abc123def456",
				SpanID:    "789xyz",
				Attributes: map[string]interface{}{
					"endpoint": "/orders",
					"method":   "POST",
				},
			},
			{
				Timestamp: time.Now().Add(-time.Minute * 3),
				Level:     "WARN",
				Message:   "Order failed - insufficient inventory",
				Service:   "order-service",
				TraceID:   "def456ghi789",
				SpanID:    "xyz123",
				Attributes: map[string]interface{}{
					"order_id":           "order_1234567890",
					"product_id":         "prod_001",
					"requested_quantity": 5,
				},
			},
		},
	}

	c.JSON(http.StatusOK, response)
}

// OpenTelemetry Logs Protocol structures
type LogsRequest struct {
	ResourceLogs []ResourceLog `json:"resourceLogs"`
}

type ResourceLog struct {
	Resource  Resource   `json:"resource"`
	ScopeLogs []ScopeLog `json:"scopeLogs"`
	SchemaURL string     `json:"schemaUrl,omitempty"`
}

type ScopeLog struct {
	Scope      Scope       `json:"scope"`
	LogRecords []LogRecord `json:"logRecords"`
}

type LogRecord struct {
	TimeUnixNano           uint64        `json:"timeUnixNano"`
	ObservedTimeUnixNano   uint64        `json:"observedTimeUnixNano,omitempty"`
	SeverityNumber         int32         `json:"severityNumber,omitempty"`
	SeverityText           string        `json:"severityText,omitempty"`
	Body                   LogRecordBody `json:"body,omitempty"`
	Attributes             []Attribute   `json:"attributes,omitempty"`
	DroppedAttributesCount uint32        `json:"droppedAttributesCount,omitempty"`
	Flags                  uint32        `json:"flags,omitempty"`
	TraceID                string        `json:"traceId,omitempty"`
	SpanID                 string        `json:"spanId,omitempty"`
}

type LogRecordBody struct {
	StringValue *string `json:"stringValue,omitempty"`
}

// Response types
type LogsResponse struct {
	Received  int       `json:"received" example:"10"`
	Timestamp time.Time `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Status    string    `json:"status" example:"accepted"`
}

type LogsQueryResponse struct {
	Service   string     `json:"service" example:"order-service"`
	Level     string     `json:"level" example:"INFO"`
	TimeRange TimeRange  `json:"time_range"`
	Query     string     `json:"query" example:"order failed"`
	Logs      []LogEntry `json:"logs"`
}

type LogEntry struct {
	Timestamp  time.Time              `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Level      string                 `json:"level" example:"INFO"`
	Message    string                 `json:"message" example:"Order creation started"`
	Service    string                 `json:"service" example:"order-service"`
	TraceID    string                 `json:"trace_id,omitempty" example:"abc123def456"`
	SpanID     string                 `json:"span_id,omitempty" example:"789xyz"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// Helper functions for extracting metadata from OTLP logs data
func extractLogsServiceName(data json.RawMessage) string {
	// Parse OTLP logs structure to extract service name
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
		} `json:"resourceLogs"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return "unknown"
	}

	for _, rl := range otlp.ResourceLogs {
		for _, attr := range rl.Resource.Attributes {
			if attr.Key == "service.name" {
				return attr.Value.StringValue
			}
		}
	}

	return "unknown"
}

func extractLogsTraceID(data json.RawMessage) string {
	// Parse OTLP logs structure to extract trace ID
	var otlp struct {
		ResourceLogs []struct {
			ScopeLogs []struct {
				LogRecords []struct {
					TraceID string `json:"traceId"`
				} `json:"logRecords"`
			} `json:"scopeLogs"`
		} `json:"resourceLogs"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return ""
	}

	for _, rl := range otlp.ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			for _, logRecord := range sl.LogRecords {
				if logRecord.TraceID != "" {
					return logRecord.TraceID
				}
			}
		}
	}

	return ""
}

func countLogs(data json.RawMessage) int {
	// Parse OTLP logs structure to count log records
	var otlp struct {
		ResourceLogs []struct {
			ScopeLogs []struct {
				LogRecords []struct {
					Body struct {
						StringValue string `json:"stringValue"`
					} `json:"body"`
				} `json:"logRecords"`
			} `json:"scopeLogs"`
		} `json:"resourceLogs"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return 0
	}

	count := 0
	for _, rl := range otlp.ResourceLogs {
		for _, sl := range rl.ScopeLogs {
			count += len(sl.LogRecords)
		}
	}

	return count
}
