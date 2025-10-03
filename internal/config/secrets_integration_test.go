package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecretsIntegration(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{"production", "production", false},
		{"staging", "staging", false},
		{"local", "local", false},
		{"local-docker", "local-docker", true}, // Will fail due to read-only filesystem
		{"dev", "dev", false},
		{"unsupported", "unsupported", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si, err := NewSecretsIntegration(tt.env)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, si)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, si)
				if si != nil {
					assert.Equal(t, tt.env, si.env)
					assert.NotNil(t, si.manager)
				}
			}
		})
	}
}

func TestSecretsIntegration_ResolveSecretPlaceholder(t *testing.T) {
	// Skip if we don't have a secrets file for testing
	if _, err := os.Stat("./config/.secrets"); os.IsNotExist(err) {
		t.Skip("No secrets file for testing")
	}

	si, err := NewSecretsIntegration("local")
	require.NoError(t, err)

	tests := []struct {
		name     string
		value    string
		expected string
		wantErr  bool
	}{
		{"no placeholder", "plain-text", "plain-text", false},
		{"placeholder with fallback", "${NONEXISTENT_SECRET:-fallback}", "fallback", false},
		{"empty placeholder", "", "", false},
		{"malformed placeholder", "${INCOMPLETE", "${INCOMPLETE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := si.resolveSecretPlaceholder(ctx, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSecretsIntegration_GetRequiredSecretsForEnvironment(t *testing.T) {
	tests := []struct {
		name            string
		env             string
		expectedSecrets []string
	}{
		{
			"production",
			"production",
			[]string{"CLICKHOUSE_PASSWORD", "REDIS_PASSWORD", "JWT_SECRET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "TLS_CERT_FILE", "TLS_KEY_FILE", "CLICKHOUSE_USERNAME"},
		},
		{
			"staging",
			"staging",
			[]string{"CLICKHOUSE_PASSWORD", "REDIS_PASSWORD", "JWT_SECRET", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "CLICKHOUSE_USERNAME"},
		},
		{
			"local",
			"local",
			[]string{},
		},
		{
			"dev",
			"dev",
			[]string{"CLICKHOUSE_PASSWORD", "REDIS_PASSWORD"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			si, err := NewSecretsIntegration(tt.env)
			require.NoError(t, err)

			secrets := si.getRequiredSecretsForEnvironment()
			assert.Equal(t, tt.expectedSecrets, secrets)
		})
	}
}

func TestSecretsIntegration_GenerateDefaults(t *testing.T) {
	si, err := NewSecretsIntegration("local")
	require.NoError(t, err)

	tests := []struct {
		name      string
		secretKey string
		expected  string
	}{
		{"clickhouse password", "CLICKHOUSE_PASSWORD", "admin123"},
		{"redis password", "REDIS_PASSWORD", "redis123"},
		{"jwt secret", "JWT_SECRET", "dev-jwt-secret-change-in-production"},
		{"aws access key", "AWS_ACCESS_KEY_ID", "test"},
		{"unknown secret", "UNKNOWN_SECRET", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := si.generateDevDefault(tt.secretKey)
			assert.Equal(t, tt.expected, result)
		})
	}

	// Test Docker defaults
	tests = []struct {
		name      string
		secretKey string
		expected  string
	}{
		{"clickhouse password", "CLICKHOUSE_PASSWORD", "admin123"},
		{"redis password", "REDIS_PASSWORD", "redis123"},
		{"jwt secret", "JWT_SECRET", "docker-jwt-secret"},
		{"aws access key", "AWS_ACCESS_KEY_ID", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_docker", func(t *testing.T) {
			result := si.generateDockerDefault(tt.secretKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSecretsIntegration_HealthCheck(t *testing.T) {
	si, err := NewSecretsIntegration("local")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Health check should not fail even if the secret doesn't exist
	err = si.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestSecretsIntegration_ResolveSecrets(t *testing.T) {
	si, err := NewSecretsIntegration("local")
	require.NoError(t, err)

	config := map[string]interface{}{
		"database": map[string]interface{}{
			"password": "${CLICKHOUSE_PASSWORD:-default_password}",
			"host":     "localhost",
			"port":     9000,
		},
		"redis": map[string]interface{}{
			"password": "${REDIS_PASSWORD:-redis_default}",
			"urls":     []interface{}{"redis://localhost:6379"},
		},
		"plain_value": "no_secret_here",
	}

	err = si.ResolveSecrets(config)
	assert.NoError(t, err)

	// Verify structure is preserved
	assert.Contains(t, config, "database")
	assert.Contains(t, config, "redis")
	assert.Contains(t, config, "plain_value")

	// Verify non-secret values are unchanged
	dbConfig := config["database"].(map[string]interface{})
	assert.Equal(t, "localhost", dbConfig["host"])
	assert.Equal(t, 9000, dbConfig["port"])
	assert.Equal(t, "no_secret_here", config["plain_value"])
}

func BenchmarkSecretsIntegration_ResolveSecretPlaceholder(b *testing.B) {
	si, err := NewSecretsIntegration("local")
	require.NoError(b, err)

	ctx := context.Background()
	testValue := "${NONEXISTENT_SECRET:-fallback_value}"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := si.resolveSecretPlaceholder(ctx, testValue)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSecretsIntegration_ResolveSecrets(b *testing.B) {
	si, err := NewSecretsIntegration("local")
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Create a fresh config for each iteration
		configCopy := map[string]interface{}{
			"database": map[string]interface{}{
				"password": "${CLICKHOUSE_PASSWORD:-default_password}",
				"host":     "localhost",
			},
			"redis": map[string]interface{}{
				"password": "${REDIS_PASSWORD:-redis_default}",
			},
		}

		err := si.ResolveSecrets(configCopy)
		if err != nil {
			b.Fatal(err)
		}
	}
}
