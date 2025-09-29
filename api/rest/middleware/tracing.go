package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/logging"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Context key types to avoid collisions with other packages
type (
	traceIDKey    struct{}
	spanIDKey     struct{}
	requestIDKey  struct{}
)

// Context key instances
var (
	TraceIDContextKey    = traceIDKey{}
	SpanIDContextKey     = spanIDKey{}
	RequestIDContextKey  = requestIDKey{}
)

// Distributed tracing constants following W3C Trace Context specification
const (
	// W3C Trace Context headers
	TraceParentHeader = "traceparent"
	TraceStateHeader  = "tracestate"

	// Common trace headers for backwards compatibility
	XTraceIDHeader     = "X-Trace-Id"
	XSpanIDHeader      = "X-Span-Id"
	XRequestIDHeader   = "X-Request-Id"

	// Trace ID formats
	TraceIDLength = 32 // 16 bytes as 32 hex chars
	SpanIDLength  = 16 // 8 bytes as 16 hex chars

	// W3C traceparent format: "00-{trace-id}-{parent-id}-{flags}"
	TraceParentFormat = "00-%s-%s-%02x"

	// Context keys for trace correlation
	TraceIDKey    = "trace_id"
	SpanIDKey     = "span_id"
	RequestIDKey  = "request_id"
	TraceParentKey = "traceparent"

	// Trace flags
	TraceFlagsSampled = 0x01
	TraceFlagsDefault = 0x01 // Default to sampled for observability
)

var (
	// Pre-compiled regex for traceparent validation
	traceParentRegex = regexp.MustCompile(`^[0-9a-f]{2}-[0-9a-f]{32}-[0-9a-f]{16}-[0-9a-f]{2}$`)

)

// TraceContext holds distributed tracing correlation information
type TraceContext struct {
	TraceID     string
	SpanID      string
	ParentID    string
	RequestID   string
	TraceParent string
	TraceState  string
	Flags       byte
	Sampled     bool
}

// DistributedTracingMiddleware provides W3C-compliant distributed tracing correlation
func DistributedTracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract or generate trace context
		traceCtx := extractOrGenerateTraceContext(c)

		// Set trace context in Gin context for downstream handlers
		setTraceContextInGinContext(c, traceCtx)

		// Set correlation headers for downstream services
		setCorrelationHeaders(c, traceCtx)

		// Add structured logging fields for trace correlation
		addTracingToLogger(c, traceCtx)

		// Log request start with trace correlation
		logRequestStart(c, traceCtx)

		// Process request
		c.Next()

		// Log request completion with trace correlation
		logRequestComplete(c, traceCtx)
	}
}

