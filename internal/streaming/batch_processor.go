package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Batch processing constants
const (
	// FlushTimeoutDuration defines maximum time to wait for final flush during shutdown
	FlushTimeoutDuration = 10 * time.Second

	// MinBatchSize defines minimum batch size to prevent resource waste
	MinBatchSize = 1

	// MinFlushInterval defines minimum flush interval to prevent excessive API calls
	MinFlushInterval = 100 * time.Millisecond

	// DefaultPartitionKey used when no trace ID or service name is available
	DefaultPartitionKey = "unknown"

	// BackoffExponentBase used for exponential backoff calculation (2^attempt)
	BackoffExponentBase = 2

	// InitialRetryAttempt represents the first retry attempt number
	InitialRetryAttempt = 0

	// EventBufferResetSize used when resetting event buffer slice
	EventBufferResetSize = 0

	// NoEventsCount represents zero events
	NoEventsCount = 0

	// NoChannelsCount represents zero channels
	NoChannelsCount = 0

	// FirstAttemptNumber represents attempt number 1 for logging
	FirstAttemptNumber = 1

	// NextAttemptOffset used to calculate next attempt number for logging
	NextAttemptOffset = 2

	// ExpectedChannelCapacity provides hint for channel capacity allocation
	ExpectedChannelCapacity = 8

	// ExpectedRecordCapacity provides hint for records slice allocation
	ExpectedRecordCapacity = 100
)

// Batch processing error messages
const (
	ErrNilKinesisClient     = "kinesis client cannot be nil"
	ErrNilStreamManager     = "stream manager cannot be nil"
	ErrNilConfiguration     = "configuration cannot be nil"
	ErrAlreadyRunning       = "batch processor is already running"
	ErrNoStreamsConfigured  = "no streams configured for batch processing"
	ErrNoActiveStreams      = "no active streams available for batch processing"
	ErrNotRunning           = "batch processor is not running"
	ErrEmptyStreamType      = "stream type cannot be empty"
	ErrChannelFull          = "batch channel full for stream type: %s"
	ErrNoChannelConfigured  = "no batch channel configured for stream type: %s"
	ErrInvalidConfiguration = "invalid configuration: %w"
	ErrGetStreamName        = "failed to get stream name for type %s: %w"
	ErrNoValidRecords       = "no valid records to publish after marshaling %d events"
	ErrPutRecords           = "failed to put %d records to stream %s: %w"
	ErrShutdownTimeout      = "shutdown timeout exceeded after %v"
)

// Log messages
const (
	LogSkippingUnconfiguredStream        = "Skipping unconfigured stream"
	LogBatchProcessorStarted             = "Batch processor started"
	LogStartingBatchProcessor            = "Starting batch processor for stream"
	LogFlushedFullBatch                  = "Flushed full batch"
	LogFlushedOnTimer                    = "Flushed on timer"
	LogContextCancelled                  = "Context cancelled, shutting down batch processor"
	LogFailedToMarshalEvent              = "Failed to marshal event, skipping"
	LogSuccessfullyPublishedBatch        = "Successfully published batch"
	LogNoEventsToFlush                   = "No events to flush during shutdown"
	LogFlushingRemainingEvents           = "Flushing remaining events during shutdown"
	LogFailedToFlushDuringShutdown       = "Failed to flush events during shutdown"
	LogSuccessfullyFlushedDuringShutdown = "Successfully flushed events during shutdown"
	LogFailedToFlushBatch                = "Failed to flush batch"
	LogRetryingAfterBackoff              = "Retrying after backoff"
	LogContextCancelledDuringRetry       = "Context cancelled during retry backoff"
	LogFailedAfterAllRetries             = "Failed to flush batch after all retry attempts"
	LogNotRunningNothingToStop           = "Batch processor is not running, nothing to stop"
	LogStoppingBatchProcessor            = "Stopping batch processor"
	LogAllProcessorsShutDown             = "All batch processors shut down gracefully"
	LogShutdownTimeout                   = "Shutdown timeout, some processors may not have finished"
	LogBatchProcessorStopped             = "Batch processor stopped"
)

// BatchProcessor handles high-throughput batch processing with proper resource management
// and memory leak prevention. It provides async event publishing with configurable
// batching and flush intervals.
type BatchProcessor struct {
	client        *kinesis.Client
	streamManager *StreamManager

	// Batching configuration - immutable after creation
	batchSize     int
	flushInterval time.Duration

	// Resource management - protected by mutex
	mu            sync.RWMutex
	isRunning     bool
	shutdownCh    chan struct{}
	wg            sync.WaitGroup
	batchChannels map[string]chan LegacyTelemetryEvent
}

