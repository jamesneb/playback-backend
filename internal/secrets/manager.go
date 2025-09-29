package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

// SecretProvider defines interface for different secret storage backends
type SecretProvider interface {
	GetSecret(ctx context.Context, key string) (string, error)
	SetSecret(ctx context.Context, key, value string) error
	DeleteSecret(ctx context.Context, key string) error
	ListSecrets(ctx context.Context) ([]string, error)
	RotateSecret(ctx context.Context, key string) error
}

// Manager provides high-performance secrets management with multiple backends
type Manager struct {
	providers map[string]SecretProvider
	cache     map[string]*cachedSecret
	cacheTTL  time.Duration
	mu        sync.RWMutex
}

// cachedSecret holds encrypted secrets with TTL
type cachedSecret struct {
	value     string
	encrypted bool
	expiresAt time.Time
}

// Config holds secrets manager configuration
type Config struct {
	DefaultProvider string        `json:"default_provider"`
	CacheTTL       time.Duration `json:"cache_ttl"`
	EncryptionKey  string        `json:"encryption_key,omitempty"`

	// Provider-specific configs
	AWSConfig   *AWSSecretsConfig   `json:"aws,omitempty"`
	VaultConfig *VaultConfig       `json:"vault,omitempty"`
	FileConfig  *FileConfig        `json:"file,omitempty"`
}

// AWSSecretsConfig for AWS Secrets Manager
type AWSSecretsConfig struct {
	Region          string `json:"region"`
	Profile         string `json:"profile,omitempty"`
	AssumeRole      string `json:"assume_role,omitempty"`
	EndpointURL     string `json:"endpoint_url,omitempty"`
	SecretPrefix    string `json:"secret_prefix"`
	RotationEnabled bool   `json:"rotation_enabled"`
}

// VaultConfig for HashiCorp Vault
type VaultConfig struct {
	Address      string `json:"address"`
	Token        string `json:"token,omitempty"`
	Mount        string `json:"mount"`
	Namespace    string `json:"namespace,omitempty"`
	AuthMethod   string `json:"auth_method"`
	RoleID       string `json:"role_id,omitempty"`
	SecretID     string `json:"secret_id,omitempty"`
	CACert       string `json:"ca_cert,omitempty"`
	ClientCert   string `json:"client_cert,omitempty"`
	ClientKey    string `json:"client_key,omitempty"`
	InsecureSkip bool   `json:"insecure_skip_verify"`
}

// FileConfig for encrypted file-based secrets (dev/testing)
type FileConfig struct {
	StorePath     string `json:"store_path"`
	EncryptionKey string `json:"encryption_key"`
	BackupCount   int    `json:"backup_count"`
	FileMode      string `json:"file_mode"`
}

// NewManager creates a new secrets manager with optimized caching
func NewManager(cfg *Config) (*Manager, error) {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute // Default 5-minute cache
	}

	manager := &Manager{
		providers: make(map[string]SecretProvider),
		cache:     make(map[string]*cachedSecret),
		cacheTTL:  cfg.CacheTTL,
	}

	// Initialize AWS Secrets Manager if configured
	if cfg.AWSConfig != nil {
		awsProvider, err := NewAWSSecretsProvider(cfg.AWSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize AWS secrets provider: %w", err)
		}
		manager.providers["aws"] = awsProvider
	}

	// Initialize Vault if configured
	if cfg.VaultConfig != nil {
		vaultProvider, err := NewVaultProvider(cfg.VaultConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize Vault provider: %w", err)
		}
		manager.providers["vault"] = vaultProvider
	}

	// Initialize file provider if configured (dev/testing)
	if cfg.FileConfig != nil {
		fileProvider, err := NewFileProvider(cfg.FileConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize file provider: %w", err)
		}
		manager.providers["file"] = fileProvider
	}

	// Add environment variable provider as fallback
	manager.providers["env"] = &EnvProvider{}

	return manager, nil
}

// GetSecret retrieves a secret with caching and fallback providers
func (m *Manager) GetSecret(ctx context.Context, key string) (string, error) {
	// Check cache first (hot path optimization)
	m.mu.RLock()
	if cached, exists := m.cache[key]; exists && time.Now().Before(cached.expiresAt) {
		value := cached.value
		encrypted := cached.encrypted
		m.mu.RUnlock()

		if encrypted {
			return m.decrypt(value)
		}
		return value, nil
	}
	m.mu.RUnlock()

	// Try providers in priority order
	providers := []string{"aws", "vault", "file", "env"}
	for _, providerName := range providers {
		if provider, exists := m.providers[providerName]; exists {
			value, err := provider.GetSecret(ctx, key)
			if err == nil && value != "" {
				// Cache the result
				m.cacheSecret(key, value, false)
				return value, nil
			}
		}
	}

	return "", fmt.Errorf("secret not found: %s", key)
}

// GetSecretWithFallback gets secret with explicit fallback value
func (m *Manager) GetSecretWithFallback(ctx context.Context, key, fallback string) string {
	if value, err := m.GetSecret(ctx, key); err == nil && value != "" {
		return value
	}
	return fallback
}

