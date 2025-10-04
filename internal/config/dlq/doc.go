// Package dlq defines configuration for dead letter queue handling.
//
// This package provides configuration for handling failed messages that cannot be
// processed successfully. Messages are moved to a DLQ after exhausting retries.
//
// # Dead Letter Queue
//
// A DLQ captures messages that:
//   - Failed processing after max retries
//   - Caused validation errors
//   - Exceeded processing timeouts
//   - Triggered circuit breaker failures
//
// # Configuration
//
// Queue settings:
//   - QueueName: Name of the DLQ (SQS/Kinesis stream/etc.)
//   - MaxMessageSize: Maximum size of a single message
//
// Retention and limits:
//   - RetentionPeriod: How long messages are kept in DLQ
//   - MaxMessages: Maximum number of messages to retain
//
// Processing:
//   - EnableReprocessing: Allow retrying messages from DLQ
//   - ReprocessingCooldown: Minimum time before reprocessing
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// DLQ_ prefix:
//
//	DLQ_QUEUE_NAME=failed-events
//	DLQ_MAX_MESSAGE_SIZE=256KB
//	DLQ_RETENTION_PERIOD=168h
//	DLQ_MAX_MESSAGES=10000
//	DLQ_ENABLE_REPROCESSING=true
//	DLQ_REPROCESSING_COOLDOWN=1h
//
// # Files in This Package
//
// constants.go:
//   - DLQ_PREFIX for environment variable namespacing
//   - Default values (queue name, sizes, retention)
//   - Min/max bounds for validation
//
// section.go:
//   - Config struct with DLQ parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package dlq
