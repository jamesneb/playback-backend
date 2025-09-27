package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
)

type MetricsHandler struct {
	eventPublisher telemetry.EventPublisher
}

func NewMetricsHandler(eventPublisher telemetry.EventPublisher) *MetricsHandler {
	return &MetricsHandler{
		eventPublisher: eventPublisher,
	}
}

// CreateMetrics receives metrics data
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

	// Parse the OTLP metrics data (raw JSON)
	var otlpData json.RawMessage
	if err := c.ShouldBindJSON(&otlpData); err != nil {
		logger.Error("Failed to parse OTLP metrics data",
			zap.Error(err),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid OTLP metrics data",
			Message: err.Error(),
		})
		return
	}

	// Extract service name and metrics count with single parse operation
	serviceName, metricsCount := extractMetricsServiceNameAndCount(otlpData)

	// Log the ingestion event
	logger.Debug("Received OTLP metrics data",
		zap.String("service_name", logging.SanitizeServiceName(serviceName)),
		zap.Int("metrics_count", metricsCount),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.String("data_size", logging.SanitizeDataSize(len(otlpData))),
	)

	// Publish to Kinesis
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	err := h.eventPublisher.PublishMetrics(
		ctx,
		otlpData,
		serviceName,
		c.ClientIP(),
		c.GetHeader("User-Agent"),
	)
	if err != nil {
		logger.Error("Failed to publish metrics to Kinesis",
			zap.Error(err),
			zap.String("service_name", serviceName),
			zap.Int("metrics_count", metricsCount),
		)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to process metrics data",
			Message: "Internal server error",
		})
		return
	}

	// Log successful ingestion
	logger.Debug("Successfully published metrics to Kinesis",
		zap.String("service_name", serviceName),
		zap.Int("metrics_count", metricsCount),
	)

	// Return success response
	response := MetricsResponse{
		Received:  metricsCount,
		Timestamp: time.Now(),
		Status:    "accepted",
	}

	c.JSON(http.StatusAccepted, response)
}

// GetMetrics retrieves metrics (placeholder for querying)
// @Summary Get metrics
// @Description Get metrics data for analysis
// @Tags metrics
// @Produce json
// @Param service query string false "Service name"
// @Param from query string false "Start time (RFC3339)"
// @Param to query string false "End time (RFC3339)"
// @Success 200 {object} MetricsQueryResponse
// @Router /api/v1/metrics [get]
func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	service := c.Query("service")
	from := c.Query("from")
	to := c.Query("to")

	// Mock response for now
	response := MetricsQueryResponse{
		Service:   service,
		TimeRange: TimeRange{From: from, To: to},
		Metrics: []MetricData{
			{
				Name:      "orders_total",
				Type:      "counter",
				Value:     156,
				Labels:    map[string]string{"status": "success"},
				Timestamp: time.Now(),
			},
			{
				Name:      "order_duration_seconds",
				Type:      "histogram",
				Value:     0.235,
				Labels:    map[string]string{"status": "success"},
				Timestamp: time.Now(),
			},
		},
	}

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

type MetricsQueryResponse struct {
	Service   string       `json:"service" example:"order-service"`
	TimeRange TimeRange    `json:"time_range"`
	Metrics   []MetricData `json:"metrics"`
}

type TimeRange struct {
	From string `json:"from" example:"2023-01-01T00:00:00Z"`
	To   string `json:"to" example:"2023-01-01T01:00:00Z"`
}

type MetricData struct {
	Name      string            `json:"name" example:"orders_total"`
	Type      string            `json:"type" example:"counter"`
	Value     float64           `json:"value" example:"156"`
	Labels    map[string]string `json:"labels" example:"status:success"`
	Timestamp time.Time         `json:"timestamp" example:"2023-01-01T00:00:00Z"`
}

// Helper functions for extracting metadata from OTLP metrics data
func extractMetricsServiceNameAndCount(data json.RawMessage) (string, int) {
	// Parse OTLP metrics structure once to extract both service name and count
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
		// Extract service name from first ResourceMetric with service.name attribute
		if serviceName == "unknown" {
			for _, attr := range rm.Resource.Attributes {
				if attr.Key == "service.name" && attr.Value.StringValue != "" {
					serviceName = attr.Value.StringValue
					break
				}
			}
		}

		// Count metrics in all scope metrics
		for _, sm := range rm.ScopeMetrics {
			metricsCount += len(sm.Metrics)
		}
	}

	return serviceName, metricsCount
}
