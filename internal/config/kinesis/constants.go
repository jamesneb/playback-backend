package kinesis

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

// Environment variable prefix for Kinesis configuration.
//
// All Kinesis configuration environment variables start with this prefix.
// Example variables:
//   - KINESIS_REGION
//   - KINESIS_TRACES_STREAM
//   - KINESIS_BATCH_SIZE
//   - KINESIS_FLUSH_INTERVAL
const (
	KINESIS_PREFIX = "KINESIS_"
)

// Batch processing constants for Kinesis PutRecords operations.
//
// Kinesis supports batching up to 500 records per API call for efficiency.
// Batching reduces cost (fewer API calls) and improves throughput.
const (
	// MIN_BATCH_SIZE is the minimum number of records per batch.
	// Single record batches are allowed but inefficient.
	MIN_BATCH_SIZE = 1

	// MAX_BATCH_SIZE is the maximum number of records per batch.
	// Set by AWS Kinesis PutRecords API limit of 500 records per call.
	// See: https://docs.aws.amazon.com/kinesis/latest/APIReference/API_PutRecords.html
	MAX_BATCH_SIZE = 500

	// DEFAULT_BATCH_SIZE is the default number of records per batch.
	// 100 provides a good balance between throughput and latency.
	// For high-volume scenarios, consider increasing to 500.
	// For low-latency scenarios, consider decreasing to 10-50.
	DEFAULT_BATCH_SIZE = 100

	// MIN_FLUSH_INTERVAL is the minimum time to wait before flushing a partial batch.
	// 100ms is the practical minimum for meaningful buffering.
	MIN_FLUSH_INTERVAL = 100 * time.Millisecond

	// MAX_FLUSH_INTERVAL is the maximum time to wait before flushing a partial batch.
	// 1 minute is the maximum to prevent stale data.
	MAX_FLUSH_INTERVAL = 1 * time.Minute

	// DEFAULT_FLUSH_INTERVAL is the default maximum time before flushing.
	// 5 seconds balances latency and throughput for typical workloads.
	// Records are sent either when batch is full or after this interval.
	DEFAULT_FLUSH_INTERVAL = 5 * time.Second
)

// Retry constants for handling transient failures and throttling.
//
// Kinesis operations may fail due to:
//   - ProvisionedThroughputExceededException (throttling)
//   - Network errors
//   - Temporary service issues
//
// Retries use exponential backoff with jitter to prevent thundering herd.
const (
	// MIN_RETRIES is the minimum number of retry attempts.
	// 0 means fail immediately on first error (not recommended for production).
	MIN_RETRIES = 0

	// MAX_RETRIES is the maximum number of retry attempts.
	// 10 retries with exponential backoff can span several minutes.
	// Higher values provide resilience but delay failure detection.
	MAX_RETRIES = 10

	// DEFAULT_RETRIES is the default number of retry attempts.
	// 3 retries provides good balance: tolerates transient errors without excessive delay.
	// With 1s initial delay: 1s, 2s, 4s = ~7s total retry time.
	DEFAULT_RETRIES = 3

	// MIN_RETRY_DELAY is the minimum initial delay between retry attempts.
	// 100ms is the minimum practical delay for network operations.
	MIN_RETRY_DELAY = 100 * time.Millisecond

	// MAX_RETRY_DELAY is the maximum initial delay between retry attempts.
	// 1 minute is the maximum to prevent excessive wait times.
	// With exponential backoff, actual delays can be longer.
	MAX_RETRY_DELAY = 1 * time.Minute

	// DEFAULT_RETRY_DELAY is the default initial delay between retry attempts.
	// 1 second is a reasonable starting point for exponential backoff.
	// Subsequent retries double the delay: 1s, 2s, 4s, 8s, etc.
	DEFAULT_RETRY_DELAY = 1 * time.Second
)

// Default configuration values for Kinesis.
//
// These constants define sensible defaults for AWS Kinesis in production.
const (
	// DEFAULT_REGION is the default AWS region for Kinesis operations.
	// Set to us-east-1 (US East, N. Virginia) as the most common region.
	DEFAULT_REGION = base.AWS_US_EAST_1

	// DEFAULT_TRACES_STREAM is the default Kinesis stream name for distributed traces.
	// Stores OpenTelemetry traces, Jaeger spans, and similar tracing data.
	DEFAULT_TRACES_STREAM = "telemetry-traces"

	// DEFAULT_METRICS_STREAM is the default Kinesis stream name for time-series metrics.
	// Stores Prometheus metrics, StatsD metrics, and similar metric data.
	DEFAULT_METRICS_STREAM = "telemetry-metrics"

	// DEFAULT_LOGS_STREAM is the default Kinesis stream name for application logs.
	// Stores structured logs, JSON logs, and similar log data.
	DEFAULT_LOGS_STREAM = "telemetry-logs"
)
