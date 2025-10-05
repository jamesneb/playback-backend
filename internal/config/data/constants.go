package data

import (
	"time"
)

// Environment variable prefix for data processing configuration.
//
// All data processing configuration environment variables start with this prefix.
// Example variables:
//   - DATA_BATCH_SIZE
//   - DATA_WORKER_COUNT
//   - DATA_ENABLE_COMPRESSION
const (
	DATA_PREFIX = "DATA_"
)

// Time period constants for retention policies.
//
// These constants provide readable time durations for data retention configuration.
const (
	ONE_DAY   = 24 * time.Hour // 24 hours
	ONE_WEEK  = 7 * ONE_DAY    // 7 days = 168 hours
	ONE_MONTH = 30 * ONE_DAY   // 30 days = 720 hours
	ONE_YEAR  = 365 * ONE_DAY  // 365 days = 8760 hours
	TEN_YEARS = 10 * ONE_YEAR  // 10 years = 87600 hours
)

// Batch processing constants for data processing pipelines.
//
// Batching improves throughput by reducing per-record overhead.
const (
	// MIN_BATCH_SIZE is the minimum number of records per batch.
	// Single record batches are allowed but inefficient.
	MIN_BATCH_SIZE = 1

	// MAX_BATCH_SIZE is the maximum number of records per batch.
	// 100,000 is practical limit for memory and processing time.
	MAX_BATCH_SIZE = 100_000

	// DEFAULT_BATCH_SIZE is the default number of records per batch.
	// 1000 provides good balance for typical telemetry workloads.
	DEFAULT_BATCH_SIZE = 1000

	// MIN_FLUSH_INTERVAL is the minimum time to wait before flushing.
	// 100ms is practical minimum for meaningful buffering.
	MIN_FLUSH_INTERVAL = 100 * time.Millisecond

	// MAX_FLUSH_INTERVAL is the maximum time to wait before flushing.
	// 1 minute prevents stale data accumulation.
	MAX_FLUSH_INTERVAL = 1 * time.Minute

	// DEFAULT_FLUSH_INTERVAL is the default maximum time before flushing.
	// 5 seconds balances latency and throughput.
	DEFAULT_FLUSH_INTERVAL = 5 * time.Second

	// MIN_WORKER_COUNT is the minimum number of parallel workers.
	// At least one worker is required for processing.
	MIN_WORKER_COUNT = 1

	// MAX_WORKER_COUNT is the maximum number of parallel workers.
	// 1000 workers is practical limit for most systems.
	MAX_WORKER_COUNT = 1000

	// DEFAULT_WORKER_COUNT is the default number of parallel workers.
	// 4 workers suitable for typical multi-core systems.
	DEFAULT_WORKER_COUNT = 4

	// MIN_QUEUE_SIZE is the minimum queue size for buffering.
	// 100 items minimum for meaningful buffering.
	MIN_QUEUE_SIZE = 100

	// MAX_QUEUE_SIZE is the maximum queue size for buffering.
	// 10 million items is practical memory limit.
	MAX_QUEUE_SIZE = 10_000_000

	// DEFAULT_QUEUE_SIZE is the default queue size for buffering.
	// 10,000 items provides reasonable buffer.
	DEFAULT_QUEUE_SIZE = 10_000
)

// Retention constants define data lifecycle policies.
//
// Retention periods control how long data is kept before automatic deletion.
const (
	// MIN_RETENTION is the minimum data retention period.
	// At least one day to allow for debugging and analysis.
	MIN_RETENTION = ONE_DAY

	// MAX_RETENTION is the maximum data retention period.
	// 10 years is maximum for compliance and audit requirements.
	MAX_RETENTION = TEN_YEARS

	// DEFAULT_RETENTION_TRACES is the default retention for distributed traces.
	// 7 days is sufficient for recent debugging without excessive storage.
	DEFAULT_RETENTION_TRACES = ONE_WEEK

	// DEFAULT_RETENTION_METRICS is the default retention for time-series metrics.
	// 30 days allows trending and analysis without large storage costs.
	DEFAULT_RETENTION_METRICS = ONE_MONTH

	// DEFAULT_RETENTION_LOGS is the default retention for application logs.
	// 7 days is sufficient for recent debugging without excessive storage.
	DEFAULT_RETENTION_LOGS = ONE_WEEK
)

// Cleanup interval constants control automatic data deletion frequency.
//
// Cleanup runs periodically to delete data exceeding retention periods.
const (
	// MIN_CLEANUP_INTERVAL is the minimum time between cleanup runs.
	// 1 hour is minimum to avoid excessive overhead.
	MIN_CLEANUP_INTERVAL = 1 * time.Hour

	// MAX_CLEANUP_INTERVAL is the maximum time between cleanup runs.
	// 1 week is maximum to prevent unbounded storage growth.
	MAX_CLEANUP_INTERVAL = ONE_WEEK

	// DEFAULT_CLEANUP_INTERVAL is the default time between cleanup runs.
	// Daily cleanup balances storage management and overhead.
	DEFAULT_CLEANUP_INTERVAL = ONE_DAY
)

// Default configuration values for data processing.
//
// These booleans enable/disable various processing features.
const (
	// DEFAULT_ENABLE_COMPRESSION enables gzip compression by default.
	// Reduces storage and network bandwidth for text-heavy telemetry data.
	DEFAULT_ENABLE_COMPRESSION = true

	// DEFAULT_ENABLE_ASYNC enables asynchronous processing by default.
	// Improves producer latency by queuing records for background processing.
	DEFAULT_ENABLE_ASYNC = true

	// DEFAULT_ENABLE_PARALLEL enables parallel processing by default.
	// Uses multiple workers for higher throughput on multi-core systems.
	DEFAULT_ENABLE_PARALLEL = true

	// DEFAULT_ENABLE_VALIDATION enables data validation by default.
	// Ensures data quality by validating before processing.
	DEFAULT_ENABLE_VALIDATION = true

	// DEFAULT_ENABLE_AUTO_CLEANUP enables automatic cleanup by default.
	// Prevents unbounded storage growth by deleting expired data.
	DEFAULT_ENABLE_AUTO_CLEANUP = true
)
