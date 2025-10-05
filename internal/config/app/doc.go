// Package app defines the configuration for application-level metadata and global infrastructure settings.
//
// # Overview
//
// This package provides core application configuration including:
//
//   - Application identity (name, version)
//   - Environment designation (development, staging, production)
//   - Logging configuration (level, format)
//
// These settings define the application's runtime behavior and operational characteristics.
// The configuration is loaded once at startup and remains static throughout the application lifecycle.
//
// # Configuration Keys
//
// All settings use the APP_ prefix:
//
//	APP_NAME        - Application name (default: playback)
//	APP_VERSION     - Semantic version (default: 0.0.0)
//	APP_ENVIRONMENT - Environment: dev|staging|prod (default: dev)
//	APP_LOG_LEVEL   - Log level: debug|info|warn|error (default: info)
//	APP_LOG_FORMAT  - Log format: json|text (default: json)
//
// # Example Usage
//
//	// Get app config from manager
//	snapshot := mgr.Snapshot()
//	appCfg := snapshot.App
//
//	// Use configuration
//	logger := NewLogger(appCfg.LogLevel, appCfg.LogFormat)
//	logger.Info("starting application",
//	    "name", appCfg.Name,
//	    "version", appCfg.Version.String(),
//	    "environment", appCfg.Environment.String(),
//	)
//
//	// Version-based feature flags
//	if appCfg.Version.Major() >= 2 {
//	    // Enable new features
//	}
//
//	// Environment-aware behavior
//	if appCfg.Environment == base.PROD_ENV {
//	    // Enable production-specific security measures
//	    server.EnableStrictCSRF()
//	    server.EnableRateLimiting()
//	}
//
// # Validation
//
// The configuration is validated on load with:
//
//   - Version must be valid semantic version format (e.g., 1.2.3)
//   - Environment must be one of: dev, staging, prod
//   - LogLevel must be one of: debug, info, warn, error
//   - LogFormat must be one of: json, text
//
// # Semantic Versioning
//
// The version field follows [Semantic Versioning 2.0.0](https://semver.org/) with format MAJOR.MINOR.PATCH:
//
//   - MAJOR: Incompatible API changes that require client updates
//   - MINOR: Backwards-compatible functionality additions
//   - PATCH: Backwards-compatible bug fixes
//
// Example versions:
//
//	APP_VERSION=1.0.0   # Initial stable release
//	APP_VERSION=1.1.0   # New features added (backwards compatible)
//	APP_VERSION=1.1.1   # Bug fixes (backwards compatible)
//	APP_VERSION=2.0.0   # Breaking changes (API compatibility broken)
//
// Pre-release and build metadata are also supported:
//
//	APP_VERSION=1.0.0-alpha.1      # Alpha pre-release
//	APP_VERSION=1.0.0-beta.2       # Beta pre-release
//	APP_VERSION=1.0.0-rc.1         # Release candidate
//	APP_VERSION=1.0.0+20250103     # Build metadata
//	APP_VERSION=1.0.0-beta.1+exp.sha.5114f85  # Combined
//
// Version comparison examples:
//
//	// Check major version for breaking changes
//	if appCfg.Version.Major() >= 2 {
//	    useNewAPIFormat()
//	}
//
//	// Check minimum version for features
//	minVersion := semver.MustParse("1.5.0")
//	if appCfg.Version.GreaterThan(minVersion) {
//	    enableAdvancedFeatures()
//	}
//
//	// Check pre-release status
//	if appCfg.Version.Prerelease() != "" {
//	    logger.Warn("running pre-release version", "version", appCfg.Version)
//	}
//
// For complete semver specification, see https://semver.org/
//
// # Environment Settings
//
// The environment setting affects application behavior:
//
// Development (dev):
//   - More verbose logging (default: debug level)
//   - Relaxed validation for faster iteration
//   - Local development optimizations
//   - Debug features enabled (profiling, detailed errors)
//   - Hot-reload support where applicable
//   - Permissive CORS policies
//
// Staging (staging):
//   - Production-like behavior for testing
//   - Full validation enabled
//   - Performance monitoring and profiling
//   - Integration testing against production-like services
//   - Realistic traffic patterns for load testing
//   - Separate data isolation from production
//
// Production (prod):
//   - Optimized performance (compiled optimizations)
//   - Strict validation and error handling
//   - Minimal logging (info level default)
//   - Security hardening (CSRF, rate limiting)
//   - Request tracing and monitoring
//   - Graceful degradation and circuit breakers
//
// # Log Level Settings
//
// Log levels in order of verbosity (each level includes all lower levels):
//
//	debug - Detailed diagnostic information (function calls, variable states)
//	        Use for: Development debugging, troubleshooting issues
//	        Warning: Very verbose, may impact performance
//
//	info  - General informational messages (requests, operations) [DEFAULT]
//	        Use for: Production monitoring, audit trails
//	        Recommended: Production default
//
//	warn  - Warning messages for potential issues (deprecated features, non-critical errors)
//	        Use for: Alerting on degraded performance
//	        Example: "connection pool 90% full"
//
//	error - Error messages for failures (failed requests, exceptions)
//	        Use for: Critical issue alerting
//	        Example: "database connection failed"
//
// Log level impact on performance:
//
//   - debug: 10-20% overhead (string formatting, allocations)
//   - info: 2-5% overhead (typical production use)
//   - warn/error: <1% overhead (minimal in healthy system)
//
// # Log Format Settings
//
// JSON format (recommended for production):
//   - Machine-readable for log aggregation (Elasticsearch, Splunk, Datadog)
//   - Easy to parse and query with structured data
//   - Structured key-value pairs
//   - Compatible with log aggregation tools
//   - Better for automated alerting and dashboards
//
// Text format (recommended for development):
//   - Human-readable output
//   - Easier to read in terminal during development
//   - Better for interactive debugging
//   - Color-coded output support (if terminal supports it)
//   - Faster visual scanning
//
// Example JSON log:
//
//	{"level":"info","msg":"request completed","method":"GET","path":"/api/v1/health","status":200,"duration_ms":23}
//
// Example text log:
//
//	INFO request completed method=GET path=/api/v1/health status=200 duration_ms=23
//
// # Best Practices
//
// Production deployments:
//
//	APP_ENVIRONMENT=prod
//	APP_LOG_LEVEL=info          # Balance observability and performance
//	APP_LOG_FORMAT=json         # Machine-readable for aggregation
//	APP_VERSION=1.2.3           # Set via CI/CD, track in releases
//	APP_NAME=playback-backend   # Consistent naming for service mesh
//
// Development deployments:
//
//	APP_ENVIRONMENT=dev
//	APP_LOG_LEVEL=debug         # Maximum visibility for debugging
//	APP_LOG_FORMAT=text         # Human-readable console output
//	APP_VERSION=0.0.0           # Can use default for local dev
//	APP_NAME=playback           # Default is sufficient
//
// Staging deployments:
//
//	APP_ENVIRONMENT=staging
//	APP_LOG_LEVEL=info          # Production parity for testing
//	APP_LOG_FORMAT=json         # Test log aggregation pipeline
//	APP_VERSION=1.2.3-rc.1      # Release candidate version
//	APP_NAME=playback-staging   # Distinguish from production
//
// CI/CD integration example:
//
//	# In CI/CD pipeline (GitHub Actions, GitLab CI, etc.)
//	export APP_VERSION=$(git describe --tags --always)
//	export APP_ENVIRONMENT=prod
//	export APP_LOG_LEVEL=info
//	export APP_LOG_FORMAT=json
//
// Docker deployment example:
//
//	# Dockerfile
//	ENV APP_VERSION=${VERSION}
//	ENV APP_ENVIRONMENT=prod
//	ENV APP_LOG_FORMAT=json
//
//	# docker-compose.yml
//	environment:
//	  - APP_VERSION=${GIT_TAG:-dev}
//	  - APP_ENVIRONMENT=${ENV:-dev}
//	  - APP_LOG_LEVEL=${LOG_LEVEL:-info}
//
// Kubernetes deployment example:
//
//	# ConfigMap or Deployment spec
//	env:
//	  - name: APP_VERSION
//	    value: "1.2.3"
//	  - name: APP_ENVIRONMENT
//	    value: "prod"
//	  - name: APP_LOG_FORMAT
//	    value: "json"
//	  - name: APP_LOG_LEVEL
//	    value: "info"
//
// # Cross-References
//
// Related packages:
//   - [base.Environment] - Environment type definitions (DEV_ENV, STAGE_ENV, PROD_ENV)
//   - [base.LogLevel] - Log level type definitions (LOG_DEBUG, LOG_INFO, etc.)
//   - [base.LogFormat] - Log format type definitions (LOG_JSON, LOG_CONSOLE)
//   - [base.Validator] - Validation framework used by Config.Validate()
//
// Related standards:
//   - Semantic Versioning: https://semver.org/
//   - 12-Factor App Config: https://12factor.net/config
//   - JSON Lines logging: https://jsonlines.org/
//
// # Troubleshooting
//
// Version parsing errors:
//
//	Error: "invalid semantic version: 1.2"
//	Fix: Use complete MAJOR.MINOR.PATCH format (1.2.0)
//
//	Error: "invalid semantic version: v1.2.3"
//	Fix: Remove 'v' prefix (1.2.3, not v1.2.3)
//
// Environment validation errors:
//
//	Error: "unknown environment: development"
//	Fix: Use short form (dev, not development)
//
// Log level issues:
//
//	Problem: Too many logs in production
//	Fix: Set APP_LOG_LEVEL=warn or APP_LOG_LEVEL=error
//
//	Problem: Missing debug information
//	Fix: Temporarily set APP_LOG_LEVEL=debug (remember to revert)
//
// Log format issues:
//
//	Problem: Logs not parsing in aggregation tool
//	Fix: Ensure APP_LOG_FORMAT=json (not text)
//
//	Problem: Unreadable logs in console
//	Fix: Use APP_LOG_FORMAT=text for local development
package app
