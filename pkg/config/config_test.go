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

server:
  host: "localhost"
  port: 8080
  mode: "release"

logging:
  level: "info"
  format: "json"

database:
  clickhouse:
    host: "localhost:9000"
    database: "telemetry"
    username: "default"
    password: "password123"
  redis:
    host: "localhost:6379"
    password: "redis123"
    database: 0

streaming:
  provider: "kinesis"
  kinesis:
    region: "us-east-1"
    streams:
      traces: "traces-stream"
      metrics: "metrics-stream"
      logs: "logs-stream"
    batch_size: 100
    flush_interval: "5s"
    max_retries: 3
    retry_delay: "1s"
`,
			expectedError: false,
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "test-service", cfg.App.Name)
				assert.Equal(t, "1.0.0", cfg.App.Version)
				assert.Equal(t, "localhost", cfg.Server.Host)
				assert.Equal(t, 8080, cfg.Server.Port)
				assert.Equal(t, "localhost:9000", cfg.Database.ClickHouse.Host)
				assert.Equal(t, "telemetry", cfg.Database.ClickHouse.Database)
				assert.Equal(t, "us-east-1", cfg.Streaming.Kinesis.Region)
				assert.Equal(t, "traces-stream", cfg.Streaming.Kinesis.Streams["traces"])
				assert.Equal(t, "info", cfg.Logging.Level)
			},
		},
		{
			name: "environment variable overrides",
			configContent: `
app:
  name: "test-service"

server:
  host: "localhost"
  port: 8080

database:
  clickhouse:
    host: "localhost:9000"
    database: "telemetry"
    password: "config_password"

streaming:
  kinesis:
    region: "us-west-2"
`,
			envVars: map[string]string{
				"CLICKHOUSE_HOST":     "override-host:9000",
				"CLICKHOUSE_PASSWORD": "env_password",
				"AWS_DEFAULT_REGION":  "eu-west-1",
				"LOG_LEVEL":           "debug",
			},
			expectedError: false,
			validateFunc: func(t *testing.T, cfg *Config) {
				// Verify environment overrides
				assert.Equal(t, "override-host:9000", cfg.Database.ClickHouse.Host)
				assert.Equal(t, "env_password", cfg.Database.ClickHouse.Password)
				assert.Equal(t, "eu-west-1", cfg.Streaming.Kinesis.Region)
				assert.Equal(t, "debug", cfg.Logging.Level)

				// Verify non-overridden values remain
				assert.Equal(t, "localhost", cfg.Server.Host)
				assert.Equal(t, "telemetry", cfg.Database.ClickHouse.Database)
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
			name: "server overrides",
			baseConfig: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
					Mode: "debug",
				},
			},
			envVars: map[string]string{
				"HOST":     "0.0.0.0",
				"GIN_MODE": "release",
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "0.0.0.0", cfg.Server.Host)
				assert.Equal(t, "release", cfg.Server.Mode)
				assert.Equal(t, 8080, cfg.Server.Port) // Unchanged
			},
		},
		{
			name: "database overrides",
			baseConfig: &Config{
				Database: DatabaseConfig{
					ClickHouse: ClickHouseConfig{
						Host:     "localhost:9000",
						Database: "telemetry",
						Username: "default",
						Password: "password",
					},
					Redis: RedisConfig{
						Host:     "localhost:6379",
						Password: "password",
						Database: 0,
					},
				},
			},
			envVars: map[string]string{
				"CLICKHOUSE_HOST":     "prod-ch:9000",
				"CLICKHOUSE_DB":       "production",
				"CLICKHOUSE_USER":     "admin",
				"CLICKHOUSE_PASSWORD": "secret123",
				"REDIS_HOST":          "redis-cluster:6379",
				"REDIS_PASSWORD":      "newpassword",
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "prod-ch:9000", cfg.Database.ClickHouse.Host)
				assert.Equal(t, "production", cfg.Database.ClickHouse.Database)
				assert.Equal(t, "admin", cfg.Database.ClickHouse.Username)
				assert.Equal(t, "secret123", cfg.Database.ClickHouse.Password)
				assert.Equal(t, "redis-cluster:6379", cfg.Database.Redis.Host)
				assert.Equal(t, "newpassword", cfg.Database.Redis.Password)
			},
		},
		{
			name: "streaming overrides",
			baseConfig: &Config{
				Streaming: StreamingConfig{
					Kinesis: KinesisConfig{
						Region:          "us-east-1",
						AccessKeyID:     "key",
						SecretAccessKey: "secret",
					},
				},
			},
			envVars: map[string]string{
				"AWS_DEFAULT_REGION":    "eu-west-1",
				"AWS_ACCESS_KEY_ID":     "new-key",
				"AWS_SECRET_ACCESS_KEY": "new-secret",
				"AWS_ENDPOINT_URL":      "http://localhost:4566",
			},
			validateFunc: func(t *testing.T, cfg *Config) {
				assert.Equal(t, "eu-west-1", cfg.Streaming.Kinesis.Region)
				assert.Equal(t, "new-key", cfg.Streaming.Kinesis.AccessKeyID)
				assert.Equal(t, "new-secret", cfg.Streaming.Kinesis.SecretAccessKey)
				assert.Equal(t, "http://localhost:4566", cfg.Streaming.Kinesis.EndpointURL)
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
			applyEnvOverrides(tt.baseConfig)

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

server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"

database:
  clickhouse:
    host: "clickhouse:9000"
    database: "telemetry"
    username: "admin"
    password: "admin123"
  redis:
    host: "redis:6379"
    password: "redis123"
    database: 0

streaming:
  provider: "kinesis"
  kinesis:
    region: "us-east-1"
    streams:
      traces: "telemetry-traces"
      metrics: "telemetry-metrics"
      logs: "telemetry-logs"
    batch_size: 100
    flush_interval: "5s"
    max_retries: 3
    retry_delay: "1s"

logging:
  level: "info"
  format: "json"
`

	configFile := filepath.Join(tempDir, "integration_config.yaml")
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment overrides
	if err := os.Setenv("CLICKHOUSE_PASSWORD", "env_override_password"); err != nil {
		t.Fatalf("Failed to set CLICKHOUSE_PASSWORD: %v", err)
	}
	if err := os.Setenv("LOG_LEVEL", "debug"); err != nil {
		t.Fatalf("Failed to set LOG_LEVEL: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("CLICKHOUSE_PASSWORD"); err != nil {
			t.Errorf("Failed to unset CLICKHOUSE_PASSWORD: %v", err)
		}
		if err := os.Unsetenv("LOG_LEVEL"); err != nil {
			t.Errorf("Failed to unset LOG_LEVEL: %v", err)
		}
	}()

	// Load configuration
	config, err := Load(configFile)
	require.NoError(t, err)
	require.NotNil(t, config)

	// Verify configuration values
	assert.Equal(t, "0.0.0.0", config.Server.Host)
	assert.Equal(t, 8080, config.Server.Port)

	assert.Equal(t, "clickhouse:9000", config.Database.ClickHouse.Host)
	assert.Equal(t, "telemetry", config.Database.ClickHouse.Database)
	assert.Equal(t, "env_override_password", config.Database.ClickHouse.Password) // Overridden by env

	assert.Equal(t, "us-east-1", config.Streaming.Kinesis.Region)
	assert.Equal(t, "telemetry-traces", config.Streaming.Kinesis.Streams["traces"])

	assert.Equal(t, "redis:6379", config.Database.Redis.Host)
	assert.Equal(t, "debug", config.Logging.Level) // Overridden by env
}
