package kinesis

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	KINESIS_PREFIX = "KINESIS_"
)

// Batch processing constants
const (
	MIN_BATCH_SIZE     = 1
	MAX_BATCH_SIZE     = 500 // Kinesis limit
	DEFAULT_BATCH_SIZE = 100

	MIN_FLUSH_INTERVAL     = 100 * time.Millisecond
	MAX_FLUSH_INTERVAL     = 1 * time.Minute
	DEFAULT_FLUSH_INTERVAL = 5 * time.Second
)

// Retry constants
const (
	MIN_RETRIES     = 0
	MAX_RETRIES     = 10
	DEFAULT_RETRIES = 3

	MIN_RETRY_DELAY     = 100 * time.Millisecond
	MAX_RETRY_DELAY     = 1 * time.Minute
	DEFAULT_RETRY_DELAY = 1 * time.Second
)

// Default values
const (
	DEFAULT_REGION         = base.AWS_US_EAST_1
	DEFAULT_TRACES_STREAM  = "telemetry-traces"
	DEFAULT_METRICS_STREAM = "telemetry-metrics"
	DEFAULT_LOGS_STREAM    = "telemetry-logs"
)
