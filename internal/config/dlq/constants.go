package dlq

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

// Environment variable prefix for DLQ configuration.
//
// All DLQ configuration environment variables start with this prefix.
// Example variables:
//   - DLQ_ENABLED
//   - DLQ_QUEUE_NAME
//   - DLQ_RETENTION_PERIOD
const (
	DLQ_PREFIX = "DLQ_"
)

// Time constants for DLQ configuration.
const (
	ONE_HOUR = 1 * time.Hour // 1 hour = 60 minutes
	ONE_DAY  = 24 * ONE_HOUR // 1 day = 24 hours
	ONE_WEEK = 7 * ONE_DAY   // 1 week = 7 days = 168 hours
)

// Message size constants for DLQ message limits.
//
// AWS SQS standard queue supports up to 256 KB per message.
// Extended client library supports up to 2 GB (stores payload in S3).
const (
	// MIN_MESSAGE_SIZE is the minimum message size for DLQ.
	// 1KB is practical minimum for meaningful messages.
	MIN_MESSAGE_SIZE = base.Byte(1 * base.KILO) // 1KB

	// MAX_MESSAGE_SIZE is the maximum message size for DLQ.
	// 10MB is practical limit for most queue systems.
	// AWS SQS Extended Client supports up to 2GB via S3.
	MAX_MESSAGE_SIZE = base.Byte(10 * base.MEGA) // 10MB

	// DEFAULT_MESSAGE_SIZE is the default maximum message size.
	// 256KB matches AWS SQS standard queue limit.
	// See: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/quotas-messages.html
	DEFAULT_MESSAGE_SIZE = base.Byte(256 * base.KILO) // 256KB
)

// Retention period constants for DLQ message retention.
//
// Retention period determines how long failed messages are kept before deletion.
// AWS SQS supports retention from 1 minute to 14 days.
const (
	// MIN_RETENTION_PERIOD is the minimum retention period.
	// 1 hour minimum to allow for investigation and reprocessing.
	MIN_RETENTION_PERIOD = ONE_HOUR

	// MAX_RETENTION_PERIOD is the maximum retention period.
	// 14 days is AWS SQS maximum retention period.
	// See: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-quotas.html
	MAX_RETENTION_PERIOD = 2 * ONE_WEEK // 14 days

	// DEFAULT_RETENTION_PERIOD is the default retention period.
	// 7 days balances investigation time and storage costs.
	DEFAULT_RETENTION_PERIOD = ONE_WEEK
)

// Queue capacity constants for DLQ size limits.
//
// Queue capacity limits the maximum number of messages stored in the DLQ.
const (
	// MIN_QUEUE_CAPACITY is the minimum queue capacity.
	// 100 messages minimum for meaningful buffering.
	MIN_QUEUE_CAPACITY = 100

	// MAX_QUEUE_CAPACITY is the maximum queue capacity.
	// 1 million messages is practical limit for most systems.
	MAX_QUEUE_CAPACITY = 1_000_000

	// DEFAULT_QUEUE_CAPACITY is the default queue capacity.
	// 10,000 messages handles typical failure scenarios.
	DEFAULT_QUEUE_CAPACITY = 10_000
)

// Reprocessing cooldown constants for DLQ message reprocessing.
//
// Cooldown period prevents immediate reprocessing before fixes are deployed.
const (
	// MIN_REPROCESSING_COOLDOWN is the minimum cooldown period.
	// 1 minute minimum to allow for system stabilization.
	MIN_REPROCESSING_COOLDOWN = 1 * time.Minute

	// MAX_REPROCESSING_COOLDOWN is the maximum cooldown period.
	// 24 hours maximum to prevent excessive delays.
	MAX_REPROCESSING_COOLDOWN = 24 * ONE_HOUR

	// DEFAULT_REPROCESSING_COOLDOWN is the default cooldown period.
	// 1 hour allows time for fix deployment and stabilization.
	DEFAULT_REPROCESSING_COOLDOWN = ONE_HOUR
)

// Local buffer constants for in-memory buffering before sending to DLQ.
//
// Local buffer reduces network overhead by batching messages.
const (
	// MIN_LOCAL_BUFFER_SIZE is the minimum local buffer size.
	// 10 messages minimum for basic buffering.
	MIN_LOCAL_BUFFER_SIZE = 10

	// MAX_LOCAL_BUFFER_SIZE is the maximum local buffer size.
	// 100,000 messages is practical memory limit.
	MAX_LOCAL_BUFFER_SIZE = 100_000

	// DEFAULT_LOCAL_BUFFER_SIZE is the default local buffer size.
	// 1000 messages provides good balance between memory and batching efficiency.
	DEFAULT_LOCAL_BUFFER_SIZE = 1000
)

// Default configuration values for DLQ.
//
// These constants define sensible defaults for production use.
const (
	// DEFAULT_QUEUE_NAME is the default DLQ name.
	// Use different names for different environments (prod-, staging-, dev-).
	DEFAULT_QUEUE_NAME = "failed-events-dlq"

	// DEFAULT_REGION is the default AWS region for DLQ.
	// Set to us-east-1 as the most common region.
	DEFAULT_REGION = base.AWS_US_EAST_1

	// DEFAULT_ENABLED enables DLQ by default.
	// DLQ is critical for production resilience.
	DEFAULT_ENABLED = true

	// DEFAULT_ENABLE_REPROCESSING disables automatic reprocessing by default.
	// Manual reprocessing is safer and allows verification before retry.
	// Enable only after careful consideration of failure scenarios.
	DEFAULT_ENABLE_REPROCESSING = false
)
