package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jamesneb/playback-backend/internal/secrets"
)

var (
	configPath   string
	environment  string
	provider     string
	outputFormat string
	verbose      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "secrets",
		Short: "Playback Backend Secrets Management CLI",
		Long: `High-performance secrets management tool for the Playback Backend.
Supports AWS Secrets Manager, HashiCorp Vault, encrypted file storage, and environment variables.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if verbose {
				log.SetFlags(log.LstdFlags | log.Lshortfile)
			}
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "./config/secrets.yaml", "Secrets configuration file path")
	rootCmd.PersistentFlags().StringVarP(&environment, "env", "e", "local", "Environment (local, staging, production)")
	rootCmd.PersistentFlags().StringVarP(&provider, "provider", "p", "", "Override provider (aws, vault, file, env)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format (table, json, yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	// Add subcommands
	rootCmd.AddCommand(
		createGetCommand(),
		createSetCommand(),
		createListCommand(),
		createDeleteCommand(),
		createRotateCommand(),
		createValidateCommand(),
		createInitCommand(),
		createExportCommand(),
		createImportCommand(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func createGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "get <secret-key>",
		Short:   "Get a secret value",
		Args:    cobra.ExactArgs(1),
		Example: `  secrets get CLICKHOUSE_PASSWORD`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			key := args[0]
			value, err := manager.GetSecret(ctx, key)
			if err != nil {
				return fmt.Errorf("failed to get secret '%s': %w", key, err)
			}

			switch outputFormat {
			case "json":
				result := map[string]string{key: value}
				return json.NewEncoder(os.Stdout).Encode(result)
			case "yaml":
				result := map[string]string{key: value}
				return yaml.NewEncoder(os.Stdout).Encode(result)
			default:
				fmt.Printf("%s: %s\n", key, value)
			}

			return nil
		},
	}
}

func createSetCommand() *cobra.Command {
	var fromFile string
	var generatePassword bool

	cmd := &cobra.Command{
		Use:     "set <secret-key> [secret-value]",
		Short:   "Set a secret value",
		Args:    cobra.RangeArgs(1, 2),
		Example: `  secrets set CLICKHOUSE_PASSWORD mypassword
  secrets set JWT_SECRET --generate
  secrets set TLS_CERT --from-file ./cert.pem`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			key := args[0]
			var value string

			if fromFile != "" {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("failed to read file '%s': %w", fromFile, err)
				}
				value = string(data)
			} else if generatePassword {
				value = generateSecurePassword(32)
				fmt.Printf("Generated secure password for %s\n", key)
			} else if len(args) == 2 {
				value = args[1]
			} else {
				return fmt.Errorf("secret value required (use --generate or --from-file)")
			}

			if err := manager.SetSecret(ctx, key, value, provider); err != nil {
				return fmt.Errorf("failed to set secret '%s': %w", key, err)
			}

			fmt.Printf("Secret '%s' set successfully\n", key)
			return nil
		},
	}

	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read secret value from file")
	cmd.Flags().BoolVar(&generatePassword, "generate", false, "Generate secure random password")

	return cmd
}

func createListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all secret keys",
		Aliases: []string{"ls"},
		Example: `  secrets list
  secrets ls --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// List secrets from manager
			keys, err := manager.ListSecrets(ctx)
			if err != nil {
				return fmt.Errorf("failed to list secrets: %w", err)
			}

			switch outputFormat {
			case "json":
				return json.NewEncoder(os.Stdout).Encode(map[string][]string{"secrets": keys})
			case "yaml":
				return yaml.NewEncoder(os.Stdout).Encode(map[string][]string{"secrets": keys})
			default:
				fmt.Printf("Secret Keys (%s):\n", environment)
				for _, key := range keys {
					fmt.Printf("  %s\n", key)
				}
			}

			return nil
		},
	}
}

