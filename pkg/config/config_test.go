package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create a temporary directory for test configs
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		configContent string
		configPath    string
		envVars       map[string]string
		expectedError bool
		validateFunc  func(*testing.T, *Config)
	}{
		{
			name: "valid basic configuration",
			configContent: `
app:
  name: "test-service"
  version: "1.0.0"
  environment: "test"
  log_level: "info"
  log_format: "json"

network:
  http:
    host: "localhost"
    port: 8080
    mode: "release"

data:
  clickhouse:
    host: "localhost:9000"
    database: "telemetry"
    username: "default"
    password: "password123"
  redis:
    host: "localhost:6379"
    password: "redis123"
    database: 0
  kinesis:
    region: "us-east-1"
    traces_stream: "traces-stream"
    metrics_stream: "metrics-stream"
    logs_stream: "logs-stream"
`,
			expectedError: false,
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-service", cfg.ConsolidatedConfig.App.Name)
				assert.Equal(t, "1.0.0", cfg.ConsolidatedConfig.App.Version)
				assert.Equal(t, "localhost", cfg.Network.HTTP.Host)
				assert.Equal(t, 8080, cfg.Network.HTTP.Port)
				assert.Equal(t, "localhost:9000", cfg.Data.ClickHouse.Host)
				assert.Equal(t, "telemetry", cfg.Data.ClickHouse.Database)
				assert.Equal(t, "us-east-1", cfg.Data.Kinesis.Region)
				assert.Equal(t, "traces-stream", cfg.Data.Kinesis.TracesStream)
				assert.Equal(t, "info", cfg.ConsolidatedConfig.App.LogLevel)
			},
		},
		{
			name: "log level environment override",
			configContent: `
app:
  name: "test-service"
  log_level: "info"

network:
  http:
    host: "localhost"
    port: 8080

data:
  clickhouse:
    host: "localhost:9000"
    database: "telemetry"
    password: "config_password"
  kinesis:
    region: "us-west-2"
`,
			envVars: map[string]string{
				"PLAYBACK_APP_LOG_LEVEL": "debug",
			},
			expectedError: false,
			validateFunc: func(t *testing.T, cfg *Config) {
				// Verify only log level override works
				assert.Equal(t, "debug", cfg.ConsolidatedConfig.App.LogLevel)

				// Verify all other values remain from config file
				assert.Equal(t, "localhost", cfg.Network.HTTP.Host)
				assert.Equal(t, "telemetry", cfg.Data.ClickHouse.Database)
				assert.Equal(t, "localhost:9000", cfg.Data.ClickHouse.Host)
				assert.Equal(t, "config_password", cfg.Data.ClickHouse.Password)
				assert.Equal(t, "us-west-2", cfg.Data.Kinesis.Region)
			},
		},
		{
			name:          "invalid YAML syntax",
			configContent: `invalid: yaml: content: [unclosed`,
			expectedError: true,
		},
		{
			name:          "nonexistent config file",
			configPath:    "/nonexistent/path/config.yaml",
			expectedError: true,
		},
		{
			name:          "empty configuration file",
			configContent: `{}`, // Valid empty YAML
			expectedError: false,
			validateFunc: func(t *testing.T, cfg *Config) {
				// Should apply defaults for required fields
				assert.NotNil(t, cfg)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			originalEnv := make(map[string]string)
			for key, value := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", key, err)
				}
			}
			defer func() {
				// Restore original environment
				for key, originalValue := range originalEnv {
					if originalValue == "" {
						if err := os.Unsetenv(key); err != nil {
							t.Errorf("Failed to unset environment variable %s: %v", key, err)
						}
					} else {
						if err := os.Setenv(key, originalValue); err != nil {
							t.Errorf("Failed to restore environment variable %s: %v", key, err)
						}
					}
				}
			}()

			var configPath string
			if tt.configPath != "" {
				configPath = tt.configPath
			} else if tt.configContent != "" {
				// Create temporary config file
				configFile := filepath.Join(tempDir, "test_config.yaml")
				err := os.WriteFile(configFile, []byte(tt.configContent), 0644)
				require.NoError(t, err)
				configPath = configFile
			}

			// Test configuration loading
			config, err := Load(configPath)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)

				if tt.validateFunc != nil {
					tt.validateFunc(t, config)
				}
			}
		})
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	tests := []struct {
		name        string
		envVar      string
		expectError bool
	}{
		{
			name:        "default path when no ENV set",
			envVar:      "", // Will use default
			expectError: false,
		},
		{
			name:        "custom environment",
			envVar:      "production",
			expectError: false,
		},
		{
			name:        "dev environment",
			envVar:      "dev",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			originalEnv := os.Getenv("ENV")
			if tt.envVar != "" {
				if err := os.Setenv("ENV", tt.envVar); err != nil {
					t.Fatalf("Failed to set ENV variable: %v", err)
				}
			} else {
				if err := os.Unsetenv("ENV"); err != nil {
					t.Fatalf("Failed to unset ENV variable: %v", err)
				}
			}
			defer func() {
				if originalEnv == "" {
					if err := os.Unsetenv("ENV"); err != nil {
						t.Errorf("Failed to restore ENV variable (unset): %v", err)
					}
				} else {
					if err := os.Setenv("ENV", originalEnv); err != nil {
						t.Errorf("Failed to restore ENV variable: %v", err)
					}
				}
			}()

			path := getDefaultConfigPath()

			if tt.expectError {
				assert.Empty(t, path)
			} else {
				assert.NotEmpty(t, path)
				assert.Contains(t, path, ".yaml")

				if tt.envVar != "" {
					assert.Contains(t, path, tt.envVar)
				}
			}
		})
	}
}

