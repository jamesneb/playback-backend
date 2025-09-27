package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/jamesneb/playback-backend/internal/validation"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Kinesis client resource management constants
const (
	// MaxChannelBufferSize defines maximum channel buffer size to prevent unbounded growth
	MaxChannelBufferSize = 10000

	// DefaultChannelBufferSize provides a reasonable default for channel buffering
	DefaultChannelBufferSize = 1000

	// ShutdownTimeout defines maximum time to wait for graceful shutdown
	ShutdownTimeout = 30 * time.Second

	// MaxRetryAttempts for failed batch operations
	MaxRetryAttempts = 3

	// BackoffBaseDuration for exponential backoff between retries
	BackoffBaseDuration = 100 * time.Millisecond
)

// KinesisClient provides thread-safe, resource-managed Kinesis streaming functionality
// with proper lifecycle management and memory leak prevention.
type KinesisClient struct {
	client    *kinesis.Client
	streams   map[string]string             // stream name mapping
	validator *validation.ProtobufValidator // Add protobuf validator

	// Resource management
	mu           sync.RWMutex
	isRunning    bool
	shutdownOnce sync.Once

	// Goroutine coordination
	wg         sync.WaitGroup
	shutdownCh chan struct{}

	// Batching support with bounded channels (prevents unbounded growth)
	batchChannels map[string]chan LegacyTelemetryEvent
	batchSize     int
	flushInterval time.Duration
}

// Note: TelemetryEvent and TelemetryMetadata are now defined in handler.go

func NewKinesisClient(cfg *config.KinesisConfig) (*KinesisClient, error) {
	// Load AWS configuration with proper credential handling
	var awsCfg aws.Config
	var err error

	// Honor configured credentials if provided (for LocalStack or dedicated IAM users)
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		// Use explicit credentials
		awsCfg, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithRegion(cfg.Region),
			awsconfig.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     cfg.AccessKeyID,
					SecretAccessKey: cfg.SecretAccessKey,
				}, nil
			})),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with credentials: %w", err)
		}
	} else {
		// Fall back to default credential chain (environment, instance role, etc.)
		awsCfg, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithRegion(cfg.Region),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
	}

	// Override endpoint if specified (for LocalStack)
	if cfg.EndpointURL != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.EndpointURL)
	}

	client := kinesis.NewFromConfig(awsCfg)

	// Map stream types to actual stream names
	streams := map[string]string{
		"traces":  cfg.Streams["traces"],
		"metrics": cfg.Streams["metrics"],
		"logs":    cfg.Streams["logs"],
	}

	// Use provided batch configuration with safe defaults
	batchSize := DefaultChannelBufferSize
	flushInterval := 5 * time.Second

	if cfg.BatchSize > 0 && cfg.BatchSize <= MaxChannelBufferSize {
		batchSize = cfg.BatchSize
	} else if cfg.BatchSize > MaxChannelBufferSize {
		logger.Warn("Batch size exceeds maximum, using maximum", zap.Int("requested_batch_size", cfg.BatchSize), zap.Int("max_batch_size", MaxChannelBufferSize))
		batchSize = MaxChannelBufferSize
	}

	if cfg.FlushInterval != "" {
		if parsedInterval, err := time.ParseDuration(cfg.FlushInterval); err == nil {
			flushInterval = parsedInterval
		} else {
			logger.Warn("Invalid flush interval, using default 5s", zap.String("invalid_interval", cfg.FlushInterval))
		}
	}

	kc := &KinesisClient{
		client:        client,
		streams:       streams,
		validator:     validation.NewProtobufValidator(),
		batchChannels: make(map[string]chan LegacyTelemetryEvent),
		shutdownCh:    make(chan struct{}),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		isRunning:     false,
	}

	// Verify streams exist
	if err := kc.verifyStreams(context.Background()); err != nil {
		logger.Warn("Stream verification failed", zap.Error(err))
		// Continue anyway for development/LocalStack scenarios
	}

	logger.Info("Kinesis client initialized", zap.Any("streams", streams))
	return kc, nil
}

