package app

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/jamesneb/playback-backend/internal/config/base"
	"github.com/jamesneb/playback-backend/internal/config/decodeutil"
	resolver "github.com/jamesneb/playback-backend/internal/config/propertyresolver"
)

// Config holds application-level metadata and global infrastructure settings.
//
// This configuration defines the application's identity, runtime environment, and logging behavior.
// All fields are loaded once at startup and remain immutable during the application lifecycle.
//
// Fields can be set via environment variables with the APP_ prefix, or loaded from
// a configuration provider via [FromResolver].
//
// Example configuration:
//
//	cfg := app.Config{
//	    Name:        "playback-backend",
//	    Version:     semver.MustParse("1.2.3"),
//	    Environment: base.PROD_ENV,
//	    LogLevel:    base.LOG_INFO,
//	    LogFormat:   base.LOG_JSON,
//	}
type Config struct {
	// Name is the application identifier used in logs, metrics, and distributed traces.
	//
	// This name appears in:
	//   - Structured log output as the "service" or "app" field
	//   - Distributed trace spans as the service name
	//   - Metrics labels for service identification
	//   - Health check endpoints
	//   - Service discovery registrations
	//
	// Best practices:
	//   - Use lowercase with hyphens (e.g., "playback-backend")
	//   - Keep it short and descriptive (2-3 words max)
	//   - Include component suffix if multiple services (e.g., "-api", "-worker")
	//   - Avoid special characters except hyphens
	//
	// Environment variable: APP_NAME
	// Default: "playback"
	Name string `mapstructure:"app_name"`

	// Version is the semantic version of the application following [semver.org](https://semver.org/).
	//
	// The version follows MAJOR.MINOR.PATCH format where:
	//   - MAJOR: Incompatible API changes
	//   - MINOR: Backwards-compatible new features
	//   - PATCH: Backwards-compatible bug fixes
	//
	// The version is used for:
	//   - Feature flags (e.g., if Version.Major() >= 2)
	//   - API compatibility checking
	//   - Deployment tracking and rollback
	//   - Client version negotiation
	//   - Metrics and trace annotations
	//
	// Pre-release and build metadata:
	//   - Pre-release: "1.0.0-alpha.1", "1.0.0-beta.2", "1.0.0-rc.1"
	//   - Build metadata: "1.0.0+20250103", "1.0.0+exp.sha.5114f85"
	//
	// Environment variable: APP_VERSION
	// Default: 0.0.0
	// Format: MAJOR.MINOR.PATCH[-prerelease][+buildmetadata]
	Version *semver.Version `mapstructure:"version"`

	// Environment specifies the deployment environment.
	//
	// Valid values (defined in [base.Environment]):
	//   - base.LOCAL_ENV: Local development on developer machine
	//   - base.DEV_ENV: Shared development environment
	//   - base.STAGE_ENV: Staging/pre-production environment
	//   - base.PROD_ENV: Production environment
	//   - base.TEST_ENV: Automated testing environment
	//
	// The environment affects:
	//   - Default log levels (dev=debug, prod=info)
	//   - Validation strictness (relaxed in dev, strict in prod)
	//   - Feature availability (debug endpoints only in dev/staging)
	//   - Security policies (CSRF, rate limiting stricter in prod)
	//   - Performance optimizations (enabled in prod)
	//   - Circuit breaker thresholds
	//
	// Environment variable: APP_ENVIRONMENT
	// Default: base.DEV_ENV
	// Valid values: "local", "dev", "staging", "prod", "test"
	Environment base.Environment `mapstructure:"environment"`

	// LogLevel controls the minimum log level to output.
	//
	// Valid values (defined in [base.LogLevel]):
	//   - base.LOG_DEBUG: Detailed diagnostic information (verbose)
	//   - base.LOG_INFO: General informational messages (default)
	//   - base.LOG_WARN: Warning messages for potential issues
	//   - base.LOG_ERR: Error messages for failures
	//   - base.LOG_FATAL: Fatal errors causing shutdown
	//
	// Log level hierarchy (each level includes all lower levels):
	//   FATAL > ERROR > WARN > INFO > DEBUG
	//
	// Performance impact:
	//   - DEBUG: 10-20% overhead (extensive string formatting)
	//   - INFO: 2-5% overhead (normal production use)
	//   - WARN/ERROR: <1% overhead (minimal in healthy systems)
	//
	// Recommendations:
	//   - Production: INFO (balance observability and performance)
	//   - Staging: INFO (production parity)
	//   - Development: DEBUG (maximum visibility)
	//   - Troubleshooting: DEBUG (temporarily for issue investigation)
	//
	// Environment variable: APP_LOG_LEVEL
	// Default: base.LOG_INFO
	// Valid values: "debug", "info", "warn", "error", "fatal"
	LogLevel base.LogLevel `mapstructure:"log_level"`

	// LogFormat specifies the log output format.
	//
	// Valid values (defined in [base.LogFormat]):
	//   - base.LOG_JSON: Structured JSON output (recommended for production)
	//   - base.LOG_CONSOLE: Human-readable text output (recommended for development)
	//
	// JSON format benefits:
	//   - Machine-readable for log aggregation (Elasticsearch, Splunk, Datadog)
	//   - Structured data for querying and filtering
	//   - Consistent format across services
	//   - Better for automated alerting
	//   - Required for most observability platforms
	//
	// Console/text format benefits:
	//   - Human-readable in terminal
	//   - Easier visual scanning during development
	//   - Better for interactive debugging
	//   - Supports color coding (if terminal supports)
	//   - Faster for ad-hoc inspection
	//
	// Example JSON output:
	//   {"time":"2025-10-03T10:15:30Z","level":"info","msg":"request completed","method":"GET","path":"/health","status":200}
	//
	// Example console output:
	//   2025-10-03T10:15:30Z INFO request completed method=GET path=/health status=200
	//
	// Recommendations:
	//   - Production: JSON (required for log aggregation)
	//   - Staging: JSON (test log pipeline)
	//   - Development: CONSOLE (easier to read)
	//
	// Environment variable: APP_LOG_FORMAT
	// Default: base.LOG_JSON
	// Valid values: "json", "console"
	LogFormat base.LogFormat `mapstructure:"log_format"`
}

