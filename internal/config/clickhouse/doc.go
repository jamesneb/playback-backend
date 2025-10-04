// Package clickhouse defines configuration for ClickHouse database connections.
//
// ClickHouse is a high-performance columnar OLAP database optimized for analytical queries
// and real-time data ingestion. This package provides configuration for both native TCP
// protocol connections (port 9000) and HTTP interface connections (port 8123).
//
// # Architecture
//
// The ClickHouse configuration follows the standard config pattern:
//   - Defaults() provides production-ready baseline values
//   - FromResolver() overlays environment/provider values onto defaults
//   - Validate() ensures configuration correctness before use
//
// Configuration is loaded via the config.Manager and accessed through Snapshot:
//
//	snapshot := manager.Snapshot()
//	db := clickhouse.Connect(snapshot.ClickHouse)
//
// # Connection Modes
//
// ClickHouse supports two connection protocols:
//
// Native Protocol (TCP, port 9000):
//   - Binary protocol, higher performance
//   - Used for bulk inserts and high-throughput queries
//   - Configured via Host field (e.g., "localhost:9000")
//
// HTTP Protocol (port 8123):
//   - RESTful interface, easier debugging
//   - Used for ad-hoc queries and administration
//   - Configured via HTTPHost field (e.g., "localhost:8123")
//
// Both protocols can be used simultaneously depending on application needs.
//
// # Connection Pooling
//
// The package defines connection pool limits to prevent resource exhaustion:
//
//   - MaxConnections: Maximum concurrent connections to ClickHouse
//   - MaxIdleConnections: Idle connections kept alive for reuse
//   - ConnectionTimeout: Maximum time to establish a new connection
//
// Proper pool sizing is critical for performance. Too few connections create
// contention; too many exhaust server resources. Default values (10 max, 5 idle)
// are suitable for moderate workloads.
//
// # Compression
//
// ClickHouse supports transparent compression for network traffic. When enabled,
// all data transferred between client and server is compressed using LZ4 or ZSTD.
// This reduces bandwidth usage at the cost of CPU overhead.
//
// Default: Enabled (recommended for production)
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// CLICKHOUSE_ prefix:
//
//	CLICKHOUSE_HOST=prod-db:9000
//	CLICKHOUSE_HTTP_HOST=prod-db:8123
//	CLICKHOUSE_DATABASE=telemetry_prod
//	CLICKHOUSE_USERNAME=app_user
//	CLICKHOUSE_PASSWORD=secret
//	CLICKHOUSE_MAX_CONNECTIONS=20
//	CLICKHOUSE_MAX_IDLE_CONNECTIONS=10
//	CLICKHOUSE_CONNECTION_TIMEOUT=30s
//	CLICKHOUSE_ENABLE_COMPRESSION=true
//
// # Security Considerations
//
// Passwords are stored in plain text in configuration. In production:
//   - Use environment variables or secret management systems
//   - Restrict file permissions on config files
//   - Never commit passwords to version control
//   - Consider using IAM authentication where available
//
// # Performance Tuning
//
// For high-throughput ingestion workloads:
//   - Increase MaxConnections (20-50 for busy systems)
//   - Enable compression to reduce network bottlenecks
//   - Use native protocol (Host) instead of HTTP for bulk operations
//   - Increase ConnectionTimeout if connecting to remote databases
//
// For read-heavy analytical workloads:
//   - Moderate MaxConnections (10-20 sufficient)
//   - Enable compression for large result sets
//   - HTTP protocol acceptable for interactive queries
//
// # Validation Rules
//
// The Validate() method enforces:
//   - Host and HTTPHost are non-empty
//   - Database name is non-empty
//   - Username is non-empty
//   - MaxConnections > 0 and <= 1000
//   - MaxIdleConnections > 0 and <= MaxConnections
//   - ConnectionTimeout between 1s and 5 minutes
//
// # Example Usage
//
//	// Load configuration
//	ctx := context.Background()
//	envProvider := &provider.EnvVarProvider{Prefix: "CLICKHOUSE_"}
//	manager, err := config.NewManager(ctx, envProvider)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Access ClickHouse config
//	cfg := manager.Snapshot().ClickHouse
//	fmt.Printf("Connecting to: %s/%s\n", cfg.Host, cfg.Database)
//
//	// Subscribe to config changes (hot-reload)
//	manager.Subscribe("clickhouse-client", func(old, new config.Snapshot) {
//		if old.ClickHouse.Host != new.ClickHouse.Host {
//			log.Printf("ClickHouse host changed: %s -> %s",
//				old.ClickHouse.Host, new.ClickHouse.Host)
//			// Reconnect with new configuration
//		}
//	})
//
// # Files in This Package
//
// constants.go:
//   - CLICKHOUSE_PREFIX for environment variable namespacing
//   - Default values (host, ports, connection limits, timeouts)
//   - Min/max bounds for validation
//
// section.go:
//   - Config struct with connection parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package clickhouse