func (kc *KinesisClient) verifyStreams(ctx context.Context) error {
	for streamType, streamName := range kc.streams {
		if streamName == "" {
			return fmt.Errorf("stream name not configured for %s", streamType)
		}

		// Check if stream exists
		_, err := kc.client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
			StreamName: aws.String(streamName),
		})
		if err != nil {
			return fmt.Errorf("stream %s (%s) not accessible: %w", streamName, streamType, err)
		}
	}
	return nil
}

func (kc *KinesisClient) PublishTrace(ctx context.Context, traceData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	event := LegacyTelemetryEvent{
		Type:        "traces",
		ServiceName: serviceName,
		TraceID:     traceID,
		Data:        traceData,
		Metadata: LegacyTelemetryMetadata{
			IngestedAt: time.Now(),
			SourceIP:   sourceIP,
			UserAgent:  userAgent,
			Version:    "1.0",
		},
	}

	// Use traceID as partition key, fallback to serviceName if empty
	partitionKey := traceID
	if partitionKey == "" {
		partitionKey = serviceName
		if partitionKey == "" {
			partitionKey = "unknown"
		}
	}

	return kc.publishLegacyEvent(ctx, "traces", event, partitionKey)
}

func (kc *KinesisClient) PublishMetrics(ctx context.Context, metricsData json.RawMessage, serviceName, sourceIP, userAgent string) error {
	event := LegacyTelemetryEvent{
		Type:        "metrics",
		ServiceName: serviceName,
		Data:        metricsData,
		Metadata: LegacyTelemetryMetadata{
			IngestedAt: time.Now(),
			SourceIP:   sourceIP,
			UserAgent:  userAgent,
			Version:    "1.0",
		},
	}

	// Use service name as partition key for metrics
	partitionKey := fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()%1000)
	return kc.publishLegacyEvent(ctx, "metrics", event, partitionKey)
}

func (kc *KinesisClient) PublishLogs(ctx context.Context, logsData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	event := LegacyTelemetryEvent{
		Type:        "logs",
		ServiceName: serviceName,
		TraceID:     traceID,
		Data:        logsData,
		Metadata: LegacyTelemetryMetadata{
			IngestedAt: time.Now(),
			SourceIP:   sourceIP,
			UserAgent:  userAgent,
			Version:    "1.0",
		},
	}

	// Use trace ID if available, otherwise service name
	partitionKey := traceID
	if partitionKey == "" {
		partitionKey = fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()%1000)
	}

	return kc.publishLegacyEvent(ctx, "logs", event, partitionKey)
}

// publishLegacyEvent handles legacy JSON events from HTTP REST API
func (kc *KinesisClient) publishLegacyEvent(ctx context.Context, streamType string, event LegacyTelemetryEvent, partitionKey string) error {
	streamName, exists := kc.streams[streamType]
	if !exists {
		return fmt.Errorf("stream not configured for type: %s", streamType)
	}

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Prepare Kinesis record
	record := &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         data,
		PartitionKey: aws.String(partitionKey),
	}

	// Add explicit hash key for better distribution if partition key is predictable
	if len(partitionKey) < 10 {
		record.ExplicitHashKey = aws.String(fmt.Sprintf("%d", time.Now().UnixNano()))
	}

	// Publish to Kinesis
	result, err := kc.client.PutRecord(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to publish to Kinesis stream %s: %w", streamName, err)
	}

	logger.Debug("Published event to stream",
		zap.String("stream_type", streamType),
		zap.String("stream_name", streamName),
		zap.String("shard_id", *result.ShardId),
		zap.String("sequence_number", *result.SequenceNumber))

	return nil
}