// BatchProcessorConfig holds configuration for BatchProcessor creation
type BatchProcessorConfig struct {
	BatchSize     int
	FlushInterval time.Duration
}

// ValidateConfig validates BatchProcessor configuration parameters
func (cfg *BatchProcessorConfig) ValidateConfig() error {
	if cfg.BatchSize < MinBatchSize {
		return fmt.Errorf("batch size %d is below minimum %d", cfg.BatchSize, MinBatchSize)
	}
	if cfg.BatchSize > MaxChannelBufferSize {
		return fmt.Errorf("batch size %d exceeds maximum %d", cfg.BatchSize, MaxChannelBufferSize)
	}
	if cfg.FlushInterval < MinFlushInterval {
		return fmt.Errorf("flush interval %v is below minimum %v", cfg.FlushInterval, MinFlushInterval)
	}
	return nil
}

// NewBatchProcessor creates a new batch processor with validated configuration
func NewBatchProcessor(client *kinesis.Client, streamManager *StreamManager, cfg *BatchProcessorConfig) (*BatchProcessor, error) {
	if client == nil {
		return nil, errors.New(ErrNilKinesisClient)
	}
	if streamManager == nil {
		return nil, errors.New(ErrNilStreamManager)
	}
	if cfg == nil {
		return nil, errors.New(ErrNilConfiguration)
	}

	if err := cfg.ValidateConfig(); err != nil {
		return nil, fmt.Errorf(ErrInvalidConfiguration, err)
	}

	return &BatchProcessor{
		client:        client,
		streamManager: streamManager,
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		shutdownCh:    make(chan struct{}),
		batchChannels: make(map[string]chan LegacyTelemetryEvent, ExpectedChannelCapacity),
	}, nil
}

// Start enables high-throughput batch processing with proper resource management
// This method is idempotent and thread-safe
func (bp *BatchProcessor) Start(ctx context.Context) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.isRunning {
		return errors.New(ErrAlreadyRunning)
	}

	// Get configured streams
	streams := bp.streamManager.GetStreams()
	if len(streams) == NoChannelsCount {
		return errors.New(ErrNoStreamsConfigured)
	}

	// Initialize batch channels for each configured stream type
	activeStreams := NoChannelsCount
	for streamType, streamName := range streams {
		if streamName == "" {
			logger.Debug(LogSkippingUnconfiguredStream, zap.String("stream_type", streamType))
			continue
		}

		bp.batchChannels[streamType] = make(chan LegacyTelemetryEvent, bp.batchSize)

		// Start batch processor for this stream type
		bp.wg.Add(FirstAttemptNumber)
		go bp.processBatch(ctx, streamType)
		activeStreams++
	}

	if activeStreams == NoChannelsCount {
		return errors.New(ErrNoActiveStreams)
	}

	bp.isRunning = true

	logger.Info(LogBatchProcessorStarted,
		zap.Int("batch_size", bp.batchSize),
		zap.Duration("flush_interval", bp.flushInterval),
		zap.Int("active_streams", activeStreams))

	return nil
}

// PublishAsync sends events to batch processor for high-throughput scenarios
// Returns error if processor is not running or channel is full
func (bp *BatchProcessor) PublishAsync(streamType string, event LegacyTelemetryEvent) error {
	if streamType == "" {
		return errors.New(ErrEmptyStreamType)
	}

	bp.mu.RLock()
	defer bp.mu.RUnlock()

	if !bp.isRunning {
		return errors.New(ErrNotRunning)
	}

	ch, exists := bp.batchChannels[streamType]
	if !exists {
		return fmt.Errorf(ErrNoChannelConfigured, streamType)
	}

	select {
	case ch <- event:
		return nil
	default:
		// Channel is full, reject event to prevent blocking
		return fmt.Errorf(ErrChannelFull, streamType)
	}
}

