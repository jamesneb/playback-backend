// Package config provides a comprehensive, type-safe configuration management system
// for the Playback backend application.
//
// # Overview
//
// The config package implements a layered, hot-reloadable configuration system with:
//
//   - Type-safe configuration sections with validation
//   - Multiple provider support (environment, files, AWS Secrets Manager)
//   - Atomic snapshot-based reads for zero-lock performance
//   - Hot reload with subscriber notifications
//   - Cross-section validation (e.g., port uniqueness)
//
// # Architecture
//
// The system is organized into three main layers:
//
//  1. Providers - Load raw key-value pairs from various sources
//  2. Manager - Merges, decodes, validates, and manages configuration snapshots
//  3. Sections - Strongly-typed config structs for each application domain
//
// # Configuration Sections
//
// Each section represents a logical grouping of related settings:
//
//   - app - Application metadata (name, version, environment)
//   - grpc - gRPC server configuration
//   - http - HTTP/REST server configuration
//   - data - Data processing and batch settings
//   - clickhouse - ClickHouse database configuration
//   - redis - Redis cache configuration
//   - s3 - S3 storage configuration
//   - kinesis - Kinesis streaming configuration
//   - monitoring - Observability and health checks
//   - circuitbreaker - Resilience patterns
//   - dlq - Dead letter queue configuration
//   - testing - Test environment settings
//   - features - Optional feature flags
//
// # Usage
//
// Initialize a configuration manager with one or more providers:
//
//	ctx := context.Background()
//	envProvider := provider.NewEnvironment()
//	mgr, err := config.NewManager(ctx, envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Read configuration atomically:
//
//	snapshot := mgr.Snapshot()
//	httpConfig := snapshot.HTTP
//	log.Printf("Starting HTTP server on %s:%d", httpConfig.Host, httpConfig.Port)
//
// Subscribe to configuration changes:
//
//	mgr.Subscribe("http-server", func(old, new config.Snapshot) {
//	    if old.HTTP.Port != new.HTTP.Port {
//	        log.Printf("HTTP port changed: %d -> %d", old.HTTP.Port, new.HTTP.Port)
//	    }
//	})
//
// # Environment Variables
//
// Configuration is primarily driven by environment variables with a consistent naming scheme:
//
//	{SECTION}_{SETTING}
//
// Examples:
//
//	HTTP_PORT=8080
//	HTTP_HOST=0.0.0.0
//	GRPC_SERVER_PORT=4317
//	CLICKHOUSE_HOST=localhost
//	REDIS_POOL_SIZE=10
//
// # Type Safety
//
// The system uses Go's type system extensively:
//
//   - Custom types for ports, bytes, paths, hosts
//   - Enums with decode hooks (LogLevel, Environment, TLSVersion, etc.)
//   - Validated wrapper types (Percentage with 0-100 constraint)
//
// # Validation
//
// Each section implements a Validate() method with:
//
//   - Range checks (ports, timeouts, sizes)
//   - Conditional validation (TLS cert required when enabled)
//   - Cross-field validation (burst must relate to RPS)
//   - Type-specific validation (percentage 0-100)
//
// The Manager also performs cross-section validation:
//
//   - Port uniqueness across HTTP, gRPC, metrics
//   - HTTP path uniqueness
//
// # Hot Reload
//
// The Manager watches providers for changes and automatically reloads:
//
//  1. Provider signals change
//  2. Manager debounces (150ms) to batch rapid changes
//  3. Fetches all provider layers
//  4. Decodes and validates new configuration
//  5. Updates atomic pointer
//  6. Notifies all subscribers
//
// Subscribers are called asynchronously with both old and new snapshots.
//
// # Performance
//
// Configuration reads are extremely fast:
//
//   - Lock-free atomic pointer loads
//   - Immutable snapshots prevent concurrent modification
//   - No parsing or validation on read path
//
// # Provider Order
//
// When multiple providers are used, later providers override earlier ones:
//
//	mgr, err := config.NewManager(ctx,
//	    provider.NewFile("defaults.env"),    // Base defaults
//	    provider.NewEnvironment(),            // Override with env vars
//	    provider.NewSecretsManager(client),  // Override with secrets
//	)
//
// # Error Handling
//
// The system fails fast with detailed error messages:
//
//   - Decode errors include field name and reason
//   - Validation errors are aggregated (see all issues at once)
//   - Provider errors include provider name
//   - Cross-validation errors clearly indicate conflicting sections
//
// # Extension
//
// To add a new configuration section:
//
//  1. Create package under internal/config/{section}/
//  2. Define constants.go with PREFIX, defaults, ranges
//  3. Define section.go with Config struct and validation
//  4. Add decode hook in decodeutil/ if using custom types
//  5. Add section to Snapshot struct in manager.go
//  6. Add section decode call in Manager.decode()
//  7. Add validation call in validateAll()
//
// # Best Practices
//
//   - Use named constants for all defaults (no magic numbers)
//   - Document ranges and units in comments
//   - Use highest readable time unit (ONE_WEEK not 168 * time.Hour)
//   - Use base types (base.Port, base.Byte) for semantic clarity
//   - Validate relationships between fields with v.When()
//   - Keep decode hooks simple and focused
package config
