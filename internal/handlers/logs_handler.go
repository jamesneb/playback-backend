package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/handlers/services"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/metrics"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type LogsHandler struct {
	*BaseTelemetryHandler
	queryService services.LogsQueryService
}

func NewLogsHandler(eventPublisher telemetry.EventPublisher, resilienceComponents *interfaces.ResilienceComponents) *LogsHandler {
	baseHandler := NewBaseTelemetryHandler(
		eventPublisher,
		&LogMetadataExtractor{},
		NewStreamingTelemetryProcessor(eventPublisher, TelemetryLog),
		TelemetryLog,
	)

	return &LogsHandler{
		BaseTelemetryHandler: baseHandler,
		queryService:         services.NewDefaultLogsQueryService(),
	}
}

func NewLogsHandlerWithClickHouse(eventPublisher telemetry.EventPublisher, clickhouse *storage.ClickHouseClient, resilienceComponents *interfaces.ResilienceComponents) *LogsHandler {
	baseHandler := NewBaseTelemetryHandler(
		eventPublisher,
		&LogMetadataExtractor{},
		NewStreamingTelemetryProcessor(eventPublisher, TelemetryLog),
		TelemetryLog,
	)

	return &LogsHandler{
		BaseTelemetryHandler: baseHandler,
		queryService:         services.NewClickHouseLogsQueryService(clickhouse),
	}
}

// CreateLogs - hot path HTTP ingestion using consolidated base handler
// @Summary Receive logs
// @Description Receive log data from OpenTelemetry
// @Tags logs
// @Accept json
// @Produce json
// @Param logs body schema.LogsRequest true "Log data"
// @Success 200 {object} dto.LogsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/logs [post]
func (h *LogsHandler) CreateLogs(c *gin.Context) {
	h.HandleIngestion(c)
}

// CreateLogsGRPC - zero-allocation gRPC ingestion bypassing JSON entirely
func (h *LogsHandler) CreateLogsGRPC(ctx context.Context, req *logscollectorpb.ExportLogsServiceRequest) (*logscollectorpb.ExportLogsServiceResponse, error) {
	// Zero-copy protobuf serialization
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Use base handler's metadata extraction (no duplication)
	metadata := h.extractor.ExtractMetadata(data)
	metadata.DataSize = int32(len(data))

	// Hot path processing with optimized timeout
	processCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := h.processor.ProcessTelemetryData(processCtx, data, &metadata); err != nil {
		return nil, err
	}

	return &logscollectorpb.ExportLogsServiceResponse{}, nil
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
// @Success 200 {object} dto.LogsQueryResponse
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

	// Record query metrics
	queryStart := time.Now()
	response, err := h.queryService.QueryLogs(c.Request.Context(), params)
	queryLatency := time.Since(queryStart).Seconds()

	if err != nil {
		// Record query failure metrics
		metrics.Global().RecordBusinessError("query", params.Service, "error")
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

	// Record successful query metrics - estimate response size for business tracking
	responseSize := len(response.Logs) * 150 // Rough estimate per log record
	metricsRegistry := metrics.Global()
	metricsRegistry.RecordDataQuery("logs", "query", float64(responseSize), queryLatency)
	metricsRegistry.RecordStorageOperation("read", "success")

	c.JSON(http.StatusOK, response)
}

// Keep existing helper function for backward compatibility
// The base handler now uses this via LogMetadataExtractor
func extractLogsMetadata(data json.RawMessage) (string, string, int) {
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
		if serviceName == "unknown" {
			for _, attr := range rl.Resource.Attributes {
				if attr.Key == "service.name" && attr.Value.StringValue != "" {
					serviceName = attr.Value.StringValue
					break
				}
			}
		}
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