func createDeleteCommand() *cobra.Command {
	var confirm bool

	cmd := &cobra.Command{
		Use:     "delete <secret-key>",
		Short:   "Delete a secret",
		Aliases: []string{"del", "rm"},
		Args:    cobra.ExactArgs(1),
		Example: `  secrets delete OLD_SECRET --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !confirm {
				return fmt.Errorf("use --confirm flag to confirm deletion")
			}

			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			key := args[0]
			if provider != "" {
				err = manager.DeleteSecret(ctx, key, provider)
			} else {
				err = manager.DeleteSecret(ctx, key)
			}
			if err != nil {
				return fmt.Errorf("failed to delete secret '%s': %w", key, err)
			}
			fmt.Printf("Secret '%s' deleted successfully\n", key)
			return nil
		},
	}

	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm deletion")

	return cmd
}

func createRotateCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "rotate <secret-key>",
		Short:   "Rotate a secret (generate new value)",
		Args:    cobra.ExactArgs(1),
		Example: `  secrets rotate JWT_SECRET`,
		RunE: func(cmd *cobra.Command, args []string) error {
			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			key := args[0]
			if provider != "" {
				err = manager.RotateSecret(ctx, key, provider)
			} else {
				err = manager.RotateSecret(ctx, key)
			}
			if err != nil {
				return fmt.Errorf("failed to rotate secret '%s': %w", key, err)
			}

			fmt.Printf("Secret '%s' rotated successfully\n", key)
			return nil
		},
	}
}

func createValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "validate",
		Short:   "Validate all required secrets are present",
		Example: `  secrets validate`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadSecretsConfig()
			if err != nil {
				return err
			}

			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			var errors []string
			var warnings []string

			// Validate required secrets exist
			for category, secretDefs := range config.Secrets {
				for secretName, secretDef := range secretDefs {
					key := secretDef.Key
					if key == "" {
						key = strings.ToUpper(category + "_" + secretName)
					}

					// Check if secret is required for this environment
					required := secretDef.Required
					if len(secretDef.RequiredEnvs) > 0 {
						required = false
						for _, env := range secretDef.RequiredEnvs {
							if env == environment {
								required = true
								break
							}
						}
					}

					if !required {
						continue
					}

					// Try to get the secret
					value, err := manager.GetSecret(ctx, key)
					if err != nil || value == "" {
						errors = append(errors, fmt.Sprintf("Missing required secret: %s (%s)", key, secretDef.Description))
						continue
					}

					// Validate secret properties
					if secretDef.MinLength > 0 && len(value) < secretDef.MinLength {
						errors = append(errors, fmt.Sprintf("Secret %s too short (minimum %d characters)", key, secretDef.MinLength))
					}

					// Check rotation age if specified
					if secretDef.RotationDays > 0 {
						// Would need metadata about when secret was last rotated
						warnings = append(warnings, fmt.Sprintf("Secret %s should be rotated every %d days", key, secretDef.RotationDays))
					}
				}
			}

			// Output results
			if len(errors) > 0 {
				fmt.Fprintf(os.Stderr, "Validation Errors:\n")
				for _, err := range errors {
					fmt.Fprintf(os.Stderr, "  ❌ %s\n", err)
				}
			}

			if len(warnings) > 0 {
				fmt.Fprintf(os.Stderr, "Warnings:\n")
				for _, warn := range warnings {
					fmt.Fprintf(os.Stderr, "  ⚠️  %s\n", warn)
				}
			}

			if len(errors) == 0 && len(warnings) == 0 {
				fmt.Printf("✅ All secrets validated successfully\n")
			}

			if len(errors) > 0 {
				return fmt.Errorf("validation failed with %d errors", len(errors))
			}

			return nil
		},
	}
}

func createInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize secrets for current environment",
		Long: `Initialize all required secrets for the current environment.
This will prompt for missing required secrets and generate secure defaults where appropriate.`,
		Example: `  secrets init --env production`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadSecretsConfig()
			if err != nil {
				return err
			}

			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			fmt.Printf("Initializing secrets for environment: %s\n", environment)

			for category, secretDefs := range config.Secrets {
				for secretName, secretDef := range secretDefs {
					key := secretDef.Key
					if key == "" {
						key = strings.ToUpper(category + "_" + secretName)
					}

					// Check if secret is required for this environment
					required := secretDef.Required
					if len(secretDef.RequiredEnvs) > 0 {
						required = false
						for _, env := range secretDef.RequiredEnvs {
							if env == environment {
								required = true
								break
							}
						}
					}

					if !required {
						continue
					}

					// Check if secret already exists
					if value, err := manager.GetSecret(ctx, key); err == nil && value != "" {
						fmt.Printf("  ✓ %s already exists\n", key)
						continue
					}

					// Generate or prompt for secret
					var value string
					if secretDef.Default != "" {
						value = secretDef.Default
						fmt.Printf("  → Using default value for %s\n", key)
					} else if secretDef.Sensitive {
						value = generateSecurePassword(32)
						fmt.Printf("  → Generated secure value for %s\n", key)
					} else {
						fmt.Printf("  ? Enter value for %s (%s): ", key, secretDef.Description)
						if _, err := fmt.Scanln(&value); err != nil {
							fmt.Printf("  ! Error reading input: %v\n", err)
							continue
						}
					}

					if value != "" {
						if err := manager.SetSecret(ctx, key, value, provider); err != nil {
							return fmt.Errorf("failed to set %s: %w", key, err)
						}
						fmt.Printf("  ✓ %s initialized\n", key)
					}
				}
			}

			fmt.Printf("\nSecrets initialization complete for %s environment\n", environment)
			return nil
		},
	}
}

func createExportCommand() *cobra.Command {
	var outputFile string
	var includeValues bool

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export secrets configuration",
		Example: `  secrets export --output secrets-backup.yaml
  secrets export --include-values --output full-backup.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadSecretsConfig()
			if err != nil {
				return err
			}

			var output interface{}
			if includeValues {
				manager, err := initializeManager()
				if err != nil {
					return err
				}

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				exportData := make(map[string]string)
				for category, secretDefs := range config.Secrets {
					for secretName, secretDef := range secretDefs {
						key := secretDef.Key
						if key == "" {
							key = strings.ToUpper(category + "_" + secretName)
						}

						if value, err := manager.GetSecret(ctx, key); err == nil && value != "" {
							exportData[key] = value
						}
					}
				}
				output = exportData
			} else {
				output = config
			}

			var data []byte
			switch outputFormat {
			case "json":
				data, err = json.MarshalIndent(output, "", "  ")
			case "yaml":
				data, err = yaml.Marshal(output)
			default:
				return fmt.Errorf("unsupported output format: %s", outputFormat)
			}

			if err != nil {
				return err
			}

			if outputFile != "" {
				return os.WriteFile(outputFile, data, 0600)
			}

			fmt.Print(string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&outputFile, "output", "", "Output file path")
	cmd.Flags().BoolVar(&includeValues, "include-values", false, "Include secret values (WARNING: sensitive data)")

	return cmd
}

func createImportCommand() *cobra.Command {
	var inputFile string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import secrets from file",
		Example: `  secrets import --input secrets.yaml
  secrets import --input secrets.json --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("input file required (--input)")
			}

			data, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}

			var secrets map[string]string
			ext := filepath.Ext(inputFile)
			switch ext {
			case ".json":
				err = json.Unmarshal(data, &secrets)
			case ".yaml", ".yml":
				err = yaml.Unmarshal(data, &secrets)
			default:
				return fmt.Errorf("unsupported file format: %s", ext)
			}

			if err != nil {
				return fmt.Errorf("failed to parse input file: %w", err)
			}

			if dryRun {
				fmt.Printf("Dry run - would import %d secrets:\n", len(secrets))
				for key := range secrets {
					fmt.Printf("  %s\n", key)
				}
				return nil
			}

			manager, err := initializeManager()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			for key, value := range secrets {
				if err := manager.SetSecret(ctx, key, value, provider); err != nil {
					return fmt.Errorf("failed to import %s: %w", key, err)
				}
				fmt.Printf("  ✓ Imported %s\n", key)
			}

			fmt.Printf("Successfully imported %d secrets\n", len(secrets))
			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "input", "", "Input file path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be imported without making changes")

	return cmd
}

func initializeManager() (*secrets.Manager, error) {
	config, err := loadSecretsConfig()
	if err != nil {
		return nil, err
	}

	// Create manager config based on environment
	var envConfig map[string]interface{}
	if envs, ok := config.Environments[environment]; ok {
		envConfig = envs
	}

	managerConfig := &secrets.Config{
		DefaultProvider: config.DefaultProvider,
		CacheTTL:       time.Duration(5) * time.Minute,
	}

	// Configure providers based on environment
	if provider != "" || (envConfig != nil && envConfig["provider"] != nil) {
		providerName := provider
		if providerName == "" {
			providerName = envConfig["provider"].(string)
		}

		switch providerName {
		case "aws":
			if awsConfig, ok := envConfig["aws"].(map[string]interface{}); ok {
				managerConfig.AWSConfig = &secrets.AWSSecretsConfig{
					Region:       getStringValue(awsConfig, "region", "us-east-1"),
					SecretPrefix: getStringValue(awsConfig, "secret_prefix", ""),
					EndpointURL:  getStringValue(awsConfig, "endpoint_url", ""),
				}
			}
		case "file":
			if fileConfig, ok := envConfig["file"].(map[string]interface{}); ok {
				managerConfig.FileConfig = &secrets.FileConfig{
					StorePath:     getStringValue(fileConfig, "store_path", "./secrets.enc"),
					EncryptionKey: getStringValue(fileConfig, "encryption_key", "default-key"),
				}
			}
		case "vault":
			if vaultConfig, ok := config.Vault.(map[string]interface{}); ok {
				managerConfig.VaultConfig = &secrets.VaultConfig{
					Address: getStringValue(vaultConfig, "address", ""),
					Mount:   getStringValue(vaultConfig, "mount", "secret/v2"),
				}
			}
		}
	}

	return secrets.NewManager(managerConfig)
}

func loadSecretsConfig() (*SecretsConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config SecretsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func getStringValue(m map[string]interface{}, key, defaultValue string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func generateSecurePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to a reasonable default
			b[i] = charset[i%len(charset)]
		} else {
			b[i] = charset[n.Int64()]
		}
	}
	return string(b)
}

// SecretsConfig represents the secrets configuration structure
type SecretsConfig struct {
	DefaultProvider string                            `yaml:"default_provider"`
	CacheTTL        string                            `yaml:"cache_ttl"`
	Environments    map[string]map[string]interface{} `yaml:"environments"`
	Vault           interface{}                       `yaml:"vault"`
	Secrets         map[string]map[string]SecretDef   `yaml:"secrets"`
	Security        SecurityConfig                    `yaml:"security"`
	Performance     PerformanceConfig                 `yaml:"performance"`
	Development     DevelopmentConfig                 `yaml:"development"`
}

type SecretDef struct {
	Key           string   `yaml:"key"`
	Description   string   `yaml:"description"`
	RotationDays  int      `yaml:"rotation_days"`
	Required      bool     `yaml:"required"`
	RequiredEnvs  []string `yaml:"required_envs"`
	Default       string   `yaml:"default"`
	MinLength     int      `yaml:"min_length"`
	Sensitive     bool     `yaml:"sensitive"`
}

type SecurityConfig struct {
	EnforceRotation       bool   `yaml:"enforce_rotation"`
	MaxAgeDays           int    `yaml:"max_age_days"`
	EnableAuditLog       bool   `yaml:"enable_audit_log"`
	AuditLogPath         string `yaml:"audit_log_path"`
	ValidateOnLoad       bool   `yaml:"validate_on_load"`
}

type PerformanceConfig struct {
	CacheEnabled     bool   `yaml:"cache_enabled"`
	CacheSize        int    `yaml:"cache_size"`
	CacheTTL         string `yaml:"cache_ttl"`
	PreloadSecrets   bool   `yaml:"preload_secrets"`
	BatchOperations  bool   `yaml:"batch_operations"`
}

type DevelopmentConfig struct {
	AllowPlaintext  bool `yaml:"allow_plaintext"`
	MockProviders   bool `yaml:"mock_providers"`
	DebugLogging    bool `yaml:"debug_logging"`
	ValidateRequired bool `yaml:"validate_required"`
}