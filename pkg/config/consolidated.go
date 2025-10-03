package config

import (
	"time"
)

// ConsolidatedConfig represents a clean, simplified configuration structure
// Following clean architecture principles with logical grouping
type ConsolidatedConfig struct {
	// Core application settings
	App AppSettings `yaml:"app"`

	// Network layer - HTTP, gRPC, and networking
	Network NetworkSettings `yaml:"network"`

	// Data layer - databases, caching, and storage
	Data DataSettings `yaml:"data"`

	// Operations - monitoring, security, and deployment
	Operations OperationsSettings `yaml:"operations"`
}

// AppSettings contains core application configuration
type AppSettings struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
	LogLevel    string `yaml:"log_level" default:"info"`
	LogFormat   string `yaml:"log_format" default:"json"`
}

// NetworkSettings consolidates all networking configuration
type NetworkSettings struct {
	HTTP HTTPSettings `yaml:"http"`
	GRPC GRPCSettings `yaml:"grpc"`
}

// HTTPSettings contains HTTP server and API configuration
type HTTPSettings struct {
	// Server basics
	Host string `yaml:"host" default:"0.0.0.0"`
	Port int    `yaml:"port" default:"8080"`
	Mode string `yaml:"mode" default:"release"` // gin mode

	// Timeouts and limits
	ReadTimeout      time.Duration `yaml:"read_timeout" default:"30s"`
	WriteTimeout     time.Duration `yaml:"write_timeout" default:"30s"`
	IdleTimeout      time.Duration `yaml:"idle_timeout" default:"60s"`
	ShutdownTimeout  time.Duration `yaml:"shutdown_timeout" default:"30s"`
	MaxRequestSizeMB int           `yaml:"max_request_size_mb" default:"25"`
	MaxHeaderSizeKB  int           `yaml:"max_header_size_kb" default:"1024"`

	// API configuration
	APIPrefix      string       `yaml:"api_prefix" default:"/api/v1"`
	TrustedProxies []string     `yaml:"trusted_proxies"`
	EnableCORS     bool         `yaml:"enable_cors" default:"true"`
	CORS           CORSSettings `yaml:"cors"`

	// Rate limiting
	RateLimitRPS   int `yaml:"rate_limit_rps" default:"1000"`
	RateLimitBurst int `yaml:"rate_limit_burst" default:"2000"`

	// Security
	EnableAuth     bool   `yaml:"enable_auth" default:"false"`
	JWTSecret      string `yaml:"jwt_secret"`
	JWTExpiryHours int    `yaml:"jwt_expiry_hours" default:"24"`

	// Development features
	EnableSwagger bool   `yaml:"enable_swagger" default:"false"`
	SwaggerPath   string `yaml:"swagger_path" default:"/swagger"`
	EnableDebug   bool   `yaml:"enable_debug" default:"false"`
}

// CORSSettings contains CORS configuration
type CORSSettings struct {
	AllowedOrigins   []string `yaml:"allowed_origins" default:"[\"*\"]"`
	AllowedMethods   []string `yaml:"allowed_methods" default:"[\"GET\",\"POST\",\"PUT\",\"DELETE\",\"OPTIONS\",\"HEAD\",\"PATCH\"]"`
	AllowedHeaders   []string `yaml:"allowed_headers" default:"[\"Origin\",\"Content-Type\",\"Accept\",\"Authorization\",\"X-Requested-With\"]"`
	ExposedHeaders   []string `yaml:"exposed_headers" default:"[\"Content-Length\"]"`
	AllowCredentials bool     `yaml:"allow_credentials" default:"false"`
	MaxAge           int      `yaml:"max_age" default:"3600"`
}

// GRPCSettings contains gRPC server configuration
type GRPCSettings struct {
	Port              int           `yaml:"port" default:"4317"`
	MaxRecvSizeMB     int           `yaml:"max_recv_size_mb" default:"16"`
	MaxSendSizeMB     int           `yaml:"max_send_size_mb" default:"16"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout" default:"30s"`
}