// processBatch handles batch processing for a specific stream type with proper error handling
func (bp *BatchProcessor) processBatch(ctx context.Context, streamType string) {
	defer bp.wg.Done()

	ch := bp.batchChannels[streamType]
	ticker := time.NewTicker(bp.flushInterval)
	defer ticker.Stop()

	var eventBuffer []LegacyTelemetryEvent

	logger.Debug(LogStartingBatchProcessor, zap.String("stream_type", streamType))

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				// Channel closed, flush remaining events and exit
				bp.handleGracefulShutdown(ctx, streamType, eventBuffer)
				return
			}

			eventBuffer = append(eventBuffer, event)

			// Flush when batch is full
			if len(eventBuffer) >= bp.batchSize {
				flushedCount := bp.flushBatchSafely(ctx, streamType, eventBuffer)
				eventBuffer = eventBuffer[:EventBufferResetSize] // Reset buffer

				logger.Debug(LogFlushedFullBatch,
					zap.String("stream_type", streamType),
					zap.Int("flushed_count", flushedCount),
					zap.Int("batch_size", bp.batchSize))
			}

		case <-ticker.C:
			// Flush on timer if we have events
			if len(eventBuffer) > NoEventsCount {
				flushedCount := bp.flushBatchSafely(ctx, streamType, eventBuffer)
				eventBuffer = eventBuffer[:EventBufferResetSize] // Reset buffer

				logger.Debug(LogFlushedOnTimer,
					zap.String("stream_type", streamType),
					zap.Int("flushed_count", flushedCount),
					zap.Duration("flush_interval", bp.flushInterval))
			}

		case <-bp.shutdownCh:
			// Shutdown signal received, flush remaining events and exit
			bp.handleGracefulShutdown(ctx, streamType, eventBuffer)
			return

		case <-ctx.Done():
			// Context cancelled, flush remaining events and exit
			logger.Info(LogContextCancelled,
				zap.String("stream_type", streamType),
				zap.Int("remaining_events", len(eventBuffer)))
			bp.handleGracefulShutdown(ctx, streamType, eventBuffer)
			return
		}
	}
}

// flushBatch sends a batch of events to Kinesis with comprehensive error handling
func (bp *BatchProcessor) flushBatch(ctx context.Context, streamType string, events []LegacyTelemetryEvent) error {
	if len(events) == NoEventsCount {
		return nil
	}

	streamName, err := bp.streamManager.GetStreamName(streamType)
	if err != nil {
		return fmt.Errorf(ErrGetStreamName, streamType, err)
	}

	// Convert events to Kinesis records
	records := make([]types.PutRecordsRequestEntry, NoEventsCount, ExpectedRecordCapacity)
	for i, event := range events {
		data, err := marshalEventToJSON(event)
		if err != nil {
			logger.Error(LogFailedToMarshalEvent,
				zap.Int("event_index", i),
				zap.String("stream_type", streamType),
				zap.Error(err))
			continue
		}

		partitionKey := bp.determinePartitionKey(event)

		records = append(records, types.PutRecordsRequestEntry{
			Data:         data,
			PartitionKey: aws.String(partitionKey),
		})
	}

	if len(records) == NoEventsCount {
		return fmt.Errorf(ErrNoValidRecords, len(events))
	}

	// Send batch to Kinesis
	result, err := bp.client.PutRecords(ctx, &kinesis.PutRecordsInput{
		StreamName: aws.String(streamName),
		Records:    records,
	})
	if err != nil {
		return fmt.Errorf(ErrPutRecords, len(records), streamName, err)
	}

	logger.Debug(LogSuccessfullyPublishedBatch,
		zap.Int("total_records", len(records)),
		zap.String("stream_name", streamName),
		zap.String("stream_type", streamType),
		zap.Int32("failed_records", *result.FailedRecordCount))

	return nil
}

// determinePartitionKey selects appropriate partition key with fallback strategy
func (bp *BatchProcessor) determinePartitionKey(event LegacyTelemetryEvent) string {
	if event.TraceID != "" {
		return event.TraceID
	}
	if event.ServiceName != "" {
		return event.ServiceName
	}
	return DefaultPartitionKey
}

// handleGracefulShutdown performs graceful shutdown operations for a single batch processor
func (bp *BatchProcessor) handleGracefulShutdown(ctx context.Context, streamType string, eventBuffer []LegacyTelemetryEvent) {
	if len(eventBuffer) == NoEventsCount {
		logger.Debug(LogNoEventsToFlush, zap.String("stream_type", streamType))
		return
	}

	logger.Info(LogFlushingRemainingEvents,
		zap.Int("event_count", len(eventBuffer)),
		zap.String("stream_type", streamType))

	// Create timeout context for final flush
	flushCtx, cancel := context.WithTimeout(context.Background(), FlushTimeoutDuration)
	defer cancel()

	if err := bp.flushBatch(flushCtx, streamType, eventBuffer); err != nil {
		logger.Error(LogFailedToFlushDuringShutdown,
			zap.Int("event_count", len(eventBuffer)),
			zap.String("stream_type", streamType),
			zap.Duration("timeout", FlushTimeoutDuration),
			zap.Error(err))
	} else {
		logger.Info(LogSuccessfullyFlushedDuringShutdown,
			zap.Int("event_count", len(eventBuffer)),
			zap.String("stream_type", streamType))
	}
}

