// internal/config/http/section.go
//
// Package http defines the configuration for the HTTP Server.
//
// It consists of a Config struct and methods to resolve incoming key-values from a [config.Manager]
package http

import (
	"fmt"
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
	"github.com/jamesneb/playback-backend/internal/config/decodeutil"
	resolver "github.com/jamesneb/playback-backend/internal/config/propertyresolver"
)

// Config holds HTTP server configuration
type Config struct {
	// Server basics
	Host base.Host     `mapstructure:"host"`
	Port base.Port     `mapstructure:"port"`
	Mode base.HTTPMode `mapstructure:"mode"`

	// Timeouts
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`

	// Size limits
	MaxRequestSize base.Byte `mapstructure:"max_request_size"`
	MaxHeaderSize  base.Byte `mapstructure:"max_header_size"`

	// API configuration
	APIPrefix      string   `mapstructure:"api_prefix"`
	TrustedProxies []string `mapstructure:"trusted_proxies"`

	// CORS
	EnableCORS bool       `mapstructure:"enable_cors"`
	CORS       CORSConfig `mapstructure:"cors,squash"`

	// Rate limiting
	RateLimitRPS   int `mapstructure:"rate_limit_rps"`
	RateLimitBurst int `mapstructure:"rate_limit_burst"`

	// TLS
	TLS TLSConfig `mapstructure:"tls,squash"`

	// Security
	EnableAuth       bool          `mapstructure:"enable_auth"`
	JWTSecret        string        `mapstructure:"jwt_secret"`
	JWTExpiry        time.Duration `mapstructure:"jwt_expiry"`
	JWTRefreshWindow time.Duration `mapstructure:"jwt_refresh_window"`
	JWTIssuer        string        `mapstructure:"jwt_issuer"`
	JWTAudience      string        `mapstructure:"jwt_audience"`

	// Performance
	EnableProfiling      bool          `mapstructure:"enable_profiling"`
	CompressionLevel     int           `mapstructure:"compression_level"`
	CompressionThreshold base.Byte     `mapstructure:"compression_threshold"`
	KeepAlive            bool          `mapstructure:"keep_alive"`
	KeepAliveTimeout     time.Duration `mapstructure:"keep_alive_timeout"`

	// Development
	EnableSwagger bool      `mapstructure:"enable_swagger"`
	SwaggerPath   base.Path `mapstructure:"swagger_path"`
	EnableDebug   bool      `mapstructure:"enable_debug"`
}

// TLSConfig holds TLS/SSL configuration
type TLSConfig struct {
	Enabled    bool            `mapstructure:"tls_enabled"`
	CertFile   string          `mapstructure:"tls_cert_file"`
	KeyFile    string          `mapstructure:"tls_key_file"`
	CAFile     string          `mapstructure:"tls_ca_file"`
	MinVersion base.TLSVersion `mapstructure:"tls_min_version"`
	MaxVersion base.TLSVersion `mapstructure:"tls_max_version"`
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	AllowedOrigins   []string          `mapstructure:"cors_allowed_origins"`
	AllowedMethods   []base.HTTPMethod `mapstructure:"cors_allowed_methods"`
	AllowedHeaders   []base.HTTPHeader `mapstructure:"cors_allowed_headers"`
	ExposedHeaders   []base.HTTPHeader `mapstructure:"cors_exposed_headers"`
	AllowCredentials bool              `mapstructure:"cors_allow_credentials"`
	MaxAge           time.Duration     `mapstructure:"cors_max_age"`
}

// Defaults returns reasonable default values for the HTTP Server Configuration.
// does not accept external config
// No external reads; callers overlay values via FromResolver
func Defaults() Config {
	return Config{
		Host:            DEFAULT_HOST,
		Port:            DEFAULT_PORT,
		Mode:            DEFAULT_MODE,
		ReadTimeout:     DEFAULT_READ_TIMEOUT,
		WriteTimeout:    DEFAULT_WRITE_TIMEOUT,
		IdleTimeout:     DEFAULT_IDLE_TIMEOUT,
		ShutdownTimeout: DEFAULT_SHUTDOWN_TIMEOUT,
		MaxRequestSize:  DEFAULT_REQUEST_SIZE,
		MaxHeaderSize:   DEFAULT_HEADER_SIZE,
		APIPrefix:       DEFAULT_API_PREFIX,
		EnableCORS:      DEFAULT_ENABLE_CORS,
		CORS: CORSConfig{
			AllowedOrigins:   DEFAULT_CORS_ALLOWED_ORIGINS,
			AllowedMethods:   DEFAULT_CORS_ALLOWED_METHODS,
			AllowedHeaders:   DEFAULT_CORS_ALLOWED_HEADERS,
			ExposedHeaders:   DEFAULT_CORS_EXPOSED_HEADERS,
			AllowCredentials: DEFAULT_CORS_ALLOW_CREDENTIALS,
			MaxAge:           DEFAULT_CORS_MAX_AGE,
		},
		RateLimitRPS:   DEFAULT_RPS,
		RateLimitBurst: DEFAULT_BURST,
		TLS: TLSConfig{
			Enabled:    DEFAULT_TLS_ENABLED,
			CertFile:   "",
			KeyFile:    "",
			CAFile:     "",
			MinVersion: DEFAULT_TLS_MIN_VERSION,
			MaxVersion: DEFAULT_TLS_MAX_VERSION,
		},
		EnableAuth:           DEFAULT_ENABLE_AUTH,
		JWTSecret:            "",
		JWTExpiry:            DEFAULT_JWT_EXPIRY,
		JWTRefreshWindow:     DEFAULT_JWT_REFRESH_WINDOW,
		JWTIssuer:            DEFAULT_JWT_ISSUER,
		JWTAudience:          DEFAULT_JWT_AUDIENCE,
		EnableProfiling:      DEFAULT_ENABLE_PROFILING,
		CompressionLevel:     DEFAULT_COMPRESSION_LEVEL,
		CompressionThreshold: DEFAULT_COMPRESSION_THRESHOLD,
		KeepAlive:            DEFAULT_KEEP_ALIVE,
		KeepAliveTimeout:     DEFAULT_KEEP_ALIVE_TIMEOUT,
		EnableSwagger:        DEFAULT_ENABLE_SWAGGER,
		SwaggerPath:          DEFAULT_SWAGGER_PATH,
		EnableDebug:          DEFAULT_ENABLE_DEBUG,
	}
}

// validates the configuration generated by a propertyresolver
func (c Config) Validate() error {
	v := base.NewValidator(HTTP_PREFIX)

	// Port and timeouts
	base.RangeFNum(v, "port", c.Port, MIN_PORT, MAX_PORT, "")
	base.RangeFNum(v, "read_timeout", c.ReadTimeout, MIN_TIMEOUT, MAX_TIMEOUT, "")
	base.RangeFNum(v, "write_timeout", c.WriteTimeout, MIN_TIMEOUT, MAX_TIMEOUT, "")
	base.RangeFNum(v, "idle_timeout", c.IdleTimeout, MIN_TIMEOUT, MAX_TIMEOUT, "")
	base.RangeFNum(v, "shutdown_timeout", c.ShutdownTimeout, MIN_TIMEOUT, MAX_TIMEOUT, "")

	// Sizes
	base.RangeFNum(v, "max_request_size", c.MaxRequestSize, MIN_REQUEST_SIZE, MAX_REQUEST_SIZE, "bytes")
	base.RangeFNum(v, "max_header_size", c.MaxHeaderSize, MIN_HEADER_SIZE, MAX_HEADER_SIZE, "bytes")

	// Rate limiting - check if disabled or within range
	base.RangeOrAllowed(v, "rate_limit_rps", c.RateLimitRPS, MIN_RPS, MAX_RPS, "", RATE_LIMIT_DISABLED)

	// Couple RPS to Burst (same pattern as GRPC)
	v.When(c.RateLimitRPS == RATE_LIMIT_DISABLED, func(v *base.Validator) {
		v.Assert("rate_limit_burst", c.RateLimitBurst == RATE_LIMIT_DISABLED,
			"must be 0 when %s=0 (got %d)", "rate_limit_rps", c.RateLimitBurst)
	})

	v.When(c.RateLimitRPS > RATE_LIMIT_DISABLED, func(v *base.Validator) {
		base.RangeFNum(v, "rate_limit_burst", c.RateLimitBurst, MIN_BURST, MAX_BURST, "")

		// Bound burst by multiple of RPS
		maxByRPS := c.RateLimitRPS * MAX_BURST_MULTIPLIER
		maxByRPS = max(MIN_BURST, maxByRPS)
		v.Assert("rate_limit_burst", c.RateLimitBurst <= maxByRPS,
			"too large relative to %s: %d > %d (rps=%d)",
			"rate_limit_rps", c.RateLimitBurst, maxByRPS, c.RateLimitRPS)
	})

	// TLS validation (only if enabled)
	v.When(c.TLS.Enabled, func(v *base.Validator) {
		base.NotEmpty(v, "tls_cert_file", c.TLS.CertFile)
		base.NotEmpty(v, "tls_key_file", c.TLS.KeyFile)
	})

	// JWT validation
	v.When(c.EnableAuth, func(v *base.Validator) {
		base.NotEmpty(v, "jwt_secret", c.JWTSecret)
		base.RangeFNum(v, "jwt_expiry", c.JWTExpiry, MIN_JWT_EXPIRY, MAX_JWT_EXPIRY, "")
		base.RangeFNum(v, "jwt_refresh_window", c.JWTRefreshWindow, MIN_JWT_REFRESH_WINDOW, MAX_JWT_REFRESH_WINDOW, "")
		base.NotEmpty(v, "jwt_issuer", c.JWTIssuer)
		base.NotEmpty(v, "jwt_audience", c.JWTAudience)
	})

	// CORS validation
	v.When(c.EnableCORS, func(v *base.Validator) {
		base.RangeFNum(v, "cors_max_age", c.CORS.MaxAge, MIN_CORS_MAX_AGE, MAX_CORS_MAX_AGE, "")
	})

	// Performance validation
	base.RangeFNum(v, "compression_level", c.CompressionLevel, MIN_COMPRESSION_LEVEL, MAX_COMPRESSION_LEVEL, "")
	base.RangeFNum(
		v,
		"compression_threshold",
		c.CompressionThreshold,
		MIN_COMPRESSION_THRESHOLD,
		MAX_COMPRESSION_THRESHOLD,
		"",
	)

	v.When(c.KeepAlive, func(v *base.Validator) {
		base.RangeFNum(v, "keep_alive_timeout", c.KeepAliveTimeout, MIN_KEEP_ALIVE_TIMEOUT, MAX_KEEP_ALIVE_TIMEOUT, "")
	})

	return v.Err()
}

// overlays values from a [propertyresolver.PropertyResolver] onto Defaults and validates them
func FromResolver(r resolver.PropertyResolver) (Config, error) {
	cfg := Defaults()

	// Decodes values into mapstructure
	if err := decodeutil.DecodePrefixInto(r, HTTP_PREFIX, &cfg); err != nil {
		return Config{}, fmt.Errorf("http decode: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
