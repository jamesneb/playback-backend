// internal/config/s3/section.go
//
// Package s3 defines the configuration for AWS S3 storage.
//
// It consists of a Config struct and methods to resolve incoming key-values from a [config.Manager]
package s3

import (
	"fmt"

	"github.com/jamesneb/playback-backend/internal/config/base"
	"github.com/jamesneb/playback-backend/internal/config/decodeutil"
	resolver "github.com/jamesneb/playback-backend/internal/config/propertyresolver"
)

// Config holds S3 storage configuration for AWS S3 and S3-compatible object storage services.
//
// This configuration supports:
//   - AWS S3 in any region
//   - LocalStack for local development
//   - MinIO and other S3-compatible storage
//   - Server-side encryption (SSE)
//   - Custom endpoint URLs for testing
//
// Example production configuration:
//
//	cfg := s3.Config{
//	    Region:     base.AWS_US_EAST_1,
//	    Bucket:     "prod-telemetry",
//	    PathPrefix: "traces/",
//	    EnableSSE:  true,
//	    // AccessKeyID and SecretAccessKey left empty to use IAM role
//	}
//
// Example LocalStack configuration:
//
//	cfg := s3.Config{
//	    Region:          base.AWS_US_EAST_1,
//	    Bucket:          "test-bucket",
//	    EndpointURL:     "http://localhost:4566",
//	    AccessKeyID:     "test",
//	    SecretAccessKey: "test",
//	    ForcePathStyle:  true,
//	    EnableSSE:       false,
//	}
type Config struct {
	// Region specifies the AWS region where the S3 bucket is located.
	// Must be a valid AWS region code (e.g., us-east-1, eu-west-1).
	// Default: us-east-1
	//
	// For a complete list of regions, see: https://docs.aws.amazon.com/general/latest/gr/s3.html
	Region base.AWSRegion `mapstructure:"region"`

	// Bucket is the name of the S3 bucket for storing objects.
	// Required field - must be set via configuration or environment variable.
	//
	// Bucket naming rules:
	//   - 3-63 characters long
	//   - Lowercase letters, numbers, hyphens, periods
	//   - Must start with letter or number
	//   - No underscores, spaces, or uppercase
	//   - Must be globally unique across all AWS accounts
	//
	// Examples: "telemetry-data", "prod-traces-us-east-1"
	Bucket string `mapstructure:"bucket"`

	// EndpointURL specifies a custom S3 endpoint for local development or S3-compatible services.
	// Optional - leave empty for AWS S3, set for LocalStack/MinIO.
	//
	// When set, all S3 requests are directed to this endpoint instead of AWS.
	// Useful for:
	//   - LocalStack: http://localhost:4566
	//   - MinIO: http://localhost:9000
	//   - Custom S3-compatible services
	//
	// Must include protocol (http:// or https://).
	EndpointURL string `mapstructure:"endpoint_url"`

	// AccessKeyID is the AWS access key ID for authentication.
	// Optional - leave empty to use IAM roles or default credential chain.
	//
	// When both AccessKeyID and SecretAccessKey are provided, they are used for authentication.
	// When empty, the AWS SDK uses the default credential provider chain:
	//   1. Environment variables (AWS_ACCESS_KEY_ID)
	//   2. AWS credentials file (~/.aws/credentials)
	//   3. IAM instance role (recommended for production)
	//
	// Format: 20-character alphanumeric string starting with AKIA (standard key) or ASIA (temporary)
	AccessKeyID string `mapstructure:"access_key_id"`

	// SecretAccessKey is the AWS secret access key for authentication.
	// Optional - leave empty to use IAM roles or default credential chain.
	//
	// Must be provided if AccessKeyID is set. Validation ensures both are present or both are empty.
	// Keep this value secure - never commit to source control.
	//
	// Format: 40-character alphanumeric string with special characters
	SecretAccessKey string `mapstructure:"secret_access_key"`

	// SessionToken is the AWS session token for temporary credentials (AWS STS).
	// Optional - only needed when using temporary security credentials.
	//
	// Used with:
	//   - AWS STS AssumeRole
	//   - AWS SSO temporary credentials
	//   - AWS CLI temporary sessions
	//   - Cross-account access
	//
	// When present, used alongside AccessKeyID and SecretAccessKey.
	SessionToken string `mapstructure:"session_token"`

	// PathPrefix is a prefix added to all S3 object keys for organization.
	// Optional - defaults to empty (objects stored at bucket root).
	//
	// Used to organize objects within a bucket:
	//   - By environment: "prod/", "staging/", "dev/"
	//   - By data type: "traces/", "metrics/", "logs/"
	//   - By date: "2025/01/03/"
	//   - Combined: "prod/traces/2025/01/03/"
	//
	// Should end with "/" for directory-like organization.
	// Example: If PathPrefix is "traces/" and object is "span-123.json",
	// the full key becomes "traces/span-123.json"
	PathPrefix string `mapstructure:"path_prefix"`

	// ForcePathStyle determines the S3 URL style used for requests.
	// Default: false (use virtual-hosted style)
	//
	// URL styles:
	//   - Virtual-hosted (false): https://bucket.s3.amazonaws.com/key
	//   - Path style (true): https://s3.amazonaws.com/bucket/key
	//
	// Set to true when using:
	//   - LocalStack (required for proper DNS resolution)
	//   - MinIO (required for compatibility)
	//   - Some S3-compatible services
	//
	// AWS S3 prefers virtual-hosted style, but path style is supported for compatibility.
	ForcePathStyle bool `mapstructure:"force_path_style"`

	// EnableSSE enables server-side encryption for objects stored in S3.
	// Default: true (recommended for production)
	//
	// When enabled, objects are encrypted at rest using SSE-S3 (AES-256 encryption with AWS-managed keys).
	// The AWS SDK automatically sets the ServerSideEncryption parameter on PutObject operations.
	//
	// For other encryption options:
	//   - SSE-KMS: Configure KMS key ARN in bucket policy or object metadata
	//   - SSE-C: Provide encryption keys in each request (requires application changes)
	//
	// Encryption adds negligible performance overhead and is best practice for compliance.
	// Set to false only for local development or testing.
	EnableSSE bool `mapstructure:"enable_sse"`
}