// flushBatchSafely wraps batch flushing with error handling and exponential backoff retry logic
// Returns the number of successfully flushed events
func (bp *BatchProcessor) flushBatchSafely(ctx context.Context, streamType string, events []LegacyTelemetryEvent) int {
	if len(events) == NoEventsCount {
		return NoEventsCount
	}

	// Attempt flush with exponential backoff retry logic
	for attempt := InitialRetryAttempt; attempt < MaxRetryAttempts; attempt++ {
		err := bp.flushBatch(ctx, streamType, events)
		if err == nil {
			return len(events)
		}

		// Log error and potentially retry
		logger.Warn(LogFailedToFlushBatch,
			zap.String("stream_type", streamType),
			zap.Int("attempt", attempt+FirstAttemptNumber),
			zap.Int("max_attempts", MaxRetryAttempts),
			zap.Int("event_count", len(events)),
			zap.Error(err))

		if attempt < MaxRetryAttempts-FirstAttemptNumber {
			// Exponential backoff before retry
			backoffDuration := BackoffBaseDuration * time.Duration(BackoffExponentBase<<uint(attempt))
			logger.Debug(LogRetryingAfterBackoff,
				zap.Duration("backoff_duration", backoffDuration),
				zap.Int("next_attempt", attempt+NextAttemptOffset))

			select {
			case <-ctx.Done():
				logger.Warn(LogContextCancelledDuringRetry,
					zap.String("stream_type", streamType))
				return NoEventsCount
			case <-time.After(backoffDuration):
				// Continue to next retry attempt
			}
		}
	}

	// All retry attempts failed
	logger.Error(LogFailedAfterAllRetries,
		zap.String("stream_type", streamType),
		zap.Int("max_attempts", MaxRetryAttempts),
		zap.Int("lost_events", len(events)))
	return NoEventsCount
}

// Stop performs graceful shutdown of the batch processor with proper resource cleanup
// This method is idempotent and thread-safe
func (bp *BatchProcessor) Stop() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if !bp.isRunning {
		logger.Debug(LogNotRunningNothingToStop)
		return nil
	}

	logger.Info(LogStoppingBatchProcessor)

	// Signal all processors to shut down
	close(bp.shutdownCh)

	// Wait for all batch processors to complete with timeout
	shutdownComplete := make(chan struct{})
	go func() {
		bp.wg.Wait()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		logger.Info(LogAllProcessorsShutDown)
	case <-time.After(ShutdownTimeout):
		logger.Warn(LogShutdownTimeout,
			zap.Duration("timeout", ShutdownTimeout))
		return fmt.Errorf(ErrShutdownTimeout, ShutdownTimeout)
	}

	// Close all batch channels to prevent memory leaks
	channelsClosed := NoChannelsCount
	for streamType, ch := range bp.batchChannels {
		close(ch)
		delete(bp.batchChannels, streamType)
		channelsClosed++
	}

	bp.isRunning = false

	logger.Info(LogBatchProcessorStopped,
		zap.Int("channels_closed", channelsClosed))

	return nil
}

// IsRunning returns whether the batch processor is currently running
// This method is thread-safe
func (bp *BatchProcessor) IsRunning() bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.isRunning
}

// GetStats returns current statistics about the batch processor
// This method is thread-safe
func (bp *BatchProcessor) GetStats() map[string]interface{} {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	configuredStreams := make([]string, NoEventsCount, len(bp.batchChannels))
	for streamType := range bp.batchChannels {
		configuredStreams = append(configuredStreams, streamType)
	}

	return map[string]interface{}{
		"is_running":         bp.isRunning,
		"batch_size":         bp.batchSize,
		"flush_interval":     bp.flushInterval.String(),
		"channel_count":      len(bp.batchChannels),
		"configured_streams": configuredStreams,
	}
}

// marshalEventToJSON marshals a LegacyTelemetryEvent to JSON bytes
func marshalEventToJSON(event LegacyTelemetryEvent) ([]byte, error) {
	return json.Marshal(event)
}
