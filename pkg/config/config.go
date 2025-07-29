package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Logging       LoggingConfig       `yaml:"logging"`
	Swagger       SwaggerConfig       `yaml:"swagger"`
	Database      DatabaseConfig      `yaml:"database"`
	Observability ObservabilityConfig `yaml:"observability"`
}

type ServerConfig struct {
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	Mode           string   `yaml:"mode"` // debug, release, test
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type SwaggerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Title       string `yaml:"title"`
	Version     string `yaml:"version"`
	Description string `yaml:"description"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Name     string `yaml:"name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"ssl_mode"`
}

type ObservabilityConfig struct {
	Jaeger     JaegerConfig     `yaml:"jaeger"`
	Prometheus PrometheusConfig `yaml:"prometheus"`
}

type JaegerConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type PrometheusConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
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
	// Try configs/app.yaml first, then app.yaml.example
	paths := []string{
		"configs/app.yaml",
		"configs/app.yaml.example",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If neither exists, return the preferred path
	return "configs/app.yaml"
}

// applyEnvOverrides applies environment variable overrides to config
func applyEnvOverrides(config *Config) {
	// Server overrides
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		config.Server.Mode = mode
	}
	if port := os.Getenv("PORT"); port != "" {
		// You could parse this to int, but keeping it simple for now
	}
	if host := os.Getenv("HOST"); host != "" {
		config.Server.Host = host
	}

	// Logging overrides
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}

	// Database overrides
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		config.Database.Name = dbName
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		config.Database.User = dbUser
	}
	if dbPassword := os.Getenv("DB_PASSWORD"); dbPassword != "" {
		config.Database.Password = dbPassword
	}
}

// GetConfigDir returns the absolute path to the configs directory
func GetConfigDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(wd, "configs"), nil
}