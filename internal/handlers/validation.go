package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/logging"
	"go.uber.org/zap"
)

// Validation constants - centralized in constants.go but repeated here for clarity
const (
	// Character validation sets
	allowedServiceNameChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."
	hexChars               = "0123456789abcdefABCDEF"

	// Validation limits
	minValidStringLength = 1
	emptyStringLength    = 0
)

// RequestValidator handles validation of incoming HTTP requests with comprehensive
// error reporting and security-focused input sanitization.
type RequestValidator struct {
	logger *zap.Logger
}

// NewRequestValidator creates a new request validator with proper initialization.
// Returns a configured validator ready for use.
func NewRequestValidator() *RequestValidator {
	return &RequestValidator{
		logger: zap.L(), // Use the global zap logger
	}
}

// ValidateRequest performs comprehensive request validation including size limits,
// content type verification, and basic security checks.
//
// Parameters:
//   - c: Gin context containing the HTTP request
//
// Returns:
//   - *ValidationError: Detailed error information if validation fails, nil if successful
func (v *RequestValidator) ValidateRequest(c *gin.Context) *ValidationError {
	if err := v.validateRequestSize(c); err != nil {
		return err
	}

	if err := v.validateContentType(c); err != nil {
		return err
	}

	return nil
}

// ValidateOTLPTraceData performs comprehensive validation of OTLP trace data structure
// and content to ensure data integrity and prevent malformed input processing.
//
// Parameters:
//   - data: Raw JSON message containing OTLP trace data
//
// Returns:
//   - *ValidationError: Detailed error if validation fails, nil if successful
func (v *RequestValidator) ValidateOTLPTraceData(data json.RawMessage) *ValidationError {
	if err := v.validateDataSize(data); err != nil {
		return err
	}

	if err := v.validateOTLPStructure(data); err != nil {
		return err
	}

	if err := v.validateResourceSpansCount(data); err != nil {
		return err
	}

	return nil
}