// Defaults returns a [Config] with sensible default values for S3 configuration.
//
// This function provides baseline configuration that can be overridden via environment variables
// or configuration files using [FromResolver]. The defaults are suitable for AWS S3 in us-east-1
// with encryption enabled.
//
// Default values:
//   - Region: us-east-1 (most common AWS region)
//   - Bucket: empty (must be provided)
//   - EndpointURL: empty (uses AWS S3)
//   - AccessKeyID: empty (uses IAM role or credential chain)
//   - SecretAccessKey: empty (uses IAM role or credential chain)
//   - SessionToken: empty (no temporary credentials)
//   - PathPrefix: empty (root of bucket)
//   - ForcePathStyle: false (virtual-hosted style for AWS S3)
//   - EnableSSE: true (encryption enabled for security)
//
// Usage:
//
//	// Start with defaults
//	cfg := s3.Defaults()
//
//	// Override specific values
//	cfg.Bucket = "my-bucket"
//	cfg.PathPrefix = "traces/"
//
//	// Validate the configuration
//	if err := cfg.Validate(); err != nil {
//	    log.Fatal(err)
//	}
//
// Note: Prefer [FromResolver] for loading from environment variables or config files.
func Defaults() Config {
	return Config{
		Region:          DEFAULT_REGION,
		Bucket:          "",
		EndpointURL:     "",
		AccessKeyID:     "",
		SecretAccessKey: "",
		SessionToken:    "",
		PathPrefix:      "",
		ForcePathStyle:  DEFAULT_FORCE_PATH_STYLE,
		EnableSSE:       DEFAULT_ENABLE_SSE,
	}
}

