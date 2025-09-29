package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

// LoadConsolidatedConfig loads the new streamlined configuration
func LoadConsolidatedConfig(configPath string) (*ConsolidatedConfig, error) {
	// Validate file path for security (reuse existing validation)
	normalizedPath, err := validateConfigPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("config path validation failed: %w", err)
	}

	if err := validateFileSystemSafety(normalizedPath); err != nil {
		return nil, fmt.Errorf("config file system validation failed: %w", err)
	}

	// Start with defaults
	config := DefaultConfig()

	// If file doesn't exist, use defaults with environment overrides
	if _, err := os.Stat(normalizedPath); os.IsNotExist(err) {
		logger.Info("Config file not found, using defaults with environment overrides",
			zap.String("config_path", normalizedPath))
		applyConsolidatedEnvironmentOverrides(config)
		return config, nil
	}

	// Load from file
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Apply environment variable overrides
	applyConsolidatedEnvironmentOverrides(config)

	// Validate the loaded configuration
	if err := validateConsolidatedConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	logger.Info("Configuration loaded successfully",
		zap.String("config_path", normalizedPath),
		zap.String("environment", config.App.Environment))

	return config, nil
}

// applyConsolidatedEnvironmentOverrides applies environment variable overrides to consolidated config
func applyConsolidatedEnvironmentOverrides(config *ConsolidatedConfig) {
	// Environment variable prefix
	const envPrefix = "PLAYBACK_"

	// Apply common environment overrides using reflection for maintainability
	envOverrides := map[string]interface{}{
		// App settings
		"APP_NAME":        &config.App.Name,
		"APP_VERSION":     &config.App.Version,
		"APP_ENVIRONMENT": &config.App.Environment,
		"APP_LOG_LEVEL":   &config.App.LogLevel,

		// Network settings
		"HTTP_HOST":         &config.Network.HTTP.Host,
		"HTTP_PORT":         &config.Network.HTTP.Port,
		"HTTP_MODE":         &config.Network.HTTP.Mode,
		"GRPC_PORT":         &config.Network.GRPC.Port,
		"API_PREFIX":        &config.Network.HTTP.APIPrefix,
		"JWT_SECRET":        &config.Network.HTTP.JWTSecret,

		// Data settings
		"CLICKHOUSE_HOST":     &config.Data.ClickHouse.Host,
		"CLICKHOUSE_HTTP_HOST": &config.Data.ClickHouse.HTTPHost,
		"CLICKHOUSE_DATABASE": &config.Data.ClickHouse.Database,
		"CLICKHOUSE_USERNAME": &config.Data.ClickHouse.Username,
		"CLICKHOUSE_PASSWORD": &config.Data.ClickHouse.Password,

		"REDIS_HOST":     &config.Data.Redis.Host,
		"REDIS_PASSWORD": &config.Data.Redis.Password,
		"REDIS_DATABASE": &config.Data.Redis.Database,

		"S3_REGION":            &config.Data.S3.Region,
		"S3_BUCKET":            &config.Data.S3.Bucket,
		"S3_ENDPOINT_URL":      &config.Data.S3.EndpointURL,
		"S3_ACCESS_KEY_ID":     &config.Data.S3.AccessKeyID,
		"S3_SECRET_ACCESS_KEY": &config.Data.S3.SecretAccessKey,

		"KINESIS_REGION":            &config.Data.Kinesis.Region,
		"KINESIS_ENDPOINT_URL":      &config.Data.Kinesis.EndpointURL,
		"KINESIS_ACCESS_KEY_ID":     &config.Data.Kinesis.AccessKeyID,
		"KINESIS_SECRET_ACCESS_KEY": &config.Data.Kinesis.SecretAccessKey,
		"KINESIS_TRACES_STREAM":     &config.Data.Kinesis.TracesStream,
		"KINESIS_METRICS_STREAM":    &config.Data.Kinesis.MetricsStream,
		"KINESIS_LOGS_STREAM":       &config.Data.Kinesis.LogsStream,

		// Operations settings
		"ENABLE_METRICS":       &config.Operations.EnableMetrics,
		"METRICS_PORT":         &config.Operations.MetricsPort,
		"METRICS_PATH":         &config.Operations.MetricsPath,
		"ENABLE_TRACING":       &config.Operations.EnableTracing,
		"TRACING_ENDPOINT":     &config.Operations.TracingEndpoint,
		"IS_DEVELOPMENT":       &config.Operations.IsDevelopment,
		"ENABLE_QUERY_LOGGING": &config.Operations.EnableQueryLogging,
	}

	for envKey, configField := range envOverrides {
		fullEnvKey := envPrefix + envKey
		if envValue := os.Getenv(fullEnvKey); envValue != "" {
			setConfigValue(configField, envValue)
		}
	}
}

// setConfigValue sets a configuration value using reflection with type safety
func setConfigValue(field interface{}, value string) {
	rv := reflect.ValueOf(field).Elem()
	switch rv.Kind() {
	case reflect.String:
		rv.SetString(value)
	case reflect.Int:
		if intVal, err := strconv.Atoi(value); err == nil {
			rv.SetInt(int64(intVal))
		}
	case reflect.Bool:
		if boolVal, err := strconv.ParseBool(value); err == nil {
			rv.SetBool(boolVal)
		}
	case reflect.Float64:
		if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
			rv.SetFloat(floatVal)
		}
	}
}

// validateConsolidatedConfig validates the consolidated configuration
func validateConsolidatedConfig(config *ConsolidatedConfig) error {
	// Basic validation
	if config.App.Name == "" {
		return fmt.Errorf("app.name cannot be empty")
	}

	if config.Network.HTTP.Port <= 0 || config.Network.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", config.Network.HTTP.Port)
	}

	if config.Network.GRPC.Port <= 0 || config.Network.GRPC.Port > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", config.Network.GRPC.Port)
	}

	if config.Network.HTTP.Port == config.Network.GRPC.Port {
		return fmt.Errorf("HTTP and gRPC ports cannot be the same: %d", config.Network.HTTP.Port)
	}

	// Validate log level
	validLogLevels := []string{"debug", "info", "warn", "error", "fatal"}
	if !contains(validLogLevels, config.App.LogLevel) {
		return fmt.Errorf("invalid log level: %s", config.App.LogLevel)
	}

	// Validate gin mode
	validGinModes := []string{"debug", "release", "test"}
	if !contains(validGinModes, config.Network.HTTP.Mode) {
		return fmt.Errorf("invalid gin mode: %s", config.Network.HTTP.Mode)
	}

	// Data validation
	if config.Data.ClickHouse.Host == "" {
		return fmt.Errorf("ClickHouse host cannot be empty")
	}

	if config.Data.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive: %d", config.Data.BatchSize)
	}

	if config.Data.WorkerCount <= 0 {
		return fmt.Errorf("worker count must be positive: %d", config.Data.WorkerCount)
	}

	// Timeouts must be positive
	if config.Network.HTTP.ReadTimeout <= 0 {
		return fmt.Errorf("read timeout must be positive")
	}

	if config.Network.HTTP.WriteTimeout <= 0 {
		return fmt.Errorf("write timeout must be positive")
	}

	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// CreateSimpleYAML creates a simplified YAML configuration file
func CreateSimpleYAML(filePath string, config *ConsolidatedConfig) error {
	// Validate output path
	normalizedPath, err := validateConfigPath(filePath)
	if err != nil {
		return fmt.Errorf("output path validation failed: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(normalizedPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(normalizedPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logger.Info("Simplified configuration file created",
		zap.String("path", normalizedPath))

	return nil
}