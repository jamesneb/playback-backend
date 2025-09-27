package database

import "time"

// DatabaseConfig defines database configuration
type DatabaseConfig struct {
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	Redis      RedisConfig      `yaml:"redis"`
}

// ClickHouseConfig defines ClickHouse database configuration
type ClickHouseConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	Username     string        `yaml:"username"`
	Password     string        `yaml:"password"`
	Database     string        `yaml:"database"`
	MaxOpenConns int           `yaml:"max_open_conns"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
	MaxLifetime  time.Duration `yaml:"max_lifetime"`
	// Connection pool settings
	MaxIdleTime     time.Duration `yaml:"max_idle_time"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	// Query settings
	QueryTimeout time.Duration `yaml:"query_timeout"`
	EnableDebug  bool          `yaml:"enable_debug"`
	// Legacy backward compatibility fields
	MaxConnections     int `yaml:"max_connections"`
	MaxIdleConnections int `yaml:"max_idle_connections"`
}

// RedisConfig defines Redis cache configuration
type RedisConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	Password     string        `yaml:"password"`
	Database     int           `yaml:"database"`
	MaxRetries   int           `yaml:"max_retries"`
	PoolSize     int           `yaml:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns"`
	MaxConnAge   time.Duration `yaml:"max_conn_age"`
	PoolTimeout  time.Duration `yaml:"pool_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// CacheConfig defines caching configuration
type CacheConfig struct {
	Redis       RedisCacheConfig `yaml:"redis"`
	Application AppCacheConfig   `yaml:"application"`
}

// RedisCacheConfig defines Redis-specific caching configuration
type RedisCacheConfig struct {
	Enabled bool          `yaml:"enabled"`
	TTL     time.Duration `yaml:"ttl"`
	Prefix  string        `yaml:"prefix"`
}

// AppCacheConfig defines application-level caching configuration
type AppCacheConfig struct {
	Enabled    bool          `yaml:"enabled"`
	MaxEntries int           `yaml:"max_entries"`
	TTL        time.Duration `yaml:"ttl"`
}
