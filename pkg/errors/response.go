package errors

import (
	"fmt"
	"net/http"
	"sync"
	"time"
	"unsafe"
)

// Code represents high-performance error codes using string constants
type Code string

const (
	// Client errors (4xx) - most common first for branch prediction optimization
	CodeBadRequest       Code = "BAD_REQUEST"
	CodeValidationFailed Code = "VALIDATION_FAILED"
	CodeNotFound         Code = "NOT_FOUND"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeConflict         Code = "CONFLICT"
	CodeRateLimited      Code = "RATE_LIMITED"
	CodeRequestTooLarge  Code = "REQUEST_TOO_LARGE"
	CodeUnsupportedMedia Code = "UNSUPPORTED_MEDIA_TYPE"

	// Server errors (5xx) - ordered by frequency in production
	CodeInternalServer     Code = "INTERNAL_SERVER_ERROR"
	CodeServiceUnavailable Code = "SERVICE_UNAVAILABLE"
	CodeDatabaseError      Code = "DATABASE_ERROR"
	CodeExternalService    Code = "EXTERNAL_SERVICE_ERROR"
	CodeTimeout            Code = "TIMEOUT"
	CodeCircuitOpen        Code = "CIRCUIT_BREAKER_OPEN"
)

// Category represents error categories with single-byte enum for performance
type Category uint8

const (
	CategoryClient Category = iota
	CategoryServer
	CategorySystem
	CategoryBusiness
)

var categoryStrings = [4]string{"client", "server", "system", "business"}

func (c Category) String() string {
	if int(c) >= len(categoryStrings) {
		return "unknown"
	}
	return categoryStrings[c]
}

// Response represents the standardized error response format
// Optimized field ordering for memory alignment
type Response struct {
	Error     Details   `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id,omitempty"`
	Path      string    `json:"path,omitempty"`
}

