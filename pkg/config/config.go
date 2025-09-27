package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// Configuration security constants
const (
	// MaxConfigPathLength defines maximum allowed config path length
	MaxConfigPathLength = 256

	// AllowedConfigExtensions defines valid config file extensions
	configExtensionYAML = ".yaml"
	configExtensionYML  = ".yml"

	// ConfigPathSeparator is the standard path separator
	configPathSeparator = string(filepath.Separator)

	// DangerousPathPatterns that indicate potential traversal attacks
	parentDirPattern      = ".."
	currentDirPattern     = "."
	absolutePathIndicator = configPathSeparator
)

// Configuration path validation errors
var (
	ErrConfigPathEmpty         = fmt.Errorf("config path cannot be empty")
	ErrConfigPathTooLong       = fmt.Errorf("config path exceeds maximum length (%d)", MaxConfigPathLength)
	ErrConfigPathTraversal     = fmt.Errorf("config path contains directory traversal patterns")
	ErrConfigPathInvalidExt    = fmt.Errorf("config file must have .yaml or .yml extension")
	ErrConfigPathNotExists     = fmt.Errorf("config file does not exist")
	ErrConfigPathNotRegular    = fmt.Errorf("config path does not point to a regular file")
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
	Resilience  ResilienceConfig  `yaml:"resilience"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
	Environment string `yaml:"environment"`
}

type ServerConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	GRPCPort        int           `yaml:"grpc_port"`
	Mode            string        `yaml:"mode"`
	TrustedProxies  []string      `yaml:"trusted_proxies"`
	ReadTimeout     string        `yaml:"read_timeout"`
	WriteTimeout    string        `yaml:"write_timeout"`
	IdleTimeout     string        `yaml:"idle_timeout"`
	ShutdownTimeout string        `yaml:"shutdown_timeout"`
	MaxHeaderBytes  int           `yaml:"max_header_bytes"`
	// Parsed durations for performance
	ReadTimeoutDuration     time.Duration `yaml:"-"`
	WriteTimeoutDuration    time.Duration `yaml:"-"`
	IdleTimeoutDuration     time.Duration `yaml:"-"`
	ShutdownTimeoutDuration time.Duration `yaml:"-"`
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
	S3       S3Config      `yaml:"s3"`
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
	PollInterval		int								`yaml:"poll_interval"`
}

type S3Config struct {
	Region          string `yaml:"region"`
	EndpointURL     string `yaml:"endpoint_url,omitempty"`
	AccessKeyID     string `yaml:"access_key_id,omitempty"`
	SecretAccessKey string `yaml:"secret_access_key,omitempty"`
	Bucket          string `yaml:"bucket"`
	ForcePathStyle  bool   `yaml:"force_path_style,omitempty"`
}

type ResilienceConfig struct {
	RateLimiter    RateLimiterConfig    `yaml:"rate_limiter"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
	DeadLetterQueue DLQConfig           `yaml:"dead_letter_queue"`
	KinesisBuffer  BufferConfig        `yaml:"kinesis_buffer"`
}

type RateLimiterConfig struct {
	RequestsPerSecond int `yaml:"requests_per_second"`
	BurstCapacity     int `yaml:"burst_capacity"`
}

type CircuitBreakerConfig struct {
	Name            string  `yaml:"name"`
	MaxRequests     uint32  `yaml:"max_requests"`
	IntervalSeconds int     `yaml:"interval_seconds"`
	TimeoutSeconds  int     `yaml:"timeout_seconds"`
	MinRequests     uint32  `yaml:"min_requests"`
	FailureRate     float64 `yaml:"failure_rate"`
}

type DLQConfig struct {
	QueueURL         string `yaml:"queue_url"`         // Full SQS URL (takes precedence if provided)
	AccountID        string `yaml:"account_id"`        // AWS Account ID (for backward compatibility)
	QueueName        string `yaml:"queue_name"`        // Queue name (for backward compatibility)
	MaxRetries       int    `yaml:"max_retries"`
	RetryBaseDelayMs int    `yaml:"retry_base_delay_ms"`
	RetryMaxDelayMs  int    `yaml:"retry_max_delay_ms"`
}