// Defaults returns the default application configuration.
//
// Default values are suitable for development environments and should be
// overridden for production deployments via environment variables.
//
// Returns a [Config] with development-friendly defaults:
//   - Name: "playback" - Default service name
//   - Version: 0.0.0 - Placeholder for development
//   - Environment: dev - Development environment
//   - LogLevel: info - Balanced verbosity
//   - LogFormat: json - Structured logging
//
// Usage:
//
//	// Start with defaults, then customize
//	cfg := app.Defaults()
//	cfg.Environment = base.PROD_ENV
//	cfg.LogLevel = base.LOG_WARN
//
//	// Or use FromResolver to load from environment
//	cfg, err := app.FromResolver(envProvider)
func Defaults() Config {
	return Config{
		Name:        DEFAULT_APP_NAME,
		Version:     DEFAULT_APP_VERSION,
		Environment: DEFAULT_APP_ENVIRONMENT,
		LogLevel:    DEFAULT_APP_LOG_LEVEL,
		LogFormat:   DEFAULT_APP_LOG_FORMAT,
	}
}

// Validate checks the configuration for correctness.
//
// Currently performs minimal validation as all fields have valid defaults
// and type constraints are enforced by the [base] package enums (Environment,
// LogLevel, LogFormat). The semver.Version type also enforces valid versioning.
//
// Future validation may include:
//   - Name format validation (no special characters)
//   - Environment-specific constraints
//   - Cross-field validation rules
//
// Returns nil if validation succeeds, or an error describing validation failures.
// Multiple validation errors are combined using errors.Join.
//
// Example:
//
//	cfg := app.Config{Name: "", Version: semver.MustParse("1.0.0")}
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal("invalid config:", err)
//	}
func (c Config) Validate() error {
	v := base.NewValidator("APP")
	// Future: add validation rules here
	// base.NotEmpty(v, "name", c.Name)
	return v.Err()
}

// FromResolver loads application configuration from a property resolver.
//
// This function implements the standard config loading pattern:
//  1. Starts with default values from [Defaults]
//  2. Overlays values from the resolver using APP_ prefix
//  3. Validates the resulting configuration with [Config.Validate]
//
// The resolver typically reads from environment variables, but can also
// load from files, remote config stores, or any source implementing
// [propertyresolver.PropertyResolver].
//
// Environment variable mapping:
//   - APP_NAME → Name
//   - APP_VERSION → Version (parsed as semantic version)
//   - APP_ENVIRONMENT → Environment (parsed as enum: dev, staging, prod)
//   - APP_LOG_LEVEL → LogLevel (parsed as enum: debug, info, warn, error)
//   - APP_LOG_FORMAT → LogFormat (parsed as enum: json, console)
//
// Returns the loaded [Config] or an error if decoding or validation fails.
//
// Example with environment variables:
//
//	// Set environment variables
//	os.Setenv("APP_NAME", "playback-backend")
//	os.Setenv("APP_VERSION", "1.2.3")
//	os.Setenv("APP_ENVIRONMENT", "prod")
//	os.Setenv("APP_LOG_LEVEL", "info")
//	os.Setenv("APP_LOG_FORMAT", "json")
//
//	// Load configuration
//	envProvider := provider.NewEnvVarProvider()
//	cfg, err := app.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal("failed to load config:", err)
//	}
//
// Example error handling:
//
//	cfg, err := app.FromResolver(resolver)
//	if err != nil {
//	    if strings.Contains(err.Error(), "decode") {
//	        log.Fatal("invalid environment variable format:", err)
//	    } else {
//	        log.Fatal("configuration validation failed:", err)
//	    }
//	}
func FromResolver(r resolver.PropertyResolver) (Config, error) {
	cfg := Defaults()
	if err := decodeutil.DecodePrefixInto(r, APP_PREFIX, &cfg); err != nil {
		return Config{}, fmt.Errorf("app decode: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
