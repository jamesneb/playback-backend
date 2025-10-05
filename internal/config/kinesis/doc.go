// Package kinesis defines configuration for AWS Kinesis Data Streams for real-time data ingestion.
//
// Amazon Kinesis Data Streams is a serverless streaming data service that captures and stores
// terabytes of data per hour from hundreds of thousands of sources. This package provides
// configuration for Kinesis producer clients including stream management, batching, partitioning,
// and retry behavior for telemetry data (traces, metrics, logs).
//
// Official AWS Kinesis Documentation: https://docs.aws.amazon.com/kinesis/
// AWS Kinesis Go SDK v2: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/kinesis
// Kinesis Producer Library: https://docs.aws.amazon.com/streams/latest/dev/developing-producers-with-kpl.html
//
// # Overview
//
// Kinesis Data Streams configuration enables:
//
//   - Real-time streaming data ingestion at scale
//   - Separate streams for traces, metrics, and logs
//   - Configurable batching and buffering for throughput optimization
//   - Automatic retries with exponential backoff
//   - Partition key strategy for data distribution across shards
//   - LocalStack integration for local development
//   - Custom endpoint support for testing
//
// # Configuration Keys
//
// All settings use the KINESIS_ prefix and support environment variable overrides:
//
//	KINESIS_REGION              - AWS region (default: us-east-1)
//	KINESIS_ENDPOINT_URL        - Custom endpoint for LocalStack (optional)
//	KINESIS_ACCESS_KEY_ID       - AWS access key (optional, prefer IAM roles)
//	KINESIS_SECRET_ACCESS_KEY   - AWS secret key (optional, prefer IAM roles)
//	KINESIS_SESSION_TOKEN       - AWS session token (optional)
//	KINESIS_TRACES_STREAM       - Stream name for traces (default: telemetry-traces)
//	KINESIS_METRICS_STREAM      - Stream name for metrics (default: telemetry-metrics)
//	KINESIS_LOGS_STREAM         - Stream name for logs (default: telemetry-logs)
//	KINESIS_BATCH_SIZE          - Records per batch (default: 100, range: 1-500)
//	KINESIS_FLUSH_INTERVAL      - Max time before flush (default: 5s, range: 100ms-1m)
//	KINESIS_MAX_RETRIES         - Retry attempts (default: 3, range: 0-10)
//	KINESIS_RETRY_DELAY         - Delay between retries (default: 1s, range: 100ms-1m)
//
// # Kinesis Data Streams Architecture
//
// Stream structure:
//
//	Stream (e.g., telemetry-traces)
//	├── Shard 1 (capacity: 1 MB/s write, 2 MB/s read, 1000 records/s write)
//	├── Shard 2
//	├── Shard 3
//	└── Shard N
//
// Key concepts:
//
// Stream:
//   - Ordered sequence of data records
//   - Composed of one or more shards
//   - Records retained for 24 hours (default) up to 365 days
//   - Stream name must be unique within region
//
// Shard:
//   - Base throughput unit of a Kinesis stream
//   - Write capacity: 1 MB/second or 1,000 records/second
//   - Read capacity: 2 MB/second (shared) or 2 MB/second per consumer (enhanced fan-out)
//   - Records distributed across shards via partition key hash
//
// Partition Key:
//   - Used to distribute records across shards
//   - Same partition key → same shard (maintains ordering)
//   - Hash function determines shard assignment
//   - Choose keys with high cardinality for even distribution
//
// Sequence Number:
//   - Unique identifier for each record within a shard
//   - Monotonically increasing within a shard
//   - Used for ordering and checkpointing
//
// Learn more: https://docs.aws.amazon.com/streams/latest/dev/key-concepts.html
//
// # Throughput and Capacity Planning
//
// Shard capacity limits (per shard):
//
//	Write: 1 MB/s OR 1,000 records/s (whichever comes first)
//	Read (standard): 2 MB/s (shared across all consumers)
//	Read (enhanced fan-out): 2 MB/s per registered consumer
//
// Calculate required shards:
//
//	# Based on data rate
//	Required Shards = (Data Rate MB/s) / 1
//	Example: 10 MB/s → 10 shards
//
//	# Based on record rate
//	Required Shards = (Records/s) / 1000
//	Example: 5,000 records/s → 5 shards
//
// Capacity examples:
//
//	1 shard:    1 MB/s or 1,000 records/s
//	10 shards:  10 MB/s or 10,000 records/s
//	100 shards: 100 MB/s or 100,000 records/s
//
// Cost considerations:
//   - $0.015 per shard-hour (~$11/month per shard)
//   - $0.014 per million PUT payload units (25 KB each)
//   - Extended retention: $0.023 per shard-hour beyond 24h
//   - Enhanced fan-out: $0.015 per shard-hour per consumer, $0.013 per GB retrieved
//
// # Sharding Strategies
//
// Strategy 1: Random distribution (high throughput, no ordering):
//
//	partitionKey := uuid.New().String()
//	// Records distributed evenly across all shards
//	// Maximum throughput, no ordering guarantees
//
// Strategy 2: Tenant-based partitioning (per-tenant ordering):
//
//	partitionKey := fmt.Sprintf("tenant-%s", tenantID)
//	// All records for same tenant go to same shard
//	// Maintains ordering per tenant
//	// Risk: Hot shards if tenant has high volume
//
// Strategy 3: Time-based partitioning (time-ordered processing):
//
//	partitionKey := time.Now().Format("2006-01-02-15")
//	// Records grouped by hour
//	// Useful for time-series analysis
//	// Risk: All current data goes to same shard
//
// Strategy 4: Composite partitioning (balanced):
//
//	partitionKey := fmt.Sprintf("%s-%d", tenantID, time.Now().Unix()%10)
//	// Distributes tenant data across 10 shards
//	// Balances ordering and throughput
//
// Best practices:
//   - Use high-cardinality keys (avoid hot shards)
//   - Monitor shard metrics (avoid exceeding 1 MB/s or 1000 records/s per shard)
//   - Consider resharding if consistently over/under capacity
//   - Use enhanced fan-out for multiple consumers
//
// # Batching and Buffering
//
// Kinesis supports batching up to 500 records per PutRecords API call for efficiency:
//
//	KINESIS_BATCH_SIZE=100        # Records per batch (1-500)
//	KINESIS_FLUSH_INTERVAL=5s     # Max wait before flush
//
// Batching tradeoffs:
//
// Large batch size (500):
//   - Pros: Higher throughput, lower cost (fewer API calls), better shard utilization
//   - Cons: Higher latency, more memory usage, larger failure impact
//   - Use case: High-volume ingestion (logs, metrics)
//
// Small batch size (1-50):
//   - Pros: Lower latency, smaller memory footprint, faster failure detection
//   - Cons: Lower throughput, higher cost (more API calls), more CPU overhead
//   - Use case: Low-latency requirements (real-time traces)
//
// Recommended settings by use case:
//
// High throughput (logs, metrics):
//
//	KINESIS_BATCH_SIZE=500
//	KINESIS_FLUSH_INTERVAL=10s
//	# Optimize for cost and throughput
//
// Balanced (general telemetry):
//
//	KINESIS_BATCH_SIZE=100
//	KINESIS_FLUSH_INTERVAL=5s
//	# Default, good balance
//
// Low latency (critical traces):
//
//	KINESIS_BATCH_SIZE=10
//	KINESIS_FLUSH_INTERVAL=1s
//	# Optimize for speed
//
// # Retry Strategy
//
// Kinesis operations can fail due to throttling, network errors, or service issues:
//
//	KINESIS_MAX_RETRIES=3         # Number of retry attempts
//	KINESIS_RETRY_DELAY=1s        # Initial delay between retries
//
// Retry behavior:
//   - Exponential backoff: Delay doubles with each retry (1s, 2s, 4s, 8s...)
//   - Jitter: Random variance added to prevent thundering herd
//   - Throttling: ProvisionedThroughputExceededException triggers backoff
//   - Retryable errors: Network errors, 5xx errors, throttling
//   - Non-retryable errors: Invalid arguments, authentication failures
//
// Error types:
//
// ProvisionedThroughputExceededException:
//   - Cause: Exceeded shard capacity (1 MB/s or 1000 records/s)
//   - Solution: Increase retries, add more shards, or reduce write rate
//   - Detection: Monitor PutRecords.ThrottledRecords metric
//
// ResourceNotFoundException:
//   - Cause: Stream doesn't exist
//   - Solution: Create stream or fix stream name
//   - Non-retryable
//
// InvalidArgumentException:
//   - Cause: Invalid partition key, data, or parameters
//   - Solution: Fix data validation
//   - Non-retryable
//
// Retry configuration examples:
//
// Aggressive retries (tolerate throttling):
//
//	KINESIS_MAX_RETRIES=10
//	KINESIS_RETRY_DELAY=100ms
//	# Retry quickly, many attempts
//
// Conservative retries (fail fast):
//
//	KINESIS_MAX_RETRIES=1
//	KINESIS_RETRY_DELAY=5s
//	# Few retries, longer delays
//
// # LocalStack Integration
//
// LocalStack provides local Kinesis streams for development and testing.
//
// Docker Compose setup:
//
//	services:
//	  localstack:
//	    image: localstack/localstack:latest
//	    ports:
//	      - "4566:4566"
//	    environment:
//	      - SERVICES=kinesis
//	      - DEBUG=1
//	      - KINESIS_INITIALIZE_STREAMS=telemetry-traces:1,telemetry-metrics:1,telemetry-logs:1
//
// Configuration for LocalStack:
//
//	KINESIS_ENDPOINT_URL=http://localhost:4566
//	KINESIS_REGION=us-east-1
//	KINESIS_ACCESS_KEY_ID=test
//	KINESIS_SECRET_ACCESS_KEY=test
//	KINESIS_TRACES_STREAM=telemetry-traces
//	KINESIS_METRICS_STREAM=telemetry-metrics
//	KINESIS_LOGS_STREAM=telemetry-logs
//
// Create streams in LocalStack:
//
//	aws --endpoint-url=http://localhost:4566 kinesis create-stream \
//	    --stream-name telemetry-traces --shard-count 1
//
// Learn more: https://docs.localstack.cloud/user-guide/aws/kinesis/
//
// # Example Usage
//
//	// Load Kinesis configuration
//	cfg, err := kinesis.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create AWS config
//	awsCfg, err := config.LoadDefaultConfig(context.Background(),
//	    config.WithRegion(string(cfg.Region)),
//	    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
//	        cfg.AccessKeyID,
//	        cfg.SecretAccessKey,
//	        cfg.SessionToken,
//	    )),
//	)
//
//	// Create Kinesis client
//	kinesisClient := kinesis.NewFromConfig(awsCfg, func(o *kinesis.Options) {
//	    if cfg.EndpointURL != "" {
//	        o.BaseEndpoint = aws.String(cfg.EndpointURL)
//	    }
//	})
//
//	// Batch records for efficiency
//	var batch []types.PutRecordsRequestEntry
//	for _, trace := range traces {
//	    data, _ := json.Marshal(trace)
//	    batch = append(batch, types.PutRecordsRequestEntry{
//	        Data:         data,
//	        PartitionKey: aws.String(trace.TraceID),
//	    })
//
//	    // Flush when batch is full or flush interval exceeded
//	    if len(batch) >= cfg.BatchSize {
//	        _, err := kinesisClient.PutRecords(context.Background(), &kinesis.PutRecordsInput{
//	            StreamName: aws.String(cfg.TracesStream),
//	            Records:    batch,
//	        })
//	        batch = batch[:0] // Reset batch
//	    }
//	}
//
// # Best Practices
//
// Production configuration:
//
//	KINESIS_REGION=us-east-1
//	KINESIS_TRACES_STREAM=prod-telemetry-traces
//	KINESIS_METRICS_STREAM=prod-telemetry-metrics
//	KINESIS_LOGS_STREAM=prod-telemetry-logs
//	KINESIS_BATCH_SIZE=100
//	KINESIS_FLUSH_INTERVAL=5s
//	KINESIS_MAX_RETRIES=3
//	KINESIS_RETRY_DELAY=1s
//	# Use IAM role instead of access keys
//
// Development configuration:
//
//	KINESIS_ENDPOINT_URL=http://localhost:4566
//	KINESIS_REGION=us-east-1
//	KINESIS_ACCESS_KEY_ID=test
//	KINESIS_SECRET_ACCESS_KEY=test
//	KINESIS_TRACES_STREAM=dev-traces
//	KINESIS_BATCH_SIZE=10
//	KINESIS_FLUSH_INTERVAL=1s
//
// Monitoring and observability:
//   - Track PutRecords.Success and PutRecords.ThrottledRecords metrics
//   - Alert on throttling (indicates need for more shards)
//   - Monitor GetRecords.IteratorAgeMilliseconds (consumer lag)
//   - Track PutRecord.Latency for performance
//   - Enable CloudWatch Logs for debugging
//
// Performance optimization:
//   - Use PutRecords (batch) instead of PutRecord (single)
//   - Choose appropriate batch size for throughput/latency tradeoff
//   - Use compression for large records (gzip, snappy)
//   - Parallelize writes across multiple goroutines
//   - Use connection pooling and keep-alive
//
// Cost optimization:
//   - Right-size shard count based on actual throughput
//   - Use on-demand capacity mode for variable workloads
//   - Aggregate small records before sending
//   - Use appropriate retention period (default 24h vs extended)
//   - Consider Kinesis Data Firehose for simpler ingestion to S3
//
// # Troubleshooting
//
// ProvisionedThroughputExceededException:
//
//	Error: Rate exceeded for shard
//	Fix: Increase KINESIS_BATCH_SIZE to reduce API calls
//	Fix: Add more shards to increase capacity
//	Fix: Implement better partition key distribution
//	Fix: Increase KINESIS_RETRY_DELAY and KINESIS_MAX_RETRIES
//
// ResourceNotFoundException:
//
//	Error: Stream not found
//	Fix: Create stream with: aws kinesis create-stream --stream-name <name> --shard-count 1
//	Fix: Verify stream name matches configuration
//	Fix: Check stream exists in correct region
//
// InvalidArgumentException:
//
//	Error: Invalid partition key or data
//	Fix: Ensure partition key is not empty and ≤256 bytes
//	Fix: Ensure record size ≤1 MB
//	Fix: Validate data is valid format
//
// Hot shard issues:
//
//	Problem: One shard receiving all traffic
//	Fix: Use higher cardinality partition keys
//	Fix: Add random suffix to partition keys
//	Fix: Monitor WriteProvisionedThroughputExceeded per shard
//
// High latency:
//
//	Problem: Slow record ingestion
//	Fix: Reduce KINESIS_BATCH_SIZE for lower latency
//	Fix: Decrease KINESIS_FLUSH_INTERVAL
//	Fix: Check network latency to AWS region
//	Fix: Use regional endpoints
//
// # Cross-References
//
// Related packages:
//   - [base.AWSRegion] - AWS region type definitions
//   - [base.Validator] - Validation framework
//   - [s3] - S3 configuration for archiving Kinesis data
//   - [dlq] - Dead letter queue for failed records
//
// AWS documentation:
//   - Kinesis Developer Guide: https://docs.aws.amazon.com/kinesis/
//   - Kinesis API Reference: https://docs.aws.amazon.com/kinesis/latest/APIReference/
//   - Go SDK v2: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/kinesis
//   - Best Practices: https://docs.aws.amazon.com/streams/latest/dev/best-practices.html
//   - LocalStack Kinesis: https://docs.localstack.cloud/user-guide/aws/kinesis/
//
// # Files in This Package
//
// constants.go:
//   - KINESIS_PREFIX for environment variable namespacing
//   - Default values (region, stream names, batching, retries)
//   - Min/max bounds for validation
//   - Time constants for intervals
//
// section.go:
//   - [Config] struct with Kinesis parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package kinesis