// extractOrGenerateTraceContext extracts existing trace context or generates new one
func extractOrGenerateTraceContext(c *gin.Context) *TraceContext {
	// Try to extract W3C traceparent header
	if traceParent := c.GetHeader(TraceParentHeader); traceParent != "" {
		if traceCtx := parseTraceParent(traceParent); traceCtx != nil {
			// Extract tracestate if present
			traceCtx.TraceState = c.GetHeader(TraceStateHeader)

			// Generate new span ID for this service
			traceCtx.ParentID = traceCtx.SpanID
			traceCtx.SpanID = generateSpanID()

			// Reconstruct traceparent with new span ID
			traceCtx.TraceParent = fmt.Sprintf(TraceParentFormat, traceCtx.TraceID, traceCtx.SpanID, traceCtx.Flags)

			// Set or generate request ID
			if requestID := c.GetHeader(XRequestIDHeader); requestID != "" {
				traceCtx.RequestID = requestID
			} else {
				traceCtx.RequestID = generateRequestID()
			}

			return traceCtx
		}
	}

	// Try legacy X-Trace-Id header for backwards compatibility
	if traceID := c.GetHeader(XTraceIDHeader); traceID != "" && isValidTraceID(traceID) {
		spanID := generateSpanID()
		requestID := c.GetHeader(XRequestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		return &TraceContext{
			TraceID:     traceID,
			SpanID:      spanID,
			ParentID:    "", // No parent for legacy format
			RequestID:   requestID,
			TraceParent: fmt.Sprintf(TraceParentFormat, traceID, spanID, TraceFlagsDefault),
			Flags:       TraceFlagsDefault,
			Sampled:     true,
		}
	}

	// Generate new trace context
	return generateNewTraceContext()
}

// parseTraceParent parses W3C traceparent header format: "00-{trace-id}-{parent-id}-{flags}"
func parseTraceParent(traceParent string) *TraceContext {
	if !traceParentRegex.MatchString(traceParent) {
		return nil
	}

	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		return nil
	}

	// Parse components
	version := parts[0]
	traceID := parts[1]
	spanID := parts[2]
	flagsStr := parts[3]

	// Validate version (must be "00" for current spec)
	if version != "00" {
		return nil
	}

	// Validate trace ID (must not be all zeros)
	if traceID == strings.Repeat("0", 32) {
		return nil
	}

	// Validate span ID (must not be all zeros)
	if spanID == strings.Repeat("0", 16) {
		return nil
	}

	// Parse flags
	flags, err := strconv.ParseUint(flagsStr, 16, 8)
	if err != nil {
		return nil
	}

	return &TraceContext{
		TraceID:     traceID,
		SpanID:      spanID,
		ParentID:    spanID, // Will be updated to new span ID
		TraceParent: traceParent,
		Flags:       byte(flags),
		Sampled:     (flags & TraceFlagsSampled) != 0,
	}
}

// generateNewTraceContext creates a new trace context with fresh IDs
func generateNewTraceContext() *TraceContext {
	traceID := generateTraceID()
	spanID := generateSpanID()
	requestID := generateRequestID()

	traceParent := fmt.Sprintf(TraceParentFormat, traceID, spanID, TraceFlagsDefault)

	return &TraceContext{
		TraceID:     traceID,
		SpanID:      spanID,
		ParentID:    "", // No parent for root span
		RequestID:   requestID,
		TraceParent: traceParent,
		Flags:       TraceFlagsDefault,
		Sampled:     true,
	}
}

// generateTraceID generates a new 128-bit trace ID as 32 hex characters
func generateTraceID() string {
	bytes := make([]byte, 16) // 128 bits
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to less secure but functional ID generation
		logger.Warn("Failed to generate cryptographically secure trace ID, using fallback",
			zap.Error(err))
		return generateFallbackID(32)
	}
	return hex.EncodeToString(bytes)
}

// generateSpanID generates a new 64-bit span ID as 16 hex characters
func generateSpanID() string {
	bytes := make([]byte, 8) // 64 bits
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to less secure but functional ID generation
		logger.Warn("Failed to generate cryptographically secure span ID, using fallback",
			zap.Error(err))
		return generateFallbackID(16)
	}
	return hex.EncodeToString(bytes)
}

// generateRequestID generates a unique request ID for internal correlation
func generateRequestID() string {
	bytes := make([]byte, 8) // 64 bits for request ID
	if _, err := rand.Read(bytes); err != nil {
		logger.Warn("Failed to generate cryptographically secure request ID, using fallback",
			zap.Error(err))
		return generateFallbackID(16)
	}
	return hex.EncodeToString(bytes)
}

// generateFallbackID generates a fallback ID when crypto/rand fails
func generateFallbackID(length int) string {
	// Use a simple timestamp-based fallback (not cryptographically secure)
	timestamp := fmt.Sprintf("%016x", time.Now().UnixNano())
	if len(timestamp) >= length {
		return timestamp[:length]
	}
	// Pad with zeros if needed
	return timestamp + strings.Repeat("0", length-len(timestamp))
}

