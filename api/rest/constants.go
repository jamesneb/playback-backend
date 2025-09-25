package rest

import (
	"compress/gzip"
	"regexp"
	"time"
)

// Type aliases for enhanced type safety and semantic clarity
type (
	HTTPStatusCode    int
	HeaderName        string
	ContentType       string
	HTTPMethod        string
	CSPDirective      string
	CORSHeaderValue   string
	TimeoutDuration   time.Duration
	BufferSize        int
	PathSegment       string
)

// HTTP Status Code constants (typed)
const (
	StatusOK                  HTTPStatusCode = 200
	StatusNoContent          HTTPStatusCode = 204
	StatusInternalServerError HTTPStatusCode = 500
)

// HTTP Method constants
const (
	METHOD_OPTIONS HTTPMethod = "OPTIONS"
	METHOD_GET     HTTPMethod = "GET"
	METHOD_POST    HTTPMethod = "POST"
)

// HTTP Header constants
const (
	HEADER_XSS_PROTECTION           HeaderName = "X-XSS-Protection"
	HEADER_CONTENT_TYPE_OPTIONS     HeaderName = "X-Content-Type-Options"
	HEADER_FRAME_OPTIONS           HeaderName = "X-Frame-Options"
	HEADER_REFERRER_POLICY         HeaderName = "Referrer-Policy"
	HEADER_CONTENT_SECURITY_POLICY HeaderName = "Content-Security-Policy"
	HEADER_HSTS                    HeaderName = "Strict-Transport-Security"
	HEADER_PERMISSIONS_POLICY      HeaderName = "Permissions-Policy"
	HEADER_CROSS_DOMAIN_POLICIES   HeaderName = "X-Permitted-Cross-Domain-Policies"
	HEADER_COEP                    HeaderName = "Cross-Origin-Embedder-Policy"
	HEADER_COOP                    HeaderName = "Cross-Origin-Opener-Policy"
	HEADER_CORP                    HeaderName = "Cross-Origin-Resource-Policy"
	HEADER_X_FORWARDED_PROTO       HeaderName = "X-Forwarded-Proto"
	HEADER_X_FORWARDED_SSL         HeaderName = "X-Forwarded-Ssl"
	HEADER_X_URL_SCHEME           HeaderName = "X-Url-Scheme"
	HEADER_ACCEPT_ENCODING        HeaderName = "Accept-Encoding"
	HEADER_CONTENT_TYPE           HeaderName = "Content-Type"
	HEADER_CONTENT_ENCODING       HeaderName = "Content-Encoding"
	HEADER_VARY                   HeaderName = "Vary"

	// CORS Headers
	HEADER_ACCESS_CONTROL_ALLOW_ORIGIN      HeaderName = "Access-Control-Allow-Origin"
	HEADER_ACCESS_CONTROL_ALLOW_CREDENTIALS HeaderName = "Access-Control-Allow-Credentials"
	HEADER_ACCESS_CONTROL_ALLOW_METHODS     HeaderName = "Access-Control-Allow-Methods"
	HEADER_ACCESS_CONTROL_ALLOW_HEADERS     HeaderName = "Access-Control-Allow-Headers"
	HEADER_ACCESS_CONTROL_MAX_AGE          HeaderName = "Access-Control-Max-Age"
)

// Header Value constants
const (
	XSS_PROTECTION_VALUE      = "1; mode=block"
	CONTENT_TYPE_OPTIONS_VALUE = "nosniff"
	FRAME_OPTIONS_DENY        = "DENY"
	REFERRER_POLICY_VALUE     = "strict-origin-when-cross-origin"
	HSTS_VALUE               = "max-age=31536000; includeSubDomains; preload"
	PERMISSIONS_POLICY_VALUE = "geolocation=(), microphone=(), camera=(), payment=(), usb=(), magnetometer=(), gyroscope=()"
	CROSS_DOMAIN_POLICIES_VALUE = "none"
	COEP_VALUE               = "require-corp"
	COOP_VALUE              = "same-origin"
	CORP_VALUE              = "same-origin"

	// Protocol values
	PROTO_HTTPS        = "https"
	SSL_ON_VALUE      = "on"
	ENCODING_GZIP     = "gzip"
	CORS_WILDCARD_ORIGIN = "*"
	CORS_MAX_AGE_SECONDS = "86400"
	TRUNCATION_SUFFIX    = "..."
)

// Content Type constants
const (
	CONTENT_TYPE_TEXT_PLAIN          ContentType = "text/plain"
	CONTENT_TYPE_APPLICATION_JSON    ContentType = "application/json"
	CONTENT_TYPE_PROMETHEUS_METRICS  ContentType = "text/plain; version=0.0.4; charset=utf-8"
	CONTENT_TYPE_IMAGE_PREFIX       = "image/"
	CONTENT_TYPE_VIDEO_PREFIX       = "video/"
)

