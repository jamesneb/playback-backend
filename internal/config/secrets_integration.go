package config

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jamesneb/playback-backend/internal/secrets"
)

// SecretsIntegration provides seamless secrets management integration with configuration
type SecretsIntegration struct {
	manager        *secrets.Manager
	env            string
	resolvedValues map[string]string
}

// NewSecretsIntegration creates a new secrets integration
func NewSecretsIntegration(env string) (*SecretsIntegration, error) {
	// Determine secrets configuration based on environment
	var managerConfig *secrets.Config

	switch env {
	case "production", "prod":
		managerConfig = &secrets.Config{
			DefaultProvider: "aws",
			CacheTTL:        5 * time.Minute,
			AWSConfig: &secrets.AWSSecretsConfig{
				Region:          os.Getenv("AWS_REGION"),
				SecretPrefix:    "prod/playback-backend/",
				RotationEnabled: true,
				AssumeRole:      os.Getenv("AWS_SECRETS_ROLE_ARN"),
			},
		}

	case "staging":
		managerConfig = &secrets.Config{
			DefaultProvider: "aws",
			CacheTTL:        5 * time.Minute,
			AWSConfig: &secrets.AWSSecretsConfig{
				Region:          os.Getenv("AWS_REGION"),
				SecretPrefix:    "staging/playback-backend/",
				RotationEnabled: true,
				EndpointURL:     os.Getenv("AWS_ENDPOINT_URL"),
			},
		}

	case "local", "local-docker", "dev":
		secretsPath := "./config/.secrets"
		if env == "local-docker" {
			secretsPath = "/app/config/.secrets"
		}

		managerConfig = &secrets.Config{
			DefaultProvider: "file",
			CacheTTL:        1 * time.Minute,
			FileConfig: &secrets.FileConfig{
				StorePath:     secretsPath,
				EncryptionKey: os.Getenv("SECRETS_ENCRYPTION_KEY"),
				BackupCount:   3,
			},
		}

	default:
		return nil, fmt.Errorf("unsupported environment for secrets: %s", env)
	}

	manager, err := secrets.NewManager(managerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize secrets manager: %w", err)
	}

	return &SecretsIntegration{
		manager:        manager,
		env:            env,
		resolvedValues: make(map[string]string),
	}, nil
}

// ResolveSecrets resolves all secret placeholders in configuration
func (si *SecretsIntegration) ResolveSecrets(config map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return si.resolveSecretsRecursive(ctx, config)
}

// resolveSecretsRecursive recursively resolves secrets in nested configuration
func (si *SecretsIntegration) resolveSecretsRecursive(ctx context.Context, obj interface{}) error {
	switch v := obj.(type) {
	case map[string]interface{}:
		for _, value := range v {
			if err := si.resolveSecretsRecursive(ctx, value); err != nil {
				return err
			}
		}

	case []interface{}:
		for _, item := range v {
			if err := si.resolveSecretsRecursive(ctx, item); err != nil {
				return err
			}
		}

	case string:
		if resolved, err := si.resolveSecretPlaceholder(ctx, v); err != nil {
			return err
		} else if resolved != v {
			// Store the resolved value for later use
			si.resolvedValues[fmt.Sprintf("%p", &v)] = resolved
		}
	}

	return nil
}

// resolveSecretPlaceholder resolves a single secret placeholder
func (si *SecretsIntegration) resolveSecretPlaceholder(ctx context.Context, value string) (string, error) {
	// Handle ${SECRET_NAME} or ${SECRET_NAME:-default} patterns
	if !strings.Contains(value, "${") {
		return value, nil
	}

	// Simple placeholder resolution
	start := strings.Index(value, "${")
	if start == -1 {
		return value, nil
	}

	end := strings.Index(value[start:], "}")
	if end == -1 {
		return value, nil
	}

	placeholder := value[start+2 : start+end]
	var secretKey, fallback string

	// Handle default values: ${SECRET_NAME:-default_value}
	if strings.Contains(placeholder, ":-") {
		parts := strings.SplitN(placeholder, ":-", 2)
		secretKey = parts[0]
		if len(parts) > 1 {
			fallback = parts[1]
		}
	} else {
		secretKey = placeholder
	}

	// Get secret value
	secretValue, err := si.manager.GetSecret(ctx, secretKey)
	if err != nil {
		if fallback != "" {
			secretValue = fallback
		} else {
			return "", fmt.Errorf("failed to resolve secret %s: %w", secretKey, err)
		}
	}

	// Replace placeholder with actual value
	resolved := strings.Replace(value, "${"+placeholder+"}", secretValue, 1)

	// Recursively resolve any remaining placeholders
	if strings.Contains(resolved, "${") {
		return si.resolveSecretPlaceholder(ctx, resolved)
	}

	return resolved, nil
}

// GetSecret provides direct access to secrets manager
func (si *SecretsIntegration) GetSecret(ctx context.Context, key string) (string, error) {
	return si.manager.GetSecret(ctx, key)
}