type BufferConfig struct {
	MaxBatchSize      int `yaml:"max_batch_size"`
	MaxBatchWaitMs    int `yaml:"max_batch_wait_ms"`
	FlushIntervalMs   int `yaml:"flush_interval_ms"`
	MaxTenantBuffer   int `yaml:"max_tenant_buffer"`
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
// and comprehensive path validation to prevent directory traversal attacks.
func Load(configPath string) (*Config, error) {
	// Set default config path if not provided
	if configPath == "" {
		configPath = getDefaultConfigPath()
	}

	// Validate and sanitize the configuration file path
	validatedPath, err := validateConfigPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("invalid config path: %w", err)
	}

	// Read YAML file using validated path
	data, err := os.ReadFile(validatedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	applyEnvOverrides(&config)

	// Parse durations once for performance
	if err := parseDurations(&config); err != nil {
		return nil, fmt.Errorf("failed to parse durations: %w", err)
	}

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

// parseDurations parses string durations into time.Duration fields
func parseDurations(config *Config) error {
	var err error

	// Parse server timeouts
	config.Server.ReadTimeoutDuration, err = parseDurationWithFallback(config.Server.ReadTimeout, 30*time.Second)
	if err != nil {
		return fmt.Errorf("invalid read_timeout: %w", err)
	}

	config.Server.WriteTimeoutDuration, err = parseDurationWithFallback(config.Server.WriteTimeout, 30*time.Second)
	if err != nil {
		return fmt.Errorf("invalid write_timeout: %w", err)
	}

	config.Server.IdleTimeoutDuration, err = parseDurationWithFallback(config.Server.IdleTimeout, 60*time.Second)
	if err != nil {
		return fmt.Errorf("invalid idle_timeout: %w", err)
	}

	config.Server.ShutdownTimeoutDuration, err = parseDurationWithFallback(config.Server.ShutdownTimeout, 30*time.Second)
	if err != nil {
		return fmt.Errorf("invalid shutdown_timeout: %w", err)
	}

	return nil
}

// parseDurationWithFallback parses duration string with fallback
func parseDurationWithFallback(durationStr string, fallback time.Duration) (time.Duration, error) {
	if durationStr == "" {
		return fallback, nil
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return fallback, err
	}

	return duration, nil
}

// GetConfigDir returns the absolute path to the config directory
func GetConfigDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "config"), nil
}

// validateConfigPath performs comprehensive validation of configuration file paths
// to prevent directory traversal attacks and ensure file system security.
//
// This function implements multiple layers of security validation:
// 1. Basic path validation (length, emptiness)
// 2. Directory traversal pattern detection
// 3. File extension validation
// 4. File system validation (existence, type)
// 5. Path normalization and cleaning
//
// Parameters:
//   - configPath: Raw configuration file path to validate
//
// Returns:
//   - string: Validated and normalized absolute path
//   - error: Detailed validation error if any security check fails
func validateConfigPath(configPath string) (string, error) {
	// Primary validation: check for empty path
	if configPath == "" {
		return "", ErrConfigPathEmpty
	}

	// Length validation to prevent buffer overflow-style attacks
	if len(configPath) > MaxConfigPathLength {
		return "", ErrConfigPathTooLong
	}

	// Directory traversal pattern detection
	if err := validateNoTraversalPatterns(configPath); err != nil {
		return "", err
	}

	// File extension validation to ensure we're reading config files
	if err := validateConfigExtension(configPath); err != nil {
		return "", err
	}

	// Normalize path to prevent bypasses through path manipulation
	normalizedPath, err := normalizeConfigPath(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to normalize config path: %w", err)
	}

	// File system validation to ensure the file exists and is accessible
	if err := validateFileSystemSafety(normalizedPath); err != nil {
		return "", err
	}

	return normalizedPath, nil
}

// validateNoTraversalPatterns checks for directory traversal attack patterns
// in the configuration path to prevent unauthorized file system access.
func validateNoTraversalPatterns(configPath string) error {
	// Check for parent directory traversal
	if strings.Contains(configPath, parentDirPattern) {
		return ErrConfigPathTraversal
	}

	// Additional security: check for encoded traversal patterns
	encodedTraversalPatterns := []string{
		"%2e%2e",     // URL encoded ".."
		"..%2f",      // Mixed encoding
		"%2e%2e%2f",  // Full URL encoded "../"
		"..\\",       // Windows-style traversal
		"..\\/",      // Mixed separators
	}

	lowerPath := strings.ToLower(configPath)
	for _, pattern := range encodedTraversalPatterns {
		if strings.Contains(lowerPath, pattern) {
			return ErrConfigPathTraversal
		}
	}

	return nil
}

// validateConfigExtension ensures the file has a valid configuration file extension
// to prevent reading arbitrary system files.
func validateConfigExtension(configPath string) error {
	ext := strings.ToLower(filepath.Ext(configPath))

	validExtensions := []string{configExtensionYAML, configExtensionYML}
	for _, validExt := range validExtensions {
		if ext == validExt {
			return nil
		}
	}

	return ErrConfigPathInvalidExt
}

// normalizeConfigPath performs path normalization and cleaning to prevent
// path manipulation attacks and ensure consistent path handling.
func normalizeConfigPath(configPath string) (string, error) {
	// Clean the path to remove redundant separators and resolve . and .. elements
	cleanedPath := filepath.Clean(configPath)

	// Convert to absolute path for consistent validation
	// This also helps prevent relative path confusion
	absolutePath, err := filepath.Abs(cleanedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absolutePath, nil
}

// validateFileSystemSafety performs file system validation to ensure the target
// file exists, is a regular file, and is accessible for reading.
func validateFileSystemSafety(configPath string) error {
	// Check if file exists
	fileInfo, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrConfigPathNotExists
		}
		return fmt.Errorf("failed to stat config file: %w", err)
	}

	// Ensure it's a regular file (not a directory, symlink, device, etc.)
	if !fileInfo.Mode().IsRegular() {
		return ErrConfigPathNotRegular
	}

	// Additional security: check file permissions are readable
	// This provides early feedback if the file can't be read
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("config file exists but cannot be opened: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail validation since file was readable
			fmt.Printf("Warning: failed to close config file during validation: %v\n", closeErr)
		}
	}()

	return nil
}
