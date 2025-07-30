package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Config struct {
	App         AppConfig         `yaml:"app"`
	Server      ServerConfig      `yaml:"server"`
	Logging     LoggingConfig     `yaml:"logging"`
	API         APIConfig         `yaml:"api"`
	Database    DatabaseConfig    `yaml:"database"`
	Streaming   StreamingConfig   `yaml:"streaming"`
	Processing  ProcessingConfig  `yaml:"processing"`
	Retention   RetentionConfig   `yaml:"retention"`
	Cache       CacheConfig       `yaml:"cache"`
	Monitoring  MonitoringConfig  `yaml:"monitoring"`
	Security    SecurityConfig    `yaml:"security"`
	Features    FeaturesConfig    `yaml:"features"`
	Performance PerformanceConfig `yaml:"performance"`
	Development DevelopmentConfig `yaml:"development"`
	Swagger     SwaggerConfig     `yaml:"swagger"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Environment string `yaml:"environment"`
}

type ServerConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Mode           string `yaml:"mode"`
	TrustedProxies []string `yaml:"trusted_proxies"`
	ReadTimeout    string `yaml:"read_timeout"`
	WriteTimeout   string `yaml:"write_timeout"`
	IdleTimeout    string `yaml:"idle_timeout"`
	MaxHeaderBytes int    `yaml:"max_header_bytes"`
}

type LoggingConfig struct {
	Level           string `yaml:"level"`
	Format          string `yaml:"format"`
	Output          string `yaml:"output"`
	EnableCaller    bool   `yaml:"enable_caller"`
	EnableStacktrace bool   `yaml:"enable_stacktrace"`
}

type APIConfig struct {
	Version     string            `yaml:"version"`
	Prefix      string            `yaml:"prefix"`
	EnableCORS  bool              `yaml:"enable_cors"`
	CORS        CORSConfig        `yaml:"cors"`
	RateLimiting RateLimitConfig  `yaml:"rate_limiting"`
	Timeout     string            `yaml:"timeout"`
	MaxRequestSize string         `yaml:"max_request_size"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	MaxAge         int      `yaml:"max_age"`
}

type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerSecond int  `yaml:"requests_per_second"`
	Burst             int  `yaml:"burst"`
}

type DatabaseConfig struct {
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	Redis      RedisConfig      `yaml:"redis"`
}

type ClickHouseConfig struct {
	Host               string `yaml:"host"`
	HTTPHost           string `yaml:"http_host"`
	Database           string `yaml:"database"`
	Username           string `yaml:"username"`
	Password           string `yaml:"password"`
	MaxConnections     int    `yaml:"max_connections"`
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	ConnectionTimeout  string `yaml:"connection_timeout"`
	ReadTimeout        string `yaml:"read_timeout"`
	WriteTimeout       string `yaml:"write_timeout"`
	EnableCompression  bool   `yaml:"enable_compression"`
}

type RedisConfig struct {
	Host               string `yaml:"host"`
	Password           string `yaml:"password"`
	Database           int    `yaml:"database"`
	MaxConnections     int    `yaml:"max_connections"`
	MaxIdleConnections int    `yaml:"max_idle_connections"`
	ConnectionTimeout  string `yaml:"connection_timeout"`
	ReadTimeout        string `yaml:"read_timeout"`
	WriteTimeout       string `yaml:"write_timeout"`
	EnableCluster      bool   `yaml:"enable_cluster"`
}

type StreamingConfig struct {
	Provider string        `yaml:"provider"`
	Kinesis  KinesisConfig `yaml:"kinesis"`
}

type KinesisConfig struct {
	Region          string            `yaml:"region"`
	EndpointURL     string            `yaml:"endpoint_url,omitempty"`
	AccessKeyID     string            `yaml:"access_key_id,omitempty"`
	SecretAccessKey string            `yaml:"secret_access_key,omitempty"`
	Streams         map[string]string `yaml:"streams"`
	BatchSize       int               `yaml:"batch_size"`
	FlushInterval   string            `yaml:"flush_interval"`
	MaxRetries      int               `yaml:"max_retries"`
	RetryDelay      string            `yaml:"retry_delay"`
}

type ProcessingConfig struct {
	BatchSize         int    `yaml:"batch_size"`
	FlushInterval     string `yaml:"flush_interval"`
	MaxQueueSize      int    `yaml:"max_queue_size"`
	WorkerCount       int    `yaml:"worker_count"`
	RetryAttempts     int    `yaml:"retry_attempts"`
	RetryDelay        string `yaml:"retry_delay"`
	EnableCompression bool   `yaml:"enable_compression"`
	CompressionType   string `yaml:"compression_type"`
}

type RetentionConfig struct {
	Traces              int `yaml:"traces"`
	Metrics             int `yaml:"metrics"`
	Logs                int `yaml:"logs"`
	ServiceDependencies int `yaml:"service_dependencies"`
}

type CacheConfig struct {
	Redis       RedisCacheConfig `yaml:"redis"`
	Application AppCacheConfig   `yaml:"application"`
}

type RedisCacheConfig struct {
	Enabled           bool   `yaml:"enabled"`
	DefaultTTL        string `yaml:"default_ttl"`
	MaxMemoryPolicy   string `yaml:"max_memory_policy"`
}

type AppCacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	Size    int    `yaml:"size"`
	TTL     string `yaml:"ttl"`
}