// isValidTraceID validates trace ID format and ensures it's not all zeros
func isValidTraceID(traceID string) bool {
	if len(traceID) != TraceIDLength {
		return false
	}

	// Check if all characters are valid hex
	if _, err := hex.DecodeString(traceID); err != nil {
		return false
	}

	// Ensure it's not all zeros
	return traceID != strings.Repeat("0", TraceIDLength)
}

// setTraceContextInGinContext stores trace context in Gin context for handlers
func setTraceContextInGinContext(c *gin.Context, traceCtx *TraceContext) {
	c.Set(TraceIDKey, traceCtx.TraceID)
	c.Set(SpanIDKey, traceCtx.SpanID)
	c.Set(RequestIDKey, traceCtx.RequestID)
	c.Set(TraceParentKey, traceCtx.TraceParent)
}

// setCorrelationHeaders sets response headers for downstream service correlation
func setCorrelationHeaders(c *gin.Context, traceCtx *TraceContext) {
	// Set W3C trace context headers for downstream services
	c.Header(TraceParentHeader, traceCtx.TraceParent)
	if traceCtx.TraceState != "" {
		c.Header(TraceStateHeader, traceCtx.TraceState)
	}

	// Set legacy headers for backwards compatibility
	c.Header(XTraceIDHeader, traceCtx.TraceID)
	c.Header(XSpanIDHeader, traceCtx.SpanID)
	c.Header(XRequestIDHeader, traceCtx.RequestID)
}

// addTracingToLogger adds tracing fields to structured logging context
func addTracingToLogger(c *gin.Context, traceCtx *TraceContext) {
	// Create context with trace correlation for downstream logging
	ctx := context.WithValue(c.Request.Context(), TraceIDContextKey, traceCtx.TraceID)
	ctx = context.WithValue(ctx, SpanIDContextKey, traceCtx.SpanID)
	ctx = context.WithValue(ctx, RequestIDContextKey, traceCtx.RequestID)

	// Update request context
	c.Request = c.Request.WithContext(ctx)
}

// logRequestStart logs request initiation with trace correlation
func logRequestStart(c *gin.Context, traceCtx *TraceContext) {
	logger.Info("Request started",
		zap.String("trace_id", traceCtx.TraceID),
		zap.String("span_id", traceCtx.SpanID),
		zap.String("request_id", traceCtx.RequestID),
		zap.String("method", c.Request.Method),
		zap.String("path", logging.SanitizePath(c.Request.URL.Path)),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.String("user_agent", logging.SanitizeUserAgent(c.Request.UserAgent())),
		zap.Bool("sampled", traceCtx.Sampled))
}

// logRequestComplete logs request completion with trace correlation and metrics
func logRequestComplete(c *gin.Context, traceCtx *TraceContext) {
	logger.Info("Request completed",
		zap.String("trace_id", traceCtx.TraceID),
		zap.String("span_id", traceCtx.SpanID),
		zap.String("request_id", traceCtx.RequestID),
		zap.String("method", c.Request.Method),
		zap.String("path", logging.SanitizePath(c.Request.URL.Path)),
		zap.Int("status_code", c.Writer.Status()),
		zap.Int("response_size", c.Writer.Size()),
		zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
		zap.Bool("sampled", traceCtx.Sampled))
}

// GetTraceIDFromContext extracts trace ID from Gin context
func GetTraceIDFromContext(c *gin.Context) string {
	if traceID, exists := c.Get(TraceIDKey); exists {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	return ""
}

// GetSpanIDFromContext extracts span ID from Gin context
func GetSpanIDFromContext(c *gin.Context) string {
	if spanID, exists := c.Get(SpanIDKey); exists {
		if id, ok := spanID.(string); ok {
			return id
		}
	}
	return ""
}

// GetRequestIDFromContext extracts request ID from Gin context
func GetRequestIDFromContext(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

// GetTraceParentFromContext extracts traceparent header from Gin context
func GetTraceParentFromContext(c *gin.Context) string {
	if traceParent, exists := c.Get(TraceParentKey); exists {
		if header, ok := traceParent.(string); ok {
			return header
		}
	}
	return ""
}