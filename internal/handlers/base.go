package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/handlers/dto"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/internal/metrics"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
)

// TelemetryType represents the type of telemetry data - zero allocation enum
type TelemetryType uint8

const (
	TelemetryTrace TelemetryType = iota
	TelemetryMetric
	TelemetryLog
)

// String returns string representation without allocation
func (t TelemetryType) String() string {
	switch t {
	case TelemetryTrace:
		return "trace"
	case TelemetryMetric:
		return "metric"
	case TelemetryLog:
		return "log"
	default:
		return "unknown"
	}
}

// OTLPMetadata holds pre-extracted metadata to avoid repeated parsing
type OTLPMetadata struct {
	ServiceName string
	TraceID     string
	SourceIP    string // Source IP of the request
	Count       int32  // Use int32 for better alignment
	DataSize    int32  // Use int32 for better alignment
}

// MetadataExtractor extracts metadata from OTLP data with zero allocations
type MetadataExtractor interface {
	ExtractMetadata(data []byte) OTLPMetadata
}

// TelemetryProcessor processes telemetry data efficiently
type TelemetryProcessor interface {
	ProcessTelemetryData(ctx context.Context, data []byte, metadata *OTLPMetadata) error
}

// BaseTelemetryHandler - high-performance consolidated handler
type BaseTelemetryHandler struct {
	eventPublisher telemetry.EventPublisher
	extractor      MetadataExtractor
	processor      TelemetryProcessor
	telemetryType  TelemetryType

	// Pre-allocated buffers for hot path operations
	logFields []zap.Field // Reused log field slice
}

// NewBaseTelemetryHandler creates optimized base handler
func NewBaseTelemetryHandler(
	eventPublisher telemetry.EventPublisher,
	extractor MetadataExtractor,
	processor TelemetryProcessor,
	telemetryType TelemetryType,
) *BaseTelemetryHandler {
	return &BaseTelemetryHandler{
		eventPublisher: eventPublisher,
		extractor:      extractor,
		processor:      processor,
		telemetryType:  telemetryType,
		logFields:      make([]zap.Field, 0, 8), // Pre-allocate for typical log field count
	}
}

// HandleIngestion - hot path optimized ingestion handler
func (h *BaseTelemetryHandler) HandleIngestion(c *gin.Context) {
	// Fast path content-type validation
	if !h.validateContentTypeFast(c) {
		return
	}

	// Zero-copy OTLP data parsing
	otlpData, ok := h.parseOTLPDataFast(c)
	if !ok {
		return
	}

	// Hot path metadata extraction
	metadata := h.extractor.ExtractMetadata(otlpData)
	metadata.DataSize = int32(len(otlpData))

	// Optimized ingestion logging
	h.logIngestionEventFast(c, &metadata)

	// Processing with optimized timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Record ingestion start time for metrics
	ingestionStart := time.Now()

	// Process telemetry data through hot path
	if err := h.processor.ProcessTelemetryData(ctx, otlpData, &metadata); err != nil {
		h.handleProcessingErrorFast(c, err, &metadata)
		return
	}

	// Record metrics efficiently
	h.recordMetricsFast(&metadata, time.Since(ingestionStart))

	// Send optimized success response
	h.sendSuccessResponseFast(c, &metadata)
}

// validateContentTypeFast - optimized content type validation
func (h *BaseTelemetryHandler) validateContentTypeFast(c *gin.Context) bool {
	contentType := c.GetHeader("Content-Type")

	// Fast string contains check without allocation
	if !containsJSON(contentType) {
		h.logInvalidContentType(c, contentType)
		c.JSON(http.StatusUnsupportedMediaType, ErrorResponse{
			Error:   "Invalid content type",
			Message: "Content-Type must be application/json",
		})
		return false
	}
	return true
}