// DataSettings consolidates all data layer configuration
type DataSettings struct {
	ClickHouse ClickHouseSettings `yaml:"clickhouse"`
	Redis      RedisSettings      `yaml:"redis"`
	S3         S3Settings         `yaml:"s3"`
	Kinesis    KinesisSettings    `yaml:"kinesis"`

	// Data processing
	BatchSize         int           `yaml:"batch_size" default:"1000"`
	FlushInterval     time.Duration `yaml:"flush_interval" default:"5s"`
	WorkerCount       int           `yaml:"worker_count" default:"4"`
	MaxQueueSize      int           `yaml:"max_queue_size" default:"10000"`
	EnableCompression bool          `yaml:"enable_compression" default:"true"`

	// Retention (days)
	RetentionTraces  int `yaml:"retention_traces" default:"7"`
	RetentionMetrics int `yaml:"retention_metrics" default:"30"`
	RetentionLogs    int `yaml:"retention_logs" default:"7"`
}

// ClickHouseSettings contains ClickHouse database configuration
type ClickHouseSettings struct {
	Host               string        `yaml:"host" default:"localhost:9000"`
	HTTPHost           string        `yaml:"http_host" default:"localhost:8123"`
	Database           string        `yaml:"database" default:"telemetry"`
	Username           string        `yaml:"username" default:"default"`
	Password           string        `yaml:"password"`
	MaxConnections     int           `yaml:"max_connections" default:"10"`
	MaxIdleConnections int           `yaml:"max_idle_connections" default:"5"`
	ConnectionTimeout  time.Duration `yaml:"connection_timeout" default:"30s"`
	EnableCompression  bool          `yaml:"enable_compression" default:"true"`
}

// RedisSettings contains Redis cache configuration
type RedisSettings struct {
	Host               string        `yaml:"host" default:"localhost:6379"`
	Password           string        `yaml:"password"`
	Database           int           `yaml:"database" default:"0"`
	MaxConnections     int           `yaml:"max_connections" default:"10"`
	MaxIdleConnections int           `yaml:"max_idle_connections" default:"5"`
	ConnectionTimeout  time.Duration `yaml:"connection_timeout" default:"5s"`
	DefaultTTL         time.Duration `yaml:"default_ttl" default:"5m"`
}

// S3Settings contains S3 storage configuration
type S3Settings struct {
	Region          string `yaml:"region" default:"us-east-1"`
	Bucket          string `yaml:"bucket"`
	EndpointURL     string `yaml:"endpoint_url"` // For LocalStack
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	ForcePathStyle  bool   `yaml:"force_path_style" default:"false"`
}

