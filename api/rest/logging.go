package rest

import (
	"fmt"
	"html"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// customLogFormatter provides structured logging format
func customLogFormatter(param gin.LogFormatterParams) string {
	return fmt.Sprintf("[%s] %s %s %s %d %s %s\n",
		param.TimeStamp.Format(STANDARD_TIME_FORMAT),
		param.Method,
		param.Path,
		param.ClientIP,
		param.StatusCode,
		param.Latency,
		param.ErrorMessage,
	)
}

// customRecoveryHandler handles panics gracefully with sanitized logging
func customRecoveryHandler(c *gin.Context, recovered interface{}) {
	// Sanitize panic information for logging
	sanitizedPanic := sanitizePanicInfo(recovered)

	logger.Error("Request panic recovered",
		zap.String("panic_type", fmt.Sprintf("%T", recovered)),
		zap.String("panic_summary", sanitizedPanic),
		zap.String("method", c.Request.Method),
		zap.String("path", sanitizePath(c.Request.URL.Path)),
		zap.String("client_ip", c.ClientIP()))

	c.AbortWithStatusJSON(int(StatusInternalServerError), gin.H{
		"error":   "Internal server error",
		"message": "The server encountered an unexpected condition",
	})
}

// sanitizePanicInfo removes potentially sensitive information from panic data
func sanitizePanicInfo(recovered interface{}) string {
	if recovered == nil {
		return NIL_PANIC_MESSAGE
	}

	panicStr := fmt.Sprintf("%v", recovered)

	// Remove potential file paths, memory addresses, and other sensitive info
	panicStr = versionSanitizeRegex.ReplaceAllString(panicStr, "[SANITIZED]")

	// Truncate if too long
	if len(panicStr) > MAX_PANIC_MESSAGE_LENGTH {
		panicStr = panicStr[:MAX_PANIC_MESSAGE_LENGTH] + TRUNCATION_SUFFIX
	}

	return panicStr
}

// sanitizePath removes sensitive information from URL paths
func sanitizePath(path string) string {
	if path == "" {
		return string(ROOT_PATH)
	}

	// Remove potential UUIDs, tokens, or other sensitive path components
	sanitized := versionSanitizeRegex.ReplaceAllString(path, "[ID]")
	return html.EscapeString(sanitized)
}