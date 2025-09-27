package rest

import (
	"testing"

	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/stretchr/testify/assert"
)

func TestHTTPStatusCodes(t *testing.T) {
	// Test HTTP status codes
	assert.Equal(t, constants.HTTPStatusCode(200), constants.StatusOK)
	assert.Equal(t, constants.HTTPStatusCode(204), constants.StatusNoContent)
	assert.Equal(t, constants.HTTPStatusCode(500), constants.StatusInternalServerError)
	assert.Equal(t, constants.HTTPStatusCode(503), constants.StatusServiceUnavailable)
}

func TestHTTPMethods(t *testing.T) {
	// Test HTTP methods
	assert.Equal(t, constants.HTTPMethod("OPTIONS"), constants.MethodOptions)
	assert.Equal(t, constants.HTTPMethod("GET"), constants.MethodGet)
	assert.Equal(t, constants.HTTPMethod("POST"), constants.MethodPost)
}

func TestHeaders(t *testing.T) {
	// Test header constants
	assert.Equal(t, constants.HeaderName("X-XSS-Protection"), constants.HeaderXSSProtection)
	assert.Equal(t, constants.HeaderName("Content-Type"), constants.HeaderContentType)
	assert.Equal(t, constants.HeaderName("Access-Control-Allow-Origin"), constants.HeaderAccessControlAllowOrigin)
}

func TestContentTypes(t *testing.T) {
	// Test content types
	assert.Equal(t, constants.ContentType("text/plain"), constants.ContentTypeTextPlain)
	assert.Equal(t, constants.ContentType("application/json"), constants.ContentTypeApplicationJSON)
	assert.Equal(t, constants.ContentType("text/plain; version=0.0.4; charset=utf-8"), constants.ContentTypePrometheusMetrics)
}

func TestSecurityValues(t *testing.T) {
	// Test security header values
	assert.Equal(t, "1; mode=block", constants.XSSProtectionValue)
	assert.Equal(t, "nosniff", constants.ContentTypeOptionsValue)
	assert.Equal(t, "DENY", constants.FrameOptionsDeny)
}

func TestCSPDirectives(t *testing.T) {
	// Test CSP directives
	assert.Equal(t, constants.CSPDirective("default-src 'self'"), constants.CSPDefaultSrcSelf)
	assert.Equal(t, constants.CSPDirective("script-src 'self'"), constants.CSPScriptSrcSelf)
	assert.Equal(t, constants.CSPDirective("object-src 'none'"), constants.CSPObjectSrcNone)
}

func TestPathSegments(t *testing.T) {
	// Test path constants
	assert.Equal(t, constants.PathSegment("/"), constants.RootPath)
	assert.Equal(t, constants.PathSegment("/health"), constants.HealthEndpointPath)
	assert.Equal(t, constants.PathSegment("/metrics"), constants.MetricsEndpointPath)
}

func TestHealthConstants(t *testing.T) {
	// Test health check constants
	assert.Equal(t, "healthy", constants.HealthStatusHealthy)
	assert.Equal(t, "unhealthy", constants.HealthStatusUnhealthy)
	assert.Equal(t, "ok", constants.HealthStatusOK)
	assert.Equal(t, "database", constants.HealthDependencyDatabase)
	assert.Equal(t, "kinesis", constants.HealthDependencyKinesis)
}

func TestCORSConstants(t *testing.T) {
	// Test CORS constants
	assert.Equal(t, "*", constants.CORSWildcardOrigin)
	assert.Equal(t, "86400", constants.CORSMaxAgeSeconds)
}

func TestApplicationConstants(t *testing.T) {
	// Test application constants
	assert.Equal(t, "localhost", constants.LocalHost)
	assert.Equal(t, "replays", constants.ReplayS3BucketName)
	assert.Equal(t, 1024, constants.CompressionMinSize)
	assert.Equal(t, int64(32<<20), constants.MaxMultipartMemory)
}

func TestProtocolConstants(t *testing.T) {
	// Test protocol constants
	assert.Equal(t, "HTTP/JSON", constants.ProtocolHTTPJSON)
	assert.Equal(t, "gRPC/OTLP", constants.ProtocolGRPCOTLP)
}

func TestErrorConstants(t *testing.T) {
	// Test error message constants
	assert.Equal(t, "dependencies cannot be nil", constants.ErrorDependenciesNil)
	assert.Equal(t, "config cannot be nil", constants.ErrorConfigNil)
	assert.Equal(t, "failed to create trace handler", constants.ErrorTraceHandlerCreation)
	assert.Equal(t, "unknown", constants.ErrorUnknownVersion)
}

func TestTimeoutConstants(t *testing.T) {
	// Test timeout constants
	assert.NotZero(t, constants.RequestTimeout)
	assert.NotZero(t, constants.HealthCheckTimeout)
}

func TestRegexConstants(t *testing.T) {
	// Test regex constants
	assert.NotNil(t, constants.VersionSanitizeRegex)

	// Test the regex sanitizes invalid characters (keeps only alphanumeric, dots, and hyphens)
	result := constants.VersionSanitizeRegex.ReplaceAllString("v1.2.3-beta+build*unsafe", "")
	assert.Equal(t, "v1.2.3-betabuildunsafe", result) // + and * are removed, dots and hyphens kept
}
