// Package s3 defines configuration for AWS S3 storage connections.
//
// S3 (Simple Storage Service) is AWS's object storage service. This package provides
// configuration for S3 client connections including bucket names, regions, credentials,
// and endpoint customization for local development (LocalStack).
//
// # AWS Credentials
//
// S3 requires AWS credentials for authentication:
//   - AccessKeyID: AWS access key
//   - SecretAccessKey: AWS secret key
//   - Region: AWS region where bucket is located
//
// In production, prefer IAM roles over hardcoded credentials.
//
// # LocalStack Support
//
// For local development, EndpointURL can point to LocalStack:
//
//	S3_ENDPOINT_URL=http://localhost:4566
//	S3_FORCE_PATH_STYLE=true
//
// ForcePathStyle is required for LocalStack compatibility.
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// S3_ prefix:
//
//	S3_REGION=us-east-1
//	S3_BUCKET=telemetry-data
//	S3_ENDPOINT_URL=http://localhost:4566
//	S3_ACCESS_KEY_ID=test
//	S3_SECRET_ACCESS_KEY=test
//	S3_FORCE_PATH_STYLE=true
//
// # Files in This Package
//
// constants.go:
//   - S3_PREFIX for environment variable namespacing
//   - Default values (region, force path style)
//
// section.go:
//   - Config struct with S3 parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package s3
