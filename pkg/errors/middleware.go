package errors

import (
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides high-performance error handling middleware for Gin
type Handler struct {
	logger *zap.Logger

	// Performance optimizations
	logFieldPool sync.Pool
	responsePool sync.Pool
}

// NewHandler creates optimized error handler with object pools
func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger: logger,
		logFieldPool: sync.Pool{
			New: func() interface{} {
				return make([]zap.Field, 0, 8) // Pre-size for common field count
			},
		},
		responsePool: sync.Pool{
			New: func() interface{} {
				return &Response{}
			},
		},
	}
}

// Middleware returns the Gin middleware function
func (h *Handler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Fast path: no errors occurred
		if len(c.Errors) == 0 {
			return
		}

		// Process the last error (most specific)
		err := c.Errors.Last().Err
		h.handleError(c, err)
	}
}

// handleError processes error with zero-allocation fast paths
func (h *Handler) handleError(c *gin.Context, err error) {
	var apiErr *Error

	// Fast path: already an API error
	if e, ok := err.(*Error); ok {
		apiErr = e
	} else {
		// Convert to API error with minimal allocation
		apiErr = InternalServer("Request processing failed", err)
	}

	// Add request context if missing (branch prediction optimized)
	if apiErr.requestID == "" {
		if requestID := c.GetString("request_id"); requestID != "" {
			apiErr.requestID = requestID
		}
	}
	if apiErr.path == "" {
		apiErr.path = c.Request.URL.Path
	}

	// Log error with pooled fields
	h.logError(apiErr, c)

	// Set retry-after header if specified
	if apiErr.retryAfterSec != nil {
		c.Header("Retry-After", strconv.Itoa(*apiErr.retryAfterSec))
	}

	// Generate response with pooled objects
	response := h.generateResponse(apiErr)

	// Send JSON response and abort
	c.JSON(apiErr.httpStatus, response)
	c.Abort()

	// Return response to pool
	h.responsePool.Put(response)

	// Return error to pool after response is sent
	apiErr.Release()
}

// logError logs with optimized field allocation and appropriate levels
func (h *Handler) logError(err *Error, c *gin.Context) {
	// Get fields slice from pool
	fields := h.logFieldPool.Get().([]zap.Field)
	defer func() {
		resetFields := fields[:0] // Reset length but keep capacity
		h.logFieldPool.Put(&resetFields)
	}()

	// Build log fields with zero-allocation string operations where possible
	fields = append(fields,
		zap.String("error_code", string(err.code)),
		zap.String("error_category", err.category.String()),
		zap.Int("http_status", err.httpStatus),
		zap.String("path", err.path),
		zap.String("method", c.Request.Method),
	)

	// Add request ID only if present (avoid empty string field)
	if err.requestID != "" {
		fields = append(fields, zap.String("request_id", err.requestID))
	}

	// Add details only if present
	if len(err.details) > 0 {
		fields = append(fields, zap.Any("error_details", err.details))
	}

	// Add cause only if present
	if err.cause != nil {
		fields = append(fields, zap.Error(err.cause))
	}

	// Log at appropriate level based on HTTP status (optimized branching)
	message := err.getMessage()
	switch {
	case err.httpStatus >= 500:
		h.logger.Error(message, fields...)
	case err.httpStatus >= 400:
		h.logger.Warn(message, fields...)
	default:
		h.logger.Info(message, fields...)
	}
}

// generateResponse creates response with object pooling
func (h *Handler) generateResponse(err *Error) *Response {
	response := h.responsePool.Get().(*Response)

	// Reset response fields
	response.Error = Details{
		Code:       err.code,
		Message:    err.getMessage(),
		Category:   err.category.String(),
		Details:    err.details,
		Validation: err.validation,
		Retryable:  err.retryable,
		RetryAfter: err.retryAfterSec,
		Cause:      nil, // Reset cause
	}

	// Add cause without deep nesting
	if err.cause != nil {
		if causeErr, ok := err.cause.(*Error); ok {
			response.Error.Cause = &Details{
				Code:     causeErr.code,
				Message:  causeErr.getMessage(),
				Category: causeErr.category.String(),
			}
		}
	}

	response.Timestamp = time.Now().UTC()
	response.RequestID = err.requestID
	response.Path = err.path

	return response
}

// AbortWith* functions for direct error handling

// AbortWithError aborts request with existing error
func AbortWithError(c *gin.Context, err *Error) {
	if ginErr := c.Error(err); ginErr != nil {
		// Log the error but continue with abort - this is best effort
		c.Set("gin_error_failure", ginErr)
	}
	c.Abort()
}

// AbortWith creates and aborts with new error (optimized construction)
func AbortWith(c *gin.Context, code Code, message string) {
	err := New(code).
		Message(message).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortWithf creates and aborts with formatted message
func AbortWithf(c *gin.Context, code Code, format string, args ...interface{}) {
	err := New(code).
		Messagef(format, args...).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// High-frequency error shortcuts with pre-built messages

// AbortBadRequest aborts with bad request error
func AbortBadRequest(c *gin.Context, message string) {
	AbortWith(c, CodeBadRequest, message)
}

// AbortNotFound aborts with not found error for resource
func AbortNotFound(c *gin.Context, resource string) {
	err := New(CodeNotFound).
		Messagef("%s not found", resource).
		Detail("resource_type", resource).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortUnauthorized aborts with unauthorized error
func AbortUnauthorized(c *gin.Context) {
	AbortWith(c, CodeUnauthorized, "Authentication required")
}

// AbortForbidden aborts with forbidden error
func AbortForbidden(c *gin.Context) {
	AbortWith(c, CodeForbidden, "Access forbidden")
}

// AbortValidationFailed aborts with validation errors
func AbortValidationFailed(c *gin.Context, validationErrors []ValidationError) {
	err := New(CodeValidationFailed).
		Message("Request validation failed").
		ValidationErrors(validationErrors).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortRateLimited aborts with rate limit error
func AbortRateLimited(c *gin.Context, limit int, retryAfterSec int) {
	err := New(CodeRateLimited).
		Message("Rate limit exceeded").
		Detail("limit", limit).
		RetryAfter(retryAfterSec).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortInternalServer aborts with internal server error
func AbortInternalServer(c *gin.Context, message string, cause error) {
	err := New(CodeInternalServer).
		Message(message).
		Cause(cause).
		Retryable().
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortServiceUnavailable aborts with service unavailable error
func AbortServiceUnavailable(c *gin.Context, service string, retryAfterSec int) {
	err := New(CodeServiceUnavailable).
		Messagef("%s temporarily unavailable", service).
		Detail("service", service).
		RetryAfter(retryAfterSec).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortDatabaseError aborts with database error
func AbortDatabaseError(c *gin.Context, operation string, cause error) {
	err := New(CodeDatabaseError).
		Messagef("Database operation failed: %s", operation).
		Detail("operation", operation).
		Cause(cause).
		Retryable().
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortTimeout aborts with timeout error
func AbortTimeout(c *gin.Context, operation string, timeoutSec int) {
	err := New(CodeTimeout).
		Messagef("Operation timed out: %s", operation).
		Detail("operation", operation).
		Detail("timeout_seconds", timeoutSec).
		Retryable().
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}

// AbortCircuitOpen aborts with circuit breaker error
func AbortCircuitOpen(c *gin.Context, service string, retryAfterSec int) {
	err := New(CodeCircuitOpen).
		Messagef("Circuit breaker open for %s", service).
		Detail("service", service).
		RetryAfter(retryAfterSec).
		Context(c.GetString("request_id"), c.Request.URL.Path)
	AbortWithError(c, err)
}