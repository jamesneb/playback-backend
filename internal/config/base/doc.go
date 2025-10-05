// Package base defines shared types and validation utilities for the configuration system.
//
// # Overview
//
// This package provides foundational utilities used by all config packages:
//
//   - Type definitions: Port, Byte, LogLevel, Environment, etc.
//   - Validation framework: Validator and validation helpers
//   - Map utilities: Merge, Fingerprint, Normalize
//   - Constants: SI prefixes, mathematical symbols, component suffixes
//
// All configuration packages depend on base for consistent validation and type safety.
//
// # Type Definitions
//
// Core types:
//
//	Port        - TCP/UDP port (1-65535)
//	Byte        - Size in bytes (uint64 for readability)
//	LogLevel    - Logging level (DEBUG, INFO, WARN, ERROR, FATAL)
//	LogFormat   - Log output format (JSON, CONSOLE)
//	Environment - Deployment environment (LOCAL, DEV, STAGE, PROD, TEST)
//
// HTTP types:
//
//	HTTPMode   - HTTP server mode (DEBUG, RELEASE, TEST)
//	HTTPMethod - HTTP request method (GET, POST, PUT, DELETE, etc.)
//	HTTPHeader - HTTP header name
//	Path       - URL or file path
//	Host       - Hostname or IP address
//
// Infrastructure types:
//
//	AWSRegion        - AWS region identifier
//	TLSVersion       - TLS protocol version (1.0, 1.1, 1.2, 1.3)
//	DataExportFormat - Export file format (JSON, CSV, PARQUET)
//	Percentage       - Validated 0-100 percentage value
//
// # Validation Framework
//
// The Validator provides a fluent API for configuration validation:
//
//	v := base.NewValidator("GRPC_")
//
//	// Range validation
//	base.RangeFNum(v, "port", port, 1024, 65535, "")
//
//	// Positive number validation
//	base.GT0FNum(v, "max_size", maxSize, "bytes")
//
//	// Conditional validation
//	v.When(tlsEnabled, func(v *base.Validator) {
//	    base.NotEmpty(v, "cert_file", certFile)
//	    base.NotEmpty(v, "key_file", keyFile)
//	})
//
//	// Check for errors
//	if err := v.Err(); err != nil {
//	    return err
//	}
//
// # Validation Helpers
//
// Range validation:
//
//	RangeFNum[T Number](v, field, val, min, max, unit)
//	  - Validates val is within [min, max]
//	  - Adds unit suffix to error messages (e.g., "bytes", "seconds")
//
//	RangeOrAllowed[T Number](v, field, val, min, max, unit, allowed...)
//	  - Validates val is in [min, max] OR equals any allowed sentinel
//	  - Useful for "0 means disabled" patterns
//
// Positive validation:
//
//	GT0FNum[T Number](v, field, val, unit)
//	  - Validates val > 0
//	  - Useful for "must be positive" requirements
//
// String validation:
//
//	NotEmpty(v, field, val)
//	  - Validates string is not empty
//
// Comparison validation:
//
//	Equal[T comparable](v, field, val, expected, expectedName)
//	  - Validates val equals expected
//
//	NotEqual[T comparable](v, field, val, other, otherName)
//	  - Validates val differs from other
//
//	LTE[T Number](v, field, val, max, maxFieldName)
//	  - Validates val <= max
//	  - Useful for dependent field constraints
//
// Collection validation:
//
//	AllUnique[T comparable](v, field, vals)
//	  - Validates all slice elements are unique
//
// # Validation Patterns
//
// Conditional validation with When:
//
//	v.When(cfg.EnableMetrics, func(v *base.Validator) {
//	    base.RangeFNum(v, "metrics_port", cfg.MetricsPort, 1024, 65535, "")
//	    base.NotEmpty(v, "metrics_path", string(cfg.MetricsPath))
//	})
//
// Coupled field validation:
//
//	// Burst capacity must not exceed RPS
//	base.LTE(v, "burst", burst, rps, "requests_per_second")
//
// Sentinel value handling:
//
//	// Allow 0 to disable, otherwise require 1-10000
//	base.RangeOrAllowed(v, "rate_limit", rate, 1, 10000, "", 0)
//
// Custom assertions:
//
//	v.Assert("field", condition, "error message with %s", "args")
//
// # Map Utilities
//
// Merge maps with last-layer-wins semantics:
//
//	defaults := map[string]string{"key1": "default", "key2": "default"}
//	overrides := map[string]string{"key1": "override"}
//	result := base.Merge(defaults, overrides)
//	// result: {"key1": "override", "key2": "default"}
//
// Fingerprint for change detection:
//
//	hash1 := base.Fingerprint(configMap1)
//	hash2 := base.Fingerprint(configMap2)
//	if hash1 != hash2 {
//	    // Configuration changed, reload
//	}
//
// Note: Fingerprint uses per-process random seed. Not stable across processes.
// Suitable for in-process change detection, not for persistence or comparison.
//
// Normalize keys for consistency:
//
//	normalized := base.Normalize("my.config-key")
//	// Returns: "MY_CONFIG_KEY"
//	// Converts to uppercase, replaces . and - with _
//
// # Constants
//
// SI prefixes (base-10):
//
//	KILO = 1,000
//	MEGA = 1,000,000
//
// Usage example:
//
//	minSize := base.Byte(base.KILO * 10)      // 10KB
//	maxSize := base.Byte(base.MEGA * 100)     // 100MB
//
// Mathematical symbols:
//
//	Infinity = "\u221E"  // "∞" for display
//
// Component suffixes:
//
//	COMPONENT_BACKEND = "-backend"
//	COMPONENT_API     = "-api"
//
// # Type Conversion Methods
//
// All enum types provide String() methods:
//
//	level := base.LOG_INFO
//	fmt.Println(level.String())  // "info"
//
//	env := base.PROD_ENV
//	fmt.Println(env.String())    // "prod"
//
//	version := base.TLS_1_3
//	fmt.Println(version.String())  // "1.3"
//
// # Percentage Type
//
// The Percentage type enforces 0-100 range at construction:
//
//	// Valid
//	p, err := base.NewPercentage(50)  // err == nil, p.Value() == 50
//
//	// Invalid
//	p, err := base.NewPercentage(150)  // err != nil
//
// Use for sampling rates, progress indicators, or any 0-100% value.
//
// # Validator API
//
// Create a validator for a config section:
//
//	v := base.NewValidator("HTTP_")
//
// The prefix is prepended to all field names in error messages:
//
//	base.RangeFNum(v, "port", 99999, 1024, 65535, "")
//	// Error: "HTTP_.port out of bounds [1024, 65535]: 99999"
//
// Collect all validation errors:
//
//	v := base.NewValidator("GRPC_")
//	base.RangeFNum(v, "port", 0, 1, 65535, "")
//	base.GT0FNum(v, "max_size", -1, "bytes")
//	err := v.Err()
//	// Returns errors.Join() of all validation failures
//
// Validation is fail-continue, not fail-fast:
//
//   - All validations run even if some fail
//   - All errors returned together via Err()
//   - Allows comprehensive error reporting
//
// # Number Constraint
//
// Generic validation functions use the Number constraint:
//
//	type Number interface {
//	    ~int | ~int32 | ~int64 |
//	    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
//	    ~float32 | ~float64
//	}
//
// This allows validation of any numeric type:
//
//	base.RangeFNum(v, "port", uint16(8080), uint16(1024), uint16(65535), "")
//	base.RangeFNum(v, "timeout", 30*time.Second, 1*time.Second, 5*time.Minute, "")
//	base.RangeFNum(v, "rate", 0.5, 0.0, 1.0, "")
//
// # Error Messages
//
// Validation helpers generate descriptive error messages:
//
//	RangeFNum:
//	  "HTTP_.port out of bounds [1024, 65535]: 99999"
//	  "GRPC_.max_receive out of bounds [1, ∞] bytes: 0"
//
//	NotEmpty:
//	  "GRPC_.cert_file: cannot be empty"
//
//	Assert:
//	  "GRPC_.burst: must be 0 when requests_per_second=0 (got 100)"
//
//	LTE:
//	  "GRPC_.burst: cannot exceed requests_per_second (200 > 100)"
//
// All errors include the full field name (prefix + field) for clarity.
//
// # Best Practices
//
// Use typed constants for readability:
//
//	// Good
//	const (
//	    MIN_BUFFER = base.Byte(base.KILO * 1)   // 1KB
//	    MAX_BUFFER = base.Byte(base.MEGA * 100) // 100MB
//	)
//
//	// Avoid raw numbers
//	const MIN_BUFFER = 1000  // Is this bytes or KB?
//
// Use validators for all config structs:
//
//	func (c Config) Validate() error {
//	    v := base.NewValidator("MYCONFIG_")
//	    // Add validations...
//	    return v.Err()
//	}
//
// Group related validations with When:
//
//	v.When(c.TLS.Enabled, func(v *base.Validator) {
//	    base.NotEmpty(v, "tls_cert", c.TLS.Cert)
//	    base.NotEmpty(v, "tls_key", c.TLS.Key)
//	})
//
// Use sentinel values with RangeOrAllowed:
//
//	const DISABLED = 0
//	base.RangeOrAllowed(v, "rate", rate, 1, 10000, "", DISABLED)
//
// # Example: Complete Validation
//
// Typical configuration validation pattern:
//
//	type Config struct {
//	    Port       base.Port
//	    MaxSize    base.Byte
//	    EnableAuth bool
//	    Secret     string
//	    RateLimit  int
//	    Burst      int
//	}
//
//	func (c Config) Validate() error {
//	    v := base.NewValidator("MYAPP_")
//
//	    // Always validate
//	    base.RangeFNum(v, "port", c.Port, 1024, 65535, "")
//	    base.GT0FNum(v, "max_size", c.MaxSize, "bytes")
//
//	    // Conditional validation
//	    v.When(c.EnableAuth, func(v *base.Validator) {
//	        base.NotEmpty(v, "secret", c.Secret)
//	    })
//
//	    // Coupled validation
//	    base.RangeOrAllowed(v, "rate_limit", c.RateLimit, 1, 10000, "", 0)
//	    v.When(c.RateLimit > 0, func(v *base.Validator) {
//	        base.RangeFNum(v, "burst", c.Burst, 1, 100000, "")
//	        base.LTE(v, "burst", c.Burst, c.RateLimit*10, "rate_limit*10")
//	    })
//
//	    return v.Err()
//	}
//
// This pattern:
//
//   - Validates all required fields
//   - Validates optional fields only when enabled
//   - Validates field relationships
//   - Returns all errors together
//
// # Integration with Config System
//
// All config packages follow this pattern:
//
//	// constants.go: Define defaults and bounds
//	const (
//	    MIN_PORT base.Port = 1024
//	    MAX_PORT base.Port = 65535
//	    DEFAULT_PORT base.Port = 8080
//	)
//
//	// section.go: Define config struct
//	type Config struct {
//	    Port base.Port `mapstructure:"port"`
//	}
//
//	// section.go: Implement Validate using base helpers
//	func (c Config) Validate() error {
//	    v := base.NewValidator("HTTP_")
//	    base.RangeFNum(v, "port", c.Port, MIN_PORT, MAX_PORT, "")
//	    return v.Err()
//	}
//
// This ensures:
//
//   - Consistent validation across all packages
//   - Clear error messages with full context
//   - Type-safe configuration values
//   - Compile-time checking of constants
package base
