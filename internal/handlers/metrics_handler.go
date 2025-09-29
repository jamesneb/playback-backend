package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/handlers/services"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/internal/metrics"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

type MetricsHandler struct {
	*BaseTelemetryHandler
	queryService services.MetricsQueryService
}

func NewMetricsHandler(eventPublisher telemetry.EventPublisher, resilienceComponents *interfaces.ResilienceComponents) *MetricsHandler {
	baseHandler := NewBaseTelemetryHandler(
		eventPublisher,
		&MetricMetadataExtractor{},
		NewStreamingTelemetryProcessor(eventPublisher, TelemetryMetric),
		TelemetryMetric,
	)

	return &MetricsHandler{
		BaseTelemetryHandler: baseHandler,
		queryService:         services.NewDefaultMetricsQueryService(),
	}
}

func NewMetricsHandlerWithClickHouse(eventPublisher telemetry.EventPublisher, clickhouse *storage.ClickHouseClient, resilienceComponents *interfaces.ResilienceComponents) *MetricsHandler {
	baseHandler := NewBaseTelemetryHandler(
		eventPublisher,
		&MetricMetadataExtractor{},
		NewStreamingTelemetryProcessor(eventPublisher, TelemetryMetric),
		TelemetryMetric,
	)

	return &MetricsHandler{
		BaseTelemetryHandler: baseHandler,
		queryService:         services.NewClickHouseMetricsQueryService(clickhouse),
	}
}

// CreateMetrics - hot path HTTP ingestion using consolidated base handler
// @Summary Receive metrics
// @Description Receive metrics data from OpenTelemetry
// @Tags metrics
// @Accept json
// @Produce json
// @Param metrics body MetricsRequest true "Metrics data"
// @Success 200 {object} MetricsResponse
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/metrics [post]
func (h *MetricsHandler) CreateMetrics(c *gin.Context) {
	h.HandleIngestion(c)
}

// CreateMetricsGRPC - zero-allocation gRPC ingestion bypassing JSON entirely
func (h *MetricsHandler) CreateMetricsGRPC(ctx context.Context, req *metricscollectorpb.ExportMetricsServiceRequest) (*metricscollectorpb.ExportMetricsServiceResponse, error) {
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

	return &metricscollectorpb.ExportMetricsServiceResponse{}, nil
}

// GetMetrics retrieves metrics using the query service
// @Summary Get metrics
// @Description Get metrics data for analysis
// @Tags metrics
// @Produce json
// @Param service query string false "Service name"
// @Param metric_name query string false "Specific metric name"
// @Param from query string false "Start time (RFC3339)"
// @Param to query string false "End time (RFC3339)"
// @Param aggregation query string false "Aggregation function (sum, avg, count, min, max)"
// @Param group_by query string false "Group by field (service, metric_name, time_bucket)"
// @Param limit query int false "Limit number of results (default 100, max 10000)"
// @Param offset query int false "Offset for pagination (default 0)"
// @Success 200 {object} services.MetricsQueryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/metrics [get]
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	// Parse query parameters
	params := services.MetricsQueryParams{
		ServiceName: c.Query("service"),
		MetricName:  c.Query("metric_name"),
		Aggregation: c.Query("aggregation"),
		GroupBy:     c.Query("group_by"),
		Limit:       services.DefaultMetricsLimit,
		Offset:      services.DefaultMetricsOffset,
	}

	// Parse time range parameters
	if fromStr := c.Query("from"); fromStr != "" {
		if fromTime, err := time.Parse(time.RFC3339, fromStr); err == nil {
			params.From = fromTime
		} else {
			logger.Warn("Invalid from time format",
				zap.String("from", fromStr),
				zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())))
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid time format",
				Message: "from parameter must be in RFC3339 format",
			})
			return
		}
	}

	if toStr := c.Query("to"); toStr != "" {
		if toTime, err := time.Parse(time.RFC3339, toStr); err == nil {
			params.To = toTime
		} else {
			logger.Warn("Invalid to time format",
				zap.String("to", toStr),
				zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())))
			c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid time format",
				Message: "to parameter must be in RFC3339 format",
			})
			return
		}
	}

	// Parse pagination parameters
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			if limit > services.MaxMetricsLimit {
				params.Limit = services.MaxMetricsLimit
			} else {
				params.Limit = limit
			}
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	// Query metrics using the service
	queryStart := time.Now()
	response, err := h.queryService.QueryMetrics(c.Request.Context(), params)
	queryLatency := time.Since(queryStart).Seconds()

	if err != nil {
		// Record query failure metrics
		metrics.Global().RecordBusinessError("query", params.ServiceName, "error")
		logger.Error("Failed to query metrics",
			zap.Error(err),
			zap.String("service", params.ServiceName),
			zap.String("metric_name", params.MetricName),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to query metrics",
			Message: "Internal server error",
		})
		return
	}

	// Record successful query metrics - estimate response size for business tracking
	responseSize := len(response.Metrics) * 100 // Rough estimate per metric record
	metricsRegistry := metrics.Global()
	metricsRegistry.RecordDataQuery("metrics", "query", float64(responseSize), queryLatency)
	metricsRegistry.RecordStorageOperation("read", "success")

	c.JSON(http.StatusOK, response)
}

