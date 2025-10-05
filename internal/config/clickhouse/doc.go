// Package clickhouse defines configuration for ClickHouse database connections.
//
// ClickHouse is a high-performance columnar OLAP database optimized for analytical queries
// and real-time data ingestion. This package provides configuration for both native TCP
// protocol connections (port 9000) and HTTP interface connections (port 8123).
//
// Official documentation: https://clickhouse.com/docs
//
// # Architecture
//
// The ClickHouse configuration follows the standard config pattern:
//   - [Defaults] provides production-ready baseline values
//   - [FromResolver] overlays environment/provider values onto defaults
//   - [Config.Validate] ensures configuration correctness before use
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
//   - Binary protocol for maximum performance
//   - Used for bulk inserts and high-throughput queries
//   - Supports all ClickHouse features
//   - Better compression and lower latency
//   - Configured via Host field (e.g., "localhost:9000")
//   - Recommended for: Data ingestion, bulk operations, production workloads
//
// HTTP Protocol (port 8123):
//   - RESTful interface for easier debugging
//   - Used for ad-hoc queries and administration
//   - Simpler protocol, easier to troubleshoot
//   - Good for interactive queries and monitoring
//   - Configured via HTTPHost field (e.g., "localhost:8123")
//   - Recommended for: Admin queries, debugging, monitoring tools
//
// Both protocols can be used simultaneously depending on application needs.
//
// # MergeTree Engine
//
// ClickHouse uses the MergeTree table engine family for most use cases:
//
//	CREATE TABLE events (
//	    date Date,
//	    user_id UInt64,
//	    event_type String,
//	    timestamp DateTime
//	) ENGINE = MergeTree()
//	PARTITION BY toYYYYMM(date)
//	ORDER BY (user_id, timestamp);
//
// MergeTree features:
//   - Automatic data sorting by primary key
//   - Data partitioning for efficient querying
//   - Background merge operations for optimization
//   - TTL support for automatic data expiration
//   - Replication support (ReplicatedMergeTree)
//   - Data compression (LZ4, ZSTD)
//
// Partitioning strategies:
//   - By date: PARTITION BY toYYYYMM(date) - monthly partitions
//   - By date: PARTITION BY toMonday(date) - weekly partitions
//   - By category: PARTITION BY event_type - categorical partitions
//   - By range: PARTITION BY intDiv(user_id, 1000000) - range partitions
//
// Primary key selection:
//   - Choose columns used in WHERE clauses frequently
//   - Order from low to high cardinality
//   - Balance between query performance and storage
//   - Example: (tenant_id, date, event_type) for multi-tenant systems
//
// For more on MergeTree: https://clickhouse.com/docs/en/engines/table-engines/mergetree-family/mergetree
//
// # Connection Pooling
//
// The package defines connection pool limits to prevent resource exhaustion:
//
//   - MaxConnections: Maximum concurrent connections (default: 10, range: 1-1000)
//   - MaxIdleConnections: Idle connections kept alive (default: 5, range: 1-MaxConnections)
//   - ConnectionTimeout: Maximum time to establish connection (default: 30s, range: 1s-5m)
//   - ConnectionMaxLifetime: Maximum lifetime of a connection (default: 30m, range: 1m-24h)
//
// Pool sizing guidelines:
//   - Low traffic (<100 req/s): 5-10 connections
//   - Medium traffic (100-1000 req/s): 10-50 connections
//   - High traffic (>1000 req/s): 50-200 connections
//   - Adjust based on query complexity and duration
//
// Too few connections:
//   - High contention and wait times
//   - Reduced throughput
//   - Increased latency
//
// Too many connections:
//   - ClickHouse server resource exhaustion
//   - Memory overhead
//   - Context switching overhead
//
// Best practices:
//   - Start with defaults (10 max, 5 idle)
//   - Monitor connection pool metrics
//   - Increase gradually under load
//   - Keep MaxIdleConnections = MaxConnections/2
//
// # Compression
//
// ClickHouse supports transparent network compression:
//
//   - LZ4: Fast compression, lower CPU overhead (default)
//   - ZSTD: Better compression ratio, higher CPU overhead
//   - None: No compression, minimal CPU but high bandwidth
//
// Compression reduces network bandwidth at the cost of CPU cycles.
// Generally recommended for production as network is often the bottleneck.
//
// When to enable:
//   - Large result sets (>1MB per query)
//   - Remote ClickHouse servers (>1ms network latency)
//   - Limited network bandwidth
//   - Bulk data ingestion
//
// When to disable:
//   - Local development (same machine)
//   - Low-latency requirements (<1ms)
//   - CPU-bound workloads
//   - Already compressed data
//
// Default: Enabled (EnableCompression=true)
//
// # Configuration Reference
//
// All configuration values can be overridden via environment variables with the
// CLICKHOUSE_ prefix:
//
//	# Connection settings
//	CLICKHOUSE_HOST=prod-clickhouse:9000              # Native TCP endpoint
//	CLICKHOUSE_HTTP_HOST=prod-clickhouse:8123         # HTTP endpoint
//	CLICKHOUSE_DATABASE=telemetry_prod                # Database name
//	CLICKHOUSE_USERNAME=app_user                      # Authentication username
//	CLICKHOUSE_PASSWORD=secret_password               # Authentication password
//
//	# Connection pool settings
//	CLICKHOUSE_MAX_CONNECTIONS=50                     # Maximum concurrent connections
//	CLICKHOUSE_MAX_IDLE_CONNECTIONS=25                # Idle connections to keep alive
//	CLICKHOUSE_CONNECTION_TIMEOUT=30s                 # Connection establishment timeout
//	CLICKHOUSE_CONNECTION_MAX_LIFETIME=1h             # Maximum connection lifetime
//
//	# Performance settings
//	CLICKHOUSE_ENABLE_COMPRESSION=true                # Enable network compression
//	CLICKHOUSE_ENABLE_CONNECTION_POOLING=true         # Enable connection pooling
//	CLICKHOUSE_ENABLE_QUERY_LOGGING=false             # Enable query logging (debug)
//
// # Security Considerations
//
// Authentication:
//   - Never hardcode passwords in source code
//   - Use environment variables for credentials
//   - Consider using IAM authentication (AWS, GCP)
//   - Restrict user permissions to required operations
//
// Network security:
//   - Use TLS for production connections
//   - Restrict ClickHouse ports with firewalls
//   - Use VPC/private networks when possible
//   - Enable audit logging on ClickHouse server
//
// Password management:
//   - Store passwords in secret management systems (Vault, AWS Secrets Manager)
//   - Rotate passwords regularly
//   - Use read-only users for query workloads
//   - Use separate users per application
//
// # Performance Tuning
//
// For high-throughput ingestion:
//
//	CLICKHOUSE_MAX_CONNECTIONS=50                 # Increase for concurrency
//	CLICKHOUSE_MAX_IDLE_CONNECTIONS=25            # Keep connections warm
//	CLICKHOUSE_ENABLE_COMPRESSION=true            # Reduce network overhead
//	CLICKHOUSE_CONNECTION_TIMEOUT=60s             # Allow longer setup time
//	CLICKHOUSE_CONNECTION_MAX_LIFETIME=2h         # Longer-lived connections
//
// Use native protocol (Host) for:
//   - Bulk inserts (>1000 rows)
//   - High-frequency writes (>100/s)
//   - Production data pipelines
//   - Real-time analytics
//
// For read-heavy analytical workloads:
//
//	CLICKHOUSE_MAX_CONNECTIONS=20                 # Moderate pool size
//	CLICKHOUSE_ENABLE_COMPRESSION=true            # Compress large result sets
//	CLICKHOUSE_CONNECTION_TIMEOUT=30s             # Standard timeout
//
// Use HTTP protocol (HTTPHost) for:
//   - Interactive queries
//   - Admin operations
//   - Monitoring and debugging
//   - Low-frequency operations
//
// Query optimization:
//   - Use appropriate partitioning keys
//   - Optimize primary key selection
//   - Enable data compression in tables
//   - Use materialized views for aggregations
//   - Implement TTL for data lifecycle management
//
// # Data Lifecycle Management
//
// ClickHouse supports automatic data expiration via TTL:
//
//	ALTER TABLE events MODIFY TTL date + INTERVAL 30 DAY;
//
// TTL strategies:
//   - Hot data: 7-30 days (keep in memory-optimized tier)
//   - Warm data: 30-90 days (standard storage)
//   - Cold data: 90+ days (archive or delete)
//
// Partition management:
//   - Drop old partitions: ALTER TABLE events DROP PARTITION '202401'
//   - Detach partitions: ALTER TABLE events DETACH PARTITION '202401'
//   - Move to cold storage: ALTER TABLE events MOVE PARTITION '202401' TO DISK 'cold'
//
// # Example Usage
//
//	// Load configuration
//	ctx := context.Background()
//	envProvider := provider.NewEnvVarProvider()
//	cfg, err := clickhouse.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal("failed to load config:", err)
//	}
//
//	// Connect to ClickHouse using native protocol
//	conn := clickhouse.Open(&clickhouse.Options{
//	    Addr: []string{cfg.Host},
//	    Auth: clickhouse.Auth{
//	        Database: cfg.Database,
//	        Username: cfg.Username,
//	        Password: cfg.Password,
//	    },
//	    MaxOpenConns:    cfg.MaxConnections,
//	    MaxIdleConns:    cfg.MaxIdleConnections,
//	    ConnMaxLifetime: cfg.ConnectionMaxLifetime,
//	    Compression: &clickhouse.Compression{
//	        Method: clickhouse.CompressionLZ4,
//	    },
//	})
//
//	// Execute query
//	rows, err := conn.Query(ctx, "SELECT count() FROM events WHERE date >= today()")
//
// # Best Practices
//
// Schema design:
//   - Use appropriate data types (UInt32 vs UInt64)
//   - Choose optimal primary key (low to high cardinality)
//   - Partition by time for time-series data
//   - Use compression codecs for large columns (LZ4, ZSTD)
//   - Implement data sampling for large datasets
//
// Query patterns:
//   - Avoid SELECT * (specify columns)
//   - Use PREWHERE for filtering before reading columns
//   - Leverage materialized views for aggregations
//   - Use appropriate JOIN types (INNER vs LEFT)
//   - Batch inserts (1000+ rows per insert)
//
// Monitoring:
//   - Track query execution time (system.query_log)
//   - Monitor connection pool utilization
//   - Watch for slow queries (>1s)
//   - Alert on high memory usage
//   - Track merge operation backlog
//
// # Troubleshooting
//
// Connection issues:
//
//	Error: "connection timeout"
//	Fix: Increase CLICKHOUSE_CONNECTION_TIMEOUT or check network connectivity
//
//	Error: "too many connections"
//	Fix: Increase CLICKHOUSE_MAX_CONNECTIONS or check for connection leaks
//
//	Error: "authentication failed"
//	Fix: Verify CLICKHOUSE_USERNAME and CLICKHOUSE_PASSWORD
//
// Performance issues:
//
//	Problem: Slow queries
//	Fix: Check query execution plan with EXPLAIN, optimize primary key
//
//	Problem: High memory usage
//	Fix: Reduce CLICKHOUSE_MAX_CONNECTIONS, optimize GROUP BY queries
//
//	Problem: Insert latency
//	Fix: Batch inserts, use async inserts, enable compression
//
// Data issues:
//
//	Problem: Missing data
//	Fix: Check partitions (SHOW PARTITIONS), verify TTL settings
//
//	Problem: Duplicate data
//	Fix: Use ReplacingMergeTree or AggregatingMergeTree engines
//
// # Cross-References
//
// Related packages:
//   - [base.Validator] - Validation framework used by Config.Validate()
//   - [decodeutil] - Configuration decoding utilities
//   - [propertyresolver] - Property resolution from various sources
//
// Related documentation:
//   - ClickHouse Official Docs: https://clickhouse.com/docs
//   - MergeTree Engine: https://clickhouse.com/docs/en/engines/table-engines/mergetree-family/mergetree
//   - Data Types: https://clickhouse.com/docs/en/sql-reference/data-types
//   - Query Performance: https://clickhouse.com/docs/en/guides/improving-query-performance
//   - Go Client Library: https://github.com/ClickHouse/clickhouse-go
//
// # Files in This Package
//
// constants.go:
//   - CLICKHOUSE_PREFIX for environment variable namespacing
//   - Default values (host, ports, connection limits, timeouts)
//   - Min/max bounds for validation
//   - Time constants and connection parameters
//
// section.go:
//   - [Config] struct with connection parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package clickhouse
