package data

import (
	"time"
)

const (
	DATA_PREFIX = "DATA_"
)

// Time period constants
const (
	ONE_DAY   = 24 * time.Hour
	ONE_WEEK  = 7 * ONE_DAY
	ONE_MONTH = 30 * ONE_DAY
	ONE_YEAR  = 365 * ONE_DAY
	TEN_YEARS = 10 * ONE_YEAR
)

// Batch processing constants
const (
	MIN_BATCH_SIZE     = 1
	MAX_BATCH_SIZE     = 100_000
	DEFAULT_BATCH_SIZE = 1000

	MIN_FLUSH_INTERVAL     = 100 * time.Millisecond
	MAX_FLUSH_INTERVAL     = 1 * time.Minute
	DEFAULT_FLUSH_INTERVAL = 5 * time.Second

	MIN_WORKER_COUNT     = 1
	MAX_WORKER_COUNT     = 1000
	DEFAULT_WORKER_COUNT = 4

	MIN_QUEUE_SIZE     = 100
	MAX_QUEUE_SIZE     = 10_000_000
	DEFAULT_QUEUE_SIZE = 10_000
)

// Retention constants
const (
	MIN_RETENTION = ONE_DAY
	MAX_RETENTION = TEN_YEARS

	DEFAULT_RETENTION_TRACES  = ONE_WEEK
	DEFAULT_RETENTION_METRICS = ONE_MONTH
	DEFAULT_RETENTION_LOGS    = ONE_WEEK
)

// Cleanup interval constants
// How often to run automated cleanup of expired data based on retention policies
const (
	MIN_CLEANUP_INTERVAL     = 1 * time.Hour
	MAX_CLEANUP_INTERVAL     = ONE_WEEK
	DEFAULT_CLEANUP_INTERVAL = ONE_DAY
)

// Default values
const (
	DEFAULT_ENABLE_COMPRESSION  = true
	DEFAULT_ENABLE_ASYNC        = true // Process data asynchronously
	DEFAULT_ENABLE_PARALLEL     = true // Process data in parallel workers
	DEFAULT_ENABLE_VALIDATION   = true // Validate data before processing
	DEFAULT_ENABLE_AUTO_CLEANUP = true // Automatically cleanup expired data
)