// GetSecretWithFallback gets secret with fallback value
func (si *SecretsIntegration) GetSecretWithFallback(ctx context.Context, key, fallback string) string {
	return si.manager.GetSecretWithFallback(ctx, key, fallback)
}

// SetSecret sets a secret value
func (si *SecretsIntegration) SetSecret(ctx context.Context, key, value string) error {
	return si.manager.SetSecret(ctx, key, value)
}

// RotateSecret rotates a secret
func (si *SecretsIntegration) RotateSecret(ctx context.Context, key string) error {
	return si.manager.RotateSecret(ctx, key)
}

// ValidateRequiredSecrets validates that all required secrets are present
func (si *SecretsIntegration) ValidateRequiredSecrets(ctx context.Context) error {
	requiredSecrets := si.getRequiredSecretsForEnvironment()

	var missingSecrets []string
	for _, secretKey := range requiredSecrets {
		if _, err := si.manager.GetSecret(ctx, secretKey); err != nil {
			missingSecrets = append(missingSecrets, secretKey)
		}
	}

	if len(missingSecrets) > 0 {
		return fmt.Errorf("missing required secrets for %s: %v", si.env, missingSecrets)
	}

	return nil
}

// getRequiredSecretsForEnvironment returns required secrets based on environment
func (si *SecretsIntegration) getRequiredSecretsForEnvironment() []string {
	baseSecrets := []string{
		"CLICKHOUSE_PASSWORD",
		"REDIS_PASSWORD",
	}

	switch si.env {
	case "production", "prod":
		return append(baseSecrets,
			"JWT_SECRET",
			"AWS_ACCESS_KEY_ID",
			"AWS_SECRET_ACCESS_KEY",
			"TLS_CERT_FILE",
			"TLS_KEY_FILE",
			"CLICKHOUSE_USERNAME",
		)

	case "staging":
		return append(baseSecrets,
			"JWT_SECRET",
			"AWS_ACCESS_KEY_ID",
			"AWS_SECRET_ACCESS_KEY",
			"CLICKHOUSE_USERNAME",
		)

	case "local", "local-docker":
		// Local development has minimal requirements
		return []string{}

	case "dev":
		return baseSecrets

	default:
		return baseSecrets
	}
}

// InitializeSecretsForEnvironment sets up required secrets for the environment
func (si *SecretsIntegration) InitializeSecretsForEnvironment(ctx context.Context) error {
	requiredSecrets := si.getRequiredSecretsForEnvironment()

	for _, secretKey := range requiredSecrets {
		// Check if secret already exists
		if _, err := si.manager.GetSecret(ctx, secretKey); err == nil {
			continue // Secret already exists
		}

		// Generate appropriate default values for development environments
		var defaultValue string
		switch si.env {
		case "local", "dev":
			defaultValue = si.generateDevDefault(secretKey)
		case "local-docker":
			defaultValue = si.generateDockerDefault(secretKey)
		default:
			// Production environments should not auto-generate secrets
			return fmt.Errorf("secret %s must be manually configured for %s environment", secretKey, si.env)
		}

		if defaultValue != "" {
			if err := si.manager.SetSecret(ctx, secretKey, defaultValue); err != nil {
				return fmt.Errorf("failed to initialize secret %s: %w", secretKey, err)
			}
		}
	}

	return nil
}

// generateDevDefault generates development defaults for secrets
func (si *SecretsIntegration) generateDevDefault(secretKey string) string {
	switch secretKey {
	case "CLICKHOUSE_PASSWORD":
		return "admin123"
	case "REDIS_PASSWORD":
		return "redis123"
	case "JWT_SECRET":
		return "dev-jwt-secret-change-in-production"
	case "AWS_ACCESS_KEY_ID":
		return "test"
	case "AWS_SECRET_ACCESS_KEY":
		return "test"
	case "CLICKHOUSE_USERNAME":
		return "admin"
	default:
		return ""
	}
}

// generateDockerDefault generates Docker environment defaults
func (si *SecretsIntegration) generateDockerDefault(secretKey string) string {
	switch secretKey {
	case "CLICKHOUSE_PASSWORD":
		return "admin123"
	case "REDIS_PASSWORD":
		return "redis123"
	case "JWT_SECRET":
		return "docker-jwt-secret"
	case "AWS_ACCESS_KEY_ID":
		return "test"
	case "AWS_SECRET_ACCESS_KEY":
		return "test"
	case "CLICKHOUSE_USERNAME":
		return "admin"
	default:
		return ""
	}
}

// HealthCheck performs a health check on the secrets manager
func (si *SecretsIntegration) HealthCheck(ctx context.Context) error {
	// Try to retrieve a test secret or perform a simple operation
	_, err := si.manager.GetSecret(ctx, "HEALTH_CHECK_SECRET")
	// It's okay if the secret doesn't exist, we just want to verify connectivity
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("secrets manager health check failed: %w", err)
	}
	return nil
}