// KinesisSettings contains Kinesis streaming configuration
type KinesisSettings struct {
	Region          string `yaml:"region" default:"us-east-1"`
	EndpointURL     string `yaml:"endpoint_url"` // For LocalStack
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`

	// Stream names
	TracesStream  string `yaml:"traces_stream" default:"telemetry-traces"`
	MetricsStream string `yaml:"metrics_stream" default:"telemetry-metrics"`
	LogsStream    string `yaml:"logs_stream" default:"telemetry-logs"`

	// Processing
	BatchSize     int           `yaml:"batch_size" default:"100"`
	FlushInterval time.Duration `yaml:"flush_interval" default:"5s"`
	MaxRetries    int           `yaml:"max_retries" default:"3"`
	RetryDelay    time.Duration `yaml:"retry_delay" default:"1s"`
}

// OperationsSettings consolidates monitoring, deployment, and operational concerns
type OperationsSettings struct {
	// Monitoring
	EnableMetrics   bool   `yaml:"enable_metrics" default:"true"`
	MetricsPort     int    `yaml:"metrics_port" default:"9090"`
	MetricsPath     string `yaml:"metrics_path" default:"/metrics"`
	EnableTracing   bool   `yaml:"enable_tracing" default:"true"`
	TracingEndpoint string `yaml:"tracing_endpoint"`
	HealthCheckPath string `yaml:"health_path" default:"/health"`

	// Resilience
	CircuitBreakerEnabled   bool          `yaml:"circuit_breaker_enabled" default:"true"`
	CircuitBreakerThreshold float64       `yaml:"circuit_breaker_threshold" default:"0.6"`
	CircuitBreakerTimeout   time.Duration `yaml:"circuit_breaker_timeout" default:"10s"`

	// Dead Letter Queue
	DLQEnabled         bool          `yaml:"dlq_enabled" default:"true"`
	DLQQueueURL        string        `yaml:"dlq_queue_url"`
	DLQRegion          string        `yaml:"dlq_region" default:"us-east-1"`
	DLQLocalBufferSize int           `yaml:"dlq_local_buffer_size" default:"1000"`
	DLQMaxRetries      int           `yaml:"dlq_max_retries" default:"3"`
	DLQRetryBaseDelay  time.Duration `yaml:"dlq_retry_base_delay" default:"5s"`
	DLQRetryMaxDelay   time.Duration `yaml:"dlq_retry_max_delay" default:"5m"`

	// Performance
	EnableConnectionPooling bool          `yaml:"enable_connection_pooling" default:"true"`
	ConnectionMaxLifetime   time.Duration `yaml:"connection_max_lifetime" default:"30m"`

	// Development
	IsDevelopment        bool `yaml:"is_development" default:"false"`
	MockExternalServices bool `yaml:"mock_external_services" default:"false"`
	EnableQueryLogging   bool `yaml:"enable_query_logging" default:"false"`
}

// DefaultConfig returns a configuration with sensible defaults for production
func DefaultConfig() *ConsolidatedConfig {
	return &ConsolidatedConfig{
		App: AppSettings{
			Name:        "playback-backend",
			Version:     "1.0.0",
			Environment: "production",
			LogLevel:    "info",
			LogFormat:   "json",
		},
		Network: NetworkSettings{
			HTTP: HTTPSettings{
				Host:             "0.0.0.0",
				Port:             8080,
				Mode:             "release",
				ReadTimeout:      30 * time.Second,
				WriteTimeout:     30 * time.Second,
				IdleTimeout:      60 * time.Second,
				ShutdownTimeout:  30 * time.Second,
				MaxRequestSizeMB: 25,
				MaxHeaderSizeKB:  1024,
				APIPrefix:        "/api/v1",
				EnableCORS:       true,
				CORS: CORSSettings{
					AllowedOrigins:   []string{"*"},
					AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "HEAD", "PATCH"},
					AllowedHeaders:   []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With"},
					ExposedHeaders:   []string{"Content-Length"},
					AllowCredentials: false,
					MaxAge:           3600,
				},
				RateLimitRPS:   1000,
				RateLimitBurst: 2000,
				EnableAuth:     false,
				JWTExpiryHours: 24,
				EnableSwagger:  false,
				SwaggerPath:    "/swagger",
				EnableDebug:    false,
			},
			GRPC: GRPCSettings{
				Port:              4317,
				MaxRecvSizeMB:     16,
				MaxSendSizeMB:     16,
				ConnectionTimeout: 30 * time.Second,
			},
		},
		Data: DataSettings{
			ClickHouse: ClickHouseSettings{
				Host:               "localhost:9000",
				HTTPHost:           "localhost:8123",
				Database:           "telemetry",
				Username:           "default",
				MaxConnections:     10,
				MaxIdleConnections: 5,
				ConnectionTimeout:  30 * time.Second,
				EnableCompression:  true,
			},
			Redis: RedisSettings{
				Host:               "localhost:6379",
				Database:           0,
				MaxConnections:     10,
				MaxIdleConnections: 5,
				ConnectionTimeout:  5 * time.Second,
				DefaultTTL:         5 * time.Minute,
			},
			Kinesis: KinesisSettings{
				Region:        "us-east-1",
				TracesStream:  "telemetry-traces",
				MetricsStream: "telemetry-metrics",
				LogsStream:    "telemetry-logs",
				BatchSize:     100,
				FlushInterval: 5 * time.Second,
				MaxRetries:    3,
				RetryDelay:    1 * time.Second,
			},
			BatchSize:         1000,
			FlushInterval:     5 * time.Second,
			WorkerCount:       4,
			MaxQueueSize:      10000,
			EnableCompression: true,
			RetentionTraces:   7,
			RetentionMetrics:  30,
			RetentionLogs:     7,
		},
		Operations: OperationsSettings{
			EnableMetrics:           true,
			MetricsPort:             9090,
			MetricsPath:             "/metrics",
			EnableTracing:           true,
			HealthCheckPath:         "/health",
			CircuitBreakerEnabled:   true,
			CircuitBreakerThreshold: 0.6,
			CircuitBreakerTimeout:   10 * time.Second,
			DLQEnabled:              true,
			DLQRegion:               "us-east-1",
			DLQLocalBufferSize:      1000,
			DLQMaxRetries:           3,
			DLQRetryBaseDelay:       5 * time.Second,
			DLQRetryMaxDelay:        5 * time.Minute,
			EnableConnectionPooling: true,
			ConnectionMaxLifetime:   30 * time.Minute,
			IsDevelopment:           false,
			MockExternalServices:    false,
			EnableQueryLogging:      false,
		},
	}
}