func TestGetConfigDir(t *testing.T) {
	dir, err := GetConfigDir()
	assert.NoError(t, err)
	assert.NotEmpty(t, dir)
	assert.Contains(t, dir, "config")
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name         string
		baseConfig   *Config
		envVars      map[string]string
		validateFunc func(*testing.T, *Config)
	}{
		{
			name: "log level override only",
			baseConfig: &Config{
				App: AppConfig{
					Environment: "test",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
			},
			envVars: map[string]string{
				"LOG_LEVEL": "debug",
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "debug", cfg.Logging.Level)
				// Verify other values unchanged
				assert.Equal(t, "localhost", cfg.Server.Host)
				assert.Equal(t, 8080, cfg.Server.Port)
				assert.Equal(t, "test", cfg.App.Environment)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			originalEnv := make(map[string]string)
			for key, value := range tt.envVars {
				originalEnv[key] = os.Getenv(key)
				if err := os.Setenv(key, value); err != nil {
					t.Fatalf("Failed to set environment variable %s: %v", key, err)
				}
			}
			defer func() {
				// Restore original environment
				for key, originalValue := range originalEnv {
					if originalValue == "" {
						if err := os.Unsetenv(key); err != nil {
							t.Errorf("Failed to unset environment variable %s: %v", key, err)
						}
					} else {
						if err := os.Setenv(key, originalValue); err != nil {
							t.Errorf("Failed to restore environment variable %s: %v", key, err)
						}
					}
				}
			}()

			// Apply environment overrides
			_ = applyEnvOverrides(tt.baseConfig)

			// Validate results
			if tt.validateFunc != nil {
				tt.validateFunc(t, tt.baseConfig)
			}
		})
	}
}

// Integration test that loads a real configuration file
func TestLoadIntegration(t *testing.T) {
	// Create a realistic configuration file
	tempDir := t.TempDir()
	configContent := `
app:
  name: "playback-backend"
  version: "1.0.0"
  environment: "test"
  log_level: "info"
  log_format: "json"

network:
  http:
    host: "0.0.0.0"
    port: 8080
    mode: "release"

data:
  clickhouse:
    host: "clickhouse:9000"
    database: "telemetry"
    username: "admin"
    password: "admin123"
  redis:
    host: "redis:6379"
    password: "redis123"
    database: 0
  kinesis:
    region: "us-east-1"
    traces_stream: "telemetry-traces"
    metrics_stream: "telemetry-metrics"
    logs_stream: "telemetry-logs"
`

	configFile := filepath.Join(tempDir, "integration_config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment overrides
	if err := os.Setenv("PLAYBACK_APP_LOG_LEVEL", "debug"); err != nil {
		t.Fatalf("Failed to set PLAYBACK_APP_LOG_LEVEL: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("PLAYBACK_APP_LOG_LEVEL"); err != nil {
			t.Errorf("Failed to unset PLAYBACK_APP_LOG_LEVEL: %v", err)
		}
	}()

	// Load configuration
	config, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify configuration values using consolidated config structure
	assert.Equal(t, "0.0.0.0", config.Network.HTTP.Host)
	assert.Equal(t, 8080, config.Network.HTTP.Port)

	assert.Equal(t, "clickhouse:9000", config.Data.ClickHouse.Host)
	assert.Equal(t, "telemetry", config.Data.ClickHouse.Database)
	assert.Equal(t, "admin123", config.Data.ClickHouse.Password) // From config file

	assert.Equal(t, "us-east-1", config.Data.Kinesis.Region)
	assert.Equal(t, "telemetry-traces", config.Data.Kinesis.TracesStream)

	assert.Equal(t, "redis:6379", config.Data.Redis.Host)
	assert.Equal(t, "debug", config.ConsolidatedConfig.App.LogLevel) // Overridden by env
}