type MonitoringConfig struct {
	EnableMetrics      bool             `yaml:"enable_metrics"`
	EnableProfiling    bool             `yaml:"enable_profiling"`
	EnableTracing      bool             `yaml:"enable_tracing"`
	MetricsEndpoint    string           `yaml:"metrics_endpoint"`
	HealthEndpoint     string           `yaml:"health_endpoint"`
	ReadyEndpoint      string           `yaml:"ready_endpoint"`
	ProfilingEndpoint  string           `yaml:"profiling_endpoint"`
	Jaeger             JaegerConfig     `yaml:"jaeger"`
	Prometheus         PrometheusConfig `yaml:"prometheus"`
}

type JaegerConfig struct {
	Endpoint    string `yaml:"endpoint"`
	ServiceName string `yaml:"service_name"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

type SecurityConfig struct {
	EnableAuth bool      `yaml:"enable_auth"`
	JWT        JWTConfig `yaml:"jwt"`
	CORS       CORSConfig `yaml:"cors"`
	TLS        TLSConfig `yaml:"tls"`
}

type JWTConfig struct {
	Secret string `yaml:"secret"`
	Expiry string `yaml:"expiry"`
	Issuer string `yaml:"issuer"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type FeaturesConfig struct {
	Replay             ReplayConfig     `yaml:"replay"`
	SystemMap          SystemMapConfig  `yaml:"system_map"`
	RealTimeProcessing ProcessingFeature `yaml:"real_time_processing"`
	BatchProcessing    ProcessingFeature `yaml:"batch_processing"`
	DataExport         DataExportConfig `yaml:"data_export"`
}

type ReplayConfig struct {
	Enabled               bool   `yaml:"enabled"`
	MaxConcurrentReplays  int    `yaml:"max_concurrent_replays"`
	ReplayTimeout         string `yaml:"replay_timeout"`
}

type SystemMapConfig struct {
	Enabled        bool   `yaml:"enabled"`
	UpdateInterval string `yaml:"update_interval"`
	MaxNodes       int    `yaml:"max_nodes"`
}

type ProcessingFeature struct {
	Enabled       bool   `yaml:"enabled"`
	BufferSize    int    `yaml:"buffer_size,omitempty"`
	BatchInterval string `yaml:"batch_interval,omitempty"`
}

type DataExportConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Formats       []string `yaml:"formats"`
	MaxExportSize string   `yaml:"max_export_size"`
}

type PerformanceConfig struct {
	EnableCompression       bool   `yaml:"enable_compression"`
	CompressionLevel        int    `yaml:"compression_level"`
	EnableConnectionPooling bool   `yaml:"enable_connection_pooling"`
	MaxIdleConnections      int    `yaml:"max_idle_connections"`
	ConnectionMaxLifetime   string `yaml:"connection_max_lifetime"`
	EnableQueryCache        bool   `yaml:"enable_query_cache"`
	QueryCacheSize          string `yaml:"query_cache_size"`
}

type DevelopmentConfig struct {
	EnableDebugEndpoints   bool `yaml:"enable_debug_endpoints"`
	EnableHotReload        bool `yaml:"enable_hot_reload"`
	MockExternalServices   bool `yaml:"mock_external_services"`
	SeedTestData           bool `yaml:"seed_test_data"`
	EnableQueryLogging     bool `yaml:"enable_query_logging"`
	EnableRequestLogging   bool `yaml:"enable_request_logging"`
}

type SwaggerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Load reads configuration from YAML file with environment variable overrides
func Load(configPath string) (*Config, error) {
	// Set default config path if not provided
	if configPath == "" {
		configPath = getDefaultConfigPath()
	}

	// Read YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	applyEnvOverrides(&config)

	return &config, nil
}

// getDefaultConfigPath returns the path to the default config file
func getDefaultConfigPath() string {
	// Get environment from ENV variable, default to "local"
	env := os.Getenv("ENV")
	if env == "" {
		env = "local"
	}

	// Try environment-specific config first, then fall back to example
	paths := []string{
		fmt.Sprintf("config/environments/%s.yaml", env),
		"config/app.yaml.example",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If neither exists, return the preferred path
	return fmt.Sprintf("config/environments/%s.yaml", env)
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(config *Config) {
	// Server overrides
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		config.Server.Mode = mode
	}
	if host := os.Getenv("HOST"); host != "" {
		config.Server.Host = host
	}

	// Logging overrides
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}

	// ClickHouse overrides
	if host := os.Getenv("CLICKHOUSE_HOST"); host != "" {
		config.Database.ClickHouse.Host = host
	}
	if db := os.Getenv("CLICKHOUSE_DB"); db != "" {
		config.Database.ClickHouse.Database = db
	}
	if user := os.Getenv("CLICKHOUSE_USER"); user != "" {
		config.Database.ClickHouse.Username = user
	}
	if password := os.Getenv("CLICKHOUSE_PASSWORD"); password != "" {
		config.Database.ClickHouse.Password = password
	}

	// Redis overrides
	if host := os.Getenv("REDIS_HOST"); host != "" {
		config.Database.Redis.Host = host
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		config.Database.Redis.Password = password
	}

	// Kinesis/AWS overrides
	if endpointURL := os.Getenv("AWS_ENDPOINT_URL"); endpointURL != "" {
		config.Streaming.Kinesis.EndpointURL = endpointURL
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		config.Streaming.Kinesis.Region = region
	}
	if accessKey := os.Getenv("AWS_ACCESS_KEY_ID"); accessKey != "" {
		config.Streaming.Kinesis.AccessKeyID = accessKey
	}
	if secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY"); secretKey != "" {
		config.Streaming.Kinesis.SecretAccessKey = secretKey
	}
}

// GetConfigDir returns the absolute path to the config directory
func GetConfigDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "config"), nil
}