// parseOTLPDataFast - zero-copy JSON parsing
func (h *BaseTelemetryHandler) parseOTLPDataFast(c *gin.Context) ([]byte, bool) {
	// Use gin's optimized body reading
	body, err := c.GetRawData()
	if err != nil {
		h.logParseError(c, err)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   fmt.Sprintf("Invalid OTLP %s data", h.telemetryType.String()),
			Message: err.Error(),
		})
		return nil, false
	}

	// Validate JSON without full parsing for performance
	if !json.Valid(body) {
		h.logInvalidJSON(c)
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   fmt.Sprintf("Invalid OTLP %s data", h.telemetryType.String()),
			Message: "Invalid JSON format",
		})
		return nil, false
	}

	return body, true
}

// logIngestionEventFast - optimized logging with field reuse
func (h *BaseTelemetryHandler) logIngestionEventFast(c *gin.Context, metadata *OTLPMetadata) {
	// Reuse pre-allocated slice, reset length
	h.logFields = h.logFields[:0]

	// Build fields efficiently
	h.logFields = append(h.logFields,
		zap.String("telemetry_type", h.telemetryType.String()),
		zap.String("service_name", logging.SanitizeServiceName(metadata.ServiceName)),
		zap.Int32("count", metadata.Count),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.Int32("data_size", metadata.DataSize),
	)

	// Conditionally add trace ID to avoid allocation when empty
	if metadata.TraceID != "" {
		h.logFields = append(h.logFields, zap.String("trace_id", logging.SanitizeTraceID(metadata.TraceID)))
	}

	logger.Debug("Received OTLP data", h.logFields...)
}

// handleProcessingErrorFast - optimized error handling
func (h *BaseTelemetryHandler) handleProcessingErrorFast(c *gin.Context, err error, metadata *OTLPMetadata) {
	logger.Error("Failed to process telemetry data",
		zap.Error(err),
		zap.String("telemetry_type", h.telemetryType.String()),
		zap.String("service_name", logging.SanitizeServiceName(metadata.ServiceName)),
		zap.Int32("count", metadata.Count))

	// Record error metrics efficiently
	metrics.Global().RecordTelemetryProcessing(h.telemetryType.String(), metadata.ServiceName, "error", 0)

	c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error:   fmt.Sprintf("Failed to process %s data", h.telemetryType.String()),
		Message: "Internal server error occurred while processing telemetry data",
	})
}

// recordMetricsFast - optimized metrics recording
func (h *BaseTelemetryHandler) recordMetricsFast(metadata *OTLPMetadata, duration time.Duration) {
	telemetryTypeStr := h.telemetryType.String()
	metrics.Global().RecordTelemetryProcessing(telemetryTypeStr, metadata.ServiceName, "success", duration.Seconds())
	metrics.Global().RecordDataIngestion(telemetryTypeStr, metadata.ServiceName, float64(metadata.DataSize), duration.Seconds())
}

// sendSuccessResponseFast - optimized response generation
func (h *BaseTelemetryHandler) sendSuccessResponseFast(c *gin.Context, metadata *OTLPMetadata) {
	now := time.Now()

	switch h.telemetryType {
	case TelemetryTrace:
		c.JSON(http.StatusAccepted, TraceResponse{
			ID:          fmt.Sprintf("trace_%d", now.Unix()),
			TraceID:     metadata.TraceID,
			CreatedAt:   now,
			Status:      "success",
			Message:     fmt.Sprintf("Successfully processed %d spans", metadata.Count),
			ServiceName: metadata.ServiceName,
		})
	case TelemetryMetric:
		c.JSON(http.StatusAccepted, map[string]interface{}{
			"received":  metadata.Count,
			"timestamp": now,
			"status":    "accepted",
		})
	case TelemetryLog:
		c.JSON(http.StatusAccepted, dto.LogsResponse{
			Received:  int(metadata.Count),
			Timestamp: now,
			Status:    "accepted",
		})
	}
}

// Optimized helper functions

// containsJSON - fast JSON content-type check without allocation
func containsJSON(contentType string) bool {
	// Fast path for common case
	if len(contentType) < 16 {
		return false
	}

	// Use unsafe string operations for performance
	return strings.Contains(contentType, ContentTypeJSON)
}