// OpenTelemetry Metrics Protocol structures
type MetricsRequest struct {
	ResourceMetrics []ResourceMetric `json:"resourceMetrics"`
}

type ResourceMetric struct {
	Resource     Resource      `json:"resource"`
	ScopeMetrics []ScopeMetric `json:"scopeMetrics"`
	SchemaURL    string        `json:"schemaUrl,omitempty"`
}

type Resource struct {
	Attributes []Attribute `json:"attributes"`
}

type ScopeMetric struct {
	Scope   Scope    `json:"scope"`
	Metrics []Metric `json:"metrics"`
}

type Scope struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Metric struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Unit        string     `json:"unit,omitempty"`
	Sum         *Sum       `json:"sum,omitempty"`
	Histogram   *Histogram `json:"histogram,omitempty"`
	Gauge       *Gauge     `json:"gauge,omitempty"`
}

type Sum struct {
	DataPoints             []NumberDataPoint `json:"dataPoints"`
	AggregationTemporality int               `json:"aggregationTemporality"`
	IsMonotonic            bool              `json:"isMonotonic"`
}

type Histogram struct {
	DataPoints             []HistogramDataPoint `json:"dataPoints"`
	AggregationTemporality int                  `json:"aggregationTemporality"`
}

type Gauge struct {
	DataPoints []NumberDataPoint `json:"dataPoints"`
}

type NumberDataPoint struct {
	Attributes        []Attribute `json:"attributes,omitempty"`
	StartTimeUnixNano uint64      `json:"startTimeUnixNano,omitempty"`
	TimeUnixNano      uint64      `json:"timeUnixNano"`
	AsDouble          *float64    `json:"asDouble,omitempty"`
	AsInt             *int64      `json:"asInt,omitempty"`
}

type HistogramDataPoint struct {
	Attributes        []Attribute `json:"attributes,omitempty"`
	StartTimeUnixNano uint64      `json:"startTimeUnixNano,omitempty"`
	TimeUnixNano      uint64      `json:"timeUnixNano"`
	Count             uint64      `json:"count"`
	Sum               *float64    `json:"sum,omitempty"`
	BucketCounts      []uint64    `json:"bucketCounts"`
	ExplicitBounds    []float64   `json:"explicitBounds"`
}

type Attribute struct {
	Key   string         `json:"key"`
	Value AttributeValue `json:"value"`
}

type AttributeValue struct {
	StringValue *string  `json:"stringValue,omitempty"`
	IntValue    *int64   `json:"intValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
	BoolValue   *bool    `json:"boolValue,omitempty"`
}

// Response types
type MetricsResponse struct {
	Received  int       `json:"received" example:"5"`
	Timestamp time.Time `json:"timestamp" example:"2023-01-01T00:00:00Z"`
	Status    string    `json:"status" example:"accepted"`
}

// Keep existing helper function for backward compatibility
// The base handler now uses this via MetricMetadataExtractor
func extractMetricsServiceNameAndCount(data json.RawMessage) (string, int) {
	var otlp struct {
		ResourceMetrics []struct {
			Resource struct {
				Attributes []struct {
					Key   string `json:"key"`
					Value struct {
						StringValue string `json:"stringValue"`
					} `json:"value"`
				} `json:"attributes"`
			} `json:"resource"`
			ScopeMetrics []struct {
				Metrics []struct {
					Name string `json:"name"`
				} `json:"metrics"`
			} `json:"scopeMetrics"`
		} `json:"resourceMetrics"`
	}

	if err := json.Unmarshal(data, &otlp); err != nil {
		return "unknown", 0
	}

	serviceName := "unknown"
	metricsCount := 0

	for _, rm := range otlp.ResourceMetrics {
		if serviceName == "unknown" {
			for _, attr := range rm.Resource.Attributes {
				if attr.Key == "service.name" && attr.Value.StringValue != "" {
					serviceName = attr.Value.StringValue
					break
				}
			}
		}
		for _, sm := range rm.ScopeMetrics {
			metricsCount += len(sm.Metrics)
		}
	}

	return serviceName, metricsCount
}
