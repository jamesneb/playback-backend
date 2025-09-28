// Package config provides backward compatibility type aliases.
// These aliases are provided for compatibility with existing code that hasn't been
// migrated to use the structured config packages directly.
//
// Deprecated: New code should import the specific config packages directly:
//   - github.com/jamesneb/playback-backend/pkg/config/api
//   - github.com/jamesneb/playback-backend/pkg/config/app
//   - github.com/jamesneb/playback-backend/pkg/config/database
//   - github.com/jamesneb/playback-backend/pkg/config/processing
//   - github.com/jamesneb/playback-backend/pkg/config/security
//   - github.com/jamesneb/playback-backend/pkg/config/server
//   - github.com/jamesneb/playback-backend/pkg/config/streaming
package config

import (
	"github.com/jamesneb/playback-backend/pkg/config/api"
	"github.com/jamesneb/playback-backend/pkg/config/app"
	"github.com/jamesneb/playback-backend/pkg/config/database"
	"github.com/jamesneb/playback-backend/pkg/config/processing"
	"github.com/jamesneb/playback-backend/pkg/config/security"
	"github.com/jamesneb/playback-backend/pkg/config/server"
	"github.com/jamesneb/playback-backend/pkg/config/streaming"
)

// Backward compatibility type aliases
// Deprecated: Use streaming.KinesisConfig directly
type KinesisConfig = streaming.KinesisConfig

// Deprecated: Use server.ServerConfig directly
type ServerConfig = server.ServerConfig

// Deprecated: Use database.DatabaseConfig directly
type DatabaseConfig = database.DatabaseConfig

// Deprecated: Use api.APIConfig directly
type APIConfig = api.APIConfig

// Deprecated: Use app.AppConfig directly
type AppConfig = app.AppConfig

// Deprecated: Use app.LoggingConfig directly
type LoggingConfig = app.LoggingConfig

// Deprecated: Use security.SecurityConfig directly
type SecurityConfig = security.SecurityConfig

// Deprecated: Use streaming.StreamingConfig directly
type StreamingConfig = streaming.StreamingConfig

// Deprecated: Use processing.ProcessingConfig directly
type ProcessingConfig = processing.ProcessingConfig

// Deprecated: Use processing.RetentionConfig directly
type RetentionConfig = processing.RetentionConfig

// Deprecated: Use streaming.ResilienceConfig directly
type ResilienceConfig = streaming.ResilienceConfig

// Deprecated: Use streaming.CircuitBreakerConfig directly
type CircuitBreakerConfig = streaming.CircuitBreakerConfig

// Deprecated: Use streaming.RateLimiterConfig directly
type RateLimiterConfig = streaming.RateLimiterConfig

// Deprecated: Use streaming.DLQConfig directly
type DLQConfig = streaming.DLQConfig

// Deprecated: Use streaming.BufferConfig directly
type BufferConfig = streaming.BufferConfig

// Deprecated: Use database.ClickHouseConfig directly
type ClickHouseConfig = database.ClickHouseConfig

// Deprecated: Use database.RedisConfig directly
type RedisConfig = database.RedisConfig

// Deprecated: Use database.CacheConfig directly
type CacheConfig = database.CacheConfig

// Deprecated: Use api.CORSConfig directly
type CORSConfig = api.CORSConfig

// Deprecated: Use security.TLSConfig directly
type TLSConfig = security.TLSConfig

// Deprecated: Use security.JWTConfig directly
type JWTConfig = security.JWTConfig

// Deprecated: Use security.MonitoringConfig directly
type MonitoringConfig = security.MonitoringConfig

// Deprecated: Use security.JaegerConfig directly
type JaegerConfig = security.JaegerConfig

// Deprecated: Use security.PrometheusConfig directly
type PrometheusConfig = security.PrometheusConfig

// Deprecated: Use server.RateLimitConfig directly
type RateLimitConfig = server.RateLimitConfig