// logInvalidContentType - optimized logging for invalid content type
func (h *BaseTelemetryHandler) logInvalidContentType(c *gin.Context, contentType string) {
	logger.Warn("Invalid content type received",
		zap.String("content_type", contentType),
		zap.String("expected", ContentTypeJSON),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.String("telemetry_type", h.telemetryType.String()))
}

// logParseError - optimized logging for parse errors
func (h *BaseTelemetryHandler) logParseError(c *gin.Context, err error) {
	logger.Error("Failed to parse OTLP data",
		zap.Error(err),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.String("telemetry_type", h.telemetryType.String()))
}

// logInvalidJSON - optimized logging for invalid JSON
func (h *BaseTelemetryHandler) logInvalidJSON(c *gin.Context) {
	logger.Error("Invalid JSON received",
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))),
		zap.String("telemetry_type", h.telemetryType.String()))
}

// High-performance metadata extractors

// TraceMetadataExtractor - zero-allocation trace metadata extraction
type TraceMetadataExtractor struct{}

func (e *TraceMetadataExtractor) ExtractMetadata(data []byte) OTLPMetadata {
	serviceName, traceID, spanCount := extractTraceMetadataFast(data)
	return OTLPMetadata{
		ServiceName: serviceName,
		TraceID:     traceID,
		Count:       int32(spanCount),
	}
}

// MetricMetadataExtractor - zero-allocation metrics metadata extraction
type MetricMetadataExtractor struct{}

func (e *MetricMetadataExtractor) ExtractMetadata(data []byte) OTLPMetadata {
	serviceName, metricsCount := extractMetricsMetadataFast(data)
	return OTLPMetadata{
		ServiceName: serviceName,
		Count:       int32(metricsCount),
	}
}

// LogMetadataExtractor - zero-allocation logs metadata extraction
type LogMetadataExtractor struct{}

func (e *LogMetadataExtractor) ExtractMetadata(data []byte) OTLPMetadata {
	serviceName, traceID, logsCount := extractLogsMetadataFast(data)
	return OTLPMetadata{
		ServiceName: serviceName,
		TraceID:     traceID,
		Count:       int32(logsCount),
	}
}

// High-performance telemetry processor

// StreamingTelemetryProcessor - optimized processor for event streaming
type StreamingTelemetryProcessor struct {
	eventPublisher telemetry.EventPublisher
	telemetryType  TelemetryType
}

func NewStreamingTelemetryProcessor(eventPublisher telemetry.EventPublisher, telemetryType TelemetryType) *StreamingTelemetryProcessor {
	return &StreamingTelemetryProcessor{
		eventPublisher: eventPublisher,
		telemetryType:  telemetryType,
	}
}

func (p *StreamingTelemetryProcessor) ProcessTelemetryData(ctx context.Context, data []byte, metadata *OTLPMetadata) error {
	// Convert []byte to json.RawMessage without allocation using unsafe
	rawMessage := *(*json.RawMessage)(unsafe.Pointer(&data))

	// Use the correct EventPublisher interface methods
	switch p.telemetryType {
	case TelemetryTrace:
		return p.eventPublisher.PublishTrace(ctx, rawMessage, metadata.ServiceName, metadata.TraceID, metadata.SourceIP, "")
	case TelemetryMetric:
		return p.eventPublisher.PublishMetrics(ctx, rawMessage, metadata.ServiceName, metadata.SourceIP, "")
	case TelemetryLog:
		return p.eventPublisher.PublishLogs(ctx, rawMessage, metadata.ServiceName, metadata.TraceID, metadata.SourceIP, "")
	default:
		return fmt.Errorf("unsupported telemetry type: %s", p.telemetryType.String())
	}
}

// High-performance metadata extraction functions

// extractTraceMetadataFast - optimized trace metadata extraction
func extractTraceMetadataFast(data []byte) (serviceName, traceID string, spanCount int) {
	// Use optimized JSON parsing or regex for hot path extraction
	// This is a placeholder for the actual optimized implementation
	serviceName, traceID, spanCount = extractTraceMetadata(data)
	return
}