// Batch publishing for high-throughput scenarios
func (kc *KinesisClient) PublishBatch(ctx context.Context, streamType string, events []LegacyTelemetryEvent) error {
	streamName, exists := kc.streams[streamType]
	if !exists {
		return fmt.Errorf("stream not configured for type: %s", streamType)
	}

	// Convert events to Kinesis records
	var records []types.PutRecordsRequestEntry
	for i, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			logger.Error("Failed to marshal event", zap.Int("event_index", i), zap.Error(err))
			continue
		}

		partitionKey := event.ServiceName
		if event.TraceID != "" {
			partitionKey = event.TraceID
		}

		records = append(records, types.PutRecordsRequestEntry{
			Data:         data,
			PartitionKey: aws.String(partitionKey),
		})
	}

	if len(records) == 0 {
		return fmt.Errorf("no valid records to publish")
	}

	// Batch publish to Kinesis
	result, err := kc.client.PutRecords(ctx, &kinesis.PutRecordsInput{
		StreamName: aws.String(streamName),
		Records:    records,
	})
	if err != nil {
		return fmt.Errorf("failed to batch publish to stream %s: %w", streamName, err)
	}

	// Log results
	logger.Debug("Batch published to stream",
		zap.Int("total_records", len(records)),
		zap.String("stream_name", streamName),
		zap.Int32("failed_records", *result.FailedRecordCount))

	return nil
}

// StartBatchProcessor enables high-throughput batch processing with proper resource management.
// This method is thread-safe and prevents multiple concurrent starts.
//
// Parameters:
//   - ctx: Context for controlling processor lifecycle
//
// Returns:
//   - error: Non-nil if processor cannot be started or is already running
func (kc *KinesisClient) StartBatchProcessor(ctx context.Context) error {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	if kc.isRunning {
		return fmt.Errorf("batch processor is already running")
	}

	// Initialize bounded channels for each stream type to prevent unbounded growth
	channelBufferSize := calculateChannelBufferSize(kc.batchSize)

	for streamType := range kc.streams {
		if kc.batchChannels[streamType] != nil {
			close(kc.batchChannels[streamType])
		}
		kc.batchChannels[streamType] = make(chan LegacyTelemetryEvent, channelBufferSize)

		// Start processor goroutine with proper cleanup tracking
		kc.wg.Add(1)
		go kc.processBatchWithRecovery(ctx, streamType)
	}

	kc.isRunning = true

	logger.Info("Started Kinesis batch processors",
		zap.Int("stream_count", len(kc.streams)),
		zap.Int("batch_size", kc.batchSize),
		zap.Int("buffer_size", channelBufferSize),
		zap.Duration("flush_interval", kc.flushInterval))

	return nil
}

// PublishAsync sends events to batch processor for high-throughput scenarios
func (kc *KinesisClient) PublishAsync(streamType string, event LegacyTelemetryEvent) error {
	batchChannel, exists := kc.batchChannels[streamType]
	if !exists {
		return fmt.Errorf("batch channel not initialized for stream type: %s", streamType)
	}

	select {
	case batchChannel <- event:
		return nil
	default:
		// Channel full - fall back to direct publish
		logger.Warn("Batch channel full, falling back to direct publish", zap.String("stream_type", streamType))
		return kc.publishLegacyEvent(context.Background(), streamType, event, event.TraceID)
	}
}

// processBatchWithRecovery wraps batch processing with panic recovery to prevent
// crashing the entire application if a single batch processor fails.
func (kc *KinesisClient) processBatchWithRecovery(ctx context.Context, streamType string) {
	defer kc.wg.Done()

	defer func() {
		if r := recover(); r != nil {
			logger.Error("CRITICAL: Batch processor panicked and recovered", zap.String("stream_type", streamType), zap.Any("panic", r))
			// Processor will terminate, but application continues
		}
	}()

	kc.processBatch(ctx, streamType)
}

