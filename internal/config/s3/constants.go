package s3

import (
	"github.com/jamesneb/playback-backend/internal/config/base"
)

// Environment variable prefix for S3 configuration.
//
// All S3 configuration environment variables start with this prefix.
// Example variables:
//   - S3_REGION
//   - S3_BUCKET
//   - S3_ENDPOINT_URL
//   - S3_ACCESS_KEY_ID
//   - S3_SECRET_ACCESS_KEY
const (
	S3_PREFIX = "S3_"
)

// Default configuration values for S3.
//
// These constants define sensible defaults for AWS S3 in production.
const (
	// DEFAULT_REGION is the default AWS region for S3 operations.
	// Set to us-east-1 (US East, N. Virginia) as it is the most common and often lowest-cost region.
	// This is also the region where many AWS services are launched first.
	DEFAULT_REGION = base.AWS_US_EAST_1

	// DEFAULT_FORCE_PATH_STYLE controls the S3 URL style.
	// False means use virtual-hosted style (https://bucket.s3.amazonaws.com/key)
	// which is the modern AWS S3 standard.
	//
	// Set to true only when using LocalStack, MinIO, or other S3-compatible services
	// that require path-style URLs (https://s3.amazonaws.com/bucket/key).
	DEFAULT_FORCE_PATH_STYLE = false

	// DEFAULT_ENABLE_SSE enables server-side encryption by default.
	// True means objects are encrypted at rest using AES-256 with AWS-managed keys (SSE-S3).
	//
	// Encryption is best practice for security and compliance with minimal performance overhead.
	// Set to false only for local development or testing environments.
	DEFAULT_ENABLE_SSE = true
)
