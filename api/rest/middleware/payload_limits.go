package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Size represents payload size in bytes for better readability
type Size int64

// Size constants using type aliases for clarity
const (
	// Base size units
	KB Size = 1 << 10  // 1 KB
	MB Size = 1 << 20  // 1 MB

	// Payload size limits to prevent DoS attacks
	DefaultMaxRequestSize Size = 10 * MB // 10 MB
	MaxAllowedRequestSize Size = 50 * MB // 50 MB

	// Different size limits for different endpoint types
	TraceMaxRequestSize   Size = 25 * MB // 25 MB for traces (can be large)
	MetricsMaxRequestSize Size = 10 * MB // 10 MB for metrics
	LogsMaxRequestSize    Size = 15 * MB // 15 MB for logs
	ReplayMaxRequestSize  Size = 1 * KB  // 1 KB for replay requests (just metadata)
)

// PayloadSizeLimitFunc represents a function that creates payload size middleware
type PayloadSizeLimitFunc func() gin.HandlerFunc

// PayloadSizeLimit creates middleware that limits request body size
func PayloadSizeLimit(maxSize Size) gin.HandlerFunc {
	maxSizeInt64 := int64(maxSize)

	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSizeInt64 {
			logger.Warn("Request body too large",
				zap.Int64("contentLength", c.Request.ContentLength),
				zap.Int64("maxSize", maxSizeInt64),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method))

			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":   "Request body too large",
				"maxSize": maxSizeInt64,
			})
			c.Abort()
			return
		}

		// Apply MaxBytesReader to prevent attacks that bypass Content-Length
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSizeInt64)
		c.Next()
	}
}

// TracePayloadLimit creates middleware for trace endpoints
func TracePayloadLimit() gin.HandlerFunc {
	return PayloadSizeLimit(TraceMaxRequestSize)
}

// MetricsPayloadLimit creates middleware for metrics endpoints
func MetricsPayloadLimit() gin.HandlerFunc {
	return PayloadSizeLimit(MetricsMaxRequestSize)
}

// LogsPayloadLimit creates middleware for logs endpoints
func LogsPayloadLimit() gin.HandlerFunc {
	return PayloadSizeLimit(LogsMaxRequestSize)
}

// ReplayPayloadLimit creates middleware for replay endpoints
func ReplayPayloadLimit() gin.HandlerFunc {
	return PayloadSizeLimit(ReplayMaxRequestSize)
}

// DefaultPayloadLimit creates middleware with default size limit
func DefaultPayloadLimit() gin.HandlerFunc {
	return PayloadSizeLimit(DefaultMaxRequestSize)
}