// processBatch handles batching logic for a specific stream type with comprehensive
// resource management, graceful shutdown, and proper error handling.
func (kc *KinesisClient) processBatch(ctx context.Context, streamType string) {
	eventBuffer := make([]LegacyTelemetryEvent, 0, kc.batchSize)
	ticker := time.NewTicker(kc.flushInterval)
	defer ticker.Stop()

	// Get channel with read lock to ensure thread safety
	kc.mu.RLock()
	batchChannel := kc.batchChannels[streamType]
	kc.mu.RUnlock()

	if batchChannel == nil {
		logger.Error("No batch channel found for stream type", zap.String("stream_type", streamType))
		return
	}

	logger.Info("Batch processor started", zap.String("stream_type", streamType))
	defer logger.Info("Batch processor terminated", zap.String("stream_type", streamType))

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - perform graceful shutdown
			kc.handleGracefulShutdown(ctx, streamType, eventBuffer)
			return

		case <-kc.shutdownCh:
			// Internal shutdown signal - perform graceful shutdown
			kc.handleGracefulShutdown(ctx, streamType, eventBuffer)
			return

		case event, channelOpen := <-batchChannel:
			if !channelOpen {
				// Channel closed - flush remaining events and exit
				kc.handleGracefulShutdown(ctx, streamType, eventBuffer)
				return
			}

			eventBuffer = append(eventBuffer, event)

			// Flush when batch reaches optimal size
			if len(eventBuffer) >= kc.batchSize {
				if flushedCount := kc.flushBatchSafely(ctx, streamType, eventBuffer); flushedCount > 0 {
					eventBuffer = eventBuffer[:0] // Reset buffer after successful flush
				}
			}

		case <-ticker.C:
			// Periodic flush to ensure events don't sit too long in buffer
			if len(eventBuffer) > 0 {
				if flushedCount := kc.flushBatchSafely(ctx, streamType, eventBuffer); flushedCount > 0 {
					eventBuffer = eventBuffer[:0] // Reset buffer after successful flush
				}
			}
		}
	}
}

// flushBatch sends accumulated events to Kinesis and returns any error encountered.
// This method attempts batch publishing first, then falls back to individual publishes if needed.
func (kc *KinesisClient) flushBatch(ctx context.Context, streamType string, events []LegacyTelemetryEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Attempt batch publish first
	if err := kc.PublishBatch(ctx, streamType, events); err != nil {
		logger.Warn("Batch publish failed, attempting individual publishes", zap.String("stream_type", streamType), zap.Error(err))

		// Fallback: try individual publishes for failed batch
		var individualErrors []error
		for _, event := range events {
			partitionKey := event.TraceID
			if partitionKey == "" {
				partitionKey = event.ServiceName
			}
			if individualErr := kc.publishLegacyEvent(ctx, streamType, event, partitionKey); individualErr != nil {
				logger.Error("Failed individual publish", zap.String("stream_type", streamType), zap.Error(individualErr))
				individualErrors = append(individualErrors, individualErr)
			}
		}

		// If some individual publishes succeeded, consider it partial success
		if len(individualErrors) < len(events) {
			logger.Info("Partial success publishing events individually",
				zap.Int("published_count", len(events)-len(individualErrors)),
				zap.Int("total_events", len(events)),
				zap.String("stream_type", streamType))
			return nil // Partial success is acceptable
		}

		return fmt.Errorf("all individual publishes failed after batch failure")
	}

	logger.Debug("Successfully flushed batch", zap.Int("event_count", len(events)), zap.String("stream_type", streamType))
	return nil
}