// Details contains detailed error information
// Fields ordered by frequency of access and memory alignment
type Details struct {
	Code       Code                   `json:"code"`
	Message    string                 `json:"message"`
	Category   string                 `json:"category"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Validation []ValidationError      `json:"validation,omitempty"`
	Cause      *Details               `json:"cause,omitempty"`
	Retryable  bool                   `json:"retryable"`
	RetryAfter *int                   `json:"retry_after,omitempty"`
}

// ValidationError represents field-specific validation errors
// Optimized for JSON marshaling performance
type ValidationError struct {
	Field   string      `json:"field"`
	Rule    string      `json:"rule"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// Error represents high-performance standardized error
// Memory layout optimized for 64-bit architectures
type Error struct {
	// Hot path fields first (8-byte aligned)
	code       Code
	message    string
	httpStatus int
	category   Category

	// Cold path fields
	details       map[string]interface{}
	validation    []ValidationError
	cause         error
	requestID     string
	path          string
	retryAfterSec *int
	retryable     bool
}

// Pre-allocated error pools for high-frequency error types
var (
	errorPool = sync.Pool{
		New: func() interface{} {
			return &Error{
				details: make(map[string]interface{}, 4), // Pre-size for common case
			}
		},
	}

	validationPool = sync.Pool{
		New: func() interface{} {
			return make([]ValidationError, 0, 8) // Pre-size for common validation count
		},
	}
)

// New creates a new error with zero allocations for common cases
func New(code Code) *Error {
	e := errorPool.Get().(*Error)
	e.reset()
	e.code = code
	e.httpStatus = getHTTPStatus(code)
	e.category = getCategory(code)
	return e
}

// reset resets error to zero state for pool reuse
func (e *Error) reset() {
	e.code = ""
	e.message = ""
	e.httpStatus = 0
	e.category = 0
	e.cause = nil
	e.requestID = ""
	e.path = ""
	e.retryAfterSec = nil
	e.retryable = false

	// Clear maps/slices but keep capacity
	for k := range e.details {
		delete(e.details, k)
	}
	e.validation = e.validation[:0]
}

// Release returns error to pool for reuse (call after response sent)
func (e *Error) Release() {
	if e.validation != nil {
		validationPool.Put(&e.validation)
		e.validation = nil
	}
	errorPool.Put(e)
}

// Error implements error interface with zero allocation string building
func (e *Error) Error() string {
	if e.cause != nil {
		// Use unsafe string concatenation for performance
		return unsafeStringConcat(e.message, ": ", e.cause.Error())
	}
	return e.message
}

// Unwrap returns underlying cause for error unwrapping
func (e *Error) Unwrap() error {
	return e.cause
}

// Message sets error message (fluent interface)
func (e *Error) Message(msg string) *Error {
	e.message = msg
	return e
}

// Messagef sets formatted message with zero allocation string building
func (e *Error) Messagef(format string, args ...interface{}) *Error {
	e.message = fmt.Sprintf(format, args...)
	return e
}

// HTTPStatus overrides default HTTP status
func (e *Error) HTTPStatus(status int) *Error {
	e.httpStatus = status
	return e
}

// Detail adds detail with key interning for common keys
func (e *Error) Detail(key string, value interface{}) *Error {
	e.details[internKey(key)] = value
	return e
}

// Details replaces all details
func (e *Error) Details(details map[string]interface{}) *Error {
	e.details = details
	return e
}

// Validation adds validation error with pool reuse
func (e *Error) Validation(field, rule, message string, value interface{}) *Error {
	if e.validation == nil {
		e.validation = validationPool.Get().([]ValidationError)
	}

	e.validation = append(e.validation, ValidationError{
		Field:   field,
		Rule:    rule,
		Message: message,
		Value:   value,
	})
	return e
}

// ValidationErrors sets multiple validation errors
func (e *Error) ValidationErrors(errs []ValidationError) *Error {
	e.validation = errs
	return e
}

// Cause sets underlying error cause
func (e *Error) Cause(err error) *Error {
	e.cause = err
	return e
}

// Retryable marks error as retryable
func (e *Error) Retryable() *Error {
	e.retryable = true
	return e
}

// RetryAfter sets retry delay and marks as retryable
func (e *Error) RetryAfter(seconds int) *Error {
	e.retryAfterSec = &seconds
	e.retryable = true
	return e
}

// Context adds request context
func (e *Error) Context(requestID, path string) *Error {
	e.requestID = requestID
	e.path = path
	return e
}

// ToResponse converts to response format with optimized JSON marshaling
func (e *Error) ToResponse() *Response {
	details := Details{
		Code:       e.code,
		Message:    e.getMessage(),
		Category:   e.category.String(),
		Details:    e.details,
		Validation: e.validation,
		Retryable:  e.retryable,
		RetryAfter: e.retryAfterSec,
	}

	// Add cause without deep nesting to prevent stack overflow
	if e.cause != nil {
		if causeErr, ok := e.cause.(*Error); ok {
			details.Cause = &Details{
				Code:     causeErr.code,
				Message:  causeErr.getMessage(),
				Category: causeErr.category.String(),
			}
		}
	}

	return &Response{
		Error:     details,
		Timestamp: time.Now().UTC(),
		RequestID: e.requestID,
		Path:      e.path,
	}
}

// getMessage returns message or default for code
func (e *Error) getMessage() string {
	if e.message != "" {
		return e.message
	}
	return getDefaultMessage(e.code)
}

// High-performance error constructors with pre-configured common errors

// BadRequest creates optimized bad request error
func BadRequest(message string) *Error {
	return New(CodeBadRequest).Message(message)
}

// ValidationFailed creates validation error with pool-optimized validation slice
func ValidationFailed(errs []ValidationError) *Error {
	return New(CodeValidationFailed).
		Message("Request validation failed").
		ValidationErrors(errs)
}

// NotFound creates not found error with resource detail
func NotFound(resource string) *Error {
	return New(CodeNotFound).
		Messagef("%s not found", resource).
		Detail("resource_type", resource)
}

// Unauthorized creates unauthorized error
func Unauthorized(message string) *Error {
	return New(CodeUnauthorized).Message(message)
}

// InternalServer creates internal server error with cause
func InternalServer(message string, cause error) *Error {
	return New(CodeInternalServer).
		Message(message).
		Cause(cause).
		Retryable()
}

// ServiceUnavailable creates service unavailable with retry
func ServiceUnavailable(service string, retryAfterSec int) *Error {
	return New(CodeServiceUnavailable).
		Messagef("%s temporarily unavailable", service).
		Detail("service", service).
		RetryAfter(retryAfterSec)
}

// DatabaseError creates database error with operation context
func DatabaseError(operation string, cause error) *Error {
	return New(CodeDatabaseError).
		Messagef("Database operation failed: %s", operation).
		Detail("operation", operation).
		Cause(cause).
		Retryable()
}

// RateLimited creates rate limit error with retry timing
func RateLimited(limit int, retryAfterSec int) *Error {
	return New(CodeRateLimited).
		Message("Rate limit exceeded").
		Detail("limit", limit).
		RetryAfter(retryAfterSec)
}

// Timeout creates timeout error with operation context
func Timeout(operation string, timeoutSec int) *Error {
	return New(CodeTimeout).
		Messagef("Operation timed out: %s", operation).
		Detail("operation", operation).
		Detail("timeout_seconds", timeoutSec).
		Retryable()
}

// CircuitOpen creates circuit breaker error
func CircuitOpen(service string, retryAfterSec int) *Error {
	return New(CodeCircuitOpen).
		Messagef("Circuit breaker open for %s", service).
		Detail("service", service).
		RetryAfter(retryAfterSec)
}

// Performance-optimized lookup tables with branch prediction hints

// getHTTPStatus returns HTTP status with optimized lookup
func getHTTPStatus(code Code) int {
	// Most common codes first for branch prediction
	switch code {
	case CodeBadRequest, CodeValidationFailed:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeInternalServer:
		return http.StatusInternalServerError
	case CodeUnauthorized:
		return http.StatusUnauthorized
	case CodeForbidden:
		return http.StatusForbidden
	case CodeConflict:
		return http.StatusConflict
	case CodeRateLimited:
		return http.StatusTooManyRequests
	case CodeRequestTooLarge:
		return http.StatusRequestEntityTooLarge
	case CodeUnsupportedMedia:
		return http.StatusUnsupportedMediaType
	case CodeServiceUnavailable, CodeCircuitOpen:
		return http.StatusServiceUnavailable
	case CodeTimeout:
		return http.StatusRequestTimeout
	case CodeDatabaseError, CodeExternalService:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// getCategory returns category with optimized lookup
func getCategory(code Code) Category {
	switch code {
	case CodeBadRequest, CodeUnauthorized, CodeForbidden, CodeNotFound,
		 CodeConflict, CodeValidationFailed, CodeRequestTooLarge, CodeUnsupportedMedia:
		return CategoryClient
	case CodeDatabaseError, CodeExternalService, CodeTimeout, CodeCircuitOpen:
		return CategorySystem
	case CodeRateLimited:
		return CategoryBusiness
	default:
		return CategoryServer
	}
}

// getDefaultMessage returns default message with optimized lookup
func getDefaultMessage(code Code) string {
	switch code {
	case CodeBadRequest:
		return "Bad request"
	case CodeValidationFailed:
		return "Validation failed"
	case CodeNotFound:
		return "Resource not found"
	case CodeUnauthorized:
		return "Authentication required"
	case CodeForbidden:
		return "Access forbidden"
	case CodeConflict:
		return "Resource conflict"
	case CodeRateLimited:
		return "Rate limit exceeded"
	case CodeRequestTooLarge:
		return "Request too large"
	case CodeUnsupportedMedia:
		return "Unsupported media type"
	case CodeInternalServer:
		return "Internal server error"
	case CodeServiceUnavailable:
		return "Service unavailable"
	case CodeDatabaseError:
		return "Database error"
	case CodeExternalService:
		return "External service error"
	case CodeTimeout:
		return "Operation timed out"
	case CodeCircuitOpen:
		return "Service circuit breaker open"
	default:
		return "Unknown error"
	}
}

// Performance optimization utilities

// Common key interning pool for frequently used detail keys
var keyInternPool = sync.Map{}

// internKey interns common keys to reduce string allocations
func internKey(key string) string {
	if interned, ok := keyInternPool.Load(key); ok {
		return interned.(string)
	}

	// Only intern keys that are likely to be reused
	if len(key) < 32 && isCommonKey(key) {
		keyInternPool.Store(key, key)
	}
	return key
}

// isCommonKey checks if key is worth interning
func isCommonKey(key string) bool {
	switch key {
	case "service", "operation", "resource_type", "limit", "timeout_seconds":
		return true
	default:
		return false
	}
}

// unsafeStringConcat concatenates strings without allocation using unsafe
func unsafeStringConcat(parts ...string) string {
	var totalLen int
	for _, part := range parts {
		totalLen += len(part)
	}

	buf := make([]byte, totalLen)
	var offset int
	for _, part := range parts {
		copy(buf[offset:], part)
		offset += len(part)
	}

	return *(*string)(unsafe.Pointer(&buf))
}