// ValidationError represents a comprehensive validation error with detailed
// context information for debugging and client feedback.
type ValidationError struct {
	Type    ValidationType `json:"type"`
	Field   string         `json:"field"`
	Message string         `json:"message"`
	Code    string         `json:"code"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// Error implements the standard error interface with formatted output.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error [%s] %s: %s", e.Type, e.Field, e.Message)
}

// ValidationType represents the category of validation error for proper error handling.
type ValidationType string

const (
	ValidationTypeSize      ValidationType = "size"
	ValidationTypeFormat    ValidationType = "format"
	ValidationTypeStructure ValidationType = "structure"
	ValidationTypeLimit     ValidationType = "limit"
	ValidationTypeContent   ValidationType = "content"
)

// validateRequestSize checks if the request size is within configured limits
// to prevent DoS attacks and resource exhaustion.
func (v *RequestValidator) validateRequestSize(c *gin.Context) *ValidationError {
	contentLength := c.Request.ContentLength

	if contentLength > MaxPayloadSize {
		v.logger.Warn("Request payload exceeds size limit",
			zap.String("content_length", logging.SanitizeDataSize(int(contentLength))),
			zap.String("max_allowed", logging.SanitizeDataSize(int(MaxPayloadSize))),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())),
			zap.String("user_agent", logging.SanitizeUserAgent(c.GetHeader("User-Agent"))))

		return &ValidationError{
			Type:    ValidationTypeSize,
			Field:   "payload",
			Message: ErrPayloadTooLarge,
			Code:    "PAYLOAD_TOO_LARGE",
			Details: map[string]interface{}{
				"received_size": contentLength,
				"max_size":     MaxPayloadSize,
			},
		}
	}

	return nil
}

// validateContentType verifies that the request content type is acceptable
// for the expected JSON payload format.
func (v *RequestValidator) validateContentType(c *gin.Context) *ValidationError {
	contentType := c.GetHeader("Content-Type")

	if !strings.Contains(contentType, ContentTypeJSON) {
		v.logger.Warn("Invalid content type received",
			zap.String("content_type", contentType),
			zap.String("expected", ContentTypeJSON),
			zap.String("client_ip", logging.SanitizeClientIP(c.ClientIP())))

		return &ValidationError{
			Type:    ValidationTypeFormat,
			Field:   "content-type",
			Message: ErrInvalidContentType,
			Code:    "INVALID_CONTENT_TYPE",
			Details: map[string]interface{}{
				"received": contentType,
				"expected": ContentTypeJSON,
			},
		}
	}

	return nil
}

// validateDataSize ensures the OTLP data meets minimum size requirements
// for valid telemetry data.
func (v *RequestValidator) validateDataSize(data json.RawMessage) *ValidationError {
	dataSize := len(data)

	if dataSize < MinOTLPDataSize {
		return &ValidationError{
			Type:    ValidationTypeSize,
			Field:   "payload",
			Message: "OTLP data too small to contain valid telemetry",
			Code:    "PAYLOAD_TOO_SMALL",
			Details: map[string]interface{}{
				"received_size": dataSize,
				"min_size":     MinOTLPDataSize,
			},
		}
	}

	return nil
}

// validateOTLPStructure performs structural validation of OTLP trace data
// to ensure it conforms to the expected format specification.
func (v *RequestValidator) validateOTLPStructure(data json.RawMessage) *ValidationError {
	otlpStructure := &otlpTraceStructure{}

	if err := json.Unmarshal(data, otlpStructure); err != nil {
		v.logger.Debug("Failed to parse OTLP structure",
			zap.Error(err),
			zap.String("data_size", logging.SanitizeDataSize(len(data))))

		return &ValidationError{
			Type:    ValidationTypeStructure,
			Field:   "otlp",
			Message: ErrInvalidOTLPStructure,
			Code:    "INVALID_OTLP_STRUCTURE",
			Details: map[string]interface{}{
				"parse_error": err.Error(),
			},
		}
	}

	if !v.hasValidResourceSpans(otlpStructure) {
		return &ValidationError{
			Type:    ValidationTypeStructure,
			Field:   "resourceSpans",
			Message: "No valid resource spans found in OTLP data",
			Code:    "NO_RESOURCE_SPANS",
		}
	}

	if !v.hasValidSpans(otlpStructure) {
		return &ValidationError{
			Type:    ValidationTypeStructure,
			Field:   "spans",
			Message: "No valid spans found in resource spans",
			Code:    "NO_SPANS_FOUND",
		}
	}

	return nil
}

// validateResourceSpansCount ensures the number of resource spans is within
// processing limits to prevent resource exhaustion.
func (v *RequestValidator) validateResourceSpansCount(data json.RawMessage) *ValidationError {
	var resourceSpansCount struct {
		ResourceSpans []json.RawMessage `json:"resourceSpans"`
	}

	if err := json.Unmarshal(data, &resourceSpansCount); err != nil {
		return &ValidationError{
			Type:    ValidationTypeStructure,
			Field:   "resourceSpans",
			Message: "Failed to parse resource spans for count validation",
			Code:    "RESOURCE_SPANS_PARSE_ERROR",
		}
	}

	spanCount := len(resourceSpansCount.ResourceSpans)
	if spanCount > MaxResourceSpans {
		return &ValidationError{
			Type:    ValidationTypeLimit,
			Field:   "resourceSpans",
			Message: fmt.Sprintf("Too many resource spans (max: %d)", MaxResourceSpans),
			Code:    "TOO_MANY_RESOURCE_SPANS",
			Details: map[string]interface{}{
				"received_count": spanCount,
				"max_count":     MaxResourceSpans,
			},
		}
	}

	return nil
}

// otlpTraceStructure represents the expected OTLP trace data structure
// for validation purposes, supporting both current and legacy formats.
type otlpTraceStructure struct {
	ResourceSpans []struct {
		Resource struct {
			Attributes []json.RawMessage `json:"attributes"`
		} `json:"resource"`
		ScopeSpans []struct {
			Spans []json.RawMessage `json:"spans"`
		} `json:"scopeSpans"`
		// Legacy format support for backward compatibility
		InstrumentationLibrarySpans []struct {
			Spans []json.RawMessage `json:"spans"`
		} `json:"instrumentationLibrarySpans"`
	} `json:"resourceSpans"`
}

// hasValidResourceSpans checks if the OTLP structure contains valid resource spans.
func (v *RequestValidator) hasValidResourceSpans(otlp *otlpTraceStructure) bool {
	return len(otlp.ResourceSpans) > emptyStringLength
}

// hasValidSpans checks if any resource span contains valid span data,
// supporting both modern and legacy OTLP formats.
func (v *RequestValidator) hasValidSpans(otlp *otlpTraceStructure) bool {
	for _, rs := range otlp.ResourceSpans {
		// Check modern scopeSpans format
		if len(rs.ScopeSpans) > emptyStringLength {
			return true
		}
		// Check legacy instrumentationLibrarySpans format
		if len(rs.InstrumentationLibrarySpans) > emptyStringLength {
			return true
		}
	}
	return false
}

// ValidateServiceName performs comprehensive validation and sanitization
// of service names to ensure they meet security and format requirements.
//
// Parameters:
//   - serviceName: Raw service name extracted from telemetry data
//
// Returns:
//   - string: Validated and sanitized service name, or default if invalid
func (v *RequestValidator) ValidateServiceName(serviceName string) string {
	if len(serviceName) == emptyStringLength {
		return DefaultServiceName
	}

	// Trim whitespace and enforce length limits
	serviceName = strings.TrimSpace(serviceName)
	if len(serviceName) > MaxServiceNameLength {
		v.logger.Debug("Service name truncated due to length",
			zap.String("original", serviceName),
			zap.Int("max_length", MaxServiceNameLength))
		serviceName = serviceName[:MaxServiceNameLength]
	}

	// Sanitize to allowed characters only
	sanitized := v.sanitizeString(serviceName, allowedServiceNameChars)

	if len(sanitized) < minValidStringLength {
		v.logger.Debug("Service name sanitization resulted in empty string",
			zap.String("original", serviceName))
		return DefaultServiceName
	}

	return sanitized
}

// ValidateTraceID performs comprehensive validation of trace IDs including
// format verification, length checks, and character validation.
//
// Parameters:
//   - traceID: Raw trace ID extracted from telemetry data
//
// Returns:
//   - string: Validated trace ID in lowercase hex format, or empty if invalid
func (v *RequestValidator) ValidateTraceID(traceID string) string {
	if len(traceID) == emptyStringLength {
		return ""
	}

	// Remove whitespace
	traceID = strings.TrimSpace(traceID)

	// Validate length bounds
	if len(traceID) < MinTraceIDLength || len(traceID) > MaxTraceIDLength {
		v.logger.Debug("Trace ID failed length validation",
			zap.String("trace_id", traceID),
			zap.Int("length", len(traceID)),
			zap.Int("min_length", MinTraceIDLength),
			zap.Int("max_length", MaxTraceIDLength))
		return ""
	}

	// Validate hexadecimal characters only
	if !v.isValidHexString(traceID) {
		v.logger.Debug("Trace ID contains invalid characters",
			zap.String("trace_id", traceID))
		return ""
	}

	// Return normalized lowercase hex string
	return strings.ToLower(traceID)
}

// sanitizeString removes all characters not present in the allowed character set.
//
// Parameters:
//   - input: String to sanitize
//   - allowedChars: String containing all allowed characters
//
// Returns:
//   - string: Sanitized string containing only allowed characters
func (v *RequestValidator) sanitizeString(input, allowedChars string) string {
	var result strings.Builder
	result.Grow(len(input)) // Pre-allocate capacity for efficiency

	for _, char := range input {
		if strings.ContainsRune(allowedChars, char) {
			result.WriteRune(char)
		}
	}

	return result.String()
}

// isValidHexString verifies that all characters in the string are valid hexadecimal.
//
// Parameters:
//   - s: String to validate
//
// Returns:
//   - bool: True if all characters are valid hex, false otherwise
func (v *RequestValidator) isValidHexString(s string) bool {
	for _, char := range s {
		if !strings.ContainsRune(hexChars, char) {
			return false
		}
	}
	return true
}