// Native protobuf publishing methods for gRPC path (sends raw OTLP protobuf to Kinesis)
func (kc *KinesisClient) PublishTraceProtobuf(ctx context.Context, resourceSpans *tracepb.ResourceSpans, serviceName, traceID, sourceIP string) error {
	logger.Debug("Publishing trace protobuf", zap.String("service_name", serviceName), zap.String("trace_id", traceID))

	// Validate protobuf size before marshaling
	if err := kc.validator.ValidateProtobufSize(resourceSpans, validation.MaxProtobufTraceSize, "trace"); err != nil {
		return fmt.Errorf("trace protobuf validation failed: %w", err)
	}

	// Serialize OTLP protobuf directly - no wrapper needed!
	data, err := proto.Marshal(resourceSpans)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP trace data: %w", err)
	}

	partitionKey := traceID
	if partitionKey == "" {
		partitionKey = serviceName
		if partitionKey == "" {
			partitionKey = "unknown"
		}
	}

	// Send raw OTLP protobuf bytes directly to Kinesis with metadata in partition key
	return kc.publishRawProtobuf(ctx, "traces", data, partitionKey, serviceName, traceID, sourceIP)
}

func (kc *KinesisClient) PublishMetricsProtobuf(ctx context.Context, resourceMetrics *metricspb.ResourceMetrics, serviceName, sourceIP string) error {
	// Validate protobuf size before marshaling
	if err := kc.validator.ValidateProtobufSize(resourceMetrics, validation.MaxProtobufMetricsSize, "metrics"); err != nil {
		return fmt.Errorf("metrics protobuf validation failed: %w", err)
	}

	// Serialize OTLP protobuf directly - no wrapper needed!
	data, err := proto.Marshal(resourceMetrics)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP metrics data: %w", err)
	}

	partitionKey := fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()%1000)
	// Send raw OTLP protobuf bytes directly to Kinesis
	return kc.publishRawProtobuf(ctx, "metrics", data, partitionKey, serviceName, "", sourceIP)
}

func (kc *KinesisClient) PublishLogsProtobuf(ctx context.Context, resourceLogs *logspb.ResourceLogs, serviceName, traceID, sourceIP string) error {
	// Validate protobuf size before marshaling
	if err := kc.validator.ValidateProtobufSize(resourceLogs, validation.MaxProtobufLogsSize, "logs"); err != nil {
		return fmt.Errorf("logs protobuf validation failed: %w", err)
	}

	// Serialize OTLP protobuf directly - no wrapper needed!
	data, err := proto.Marshal(resourceLogs)
	if err != nil {
		return fmt.Errorf("failed to marshal OTLP logs data: %w", err)
	}

	partitionKey := traceID
	if partitionKey == "" {
		partitionKey = fmt.Sprintf("%s-%d", serviceName, time.Now().UnixNano()%1000)
	}

	// Send raw OTLP protobuf bytes directly to Kinesis
	return kc.publishRawProtobuf(ctx, "logs", data, partitionKey, serviceName, traceID, sourceIP)
}

// publishRawProtobuf sends raw OTLP protobuf bytes directly to Kinesis
// Metadata is encoded in partition key and can be extracted by consumer
func (kc *KinesisClient) publishRawProtobuf(ctx context.Context, streamType string, protobufData []byte, partitionKey, serviceName, traceID, sourceIP string) error {
	streamName, exists := kc.streams[streamType]
	if !exists {
		return fmt.Errorf("stream not configured for type: %s", streamType)
	}

	// Create structured partition key that consumer expects: "pb:<service>:<trace_id>:<timestamp>"
	timestamp := time.Now().UnixNano()
	protobufPartitionKey := fmt.Sprintf("pb:%s:%s:%d", serviceName, traceID, timestamp)

	// Send raw OTLP protobuf bytes directly to Kinesis - no wrapper!
	record := &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         protobufData, // Raw OTLP protobuf bytes
		PartitionKey: aws.String(protobufPartitionKey),
	}

	if len(protobufPartitionKey) < 10 {
		record.ExplicitHashKey = aws.String(fmt.Sprintf("%d", time.Now().UnixNano()))
	}

	// Publish to Kinesis
	result, err := kc.client.PutRecord(ctx, record)
	if err != nil {
		return fmt.Errorf("failed to publish raw protobuf to Kinesis stream %s: %w", streamName, err)
	}

	logger.Debug("Published raw protobuf to stream",
		zap.String("stream_type", streamType),
		zap.String("stream_name", streamName),
		zap.String("shard_id", *result.ShardId),
		zap.String("sequence_number", *result.SequenceNumber))

	return nil
}