// Validate checks the S3 configuration for correctness and returns an error if invalid.
//
// Validation rules:
//   - Bucket must not be empty (required for all S3 operations)
//   - If AccessKeyID is provided, SecretAccessKey must also be provided
//   - If SecretAccessKey is provided, AccessKeyID must also be provided
//
// Validation does not check:
//   - Whether the bucket exists in AWS
//   - Whether credentials are valid
//   - Network connectivity to S3 or custom endpoints
//   - IAM permissions for the credentials
//
// These runtime checks must be performed when using the configuration.
//
// Example:
//
//	cfg := s3.Config{
//	    Region: base.AWS_US_EAST_1,
//	    Bucket: "telemetry-data",
//	}
//
//	if err := cfg.Validate(); err != nil {
//	    log.Fatalf("Invalid S3 config: %v", err)
//	}
//
// Error examples:
//
//	# Missing bucket
//	Error: "S3_BUCKET: must not be empty"
//
//	# Incomplete credentials
//	Error: "S3_ACCESS_KEY_ID: must not be empty"
//	Error: "S3_SECRET_ACCESS_KEY: must not be empty"
func (c Config) Validate() error {
	v := base.NewValidator(S3_PREFIX)

	// Bucket is required
	base.NotEmpty(v, "bucket", c.Bucket)

	// If credentials are provided, both must be present
	v.When(c.AccessKeyID != "" || c.SecretAccessKey != "", func(v *base.Validator) {
		base.NotEmpty(v, "access_key_id", c.AccessKeyID)
		base.NotEmpty(v, "secret_access_key", c.SecretAccessKey)
	})

	return v.Err()
}

// FromResolver loads S3 configuration from a [propertyresolver.PropertyResolver] and validates it.
//
// This is the primary way to load S3 configuration from environment variables, config files,
// or other configuration sources. It starts with [Defaults], overlays values from the resolver,
// and validates the result.
//
// Configuration loading order:
//  1. Start with default values from [Defaults]
//  2. Overlay values from resolver (environment variables, config files, etc.)
//  3. Validate the merged configuration with [Config.Validate]
//  4. Return error if validation fails
//
// Environment variables (with S3_ prefix):
//
//	S3_REGION=us-west-2
//	S3_BUCKET=my-telemetry-bucket
//	S3_PATH_PREFIX=prod/traces/
//	S3_ENABLE_SSE=true
//
// Example usage:
//
//	// Create resolver from environment variables
//	resolver := propertyresolver.NewEnvResolver()
//
//	// Load and validate S3 configuration
//	cfg, err := s3.FromResolver(resolver)
//	if err != nil {
//	    log.Fatalf("Failed to load S3 config: %v", err)
//	}
//
//	// Use configuration to create S3 client
//	awsCfg := aws.Config{Region: string(cfg.Region)}
//	s3Client := s3.NewFromConfig(awsCfg)
//
// LocalStack example:
//
//	// Set environment variables
//	os.Setenv("S3_ENDPOINT_URL", "http://localhost:4566")
//	os.Setenv("S3_BUCKET", "test-bucket")
//	os.Setenv("S3_ACCESS_KEY_ID", "test")
//	os.Setenv("S3_SECRET_ACCESS_KEY", "test")
//	os.Setenv("S3_FORCE_PATH_STYLE", "true")
//
//	cfg, err := s3.FromResolver(resolver)
//	// cfg now configured for LocalStack
//
// Returns an error if:
//   - Decoding fails (invalid values, type mismatches)
//   - Validation fails (missing required fields, invalid combinations)
func FromResolver(r resolver.PropertyResolver) (Config, error) {
	cfg := Defaults()

	// Decodes values into mapstructure
	if err := decodeutil.DecodePrefixInto(r, S3_PREFIX, &cfg); err != nil {
		return Config{}, fmt.Errorf("s3 decode: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