// SetSecret stores a secret in the primary provider
func (m *Manager) SetSecret(ctx context.Context, key, value string, providerName ...string) error {
	provider := "aws" // Default to AWS Secrets Manager
	if len(providerName) > 0 {
		provider = providerName[0]
	}

	secretProvider, exists := m.providers[provider]
	if !exists {
		return fmt.Errorf("provider not found: %s", provider)
	}

	if err := secretProvider.SetSecret(ctx, key, value); err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}

	// Update cache
	m.cacheSecret(key, value, false)
	return nil
}

// RotateSecret rotates a secret and updates cache
func (m *Manager) RotateSecret(ctx context.Context, key string, providerName ...string) error {
	provider := "aws"
	if len(providerName) > 0 {
		provider = providerName[0]
	}

	secretProvider, exists := m.providers[provider]
	if !exists {
		return fmt.Errorf("provider not found: %s", provider)
	}

	if err := secretProvider.RotateSecret(ctx, key); err != nil {
		return fmt.Errorf("failed to rotate secret: %w", err)
	}

	// Invalidate cache entry
	m.mu.Lock()
	delete(m.cache, key)
	m.mu.Unlock()

	return nil
}

// RefreshCache refreshes all cached secrets
func (m *Manager) RefreshCache(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing cache
	for k := range m.cache {
		delete(m.cache, k)
	}

	return nil
}

// ListSecrets lists all secrets from the default provider
func (m *Manager) ListSecrets(ctx context.Context) ([]string, error) {
	provider, exists := m.providers["aws"] // Default to AWS
	if !exists {
		return nil, fmt.Errorf("default provider not found")
	}
	return provider.ListSecrets(ctx)
}

// DeleteSecret deletes a secret from the specified provider
func (m *Manager) DeleteSecret(ctx context.Context, key string, providerName ...string) error {
	providerType := "aws" // Default to AWS
	if len(providerName) > 0 {
		providerType = providerName[0]
	}

	provider, exists := m.providers[providerType]
	if !exists {
		return fmt.Errorf("provider not found: %s", providerType)
	}

	if err := provider.DeleteSecret(ctx, key); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	// Invalidate cache entry
	m.mu.Lock()
	delete(m.cache, key)
	m.mu.Unlock()

	return nil
}

// cacheSecret stores secret in memory cache with encryption
func (m *Manager) cacheSecret(key, value string, encrypt bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cachedValue := value
	if encrypt {
		if encrypted, err := m.encrypt(value); err == nil {
			cachedValue = encrypted
		}
	}

	m.cache[key] = &cachedSecret{
		value:     cachedValue,
		encrypted: encrypt,
		expiresAt: time.Now().Add(m.cacheTTL),
	}
}

// encrypt encrypts a secret value using AES-256-GCM
func (m *Manager) encrypt(plaintext string) (string, error) {
	// Use a hardcoded key for cache encryption (not for persistent storage)
	key := sha256.Sum256([]byte("cache-encryption-key-playback-backend"))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a secret value
func (m *Manager) decrypt(ciphertext string) (string, error) {
	key := sha256.Sum256([]byte("cache-encryption-key-playback-backend"))

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext_bytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext_bytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// EnvProvider implements environment variable-based secrets
type EnvProvider struct{}

func (p *EnvProvider) GetSecret(_ context.Context, key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable not found: %s", key)
	}
	return value, nil
}

func (p *EnvProvider) SetSecret(_ context.Context, key, value string) error {
	return os.Setenv(key, value)
}

func (p *EnvProvider) DeleteSecret(_ context.Context, key string) error {
	return os.Unsetenv(key)
}

func (p *EnvProvider) ListSecrets(_ context.Context) ([]string, error) {
	env := os.Environ()
	keys := make([]string, len(env))
	for i, pair := range env {
		keys[i] = pair[:len(pair)-len(os.Getenv(pair))-1]
	}
	return keys, nil
}

func (p *EnvProvider) RotateSecret(_ context.Context, key string) error {
	return fmt.Errorf("rotation not supported for environment provider")
}

// AWSSecretsProvider implements AWS Secrets Manager
type AWSSecretsProvider struct {
	client *secretsmanager.Client
	prefix string
}

// NewAWSSecretsProvider creates AWS Secrets Manager provider
func NewAWSSecretsProvider(cfg *AWSSecretsConfig) (*AWSSecretsProvider, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override endpoint for LocalStack
	if cfg.EndpointURL != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.EndpointURL)
	}

	client := secretsmanager.NewFromConfig(awsCfg)

	return &AWSSecretsProvider{
		client: client,
		prefix: cfg.SecretPrefix,
	}, nil
}

func (p *AWSSecretsProvider) GetSecret(ctx context.Context, key string) (string, error) {
	secretName := p.prefix + key

	result, err := p.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to get AWS secret: %w", err)
	}

	if result.SecretString != nil {
		return *result.SecretString, nil
	}

	return string(result.SecretBinary), nil
}

