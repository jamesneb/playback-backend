package server

import "time"

// ServerConfig defines HTTP server configuration
type ServerConfig struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	Mode           string        `yaml:"mode"` // gin mode: debug, release, test
	ReadTimeout    time.Duration `yaml:"read_timeout"`
	WriteTimeout   time.Duration `yaml:"write_timeout"`
	IdleTimeout    time.Duration `yaml:"idle_timeout"`
	MaxHeaderBytes int           `yaml:"max_header_bytes"`
	TrustedProxies []string      `yaml:"trusted_proxies"`
	// TLS configuration
	TLS TLSConfig `yaml:"tls"`
	// Rate limiting per IP
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	// Request timeout
	RequestTimeout time.Duration `yaml:"request_timeout"`
	// Legacy backward compatibility
	ReadTimeoutDuration     time.Duration `yaml:"read_timeout_duration"`
	WriteTimeoutDuration    time.Duration `yaml:"write_timeout_duration"`
	IdleTimeoutDuration     time.Duration `yaml:"idle_timeout_duration"`
	ShutdownTimeoutDuration time.Duration `yaml:"shutdown_timeout_duration"`
	GRPCPort                int           `yaml:"grpc_port"`
}

// TLSConfig defines TLS/SSL configuration
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// RateLimitConfig defines rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int           `yaml:"requests_per_second"`
	BurstSize         int           `yaml:"burst_size"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval"`
}
