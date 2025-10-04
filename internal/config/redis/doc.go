// Package redis defines configuration for Redis cache connections.
//
// Redis is an in-memory data structure store used as a cache, message broker, and
// session store. This package provides configuration for Redis client connections
// with connection pooling, timeouts, and TTL management.
//
// # Connection Management
//
// Redis connections are pooled to avoid connection overhead. Key settings:
//   - MaxConnections: Maximum concurrent connections to Redis
//   - MaxIdleConnections: Idle connections kept alive for reuse
//   - ConnectionTimeout: Maximum time to establish connection
//   - DefaultTTL: Default expiration time for cached values
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// REDIS_ prefix:
//
//	REDIS_HOST=localhost:6379
//	REDIS_PASSWORD=secret
//	REDIS_DATABASE=0
//	REDIS_MAX_CONNECTIONS=10
//	REDIS_MAX_IDLE_CONNECTIONS=5
//	REDIS_CONNECTION_TIMEOUT=5s
//	REDIS_DEFAULT_TTL=5m
//	REDIS_ENABLE_CONNECTION_POOLING=true
//	REDIS_CONNECTION_MAX_LIFETIME=30m
//
// # Files in This Package
//
// constants.go:
//   - REDIS_PREFIX for environment variable namespacing
//   - Default values (host, connection limits, timeouts, TTL)
//   - Min/max bounds for validation
//
// section.go:
//   - Config struct with connection parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package redis