func (p *AWSSecretsProvider) SetSecret(ctx context.Context, key, value string) error {
	secretName := p.prefix + key

	_, err := p.client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(secretName),
		SecretString: aws.String(value),
	})
	if err != nil {
		// Try update if creation fails (secret might exist)
		_, updateErr := p.client.UpdateSecret(ctx, &secretsmanager.UpdateSecretInput{
			SecretId:     aws.String(secretName),
			SecretString: aws.String(value),
		})
		return updateErr
	}

	return nil
}

func (p *AWSSecretsProvider) DeleteSecret(ctx context.Context, key string) error {
	secretName := p.prefix + key

	_, err := p.client.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretName),
	})
	return err
}

func (p *AWSSecretsProvider) ListSecrets(ctx context.Context) ([]string, error) {
	result, err := p.client.ListSecrets(ctx, &secretsmanager.ListSecretsInput{})
	if err != nil {
		return nil, err
	}

	secrets := make([]string, 0, len(result.SecretList))
	for _, secret := range result.SecretList {
		if secret.Name != nil {
			name := *secret.Name
			if p.prefix != "" && len(name) > len(p.prefix) {
				name = name[len(p.prefix):]
			}
			secrets = append(secrets, name)
		}
	}

	return secrets, nil
}

func (p *AWSSecretsProvider) RotateSecret(ctx context.Context, key string) error {
	secretName := p.prefix + key

	_, err := p.client.RotateSecret(ctx, &secretsmanager.RotateSecretInput{
		SecretId: aws.String(secretName),
	})
	return err
}

// VaultProvider implements HashiCorp Vault (placeholder - would need vault client)
type VaultProvider struct {
	config *VaultConfig
}

func NewVaultProvider(cfg *VaultConfig) (*VaultProvider, error) {
	// This would implement actual Vault client initialization
	return &VaultProvider{config: cfg}, nil
}

func (p *VaultProvider) GetSecret(ctx context.Context, key string) (string, error) {
	return "", fmt.Errorf("vault provider not implemented yet")
}

func (p *VaultProvider) SetSecret(ctx context.Context, key, value string) error {
	return fmt.Errorf("vault provider not implemented yet")
}

func (p *VaultProvider) DeleteSecret(ctx context.Context, key string) error {
	return fmt.Errorf("vault provider not implemented yet")
}

func (p *VaultProvider) ListSecrets(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("vault provider not implemented yet")
}

func (p *VaultProvider) RotateSecret(ctx context.Context, key string) error {
	return fmt.Errorf("vault provider not implemented yet")
}

// FileProvider implements encrypted file-based secrets storage
type FileProvider struct {
	storePath     string
	encryptionKey []byte
	mu           sync.RWMutex
}

func NewFileProvider(cfg *FileConfig) (*FileProvider, error) {
	if err := os.MkdirAll(filepath.Dir(cfg.StorePath), 0700); err != nil {
		return nil, fmt.Errorf("failed to create secrets directory: %w", err)
	}

	key := sha256.Sum256([]byte(cfg.EncryptionKey))

	return &FileProvider{
		storePath:     cfg.StorePath,
		encryptionKey: key[:],
	}, nil
}

func (p *FileProvider) GetSecret(ctx context.Context, key string) (string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data, err := p.loadSecrets()
	if err != nil {
		return "", err
	}

	if value, exists := data[key]; exists {
		return value, nil
	}

	return "", fmt.Errorf("secret not found: %s", key)
}

func (p *FileProvider) SetSecret(ctx context.Context, key, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := p.loadSecrets()
	if err != nil {
		data = make(map[string]string)
	}

	data[key] = value
	return p.saveSecrets(data)
}

func (p *FileProvider) DeleteSecret(ctx context.Context, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := p.loadSecrets()
	if err != nil {
		return err
	}

	delete(data, key)
	return p.saveSecrets(data)
}

func (p *FileProvider) ListSecrets(ctx context.Context) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	data, err := p.loadSecrets()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	return keys, nil
}

func (p *FileProvider) RotateSecret(ctx context.Context, key string) error {
	return fmt.Errorf("manual rotation required for file provider")
}

func (p *FileProvider) loadSecrets() (map[string]string, error) {
	if _, err := os.Stat(p.storePath); os.IsNotExist(err) {
		return make(map[string]string), nil
	}

	encryptedData, err := os.ReadFile(p.storePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secrets file: %w", err)
	}

	if len(encryptedData) == 0 {
		return make(map[string]string), nil
	}

	// Decrypt data
	plaintext, err := p.decryptData(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secrets: %w", err)
	}

	var data map[string]string
	if err := json.Unmarshal(plaintext, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secrets: %w", err)
	}

	return data, nil
}

func (p *FileProvider) saveSecrets(data map[string]string) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal secrets: %w", err)
	}

	// Encrypt data
	encryptedData, err := p.encryptData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to encrypt secrets: %w", err)
	}

	if err := os.WriteFile(p.storePath, encryptedData, 0600); err != nil {
		return fmt.Errorf("failed to write secrets file: %w", err)
	}

	return nil
}

func (p *FileProvider) encryptData(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.encryptionKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (p *FileProvider) decryptData(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(p.encryptionKey)
	if err != nil {
		return nil, err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext_bytes := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext_bytes, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}