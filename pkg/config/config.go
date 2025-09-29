// Package config provides configuration management for the playback-backend application.
//
// This package contains both the new ConsolidatedConfig (preferred) and legacy Config structures.
// New code should use ConsolidatedConfig which provides a clean, simplified configuration structure.
//
// The legacy Config structure and related imports from specific config packages
// (api, app, database, etc.) are maintained for backward compatibility only.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jamesneb/playback-backend/pkg/config/api"
	"github.com/jamesneb/playback-backend/pkg/config/app"
	"github.com/jamesneb/playback-backend/pkg/config/database"
	"github.com/jamesneb/playback-backend/pkg/config/processing"
	"github.com/jamesneb/playback-backend/pkg/config/security"
	"github.com/jamesneb/playback-backend/pkg/config/server"
	"github.com/jamesneb/playback-backend/pkg/config/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
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
	ErrConfigPathEmpty      = fmt.Errorf("config path cannot be empty")
	ErrConfigPathTooLong    = fmt.Errorf("config path exceeds maximum length (%d)", MaxConfigPathLength)
	ErrConfigPathTraversal  = fmt.Errorf("config path contains directory traversal patterns")
	ErrConfigPathInvalidExt = fmt.Errorf("config file must have .yaml or .yml extension")
	ErrConfigPathNotExists  = fmt.Errorf("config file does not exist")
	ErrConfigPathNotRegular = fmt.Errorf("config path does not point to a regular file")
)

// Config represents the complete application configuration
// This maintains backward compatibility while supporting the new consolidated structure
type Config struct {
	// New consolidated structure (preferred)
	*ConsolidatedConfig `yaml:",inline"`

	// Legacy structure for backward compatibility (deprecated)
	// Application settings
	App     app.AppConfig     `yaml:"app,omitempty"`
	Logging app.LoggingConfig `yaml:"logging,omitempty"`

	// Server and API
	Server server.ServerConfig `yaml:"server,omitempty"`
	API    api.APIConfig       `yaml:"api,omitempty"`

	// Data layer
	Database database.DatabaseConfig `yaml:"database,omitempty"`
	Cache    database.CacheConfig    `yaml:"cache,omitempty"`

	// Streaming and messaging
	Streaming streaming.StreamingConfig `yaml:"streaming,omitempty"`

	// Processing and features
	Processing processing.ProcessingConfig `yaml:"processing,omitempty"`
	Retention  processing.RetentionConfig  `yaml:"retention,omitempty"`
	Features   processing.FeaturesConfig   `yaml:"features,omitempty"`

	// Security and monitoring
	Security   security.SecurityConfig   `yaml:"security,omitempty"`
	Monitoring security.MonitoringConfig `yaml:"monitoring,omitempty"`

	// Resilience (backward compatibility)
	Resilience streaming.ResilienceConfig `yaml:"resilience,omitempty"`

	// Development settings
	Development app.DevelopmentConfig `yaml:"development,omitempty"`

	// Performance settings
	Performance api.PerformanceConfig `yaml:"performance,omitempty"`

	// Swagger documentation
	Swagger api.SwaggerConfig `yaml:"swagger,omitempty"`
}

// LoadConfig loads configuration from the specified file path with validation
// Supports both consolidated and legacy configuration formats
func LoadConfig(configPath string) (*Config, error) {
	// Try loading with new consolidated structure first
	consolidatedConfig, err := LoadConsolidatedConfig(configPath)
	if err == nil {
		// Successfully loaded consolidated format
		return &Config{
			ConsolidatedConfig: consolidatedConfig,
		}, nil
	}

	// Fallback to legacy format loading
	return loadLegacyConfig(configPath)
}

// loadLegacyConfig loads the legacy configuration format
func loadLegacyConfig(configPath string) (*Config, error) {
	// Validate file path for security
	normalizedPath, err := validateConfigPath(configPath)
	if err != nil {
		return nil, fmt.Errorf("config path validation failed: %w", err)
	}

	if err := validateFileSystemSafety(normalizedPath); err != nil {
		return nil, fmt.Errorf("config file system validation failed: %w", err)
	}

	// Read and parse configuration file
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	if err := applyEnvironmentOverrides(&config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate the loaded configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// validateConfigPath performs path validation to prevent directory traversal attacks
func validateConfigPath(configPath string) (string, error) {
	// Basic validation
	if configPath == "" {
		return "", ErrConfigPathEmpty
	}

	if len(configPath) > MaxConfigPathLength {
		return "", ErrConfigPathTooLong
	}

	// Check for dangerous path patterns
	if strings.Contains(configPath, parentDirPattern) {
		return "", ErrConfigPathTraversal
	}

	// Normalize the path
	normalizedPath := filepath.Clean(configPath)

	// Verify file extension
	ext := strings.ToLower(filepath.Ext(normalizedPath))
	if ext != configExtensionYAML && ext != configExtensionYML {
		return "", ErrConfigPathInvalidExt
	}

	return normalizedPath, nil
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
			logger.Warn("Failed to close config file during validation",
				zap.String("config_path", configPath),
				zap.Error(closeErr))
		}
	}()

	return nil
}

// applyEnvironmentOverrides applies essential environment variable overrides to configuration
func applyEnvironmentOverrides(config *Config) error {
	// Only apply critical environment overrides that make sense for production deployment

	// Log level override (common for debugging)
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		config.Logging.Level = logLevel
	}

	// Server port override (common in containerized deployments)
	if port := os.Getenv("PORT"); port != "" {
		config.Server.Port = parseIntEnv(port, config.Server.Port)
	}

	// Environment override for deployment context
	if env := os.Getenv("APP_ENV"); env != "" {
		config.App.Environment = env
	}

	return nil
}

// validateConfig performs basic configuration validation
func validateConfig(config *Config) error {
	// Only validate critical fields that could cause runtime issues

	// Validate server port range if set
	if config.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535")
	}

	// Allow empty configurations for testing purposes
	return nil
}

// Helper function to parse integer environment variables with fallback
func parseIntEnv(value string, fallback int) int {
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return fallback
}

// getDefaultConfigPath returns the default configuration file path based on environment
func getDefaultConfigPath() string {
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	// Get config directory
	configDir, err := GetConfigDir()
	if err != nil {
		// Fallback to relative path if config directory cannot be determined
		return fmt.Sprintf("config/%s.yaml", env)
	}

	return filepath.Join(configDir, fmt.Sprintf("%s.yaml", env))
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() (string, error) {
	// Try to get current working directory
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Look for config directory relative to working directory
	configDir := filepath.Join(wd, "config")

	// Check if config directory exists
	if _, err := os.Stat(configDir); err != nil {
		// Try parent directory structure commonly used in Go projects
		parentConfigDir := filepath.Join(filepath.Dir(wd), "config")
		if _, err := os.Stat(parentConfigDir); err == nil {
			return parentConfigDir, nil
		}

		// Return the original path even if it doesn't exist
		// The caller can decide what to do with it
		return configDir, nil
	}

	return configDir, nil
}

// applyEnvOverrides is an alias for applyEnvironmentOverrides for backward compatibility
func applyEnvOverrides(config *Config) error {
	return applyEnvironmentOverrides(config)
}

// Load is an alias for LoadConfig for backward compatibility
func Load(configPath string) (*Config, error) {
	return LoadConfig(configPath)
}