// SetBatchConfig allows customization of batching parameters
func (kc *KinesisClient) SetBatchConfig(batchSize int, flushInterval time.Duration) {
	kc.batchSize = batchSize
	kc.flushInterval = flushInterval
}

// Protobuf serialization methods for type-safe telemetry events

// SerializeTraceData serializes protobuf ResourceSpans to bytes
func (kc *KinesisClient) SerializeTraceData(resourceSpans *tracepb.ResourceSpans) ([]byte, error) {
	if resourceSpans == nil {
		return nil, ErrInvalidTraceData
	}

	data, err := proto.Marshal(resourceSpans)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trace data: %w", err)
	}

	return data, nil
}

// SerializeMetricsData serializes protobuf ResourceMetrics to bytes
func (kc *KinesisClient) SerializeMetricsData(resourceMetrics *metricspb.ResourceMetrics) ([]byte, error) {
	if resourceMetrics == nil {
		return nil, ErrInvalidMetricsData
	}

	data, err := proto.Marshal(resourceMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics data: %w", err)
	}

	return data, nil
}

// SerializeLogsData serializes protobuf ResourceLogs to bytes
func (kc *KinesisClient) SerializeLogsData(resourceLogs *logspb.ResourceLogs) ([]byte, error) {
	if resourceLogs == nil {
		return nil, ErrInvalidLogsData
	}

	data, err := proto.Marshal(resourceLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal logs data: %w", err)
	}

	return data, nil
}

// Close performs graceful shutdown of the Kinesis client with proper resource cleanup.
// This method is thread-safe and can be called multiple times safely.
//
// The shutdown process:
// 1. Signals all batch processors to stop
// 2. Waits for processors to finish current work (with timeout)
// 3. Closes all channels to prevent memory leaks
// 4. Ensures all goroutines terminate properly
//
// Returns:
//   - error: Non-nil if shutdown encounters issues (logged but not critical)
func (kc *KinesisClient) Close() error {
	var shutdownError error

	kc.shutdownOnce.Do(func() {
		logger.Info("Initiating graceful shutdown of Kinesis client")

		kc.mu.Lock()
		wasRunning := kc.isRunning
		kc.isRunning = false
		kc.mu.Unlock()

		if !wasRunning {
			logger.Info("Kinesis client was not running, nothing to shut down")
			// Still need to close shutdownCh for tests and cleanup consistency
			if kc.shutdownCh != nil {
				close(kc.shutdownCh)
			}
			return
		}

		// Signal all processors to shut down
		close(kc.shutdownCh)

		// Wait for all batch processors to complete with timeout
		shutdownComplete := make(chan struct{})
		go func() {
			kc.wg.Wait()
			close(shutdownComplete)
		}()

		select {
		case <-shutdownComplete:
			logger.Info("All batch processors shut down gracefully")
		case <-time.After(ShutdownTimeout):
			logger.Warn("Shutdown timeout, some processors may not have finished", zap.Duration("timeout", ShutdownTimeout))
			shutdownError = fmt.Errorf("shutdown timeout exceeded")
		}

		// Close all batch channels to prevent memory leaks
		kc.mu.Lock()
		for streamType, ch := range kc.batchChannels {
			if ch != nil {
				close(ch)
				delete(kc.batchChannels, streamType)
			}
		}
		kc.mu.Unlock()

		logger.Info("Kinesis client shutdown completed")
	})

	return shutdownError
}

// Helper methods for proper resource management