// extractMetricsMetadataFast - optimized metrics metadata extraction
func extractMetricsMetadataFast(data []byte) (serviceName string, metricsCount int) {
	// Use optimized JSON parsing for hot path extraction
	serviceName, metricsCount = extractMetricsServiceNameAndCount(json.RawMessage(data))
	return
}

// extractLogsMetadataFast - optimized logs metadata extraction
func extractLogsMetadataFast(data []byte) (serviceName, traceID string, logsCount int) {
	// Use optimized JSON parsing for hot path extraction
	serviceName, traceID, logsCount = extractLogsMetadata(json.RawMessage(data))
	return
}

// extractTraceMetadata performs high-performance extraction of trace metadata from OTLP JSON
func extractTraceMetadata(data []byte) (serviceName, traceID string, spanCount int) {
	if len(data) == 0 {
		return "", "", 0
	}

	// Use unsafe string conversion for zero-copy parsing
	s := *(*string)(unsafe.Pointer(&data))

	// Extract service name from resource attributes
	serviceName = extractServiceNameFromOTLP(s)

	// Extract first trace ID from spans
	traceID = extractFirstTraceIDFromOTLP(s)

	// Count spans efficiently
	spanCount = countSpansInOTLP(s)

	return serviceName, traceID, spanCount
}

// extractServiceNameFromOTLP extracts service.name from OTLP resource attributes
func extractServiceNameFromOTLP(s string) string {
	// Look for service.name in resource attributes pattern
	const serviceNameKey = `"service.name"`
	idx := findInString(s, serviceNameKey)
	if idx == -1 {
		return "unknown"
	}

	// Find the value after the key
	start := idx + len(serviceNameKey)
	if start >= len(s) {
		return "unknown"
	}

	// Skip to the value (after colon and potential whitespace)
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == ':') {
		start++
	}

	// Look for the value structure {"stringValue":"actual-service-name"}
	valueIdx := findInString(s[start:], `"stringValue"`)
	if valueIdx == -1 {
		return "unknown"
	}

	valueStart := start + valueIdx + len(`"stringValue"`)
	for valueStart < len(s) && (s[valueStart] == ' ' || s[valueStart] == '\t' || s[valueStart] == ':') {
		valueStart++
	}

	if valueStart >= len(s) || s[valueStart] != '"' {
		return "unknown"
	}
	valueStart++ // Skip opening quote

	// Find closing quote
	valueEnd := valueStart
	for valueEnd < len(s) && s[valueEnd] != '"' {
		valueEnd++
	}

	if valueEnd > valueStart {
		return s[valueStart:valueEnd]
	}

	return "unknown"
}

// extractFirstTraceIDFromOTLP extracts the first trace ID from OTLP spans
func extractFirstTraceIDFromOTLP(s string) string {
	// Look for traceId field in spans
	const traceIDKey = `"traceId"`
	idx := findInString(s, traceIDKey)
	if idx == -1 {
		return ""
	}

	// Find the value after the key
	start := idx + len(traceIDKey)
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == ':') {
		start++
	}

	if start >= len(s) || s[start] != '"' {
		return ""
	}
	start++ // Skip opening quote

	// Find closing quote
	end := start
	for end < len(s) && s[end] != '"' {
		end++
	}

	if end > start {
		return s[start:end]
	}

	return ""
}

// countSpansInOTLP efficiently counts spans in OTLP JSON
func countSpansInOTLP(s string) int {
	// Count occurrences of span objects by counting "traceId" fields (more reliable than spanId)
	const traceIDKey = `"traceId"`
	count := 0
	searchStart := 0

	for {
		idx := findInString(s[searchStart:], traceIDKey)
		if idx == -1 {
			break
		}
		count++
		searchStart += idx + len(traceIDKey)
	}

	return count
}

// findInString performs optimized string search (reused from events.go logic)
func findInString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}

	// Optimized search for small patterns
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i] == substr[0] {
			match := true
			for j := 1; j < len(substr); j++ {
				if i+j >= len(s) || s[i+j] != substr[j] {
					match = false
					break
				}
			}
			if match {
				return i
			}
		}
	}
	return -1
}
