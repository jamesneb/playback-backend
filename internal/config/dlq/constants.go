package dlq

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	DLQ_PREFIX = "DLQ_"
)

// Time constants
const (
	ONE_HOUR = 1 * time.Hour
	ONE_DAY  = 24 * ONE_HOUR
	ONE_WEEK = 7 * ONE_DAY
)

// Message size constants
const (
	MIN_MESSAGE_SIZE     = base.Byte(1 * base.KILO)   // 1KB
	MAX_MESSAGE_SIZE     = base.Byte(10 * base.MEGA)  // 10MB
	DEFAULT_MESSAGE_SIZE = base.Byte(256 * base.KILO) // 256KB
)

// Retention period constants
const (
	MIN_RETENTION_PERIOD     = ONE_HOUR
	MAX_RETENTION_PERIOD     = 2 * ONE_WEEK // SQS maximum (14 days)
	DEFAULT_RETENTION_PERIOD = ONE_WEEK
)

// Queue capacity constants
const (
	MIN_QUEUE_CAPACITY     = 100
	MAX_QUEUE_CAPACITY     = 1_000_000
	DEFAULT_QUEUE_CAPACITY = 10_000
)

// Reprocessing cooldown constants
const (
	MIN_REPROCESSING_COOLDOWN     = 1 * time.Minute
	MAX_REPROCESSING_COOLDOWN     = 24 * ONE_HOUR
	DEFAULT_REPROCESSING_COOLDOWN = ONE_HOUR
)

// Local buffer constants
const (
	MIN_LOCAL_BUFFER_SIZE     = 10
	MAX_LOCAL_BUFFER_SIZE     = 100_000
	DEFAULT_LOCAL_BUFFER_SIZE = 1000
)

// Default values
const (
	DEFAULT_QUEUE_NAME          = "failed-events-dlq"
	DEFAULT_REGION              = base.AWS_US_EAST_1
	DEFAULT_ENABLED             = true
	DEFAULT_ENABLE_REPROCESSING = false
)
