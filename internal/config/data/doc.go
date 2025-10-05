// Package data defines configuration for data processing pipelines, worker pools, and batch operations.
//
// This package provides configuration for managing high-throughput data processing including batching,
// buffering, parallel processing, compression, and data retention policies. It's designed for telemetry
// data ingestion, transformation, and storage pipelines.
//
// # Overview
//
// Data processing configuration enables:
//
//   - Batch processing for efficient throughput
//   - Worker pool management for parallel processing
//   - Queue management for backpressure handling
//   - Compression for reduced storage and bandwidth
//   - Async and parallel processing modes
//   - Data validation before processing
//   - Retention policies for traces, metrics, and logs
//   - Automatic cleanup of expired data
//
// # Configuration Keys
//
// All settings use the DATA_ prefix and support environment variable overrides:
//
//	DATA_BATCH_SIZE            - Records per batch (default: 1000, range: 1-100000)
//	DATA_FLUSH_INTERVAL        - Max time before flush (default: 5s, range: 100ms-1m)
//	DATA_WORKER_COUNT          - Number of parallel workers (default: 4, range: 1-1000)
//	DATA_MAX_QUEUE_SIZE        - Maximum queue size (default: 10000, range: 100-10000000)
//	DATA_ENABLE_COMPRESSION    - Enable gzip compression (default: true)
//	DATA_ENABLE_ASYNC          - Process data asynchronously (default: true)
//	DATA_ENABLE_PARALLEL       - Process data in parallel (default: true)
//	DATA_ENABLE_VALIDATION     - Validate data before processing (default: true)
//	DATA_RETENTION_TRACES      - Traces retention period (default: 7d, range: 1d-10y)
//	DATA_RETENTION_METRICS     - Metrics retention period (default: 30d, range: 1d-10y)
//	DATA_RETENTION_LOGS        - Logs retention period (default: 7d, range: 1d-10y)
//	DATA_ENABLE_AUTO_CLEANUP   - Enable automatic cleanup (default: true)
//	DATA_CLEANUP_INTERVAL      - Cleanup frequency (default: 24h, range: 1h-7d)
//
// # Batch Processing
//
// Batching groups individual records for efficient processing:
//
//	DATA_BATCH_SIZE=1000          # Process 1000 records at a time
//	DATA_FLUSH_INTERVAL=5s        # Flush partial batches after 5 seconds
//
// Batch processing benefits:
//   - Reduced overhead (fewer function calls)
//   - Better memory locality
//   - More efficient I/O operations
//   - Lower network overhead
//   - Improved throughput
//
// Batch size tradeoffs:
//
// Large batches (10000-100000):
//   - Pros: Maximum throughput, lowest overhead, best for bulk operations
//   - Cons: Higher memory usage, increased latency, larger failure impact
//   - Use case: Bulk imports, batch analytics, data migration
//
// Medium batches (1000-10000) [RECOMMENDED]:
//   - Pros: Good balance of throughput and latency
//   - Cons: Moderate memory usage
//   - Use case: Standard telemetry ingestion, real-time analytics
//
// Small batches (1-100):
//   - Pros: Low latency, small memory footprint
//   - Cons: Lower throughput, higher overhead
//   - Use case: Interactive queries, real-time dashboards
//
// Flush interval ensures timely processing:
//   - Records are processed when batch is full OR interval expires
//   - Prevents unbounded latency for low-volume streams
//   - Example: With batch_size=1000 and flush_interval=5s, a stream with 100 records/s flushes every 5s
//
// # Worker Pools
//
// Worker pools enable parallel processing of data batches:
//
//	DATA_WORKER_COUNT=4           # Use 4 parallel workers
//	DATA_ENABLE_PARALLEL=true     # Enable parallel processing
//
// Worker pool architecture:
//
//	Input Queue → Worker 1 → Process Batch → Output
//	           → Worker 2 → Process Batch → Output
//	           → Worker 3 → Process Batch → Output
//	           → Worker 4 → Process Batch → Output
//
// Worker count guidelines:
//
// CPU-bound workloads:
//   - Set worker count to number of CPU cores
//   - Example: 8-core machine → DATA_WORKER_COUNT=8
//   - Higher values provide no benefit and increase overhead
//
// I/O-bound workloads:
//   - Set worker count higher than CPU cores (2-4x)
//   - Example: 8-core machine → DATA_WORKER_COUNT=32
//   - Workers spend time waiting for I/O, not using CPU
//
// Mixed workloads:
//   - Start with 2x CPU cores
//   - Monitor CPU utilization and adjust
//   - Example: 8-core machine → DATA_WORKER_COUNT=16
//
// Memory considerations:
//   - Each worker maintains a batch in memory
//   - Total memory = worker_count × batch_size × record_size
//   - Example: 10 workers × 1000 records × 1KB = 10MB
//
// # Queue Management and Backpressure
//
// The queue buffers incoming data before worker processing:
//
//	DATA_MAX_QUEUE_SIZE=10000     # Queue up to 10000 items
//
// Queue behavior:
//   - Items added to queue until full
//   - When full, producers block (backpressure)
//   - Workers consume from queue continuously
//   - Queue size should be: batch_size × worker_count × 2-10
//
// Backpressure handling:
//
// Scenario 1: Normal operation
//
//	Producer Rate: 1000 items/s
//	Consumer Rate: 1200 items/s
//	Queue Size: Stable, low utilization
//	Action: None needed
//
// Scenario 2: Temporary spike
//
//	Producer Rate: 5000 items/s (spike)
//	Consumer Rate: 1000 items/s
//	Queue Size: Growing, absorbs spike
//	Action: Queue buffers until spike ends
//
// Scenario 3: Sustained overload
//
//	Producer Rate: 2000 items/s
//	Consumer Rate: 1000 items/s
//	Queue Size: Full, backpressure applied
//	Action: Producer blocks until queue drains
//
// Queue sizing recommendations:
//   - Too small: Frequent backpressure, reduced throughput
//   - Too large: High memory usage, slow failure detection
//   - Rule of thumb: 10-100 seconds of buffer at peak rate
//   - Example: 1000 items/s × 30s buffer = 30000 queue size
//
// # Compression
//
// Compression reduces storage space and network bandwidth:
//
//	DATA_ENABLE_COMPRESSION=true  # Enable gzip compression
//
// Compression benefits:
//   - 50-90% size reduction for text data (logs, traces, JSON)
//   - Lower storage costs
//   - Faster network transfers
//   - Reduced bandwidth costs
//
// Compression tradeoffs:
//   - CPU overhead for compression/decompression
//   - Typically 5-10% CPU usage increase
//   - More pronounced on CPU-bound workloads
//
// When to enable:
//   - Text-heavy data (logs, JSON, traces)
//   - Network transfer over WAN
//   - Long-term storage
//   - High-volume data streams
//
// When to disable:
//   - Already compressed data (images, videos)
//   - CPU-constrained systems
//   - Local-only processing (no network/storage)
//   - Real-time low-latency requirements
//
// # Async and Parallel Processing
//
//	DATA_ENABLE_ASYNC=true        # Process in background
//	DATA_ENABLE_PARALLEL=true     # Use multiple workers
//
// Async processing:
//   - Records queued and processed in background
//   - Producer doesn't wait for processing to complete
//   - Lower latency for producers
//   - At-least-once delivery semantics
//
// Sync processing:
//   - Producer waits for processing to complete
//   - Exactly-once delivery semantics
//   - Higher latency for producers
//   - Simpler error handling
//
// Parallel processing:
//   - Multiple workers process batches concurrently
//   - Higher throughput
//   - Requires thread-safe operations
//   - No ordering guarantees
//
// Sequential processing:
//   - Single worker processes batches
//   - Lower throughput
//   - Maintains ordering
//   - Simpler debugging
//
// # Data Validation
//
//	DATA_ENABLE_VALIDATION=true   # Validate before processing
//
// Validation checks:
//   - Schema validation (required fields, types)
//   - Range validation (min/max values)
//   - Format validation (timestamps, IDs)
//   - Business rule validation
//
// Validation tradeoffs:
//   - Pros: Early error detection, data quality, prevent corruption
//   - Cons: CPU overhead (typically 5-10%), increased latency
//
// When to enable:
//   - Untrusted data sources
//   - Strict data quality requirements
//   - Compliance and audit needs
//   - Production environments
//
// When to disable:
//   - Trusted data sources
//   - Performance-critical paths
//   - Pre-validated data
//   - Development/testing environments
//
// # Retention Policies
//
// Retention policies define how long data is kept before automatic deletion:
//
//	DATA_RETENTION_TRACES=168h    # Keep traces for 7 days
//	DATA_RETENTION_METRICS=720h   # Keep metrics for 30 days
//	DATA_RETENTION_LOGS=168h      # Keep logs for 7 days
//	DATA_ENABLE_AUTO_CLEANUP=true
//	DATA_CLEANUP_INTERVAL=24h     # Check daily
//
// Retention considerations:
//
// Traces (distributed tracing data):
//   - Default: 7 days
//   - High volume, large size
//   - Primarily used for recent debugging
//   - Archive to cold storage for long-term analysis
//
// Metrics (time-series data):
//   - Default: 30 days
//   - Moderate volume
//   - Used for trending and alerting
//   - Consider downsampling for longer retention
//
// Logs (application logs):
//   - Default: 7 days
//   - High volume
//   - Primarily used for recent debugging
//   - Archive to S3/Glacier for compliance
//
// Retention best practices:
//   - Balance storage costs vs data needs
//   - Comply with regulatory requirements (GDPR, HIPAA, SOC2)
//   - Use tiered storage (hot/warm/cold)
//   - Archive before deletion
//   - Monitor storage growth
//
// # Automatic Cleanup
//
//	DATA_ENABLE_AUTO_CLEANUP=true
//	DATA_CLEANUP_INTERVAL=24h
//
// Cleanup process:
//  1. Run periodically based on cleanup interval
//  2. Query for data older than retention period
//  3. Delete expired data in batches
//  4. Update cleanup metrics
//
// Cleanup scheduling:
//   - Daily (24h): Standard, low overhead
//   - Hourly (1h): Tight storage constraints
//   - Weekly (168h): Low-volume systems
//
// Cleanup considerations:
//   - Runs during low-traffic periods when possible
//   - May cause temporary CPU/disk spikes
//   - Monitor cleanup duration and data deleted
//   - Ensure backups before cleanup
//
// # Example Usage
//
//	// Load data processing configuration
//	cfg, err := data.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create worker pool
//	queue := make(chan []Record, cfg.MaxQueueSize)
//	var wg sync.WaitGroup
//
//	// Start workers
//	for i := 0; i < cfg.WorkerCount; i++ {
//	    wg.Add(1)
//	    go func() {
//	        defer wg.Done()
//	        for batch := range queue {
//	            if cfg.EnableValidation {
//	                if err := validateBatch(batch); err != nil {
//	                    log.Error("validation failed", "error", err)
//	                    continue
//	                }
//	            }
//	            if cfg.EnableCompression {
//	                batch = compressBatch(batch)
//	            }
//	            if err := processBatch(batch); err != nil {
//	                log.Error("processing failed", "error", err)
//	            }
//	        }
//	    }()
//	}
//
//	// Batch producer
//	batch := make([]Record, 0, cfg.BatchSize)
//	flushTicker := time.NewTicker(cfg.FlushInterval)
//	defer flushTicker.Stop()
//
//	for {
//	    select {
//	    case record := <-records:
//	        batch = append(batch, record)
//	        if len(batch) >= cfg.BatchSize {
//	            queue <- batch
//	            batch = make([]Record, 0, cfg.BatchSize)
//	        }
//	    case <-flushTicker.C:
//	        if len(batch) > 0 {
//	            queue <- batch
//	            batch = make([]Record, 0, cfg.BatchSize)
//	        }
//	    }
//	}
//
// # Best Practices
//
// High-throughput configuration (logs, metrics):
//
//	DATA_BATCH_SIZE=10000
//	DATA_FLUSH_INTERVAL=10s
//	DATA_WORKER_COUNT=16
//	DATA_MAX_QUEUE_SIZE=100000
//	DATA_ENABLE_COMPRESSION=true
//	DATA_ENABLE_ASYNC=true
//	DATA_ENABLE_PARALLEL=true
//	DATA_ENABLE_VALIDATION=false    # Pre-validated data
//
// Balanced configuration (general telemetry):
//
//	DATA_BATCH_SIZE=1000
//	DATA_FLUSH_INTERVAL=5s
//	DATA_WORKER_COUNT=4
//	DATA_MAX_QUEUE_SIZE=10000
//	DATA_ENABLE_COMPRESSION=true
//	DATA_ENABLE_ASYNC=true
//	DATA_ENABLE_PARALLEL=true
//	DATA_ENABLE_VALIDATION=true
//
// Low-latency configuration (real-time traces):
//
//	DATA_BATCH_SIZE=100
//	DATA_FLUSH_INTERVAL=1s
//	DATA_WORKER_COUNT=8
//	DATA_MAX_QUEUE_SIZE=1000
//	DATA_ENABLE_COMPRESSION=false   # Minimize CPU
//	DATA_ENABLE_ASYNC=true
//	DATA_ENABLE_PARALLEL=true
//	DATA_ENABLE_VALIDATION=true
//
// Performance optimization:
//   - Profile to identify bottlenecks (CPU, memory, I/O)
//   - Adjust worker count based on workload type
//   - Use compression for network/storage-bound workloads
//   - Disable validation for trusted, pre-validated data
//   - Monitor queue depth and adjust size
//   - Use appropriate batch size for latency requirements
//
// Memory management:
//   - Limit total memory: workers × batch_size × record_size + queue_size × record_size
//   - Use object pools for record allocation
//   - Release processed batches promptly
//   - Monitor heap size and GC pressure
//   - Consider reducing batch size if memory-constrained
//
// # Troubleshooting
//
// High memory usage:
//
//	Problem: Application using too much memory
//	Fix: Reduce DATA_BATCH_SIZE or DATA_MAX_QUEUE_SIZE
//	Fix: Reduce DATA_WORKER_COUNT
//	Fix: Check for memory leaks in processing code
//
// High latency:
//
//	Problem: Data processing too slow
//	Fix: Reduce DATA_BATCH_SIZE for faster batches
//	Fix: Reduce DATA_FLUSH_INTERVAL for faster flushes
//	Fix: Increase DATA_WORKER_COUNT for parallel processing
//
// Low throughput:
//
//	Problem: Not processing enough data
//	Fix: Increase DATA_BATCH_SIZE for efficiency
//	Fix: Increase DATA_WORKER_COUNT for parallelism
//	Fix: Enable DATA_ENABLE_ASYNC and DATA_ENABLE_PARALLEL
//	Fix: Disable DATA_ENABLE_VALIDATION if not needed
//	Fix: Disable DATA_ENABLE_COMPRESSION for CPU-bound workloads
//
// Queue full errors:
//
//	Problem: Producers blocked, backpressure applied
//	Fix: Increase DATA_MAX_QUEUE_SIZE to buffer more
//	Fix: Increase DATA_WORKER_COUNT to process faster
//	Fix: Check for slow processing in workers
//
// # Cross-References
//
// Related packages:
//   - [base.Validator] - Validation framework
//   - [kinesis] - Streaming data ingestion
//   - [s3] - Archival storage
//   - [dlq] - Failed record handling
//
// Related patterns:
//   - Producer-Consumer: https://en.wikipedia.org/wiki/Producer%E2%80%93consumer_problem
//   - Worker Pool: https://gobyexample.com/worker-pools
//   - Backpressure: https://medium.com/@jayphelps/backpressure-explained-the-flow-of-data-through-software-2350b3e77ce7
//
// # Files in This Package
//
// constants.go:
//   - DATA_PREFIX for environment variable namespacing
//   - Default values for batch processing, workers, retention
//   - Min/max bounds for validation
//   - Time period constants
//
// section.go:
//   - [Config] struct with data processing parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package data