// CSP Directive constants
const (
	CSP_DEFAULT_SRC_SELF        CSPDirective = "default-src 'self'"
	CSP_SCRIPT_SRC_SELF        CSPDirective = "script-src 'self'"
	CSP_SCRIPT_SRC_UNSAFE      CSPDirective = " 'unsafe-inline' 'unsafe-eval'"
	CSP_STYLE_SRC_UNSAFE_INLINE CSPDirective = "style-src 'self' 'unsafe-inline'"
	CSP_IMG_SRC_DATA_HTTPS     CSPDirective = "img-src 'self' data: https:"
	CSP_FONT_SRC_SELF         CSPDirective = "font-src 'self'"
	CSP_CONNECT_SRC_SELF      CSPDirective = "connect-src 'self'"
	CSP_CONNECT_SRC_HTTPS     CSPDirective = " https:"
	CSP_OBJECT_SRC_NONE       CSPDirective = "object-src 'none'"
	CSP_BASE_URI_SELF         CSPDirective = "base-uri 'self'"
	CSP_FORM_ACTION_SELF      CSPDirective = "form-action 'self'"
)

// Path constants
const (
	ROOT_PATH              PathSegment = "/"
	HEALTH_ENDPOINT_PATH   PathSegment = "/health"
	METRICS_ENDPOINT_PATH  PathSegment = "/metrics"
	DEBUG_PATH_PREFIX      PathSegment = "/debug"
	SWAGGER_PATH_SEGMENT   PathSegment = "swagger"
)

// Error Message constants
const (
	ERROR_DEPENDENCY_KEY_TIMEOUT     = "dependency key computation timeout"
	ERROR_TRACE_HANDLER_CREATION     = "failed to create trace handler"
	ERROR_METRICS_HANDLER_CREATION   = "failed to create metrics handler"
	ERROR_LOGS_HANDLER_CREATION      = "failed to create logs handler"
	ERROR_REPLAY_HANDLER_CREATION    = "failed to create replay handler"
	ERROR_DEPENDENCIES_NIL           = "dependencies cannot be nil"
	ERROR_CONFIG_NIL                = "config cannot be nil"
	ERROR_ENDPOINTS_NIL             = "endpoints cannot be nil"
	ERROR_GIN_SERVER_CREATION       = "gin server creation failed, possible memory issue"
	ERROR_CONFIG_MIDDLEWARE_NIL     = "config cannot be nil for middleware setup"
	CORS_NO_ORIGINS_WARNING         = "CORS enabled but no allowed origins configured"
	NIL_PANIC_MESSAGE              = "nil panic"
	UNKNOWN_VERSION                = "unknown"
)

// Dependency Hash Component constants
const (
	HASH_COMPONENT_KINESIS    = "kinesis:present"
	HASH_COMPONENT_S3        = "s3:present"
	HASH_COMPONENT_RESILIENCE = "resilience:present"
	HASH_COMPONENT_CLICKHOUSE = "clickhouse:present"
	TIMESTAMP_HASH_FORMAT    = "ts:%d"
)

// Protocol and Response constants
const (
	PROTOCOL_HTTP_JSON = "HTTP/JSON"
	PROTOCOL_GRPC_OTLP = "gRPC/OTLP"

	METRICS_PLACEHOLDER_CONTENT = "# Metrics endpoint placeholder\n# Prometheus metrics would be served here\n"
	PPROF_PLACEHOLDER_CONTENT  = "# pprof endpoint placeholder\n# Performance profiling would be served here\n"
)

// Numeric constants
const (
	SECONDS_PER_HOUR           = 3600
	DEPENDENCY_KEY_LENGTH      = 16
	MAX_PANIC_MESSAGE_LENGTH   = 200
	HOUR_PRECISION_DIVISOR     = 3600
	CHANNEL_BUFFER_SIZE        = 1
)

// Application constants
const (
	LOCAL_HOST                    string        = "localhost"
	REPLAY_S3_BUCKET_NAME        string        = "replays"
	DEFAULT_COMPRESSION_LEVEL     int           = gzip.DefaultCompression
	COMPRESSION_MIN_SIZE         int           = 1024 // Don't compress responses smaller than 1KB
	MAX_COMPRESSION_BUFFER_SIZE   int           = 64 * 1024 // 64KB max buffer
	REQUEST_TIMEOUT              time.Duration = 30 * time.Second
	MAX_MULTIPART_MEMORY         int64         = 32 << 20 // 32 MB
	STANDARD_TIME_FORMAT         string        = "2006-01-02 15:04:05"
	MAX_ROUTE_SEARCH_ITERATIONS  int           = 1000
	HASH_COMPUTATION_TIMEOUT     time.Duration = 100 * time.Millisecond
)

var (
	// Precompiled regex for version sanitization
	versionSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9.-]`)
)