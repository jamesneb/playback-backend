package rest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHTTPStatusCodeConstants(t *testing.T) {
	assert.Equal(t, HTTPStatusCode(200), StatusOK)
	assert.Equal(t, HTTPStatusCode(204), StatusNoContent)
	assert.Equal(t, HTTPStatusCode(500), StatusInternalServerError)
}

func TestHTTPMethodConstants(t *testing.T) {
	assert.Equal(t, HTTPMethod("OPTIONS"), METHOD_OPTIONS)
	assert.Equal(t, HTTPMethod("GET"), METHOD_GET)
	assert.Equal(t, HTTPMethod("POST"), METHOD_POST)
}

func TestHeaderNameConstants(t *testing.T) {
	// Test that header constants are defined and have reasonable values
	headerConstants := map[string]HeaderName{
		"X-XSS-Protection":                    HEADER_XSS_PROTECTION,
		"X-Content-Type-Options":              HEADER_CONTENT_TYPE_OPTIONS,
		"X-Frame-Options":                     HEADER_FRAME_OPTIONS,
		"Referrer-Policy":                     HEADER_REFERRER_POLICY,
		"Content-Security-Policy":             HEADER_CONTENT_SECURITY_POLICY,
		"Strict-Transport-Security":           HEADER_HSTS,
		"Permissions-Policy":                  HEADER_PERMISSIONS_POLICY,
		"X-Permitted-Cross-Domain-Policies":   HEADER_CROSS_DOMAIN_POLICIES,
		"Cross-Origin-Embedder-Policy":        HEADER_COEP,
		"Cross-Origin-Opener-Policy":          HEADER_COOP,
		"Cross-Origin-Resource-Policy":        HEADER_CORP,
		"X-Forwarded-Proto":                   HEADER_X_FORWARDED_PROTO,
		"X-Forwarded-Ssl":                     HEADER_X_FORWARDED_SSL,
	}

	for expected, actual := range headerConstants {
		assert.Equal(t, HeaderName(expected), actual)
	}
}

func TestTypeAliases(t *testing.T) {
	// Test that type aliases work correctly
	var statusCode HTTPStatusCode = 200
	var headerName HeaderName = "Content-Type"
	var contentType ContentType = "application/json"
	var httpMethod HTTPMethod = "GET"
	var cspDirective CSPDirective = "default-src"
	var corsHeaderValue CORSHeaderValue = "*"
	var timeout = TimeoutDuration(30 * time.Second)
	var bufferSize BufferSize = 1024
	var pathSegment PathSegment = "/api/v1"

	assert.Equal(t, 200, int(statusCode))
	assert.Equal(t, "Content-Type", string(headerName))
	assert.Equal(t, "application/json", string(contentType))
	assert.Equal(t, "GET", string(httpMethod))
	assert.Equal(t, "default-src", string(cspDirective))
	assert.Equal(t, "*", string(corsHeaderValue))
	assert.Equal(t, 30*time.Second, time.Duration(timeout))
	assert.Equal(t, 1024, int(bufferSize))
	assert.Equal(t, "/api/v1", string(pathSegment))
}

func TestConstantsAreNotEmpty(t *testing.T) {
	// Test that string constants are not empty
	assert.NotEmpty(t, string(HEADER_XSS_PROTECTION))
	assert.NotEmpty(t, string(HEADER_CONTENT_TYPE_OPTIONS))
	assert.NotEmpty(t, string(HEADER_FRAME_OPTIONS))
	assert.NotEmpty(t, string(METHOD_GET))
	assert.NotEmpty(t, string(METHOD_POST))
	assert.NotEmpty(t, string(METHOD_OPTIONS))
}

func TestStatusCodeValues(t *testing.T) {
	// Test that status codes have expected numeric values
	assert.Greater(t, int(StatusOK), 0)
	assert.Greater(t, int(StatusNoContent), 0)
	assert.Greater(t, int(StatusInternalServerError), 0)

	// Test specific values
	assert.Equal(t, 200, int(StatusOK))
	assert.Equal(t, 204, int(StatusNoContent))
	assert.Equal(t, 500, int(StatusInternalServerError))
}

func TestMethodStringConversion(t *testing.T) {
	// Test that HTTP methods can be converted to strings
	methods := []HTTPMethod{METHOD_GET, METHOD_POST, METHOD_OPTIONS}

	for _, method := range methods {
		strMethod := string(method)
		assert.NotEmpty(t, strMethod)
		assert.Greater(t, len(strMethod), 2) // Should be at least 3 characters
	}
}

func TestHeaderStringConversion(t *testing.T) {
	// Test that headers can be converted to strings
	headers := []HeaderName{
		HEADER_XSS_PROTECTION,
		HEADER_CONTENT_TYPE_OPTIONS,
		HEADER_FRAME_OPTIONS,
	}

	for _, header := range headers {
		strHeader := string(header)
		assert.NotEmpty(t, strHeader)
		assert.Contains(t, strHeader, "-") // Most headers contain hyphens
	}
}

func TestTypeAliasConversions(t *testing.T) {
	// Test conversions between type aliases and underlying types

	// HTTPStatusCode
	var code HTTPStatusCode = 404
	assert.Equal(t, 404, int(code))

	// TimeoutDuration
	var timeout = TimeoutDuration(5 * time.Minute)
	assert.Equal(t, 5*time.Minute, time.Duration(timeout))

	// BufferSize
	var buffer BufferSize = 2048
	assert.Equal(t, 2048, int(buffer))
}

func TestConstantConsistency(t *testing.T) {
	// Test that constants follow expected naming patterns

	// HTTP methods should be uppercase
	assert.Equal(t, "GET", string(METHOD_GET))
	assert.Equal(t, "POST", string(METHOD_POST))
	assert.Equal(t, "OPTIONS", string(METHOD_OPTIONS))

	// Status codes should have reasonable values
	assert.True(t, int(StatusOK) >= 200 && int(StatusOK) < 300)
	assert.True(t, int(StatusNoContent) >= 200 && int(StatusNoContent) < 300)
	assert.True(t, int(StatusInternalServerError) >= 500 && int(StatusInternalServerError) < 600)
}