// calculateChannelBufferSize determines optimal channel buffer size based on batch size
// to prevent unbounded growth while maintaining performance.
func calculateChannelBufferSize(batchSize int) int {
	// Buffer should be 2-3x batch size for optimal performance, but capped at maximum
	bufferSize := batchSize * 2
	if bufferSize > MaxChannelBufferSize {
		return MaxChannelBufferSize
	}
	if bufferSize < DefaultChannelBufferSize {
		return DefaultChannelBufferSize
	}
	return bufferSize
}

// handleGracefulShutdown performs graceful shutdown operations for a single batch processor.
func (kc *KinesisClient) handleGracefulShutdown(ctx context.Context, streamType string, eventBuffer []LegacyTelemetryEvent) {
	if len(eventBuffer) > 0 {
		logger.Info("Flushing remaining events during shutdown", zap.Int("event_count", len(eventBuffer)), zap.String("stream_type", streamType))

		// Create timeout context for final flush
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := kc.flushBatch(flushCtx, streamType, eventBuffer); err != nil {
			logger.Error("Failed to flush events during shutdown", zap.Int("event_count", len(eventBuffer)), zap.String("stream_type", streamType), zap.Error(err))
		} else {
			logger.Info("Successfully flushed events during shutdown", zap.Int("event_count", len(eventBuffer)), zap.String("stream_type", streamType))
		}
	}
}

// flushBatchSafely wraps batch flushing with error handling and retry logic.
// Returns the number of successfully flushed events.
func (kc *KinesisClient) flushBatchSafely(ctx context.Context, streamType string, events []LegacyTelemetryEvent) int {
	if len(events) == 0 {
		return 0
	}

	// Attempt flush with basic retry logic
	for attempt := 0; attempt < MaxRetryAttempts; attempt++ {
		err := kc.flushBatch(ctx, streamType, events)
		if err == nil {
			return len(events)
		}

		// Log error and potentially retry
		logger.Warn("Failed to flush batch", zap.String("stream_type", streamType), zap.Int("attempt", attempt+1), zap.Int("max_attempts", MaxRetryAttempts), zap.Error(err))

		if attempt < MaxRetryAttempts-1 {
			// Exponential backoff before retry
			backoffDuration := BackoffBaseDuration * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return 0 // Context cancelled, stop retrying
			case <-time.After(backoffDuration):
				// Continue to next retry attempt
			}
		}
	}

	// All retry attempts failed
	logger.Error("Failed to flush batch after all retry attempts", zap.String("stream_type", streamType), zap.Int("max_attempts", MaxRetryAttempts))
	return 0
}

// EventPublisherAdapter wraps KinesisClient to implement telemetry.EventPublisher interface
type EventPublisherAdapter struct {
	client *KinesisClient
}

// NewEventPublisherAdapter creates a new adapter for KinesisClient
func NewEventPublisherAdapter(client *KinesisClient) telemetry.EventPublisher {
	return &EventPublisherAdapter{client: client}
}

// PublishTrace implements telemetry.EventPublisher.PublishTrace
func (adapter *EventPublisherAdapter) PublishTrace(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	return adapter.client.PublishTrace(ctx, data, serviceName, traceID, sourceIP, userAgent)
}

// PublishMetrics implements telemetry.EventPublisher.PublishMetrics
func (adapter *EventPublisherAdapter) PublishMetrics(ctx context.Context, data json.RawMessage, serviceName, sourceIP, userAgent string) error {
	return adapter.client.PublishMetrics(ctx, data, serviceName, sourceIP, userAgent)
}

// PublishLogs implements telemetry.EventPublisher.PublishLogs
func (adapter *EventPublisherAdapter) PublishLogs(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	return adapter.client.PublishLogs(ctx, data, serviceName, traceID, sourceIP, userAgent)
}

// Close implements telemetry.EventPublisher.Close
func (adapter *EventPublisherAdapter) Close() error {
	return adapter.client.Close()
}

// Interface compliance checks
var (
	_ telemetry.EventPublisher = (*EventPublisherAdapter)(nil)
)
