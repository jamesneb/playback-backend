// Package dlq defines configuration for Dead Letter Queue (DLQ) handling of failed messages.
//
// A Dead Letter Queue captures messages that cannot be successfully processed after multiple retry attempts.
// This package provides configuration for DLQ storage, retention policies, and reprocessing strategies,
// enabling robust error handling and message recovery in distributed systems.
//
// AWS SQS DLQ Documentation: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html
// DLQ Pattern: https://aws.amazon.com/what-is/dead-letter-queue/
//
// # Overview
//
// Dead Letter Queue configuration enables:
//
//   - Capture failed messages for analysis and debugging
//   - Prevent poison pill messages from blocking processing
//   - Configurable retention periods for failed messages
//   - Message reprocessing after fixing underlying issues
//   - Local buffering for high availability
//   - AWS SQS or custom queue integration
//
// # Configuration Keys
//
// All settings use the DLQ_ prefix and support environment variable overrides:
//
//	DLQ_ENABLED                 - Enable DLQ (default: true)
//	DLQ_QUEUE_NAME              - Queue name (default: failed-events-dlq)
//	DLQ_REGION                  - AWS region (default: us-east-1)
//	DLQ_MAX_MESSAGE_SIZE        - Max message size (default: 256KB, range: 1KB-10MB)
//	DLQ_RETENTION_PERIOD        - Retention period (default: 7d, range: 1h-14d)
//	DLQ_QUEUE_CAPACITY          - Queue capacity (default: 10000, range: 100-1000000)
//	DLQ_LOCAL_BUFFER_SIZE       - Local buffer size (default: 1000, range: 10-100000)
//	DLQ_ENABLE_REPROCESSING     - Allow reprocessing (default: false)
//	DLQ_REPROCESSING_COOLDOWN   - Cooldown period (default: 1h, range: 1m-24h)
//
// # Dead Letter Queue Pattern
//
// DLQ captures messages that fail processing:
//
//	Main Queue → Process → Success ✓
//	          → Process → Fail → Retry 1 → Fail → Retry 2 → Fail → DLQ
//
// Messages enter DLQ when:
//   - Maximum retry attempts exhausted
//   - Validation failures (malformed data)
//   - Processing timeouts exceeded
//   - Circuit breaker open (downstream failure)
//   - Poison pill messages (consistently failing)
//   - Application exceptions during processing
//
// DLQ benefits:
//   - Prevents blocking: Failed messages don't block queue processing
//   - Preserves data: Failed messages saved for analysis
//   - Debugging: Inspect failed messages to identify issues
//   - Recovery: Reprocess messages after fixing problems
//   - Monitoring: Track failure patterns and rates
//
// # Message Size Limits
//
//	DLQ_MAX_MESSAGE_SIZE=256KB    # AWS SQS standard message size
//
// Message size considerations:
//
// AWS SQS limits:
//   - Standard queue: 256 KB per message
//   - Extended client library: Up to 2 GB (stores in S3)
//   - Batch request: 256 KB total
//
// Custom queue limits:
//   - Configure based on your infrastructure
//   - Consider network bandwidth
//   - Balance memory usage
//
// Large message strategies:
//   - Store payload in S3, send reference in message
//   - Compress message content (gzip)
//   - Use SQS Extended Client Library
//   - Split large messages into chunks
//
// # Retention Policies
//
//	DLQ_RETENTION_PERIOD=168h     # 7 days (AWS SQS default)
//
// Retention period determines how long failed messages are kept:
//
// Short retention (1-3 days):
//   - Pros: Lower storage costs, forces timely resolution
//   - Cons: Messages may be lost if not processed quickly
//   - Use case: Fast-moving issues, active monitoring
//
// Medium retention (7 days) [RECOMMENDED]:
//   - Pros: Balance of time and cost
//   - Cons: Moderate storage costs
//   - Use case: Standard operations, weekly review
//
// Long retention (14 days, AWS SQS maximum):
//   - Pros: Maximum time for investigation
//   - Cons: Higher storage costs
//   - Use case: Compliance, audit requirements
//
// Retention best practices:
//   - Monitor DLQ depth (number of messages)
//   - Alert on growing DLQ (indicates systemic issue)
//   - Review and process DLQ messages regularly
//   - Archive critical failed messages before expiration
//   - Balance retention time with storage costs
//
// # Queue Capacity and Buffering
//
//	DLQ_QUEUE_CAPACITY=10000      # Maximum messages in queue
//	DLQ_LOCAL_BUFFER_SIZE=1000    # Local buffer before flushing
//
// Queue capacity:
//   - Limits total messages in DLQ
//   - When full, oldest messages may be dropped
//   - Size based on expected failure rate
//
// Local buffer:
//   - Buffers messages in memory before sending to DLQ
//   - Reduces network calls (batch sending)
//   - Provides resilience if DLQ unavailable
//   - Size: 10-100 messages typical
//
// Capacity planning:
//
// Normal operations:
//
//	Failure rate: 0.1% (1 in 1000 messages)
//	Throughput: 10,000 messages/hour
//	Failed messages: 10/hour
//	DLQ capacity: 1,680 messages (7 days retention)
//	Recommended: 10,000 capacity (buffer for spikes)
//
// Incident scenario:
//
//	Failure rate: 10% (service degradation)
//	Throughput: 10,000 messages/hour
//	Failed messages: 1,000/hour
//	DLQ capacity: 168,000 messages (7 days retention)
//	Recommended: 200,000+ capacity
//
// # Reprocessing Failed Messages
//
//	DLQ_ENABLE_REPROCESSING=false
//	DLQ_REPROCESSING_COOLDOWN=1h
//
// Reprocessing allows retrying failed messages after fixing issues:
//
// Reprocessing workflow:
//  1. Identify root cause of failures
//  2. Deploy fix (code change, configuration, dependency)
//  3. Wait for cooldown period (system stabilization)
//  4. Trigger reprocessing of DLQ messages
//  5. Monitor reprocessing success rate
//
// Cooldown period prevents:
//   - Immediate reprocessing before fix deployed
//   - Overloading system during recovery
//   - Repeated failures if fix incomplete
//
// Reprocessing strategies:
//
// Manual reprocessing (recommended):
//   - DLQ_ENABLE_REPROCESSING=false
//   - Review failed messages
//   - Fix underlying issue
//   - Manually trigger reprocessing
//   - Monitor results
//
// Automatic reprocessing (use carefully):
//   - DLQ_ENABLE_REPROCESSING=true
//   - DLQ_REPROCESSING_COOLDOWN=1h
//   - System automatically retries after cooldown
//   - Risk: Repeated failures if issue not fixed
//
// # Poison Pill Handling
//
// Poison pill: A message that consistently fails processing, blocking the queue.
//
// Poison pill characteristics:
//   - Fails validation (malformed data)
//   - Triggers application exception
//   - Exceeds processing timeout
//   - Cannot be processed by design
//
// Detection strategies:
//   - Track retry count per message
//   - Monitor message processing time
//   - Log failure reasons
//   - Identify patterns in failures
//
// Handling strategies:
//
// Strategy 1: DLQ (recommended):
//   - Move to DLQ after max retries
//   - Prevents blocking main queue
//   - Allows manual inspection
//   - Can reprocess after fix
//
// Strategy 2: Dead letter storage:
//   - Store in S3/database for analysis
//   - Remove from queue immediately
//   - Long-term retention
//   - Batch analysis possible
//
// Strategy 3: Discard:
//   - Log and discard invalid messages
//   - Use only for non-critical data
//   - Ensure monitoring/alerting
//   - Risk: Data loss
//
// # Retry Strategies
//
// Exponential backoff with DLQ:
//
//	Attempt 1: Process immediately → Fail
//	Attempt 2: Wait 1s → Process → Fail
//	Attempt 3: Wait 2s → Process → Fail
//	Attempt 4: Wait 4s → Process → Fail
//	After max retries: Move to DLQ
//
// Failure classification:
//
// Transient failures (retryable):
//   - Network errors
//   - Timeout errors
//   - Rate limiting (429)
//   - Temporary service unavailability (503)
//   - Action: Retry with backoff
//
// Permanent failures (not retryable):
//   - Validation errors (400)
//   - Authentication errors (401, 403)
//   - Not found errors (404)
//   - Malformed data
//   - Action: Move to DLQ immediately
//
// # Example Usage
//
//	// Load DLQ configuration
//	cfg, err := dlq.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create SQS DLQ client
//	sqsClient := sqs.NewFromConfig(awsConfig)
//
//	// Process message with DLQ fallback
//	func processMessage(msg Message) error {
//	    retries := 0
//	    maxRetries := 3
//
//	    for retries < maxRetries {
//	        err := doProcessing(msg)
//	        if err == nil {
//	            return nil // Success
//	        }
//
//	        if isPermanentError(err) {
//	            // Don't retry permanent errors
//	            break
//	        }
//
//	        retries++
//	        backoff := time.Duration(1<<retries) * time.Second
//	        time.Sleep(backoff)
//	    }
//
//	    // All retries exhausted, send to DLQ
//	    if cfg.Enabled {
//	        return sendToDLQ(sqsClient, cfg.QueueName, msg)
//	    }
//
//	    return fmt.Errorf("processing failed after %d retries", maxRetries)
//	}
//
// # Best Practices
//
// Production configuration:
//
//	DLQ_ENABLED=true
//	DLQ_QUEUE_NAME=prod-failed-events-dlq
//	DLQ_REGION=us-east-1
//	DLQ_MAX_MESSAGE_SIZE=256KB
//	DLQ_RETENTION_PERIOD=168h         # 7 days
//	DLQ_QUEUE_CAPACITY=100000
//	DLQ_LOCAL_BUFFER_SIZE=1000
//	DLQ_ENABLE_REPROCESSING=false     # Manual reprocessing
//	DLQ_REPROCESSING_COOLDOWN=1h
//
// Development configuration:
//
//	DLQ_ENABLED=true
//	DLQ_QUEUE_NAME=dev-failed-events-dlq
//	DLQ_RETENTION_PERIOD=24h          # 1 day
//	DLQ_QUEUE_CAPACITY=1000
//	DLQ_LOCAL_BUFFER_SIZE=100
//	DLQ_ENABLE_REPROCESSING=true      # Easier testing
//
// Monitoring and alerting:
//   - Alert on DLQ depth > threshold (e.g., 100 messages)
//   - Track DLQ ingestion rate (messages/hour)
//   - Monitor oldest message age
//   - Alert on approaching retention expiration
//   - Track reprocessing success rate
//   - Dashboard for failure patterns
//
// DLQ review process:
//  1. Daily: Check DLQ depth and trends
//  2. Weekly: Review failed message samples
//  3. Monthly: Analyze failure patterns
//  4. Identify root causes and fix
//  5. Reprocess after fixes deployed
//  6. Archive critical failed messages
//
// # Troubleshooting
//
// Growing DLQ:
//
//	Problem: DLQ message count increasing
//	Fix: Identify root cause of failures
//	Fix: Check application logs for errors
//	Fix: Verify downstream services are healthy
//	Fix: Review recent code/config changes
//
// Message size errors:
//
//	Error: Message exceeds DLQ_MAX_MESSAGE_SIZE
//	Fix: Increase DLQ_MAX_MESSAGE_SIZE if possible
//	Fix: Compress message content before sending
//	Fix: Store large payloads in S3, send reference
//
// Retention expiration:
//
//	Problem: Messages expiring before review
//	Fix: Increase DLQ_RETENTION_PERIOD
//	Fix: Implement automated DLQ monitoring
//	Fix: Archive critical messages to S3
//
// # Cross-References
//
// Related packages:
//   - [base.AWSRegion] - AWS region type definitions
//   - [base.Byte] - Byte size type
//   - [base.Validator] - Validation framework
//   - [circuitbreaker] - Circuit breaker for preventing cascading failures
//
// AWS documentation:
//   - SQS DLQ: https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/sqs-dead-letter-queues.html
//   - DLQ Best Practices: https://aws.amazon.com/blogs/compute/best-practices-for-implementing-amazon-sqs-dead-letter-queues/
//
// # Files in This Package
//
// constants.go:
//   - DLQ_PREFIX for environment variable namespacing
//   - Default values (queue name, sizes, retention)
//   - Min/max bounds for validation
//   - Time constants
//
// section.go:
//   - [Config] struct with DLQ parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package dlq
