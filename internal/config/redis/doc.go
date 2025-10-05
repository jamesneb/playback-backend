// Package redis defines configuration for Redis cache connections.
//
// Redis is an in-memory data structure store used as a cache, message broker, session store,
// and real-time analytics engine. This package provides configuration for Redis client connections
// with connection pooling, timeouts, and TTL management.
//
// Official documentation: https://redis.io/docs
//
// # Overview
//
// Redis features:
//   - In-memory storage with optional persistence
//   - Sub-millisecond latency for most operations
//   - Rich data structures (strings, hashes, lists, sets, sorted sets, streams)
//   - Built-in replication and high availability
//   - Pub/Sub messaging
//   - Lua scripting support
//   - Transactions and atomic operations
//
// Common use cases:
//   - Application caching (session data, query results)
//   - Rate limiting and throttling
//   - Real-time analytics and leaderboards
//   - Message queues and pub/sub
//   - Distributed locks
//   - Session storage
//
// # Connection Management
//
// Redis connections are pooled to avoid connection overhead:
//
//   - MaxConnections: Maximum concurrent connections (default: 10, range: 1-1000)
//   - MaxIdleConnections: Idle connections kept alive (default: 5, range: 1-MaxConnections)
//   - ConnectionTimeout: Maximum time to establish connection (default: 5s, range: 1s-1m)
//   - ConnectionMaxLifetime: Maximum connection lifetime (default: 30m, range: 1m-24h)
//   - DefaultTTL: Default expiration time for cached values (default: 5m, range: 1s-24h)
//
// Connection pool sizing:
//   - Low traffic: 5-10 connections
//   - Medium traffic: 10-50 connections
//   - High traffic: 50-200 connections
//
// # Connection Pooling Best Practices
//
// Optimal pool sizing:
//
//	# Low traffic (<100 ops/s)
//	REDIS_MAX_CONNECTIONS=10
//	REDIS_MAX_IDLE_CONNECTIONS=5
//
//	# Medium traffic (100-1000 ops/s)
//	REDIS_MAX_CONNECTIONS=50
//	REDIS_MAX_IDLE_CONNECTIONS=25
//
//	# High traffic (>1000 ops/s)
//	REDIS_MAX_CONNECTIONS=200
//	REDIS_MAX_IDLE_CONNECTIONS=100
//
// Pool tuning guidelines:
//   - Monitor connection acquisition time
//   - Watch for connection timeouts
//   - Track connection pool exhaustion
//   - Balance connections vs memory overhead
//   - Keep MaxIdleConnections = MaxConnections / 2
//
// # Redis Databases
//
// Redis supports multiple logical databases (0-15 by default):
//
//	REDIS_DATABASE=0   # Default database
//	REDIS_DATABASE=1   # Session storage
//	REDIS_DATABASE=2   # Cache
//	REDIS_DATABASE=3   # Rate limiting
//
// Database selection strategy:
//   - Use separate databases for different concerns
//   - Avoid mixing cache and persistent data
//   - Consider separate Redis instances for critical workloads
//   - Use key prefixes within databases for namespacing
//
// Note: Redis Cluster does not support multiple databases (only database 0).
//
// # Eviction Policies
//
// Redis eviction policies control what happens when memory limit is reached.
// Configure on the Redis server (not in client config):
//
//	# In redis.conf
//	maxmemory 2gb
//	maxmemory-policy allkeys-lru
//
// Common eviction policies:
//   - noeviction: Return errors when memory limit reached (default)
//   - allkeys-lru: Evict least recently used keys
//   - allkeys-lfu: Evict least frequently used keys
//   - volatile-lru: Evict LRU keys with TTL set
//   - volatile-lfu: Evict LFU keys with TTL set
//   - volatile-ttl: Evict keys with shortest TTL
//   - allkeys-random: Evict random keys
//   - volatile-random: Evict random keys with TTL
//
// Recommendations:
//   - Cache-only: allkeys-lru or allkeys-lfu
//   - Mixed workload: volatile-lru or volatile-lfu
//   - Session storage: volatile-ttl
//   - Critical data: noeviction (provision enough memory)
//
// For more: https://redis.io/docs/manual/eviction/
//
// # TTL Management
//
// Time-To-Live (TTL) controls data expiration:
//
//	// Set with TTL
//	client.Set(ctx, "key", "value", 5*time.Minute)
//
//	// Set TTL on existing key
//	client.Expire(ctx, "key", 10*time.Minute)
//
//	// Remove TTL (make persistent)
//	client.Persist(ctx, "key")
//
//	// Check remaining TTL
//	ttl := client.TTL(ctx, "key")
//
// TTL strategies:
//   - Short TTL (1-5 min): Frequently changing data, rate limits
//   - Medium TTL (5-60 min): Application cache, session data
//   - Long TTL (1-24 hours): Expensive computations, aggregated data
//   - No TTL (persistent): Configuration, feature flags
//
// Default TTL (REDIS_DEFAULT_TTL) is used when not specified.
//
// # Configuration Reference
//
// All configuration values can be overridden via environment variables:
//
//	# Connection settings
//	REDIS_HOST=localhost:6379                    # Redis server address
//	REDIS_PASSWORD=secret_password               # Authentication password
//	REDIS_DATABASE=0                             # Logical database (0-15)
//
//	# Connection pool settings
//	REDIS_MAX_CONNECTIONS=10                     # Maximum connections
//	REDIS_MAX_IDLE_CONNECTIONS=5                 # Idle connections kept alive
//	REDIS_CONNECTION_TIMEOUT=5s                  # Connection timeout
//	REDIS_CONNECTION_MAX_LIFETIME=30m            # Maximum connection lifetime
//
//	# Cache behavior
//	REDIS_DEFAULT_TTL=5m                         # Default expiration time
//	REDIS_ENABLE_CONNECTION_POOLING=true         # Enable connection pooling
//
// # Security
//
// Authentication:
//   - Use strong passwords (REDIS_PASSWORD)
//   - Disable anonymous access (requirepass in redis.conf)
//   - Use ACLs for fine-grained permissions (Redis 6+)
//   - Consider TLS for production (redis:// vs rediss://)
//
// Network security:
//   - Bind to localhost or private network only
//   - Use firewalls to restrict access
//   - Enable TLS for encrypted communication
//   - Use VPC/private networks in cloud environments
//
// # Performance Tuning
//
// High-throughput caching:
//
//	REDIS_MAX_CONNECTIONS=100
//	REDIS_MAX_IDLE_CONNECTIONS=50
//	REDIS_CONNECTION_TIMEOUT=10s
//	REDIS_DEFAULT_TTL=10m
//
// Session storage:
//
//	REDIS_MAX_CONNECTIONS=50
//	REDIS_DEFAULT_TTL=30m
//	REDIS_DATABASE=1
//
// Rate limiting:
//
//	REDIS_MAX_CONNECTIONS=200
//	REDIS_CONNECTION_TIMEOUT=1s
//	REDIS_DEFAULT_TTL=1m
//	REDIS_DATABASE=2
//
// # Best Practices
//
// Key naming:
//   - Use consistent prefixes: "user:1234:profile", "cache:query:abc"
//   - Include entity type and ID
//   - Use colons for hierarchy
//   - Keep keys short but descriptive
//   - Use namespaces for multi-tenant systems
//
// Data modeling:
//   - Choose appropriate data structures
//   - Use hashes for objects (more memory efficient)
//   - Use sorted sets for rankings and leaderboards
//   - Use sets for unique collections
//   - Use streams for event logs
//
// Caching patterns:
//   - Cache-aside: Application manages cache (most common)
//   - Write-through: Update cache on write
//   - Write-behind: Async write to backing store
//   - Read-through: Cache loads from backing store
//
// Monitoring:
//   - Track hit rate (INFO stats)
//   - Monitor memory usage
//   - Watch for slow commands (SLOWLOG)
//   - Alert on high eviction rates
//   - Monitor replication lag
//
// # Example Usage
//
//	// Load configuration
//	envProvider := provider.NewEnvVarProvider()
//	cfg, err := redis.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal("failed to load config:", err)
//	}
//
//	// Create Redis client
//	client := redis.NewClient(&redis.Options{
//	    Addr:            cfg.Host,
//	    Password:        cfg.Password,
//	    DB:              cfg.Database,
//	    PoolSize:        cfg.MaxConnections,
//	    MinIdleConns:    cfg.MaxIdleConnections,
//	    ConnMaxLifetime: cfg.ConnectionMaxLifetime,
//	    DialTimeout:     cfg.ConnectionTimeout,
//	})
//
//	// Use Redis
//	ctx := context.Background()
//	err = client.Set(ctx, "user:1234", "data", cfg.DefaultTTL).Err()
//	val, err := client.Get(ctx, "user:1234").Result()
//
// # Troubleshooting
//
// Connection issues:
//
//	Error: "connection refused"
//	Fix: Check Redis server is running and REDIS_HOST is correct
//
//	Error: "connection timeout"
//	Fix: Increase REDIS_CONNECTION_TIMEOUT or check network latency
//
//	Error: "NOAUTH Authentication required"
//	Fix: Set REDIS_PASSWORD if Redis requires authentication
//
// Performance issues:
//
//	Problem: High latency
//	Fix: Check network latency, move Redis closer, increase connection pool
//
//	Problem: Out of connections
//	Fix: Increase REDIS_MAX_CONNECTIONS or check for connection leaks
//
//	Problem: High eviction rate
//	Fix: Increase Redis memory, adjust TTL, optimize data size
//
// # Cross-References
//
// Related packages:
//   - [base.Validator] - Validation framework
//   - [decodeutil] - Configuration decoding
//   - [propertyresolver] - Property resolution
//
// Related documentation:
//   - Redis Documentation: https://redis.io/docs
//   - Redis Commands: https://redis.io/commands
//   - Go Redis Client: https://github.com/redis/go-redis
//   - Eviction Policies: https://redis.io/docs/manual/eviction/
//   - Best Practices: https://redis.io/docs/manual/patterns/
//
// # Files in This Package
//
// constants.go:
//   - REDIS_PREFIX for environment variable namespacing
//   - Default values and connection parameters
//   - Min/max bounds for validation
//
// section.go:
//   - [Config] struct with Redis parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package redis
