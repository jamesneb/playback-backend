package http

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/app"
	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	HTTP_PREFIX = "HTTP_"
)

// Time period constants
const (
	ONE_HOUR  = 1 * time.Hour
	ONE_DAY   = 24 * time.Hour
	ONE_WEEK  = 168 * time.Hour
	ONE_MONTH = 720 * time.Hour // 30 days
)

// Port constants
const (
	MIN_PORT     base.Port = 1024
	MAX_PORT     base.Port = 65535
	DEFAULT_PORT base.Port = 8080
)

// Timeout constants
const (
	MIN_TIMEOUT = 1 * time.Second
	MAX_TIMEOUT = 5 * time.Minute

	DEFAULT_READ_TIMEOUT     = 30 * time.Second
	DEFAULT_WRITE_TIMEOUT    = 30 * time.Second
	DEFAULT_IDLE_TIMEOUT     = 60 * time.Second
	DEFAULT_SHUTDOWN_TIMEOUT = 30 * time.Second
)

// Size constants
const (
	MIN_REQUEST_SIZE     = base.Byte(base.MEGA * 1)
	MAX_REQUEST_SIZE     = base.Byte(base.MEGA * 100)
	DEFAULT_REQUEST_SIZE = base.Byte(base.MEGA * 25)

	MIN_HEADER_SIZE     = base.Byte(base.KILO * 1)
	MAX_HEADER_SIZE     = base.Byte(base.MEGA * 10)
	DEFAULT_HEADER_SIZE = base.Byte(base.MEGA * 1)
)

// Rate limiting constants
const (
	RATE_LIMIT_DISABLED = 0

	MIN_RPS     = 1
	MAX_RPS     = 1_000_000
	DEFAULT_RPS = 1000

	MIN_BURST     = 1
	MAX_BURST     = 2_000_000
	DEFAULT_BURST = 2000

	MAX_BURST_MULTIPLIER = 10
)

// JWT expiry and refresh
const (
	MIN_JWT_EXPIRY     = ONE_HOUR
	MAX_JWT_EXPIRY     = ONE_WEEK
	DEFAULT_JWT_EXPIRY = ONE_DAY

	// Refresh window: How long before expiry a token can be refreshed
	MIN_JWT_REFRESH_WINDOW     = ONE_HOUR
	MAX_JWT_REFRESH_WINDOW     = ONE_MONTH
	DEFAULT_JWT_REFRESH_WINDOW = ONE_WEEK
)

// CORS max age
const (
	MIN_CORS_MAX_AGE     = 0 * time.Second
	MAX_CORS_MAX_AGE     = ONE_DAY
	DEFAULT_CORS_MAX_AGE = ONE_HOUR
)

// Compression constants
const (
	MIN_COMPRESSION_LEVEL     = 1
	MAX_COMPRESSION_LEVEL     = 9
	DEFAULT_COMPRESSION_LEVEL = 6

	MIN_COMPRESSION_THRESHOLD     = base.Byte(base.KILO * 1)
	MAX_COMPRESSION_THRESHOLD     = base.Byte(base.MEGA * 1)
	DEFAULT_COMPRESSION_THRESHOLD = base.Byte(base.KILO * 1)
)

// Keep-alive timeout constants
const (
	MIN_KEEP_ALIVE_TIMEOUT     = 10 * time.Second
	MAX_KEEP_ALIVE_TIMEOUT     = 5 * time.Minute
	DEFAULT_KEEP_ALIVE_TIMEOUT = 1 * time.Minute
)

// Default TLS values
const (
	DEFAULT_TLS_ENABLED     = false
	DEFAULT_TLS_MIN_VERSION = base.TLS_1_2
	DEFAULT_TLS_MAX_VERSION = base.TLS_1_3
)

// Default values
const (
	DEFAULT_HOST             base.Host     = "0.0.0.0"
	DEFAULT_MODE             base.HTTPMode = base.HTTP_MODE_RELEASE
	DEFAULT_API_PREFIX                     = "/api/v1"
	DEFAULT_SWAGGER_PATH     base.Path     = "/swagger"
	DEFAULT_ENABLE_CORS                    = true
	DEFAULT_ENABLE_AUTH                    = false
	DEFAULT_ENABLE_PROFILING               = false
	DEFAULT_ENABLE_SWAGGER                 = false
	DEFAULT_ENABLE_DEBUG                   = false
	DEFAULT_KEEP_ALIVE                     = true
)

// JWT defaults (composed from app name + component suffixes)
var (
	DEFAULT_JWT_ISSUER   = app.DEFAULT_APP_NAME + base.COMPONENT_BACKEND
	DEFAULT_JWT_AUDIENCE = app.DEFAULT_APP_NAME + base.COMPONENT_API
)

// Default CORS configuration
const (
	DEFAULT_CORS_ALLOW_CREDENTIALS = false
)

var (
	DEFAULT_CORS_ALLOWED_ORIGINS = []string{"*"}
	DEFAULT_CORS_ALLOWED_METHODS = []base.HTTPMethod{
		base.HTTP_METHOD_GET,
		base.HTTP_METHOD_POST,
		base.HTTP_METHOD_PUT,
		base.HTTP_METHOD_DELETE,
		base.HTTP_METHOD_OPTIONS,
		base.HTTP_METHOD_HEAD,
		base.HTTP_METHOD_PATCH,
	}
	DEFAULT_CORS_ALLOWED_HEADERS = []base.HTTPHeader{
		base.HTTP_HEADER_ORIGIN,
		base.HTTP_HEADER_CONTENT_TYPE,
		base.HTTP_HEADER_ACCEPT,
		base.HTTP_HEADER_AUTHORIZATION,
		base.HTTP_HEADER_X_REQUESTED_WITH,
	}
	DEFAULT_CORS_EXPOSED_HEADERS = []base.HTTPHeader{
		base.HTTP_HEADER_CONTENT_LENGTH,
	}
)
