// Package kinesis defines configuration for AWS Kinesis streaming connections.
//
// Kinesis is AWS's real-time data streaming service. This package provides
// configuration for Kinesis producer clients including stream names, regions,
// credentials, batching, and retry behavior.
//
// # Stream Configuration
//
// Kinesis uses separate streams for different telemetry types:
//   - TracesStream: Distributed traces
//   - MetricsStream: Time-series metrics
//   - LogsStream: Application logs
//
// # Batching and Performance
//
// Kinesis clients batch records for efficiency:
//   - BatchSize: Records per batch
//   - FlushInterval: Maximum time before forcing flush
//   - MaxRetries: Retry attempts on transient failures
//   - RetryDelay: Delay between retry attempts
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// KINESIS_ prefix:
//
//	KINESIS_REGION=us-east-1
//	KINESIS_ENDPOINT_URL=http://localhost:4566
//	KINESIS_ACCESS_KEY_ID=test
//	KINESIS_SECRET_ACCESS_KEY=test
//	KINESIS_TRACES_STREAM=telemetry-traces
//	KINESIS_METRICS_STREAM=telemetry-metrics
//	KINESIS_LOGS_STREAM=telemetry-logs
//	KINESIS_BATCH_SIZE=100
//	KINESIS_FLUSH_INTERVAL=5s
//	KINESIS_MAX_RETRIES=3
//	KINESIS_RETRY_DELAY=1s
//
// # Files in This Package
//
// constants.go:
//   - KINESIS_PREFIX for environment variable namespacing
//   - Default values (region, stream names, batching, retries)
//   - Min/max bounds for validation
//
// section.go:
//   - Config struct with Kinesis parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package kinesis
