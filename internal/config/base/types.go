// Defines universal config types
package base

import "fmt"

type (
	Port        = uint16 // TCP/UDP port (1-65535)
	Byte        = uint64 // Primarily for readability in constants representing sizes
	LogLevel    uint8
	LogFormat   uint8
	Environment uint8
)

// HTTP types
type (
	HTTPMode   uint8
	HTTPMethod string
	HTTPHeader string
	Path       string
	Host       string
)

// AWS types
type AWSRegion uint8

// TLS types
type TLSVersion uint8

// Data export types
type DataExportFormat uint8

// Percentage represents a value from 0-100 with enforced validation
type Percentage struct {
	value uint8
}

// NewPercentage creates a validated Percentage
func NewPercentage(val uint8) (Percentage, error) {
	if val > 100 {
		return Percentage{}, fmt.Errorf("percentage must be 0-100, got %d", val)
	}
	return Percentage{value: val}, nil
}

// Value returns the underlying uint8 value
func (p Percentage) Value() uint8 {
	return p.value
}

// Logging constants
const (
	// --- LEVELS ---
	LOG_DEBUG LogLevel = iota
	LOG_INFO
	LOG_WARN
	LOG_ERR
	LOG_FATAL
	// --- FORMATTING ---
	LOG_JSON LogFormat = iota
	LOG_CONSOLE
)

// Environment constants
const (
	LOCAL_ENV Environment = iota
	DEV_ENV
	STAGE_ENV
	PROD_ENV
	TEST_ENV
)

// HTTP mode constants
const (
	HTTP_MODE_DEBUG HTTPMode = iota
	HTTP_MODE_RELEASE
	HTTP_MODE_TEST
)

// HTTP method constants
const (
	HTTP_METHOD_GET     HTTPMethod = "GET"
	HTTP_METHOD_POST    HTTPMethod = "POST"
	HTTP_METHOD_PUT     HTTPMethod = "PUT"
	HTTP_METHOD_DELETE  HTTPMethod = "DELETE"
	HTTP_METHOD_OPTIONS HTTPMethod = "OPTIONS"
	HTTP_METHOD_HEAD    HTTPMethod = "HEAD"
	HTTP_METHOD_PATCH   HTTPMethod = "PATCH"
)

// HTTP header constants
const (
	HTTP_HEADER_ORIGIN           HTTPHeader = "Origin"
	HTTP_HEADER_CONTENT_TYPE     HTTPHeader = "Content-Type"
	HTTP_HEADER_ACCEPT           HTTPHeader = "Accept"
	HTTP_HEADER_AUTHORIZATION    HTTPHeader = "Authorization"
	HTTP_HEADER_X_REQUESTED_WITH HTTPHeader = "X-Requested-With"
	HTTP_HEADER_CONTENT_LENGTH   HTTPHeader = "Content-Length"
)

// AWS region constants
const (
	AWS_US_EAST_1 AWSRegion = iota
	AWS_US_EAST_2
	AWS_US_WEST_1
	AWS_US_WEST_2
	AWS_EU_WEST_1
	AWS_EU_WEST_2
	AWS_EU_CENTRAL_1
	AWS_AP_SOUTHEAST_1
	AWS_AP_SOUTHEAST_2
	AWS_AP_NORTHEAST_1
)

// TLS version constants
const (
	TLS_1_0 TLSVersion = iota
	TLS_1_1
	TLS_1_2
	TLS_1_3
)

// Data export format constants
const (
	DATA_EXPORT_JSON DataExportFormat = iota
	DATA_EXPORT_CSV
	DATA_EXPORT_PARQUET
)

// SI constants
const (
	KILO int = 1_000
	MEGA int = 1_000_000 // Useful in various calculations. i.e., can be combined with Byte for an (MB) value (not MiB)
)

// Math constants
const (
	Infinity string = "\u221E" // "∞"
)

// Application component suffixes
const (
	COMPONENT_BACKEND = "-backend"
	COMPONENT_API     = "-api"
)

// Config validator types
// A small, local constraint so we don't rely on exp/constraints.
type Number interface {
	~int | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Multiple section config sets can use a Validator
type Validator struct {
	errs   []error
	prefix string
}
