package app

import (
	"github.com/Masterminds/semver/v3"
	"github.com/jamesneb/playback-backend/internal/config/base"
)

// Configuration prefix constants
const (
	// APP_PREFIX is the environment variable prefix for all app configuration keys.
	//
	// All environment variables in this package use this prefix for namespacing,
	// preventing collisions with other configuration sections and system variables.
	//
	// Example environment variables:
	//   - APP_NAME
	//   - APP_VERSION
	//   - APP_ENVIRONMENT
	//   - APP_LOG_LEVEL
	//   - APP_LOG_FORMAT
	//
	// This constant is used by [FromResolver] to filter and extract app-specific
	// configuration from a property resolver.
	APP_PREFIX = "APP_"
)

// Application identity defaults
const (
	// DEFAULT_APP_NAME is the default application identifier.
	//
	// This name is used when APP_NAME environment variable is not set.
	// It appears in logs, metrics, distributed traces, and service discovery.
	//
	// The default "playback" is suitable for:
	//   - Local development
	//   - Single-service deployments
	//   - Quick prototyping
	//
	// For production deployments with multiple services, set APP_NAME to
	// a more specific identifier (e.g., "playback-backend", "playback-api").
	//
	// Best practices:
	//   - Use lowercase with hyphens
	//   - Keep it short and descriptive
	//   - Include service component if needed
	//
	// Example: "playback-backend", "playback-worker", "playback-api"
	DEFAULT_APP_NAME = "playback"
)

// Environment and logging defaults
//
// These defaults are chosen to provide a good balance for development
// environments while being production-safe:
//
//   - DEV_ENV: Permits relaxed validation and debug features
//   - LOG_INFO: Balanced verbosity (not too noisy, not too silent)
//   - LOG_JSON: Structured output compatible with aggregation tools
const (
	// DEFAULT_APP_ENVIRONMENT is the default deployment environment (development).
	//
	// The development environment (base.DEV_ENV) enables:
	//   - More verbose default logging
	//   - Relaxed validation for faster iteration
	//   - Debug features and endpoints
	//   - Permissive CORS policies
	//   - Hot-reload capabilities
	//
	// Always override this for staging and production deployments:
	//   - Staging: APP_ENVIRONMENT=staging
	//   - Production: APP_ENVIRONMENT=prod
	//
	// Using DEV_ENV in production is a security risk and performance concern.
	DEFAULT_APP_ENVIRONMENT base.Environment = base.DEV_ENV

	// DEFAULT_APP_LOG_LEVEL is the default minimum log level (info).
	//
	// The INFO level (base.LOG_INFO) provides:
	//   - Balanced verbosity for development and production
	//   - Sufficient observability without overwhelming logs
	//   - Reasonable performance impact (2-5% overhead)
	//
	// Log level recommendations by environment:
	//   - Development: DEBUG (maximum visibility)
	//   - Staging: INFO (production parity)
	//   - Production: INFO or WARN (balance observability and performance)
	//   - Troubleshooting: DEBUG (temporarily for investigation)
	//
	// INFO level includes:
	//   - INFO: General informational messages
	//   - WARN: Warnings for potential issues
	//   - ERROR: Error messages for failures
	//   - FATAL: Fatal errors causing shutdown
	DEFAULT_APP_LOG_LEVEL base.LogLevel = base.LOG_INFO

	// DEFAULT_APP_LOG_FORMAT is the default log output format (JSON).
	//
	// JSON format (base.LOG_JSON) is recommended because:
	//   - Machine-readable for log aggregation tools
	//   - Structured data enables powerful querying
	//   - Standard format across microservices
	//   - Required by most observability platforms
	//   - Consistent parsing across different log sources
	//
	// JSON format is ideal for:
	//   - Production deployments
	//   - Staging environments
	//   - CI/CD pipelines
	//   - Container orchestration (Kubernetes, Docker Swarm)
	//   - Log aggregation (Elasticsearch, Splunk, Datadog)
	//
	// For local development, consider using CONSOLE format for better
	// readability, but JSON remains the default for consistency and
	// to catch formatting issues early.
	//
	// Override with APP_LOG_FORMAT=console for human-readable output.
	DEFAULT_APP_LOG_FORMAT base.LogFormat = base.LOG_JSON
)

// DEFAULT_APP_VERSION is the default semantic version (0.0.0).
//
// This placeholder version follows [Semantic Versioning 2.0.0](https://semver.org/)
// and indicates an unversioned or development build.
//
// Version 0.0.0 semantics:
//   - MAJOR 0: Initial development, API not stable
//   - MINOR 0: No features released yet
//   - PATCH 0: Development/placeholder version
//
// In production, always set APP_VERSION to a meaningful version:
//   - APP_VERSION=1.0.0 for first stable release
//   - APP_VERSION=1.2.3 for versioned releases
//   - APP_VERSION=1.0.0-rc.1 for release candidates
//
// Best practices for version management:
//   - Set via CI/CD pipelines using git tags
//   - Use `git describe --tags` for automatic versioning
//   - Include build metadata for traceability (1.0.0+sha.abc123)
//   - Never use 0.0.0 in production deployments
//
// The version is parsed at package initialization using semver.MustParse,
// which panics if the version string is invalid. This ensures the default
// version is always valid and catches errors at compile/init time.
//
// CI/CD example:
//
//	# GitHub Actions
//	- name: Set version
//	  run: echo "APP_VERSION=$(git describe --tags --always)" >> $GITHUB_ENV
//
//	# GitLab CI
//	variables:
//	  APP_VERSION: ${CI_COMMIT_TAG:-0.0.0-dev}
var DEFAULT_APP_VERSION = semver.MustParse("0.0.0")
