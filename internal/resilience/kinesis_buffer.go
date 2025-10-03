package resilience

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Worker pool constants
const (
	// DefaultWorkerPoolSize defines default number of worker goroutines
	DefaultWorkerPoolSize = 10

	// MaxWorkerPoolSize defines maximum number of worker goroutines to prevent resource exhaustion
	MaxWorkerPoolSize = 50

	// WorkerChannelBufferSize defines buffer size for worker job channels
	WorkerChannelBufferSize = 100

	// AsyncFlushTimeout defines timeout for background flush operations
	AsyncFlushTimeout = 30 * time.Second

	// MaxConcurrentBatchOps defines maximum concurrent batch processing operations
	MaxConcurrentBatchOps = 4
)

var (
	ErrBufferFull      = errors.New("kinesis buffer is full")
	ErrBufferClosed    = errors.New("kinesis buffer is closed")
	ErrTenantThrottled = errors.New("tenant is being throttled")
	ErrWorkerPoolFull  = errors.New("worker pool is full, flush job rejected")
)

// FlushJob represents a background flush job for the worker pool
type FlushJob struct {
	TenantID string
	Ctx      context.Context
	Cancel   context.CancelFunc
}

// KinesisBuffer provides resilient buffering and batching for Kinesis
type KinesisBuffer struct {
	kinesisClient   *streaming.KinesisClient
	rateLimiter     *TenantRateLimiter
	circuitBreaker  *CircuitBreaker
	deadLetterQueue *DeadLetterQueue

	// Buffer settings
	maxBatchSize    int
	maxBatchWait    time.Duration
	maxBufferSize   int
	maxTenantBuffer int
	flushInterval   time.Duration

	// Per-tenant buffers
	tenantBuffers map[string]*TenantBuffer
	bufferMutex   sync.RWMutex

	// Bounded worker pool for async operations
	workerPoolSize int
	flushJobChan   chan FlushJob
	workerWg       sync.WaitGroup

	// Semaphore for bounded concurrent batch operations
	batchSemaphore chan struct{}

	// Global state
	closed    int32 // Use atomic operations (0=open, 1=closed)
	closeChan chan struct{}
	closeOnce sync.Once // Ensure channel is closed only once
	wg        sync.WaitGroup

	// Health monitoring
	healthMetrics *BufferHealthMetrics
}

// TenantBuffer holds buffered events for a specific tenant
type TenantBuffer struct {
	tenantID      string
	events        []streaming.TelemetryEvent
	mutex         sync.Mutex
	lastFlush     time.Time
	totalEvents   int64
	droppedEvents int64
	maxSize       int
}

// BufferConfig holds configuration for the Kinesis buffer
type BufferConfig struct {
	MaxBatchSize    int
	MaxBatchWait    time.Duration
	MaxBufferSize   int
	FlushInterval   time.Duration
	MaxTenantBuffer int
	WorkerPoolSize  int // Number of worker goroutines for async flush operations
}

// BufferHealthMetrics tracks buffer health and performance
type BufferHealthMetrics struct {
	mutex                sync.RWMutex
	totalEventsBuffered  int64
	totalEventsProcessed int64
	totalEventsDropped   int64
	totalEventsFailed    int64
	totalBatchesSent     int64
	totalBatchesFailed   int64
	avgBatchSize         float64
	avgProcessingTime    time.Duration
	lastFlushTime        time.Time
}

// NewKinesisBuffer creates a new resilient Kinesis buffer
func NewKinesisBuffer(
	kinesisClient *streaming.KinesisClient,
	rateLimiter *TenantRateLimiter,
	circuitBreaker *CircuitBreaker,
	deadLetterQueue *DeadLetterQueue,
	config BufferConfig,
) *KinesisBuffer {
	if config.MaxBatchSize == 0 {
		config.MaxBatchSize = 500
	}
	if config.MaxBatchWait == 0 {
		config.MaxBatchWait = 1 * time.Second
	}
	if config.MaxBufferSize == 0 {
		config.MaxBufferSize = 10000
	}
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Second
	}
	if config.MaxTenantBuffer == 0 {
		config.MaxTenantBuffer = 1000
	}
	if config.WorkerPoolSize == 0 {
		config.WorkerPoolSize = DefaultWorkerPoolSize
	} else if config.WorkerPoolSize > MaxWorkerPoolSize {
		logger.Warn("Worker pool size exceeds maximum, using maximum",
			zap.Int("requested_size", config.WorkerPoolSize),
			zap.Int("max_size", MaxWorkerPoolSize))
		config.WorkerPoolSize = MaxWorkerPoolSize
	}

	kb := &KinesisBuffer{
		kinesisClient:   kinesisClient,
		rateLimiter:     rateLimiter,
		circuitBreaker:  circuitBreaker,
		deadLetterQueue: deadLetterQueue,
		maxBatchSize:    config.MaxBatchSize,
		maxBatchWait:    config.MaxBatchWait,
		maxBufferSize:   config.MaxBufferSize,
		maxTenantBuffer: config.MaxTenantBuffer,
		flushInterval:   config.FlushInterval,
		workerPoolSize:  config.WorkerPoolSize,
		tenantBuffers:   make(map[string]*TenantBuffer),
		flushJobChan:    make(chan FlushJob, WorkerChannelBufferSize),
		batchSemaphore:  make(chan struct{}, MaxConcurrentBatchOps),
		closeChan:       make(chan struct{}),
		healthMetrics:   &BufferHealthMetrics{},
	}

	// Start bounded worker pool for async flush operations
	kb.startWorkerPool()

	// Start background flush goroutine
	kb.wg.Add(1)
	go kb.flushLoop()

	return kb
}

// startWorkerPool initializes and starts the bounded worker pool for async flush operations
func (kb *KinesisBuffer) startWorkerPool() {
	logger.Info("Starting bounded worker pool",
		zap.Int("worker_count", kb.workerPoolSize),
		zap.Int("channel_buffer", WorkerChannelBufferSize))

	// Start worker goroutines
	for i := 0; i < kb.workerPoolSize; i++ {
		kb.workerWg.Add(1)
		go kb.flushWorker(i)
	}
}

// flushWorker processes flush jobs from the job channel
func (kb *KinesisBuffer) flushWorker(workerID int) {
	defer kb.workerWg.Done()

	logger.Debug("Flush worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case job, ok := <-kb.flushJobChan:
			if !ok {
				logger.Debug("Flush worker stopping - job channel closed", zap.Int("worker_id", workerID))
				return
			}

			// Get the tenant buffer
			kb.bufferMutex.RLock()
			buffer, exists := kb.tenantBuffers[job.TenantID]
			kb.bufferMutex.RUnlock()

			if !exists || buffer == nil {
				logger.Debug("Tenant buffer not found for flush job",
					zap.String("tenant", job.TenantID),
					zap.Int("worker_id", workerID))
				job.Cancel() // Cancel context since job won't be processed
				continue
			}

			// Perform the flush operation
			if err := kb.flushTenantBuffer(job.Ctx, job.TenantID, buffer); err != nil {
				logger.Error("Failed to flush buffer in worker",
					zap.String("tenant", job.TenantID),
					zap.Int("worker_id", workerID),
					zap.Error(err))
			}

			// Always cancel the context when job is complete
			job.Cancel()

		case <-kb.closeChan:
			logger.Debug("Flush worker stopping - buffer closed", zap.Int("worker_id", workerID))
			return
		}
	}
}

// submitFlushJob submits a flush job to the worker pool with bounded queuing
func (kb *KinesisBuffer) submitFlushJob(tenantID string) error {
	if atomic.LoadInt32(&kb.closed) != 0 {
		return ErrBufferClosed
	}

	// Create background context with timeout for async flush
	// Don't inherit request context which gets cancelled when handler returns
	flushCtx, cancel := context.WithTimeout(context.Background(), AsyncFlushTimeout)

	job := FlushJob{
		TenantID: tenantID,
		Ctx:      flushCtx,
		Cancel:   cancel,
	}

	// Try to submit job without blocking
	select {
	case kb.flushJobChan <- job:
		// Job successfully queued - worker will handle context cancellation
		return nil
	default:
		// Job queue is full - cancel context and return error
		cancel()
		logger.Warn("Flush job queue is full, rejecting job",
			zap.String("tenant", tenantID),
			zap.Int("queue_size", WorkerChannelBufferSize))
		return ErrWorkerPoolFull
	}
}

// BufferEvent adds an event to the buffer with resilience features
func (kb *KinesisBuffer) BufferEvent(ctx context.Context, event streaming.TelemetryEvent, tenantID, sourceEndpoint string) error {
	if atomic.LoadInt32(&kb.closed) != 0 {
		return ErrBufferClosed
	}

	// Apply tenant rate limiting
	if !kb.rateLimiter.Allow(tenantID) {
		kb.healthMetrics.incrementDropped()
		return ErrTenantThrottled
	}

	// Get or create tenant buffer
	buffer := kb.getTenantBuffer(tenantID)

	// Try to add to buffer
	if err := buffer.addEvent(event); err != nil {
		if err == ErrBufferFull {
			// Try to flush immediately and retry once
			if flushErr := kb.flushTenantBuffer(ctx, tenantID, buffer); flushErr != nil {
				// Send to DLQ if flush fails
				dlqErr := kb.deadLetterQueue.SendToDLQ(ctx, event, flushErr, tenantID, sourceEndpoint, "buffer_full_flush_failed")
				if dlqErr != nil {
					logger.Error("Failed to send to DLQ after buffer full", zap.Error(dlqErr))
				}
				kb.healthMetrics.incrementDropped()
				return fmt.Errorf("buffer full and flush failed: %w", flushErr)
			}

			// Retry adding after flush
			if retryErr := buffer.addEvent(event); retryErr != nil {
				kb.healthMetrics.incrementDropped()
				return retryErr
			}
		} else {
			kb.healthMetrics.incrementDropped()
			return err
		}
	}

	kb.healthMetrics.incrementBuffered()

	// Check if we should flush immediately (batch size reached)
	if buffer.size() >= kb.maxBatchSize {
		// Submit flush job to bounded worker pool instead of spawning unbounded goroutines
		if err := kb.submitFlushJob(tenantID); err != nil {
			// If worker pool is full, log warning but don't fail the request
			// The periodic flush will eventually handle this buffer
			if err == ErrWorkerPoolFull {
				logger.Warn("Worker pool full, deferring flush to periodic timer",
					zap.String("tenant", tenantID),
					zap.Int("buffer_size", buffer.size()))
			} else {
				logger.Error("Failed to submit flush job",
					zap.String("tenant", tenantID),
					zap.Error(err))
			}
		}
	}

	return nil
}

func (kb *KinesisBuffer) getTenantBuffer(tenantID string) *TenantBuffer {
	kb.bufferMutex.RLock()
	buffer, exists := kb.tenantBuffers[tenantID]
	kb.bufferMutex.RUnlock()

	if exists {
		return buffer
	}

	// Create new buffer
	kb.bufferMutex.Lock()
	defer kb.bufferMutex.Unlock()

	// Double-check after acquiring write lock
	if buffer, exists := kb.tenantBuffers[tenantID]; exists {
		return buffer
	}

	buffer = &TenantBuffer{
		tenantID:  tenantID,
		events:    make([]streaming.TelemetryEvent, 0, kb.maxBatchSize),
		lastFlush: time.Now(),
		maxSize:   kb.maxTenantBuffer, // Use configured per-tenant limit
	}

	kb.tenantBuffers[tenantID] = buffer

	logger.Debug("Created new tenant buffer", zap.String("tenant", tenantID))
	return buffer
}

func (tb *TenantBuffer) addEvent(event streaming.TelemetryEvent) error {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	if len(tb.events) >= tb.maxSize {
		tb.droppedEvents++
		return ErrBufferFull
	}

	tb.events = append(tb.events, event)
	tb.totalEvents++
	return nil
}

func (tb *TenantBuffer) size() int {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	return len(tb.events)
}

func (tb *TenantBuffer) hasData() bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()
	return len(tb.events) > 0
}

func (tb *TenantBuffer) getEvents() []streaming.TelemetryEvent {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	if len(tb.events) == 0 {
		return nil
	}

	// Return copy and clear buffer
	events := make([]streaming.TelemetryEvent, len(tb.events))
	copy(events, tb.events)
	tb.events = tb.events[:0] // Clear but keep capacity
	tb.lastFlush = time.Now()

	return events
}

func (kb *KinesisBuffer) flushLoop() {
	defer kb.wg.Done()
	ticker := time.NewTicker(kb.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-kb.closeChan:
			// Final flush
			kb.flushAll(context.Background())
			return
		case <-ticker.C:
			kb.flushAll(context.Background())
		}
	}
}

func (kb *KinesisBuffer) flushAll(ctx context.Context) {
	// Collect tenants that actually have data to flush
	kb.bufferMutex.RLock()
	tenantsToFlush := make([]string, 0, len(kb.tenantBuffers))
	for tenantID, buffer := range kb.tenantBuffers {
		if buffer.hasData() {
			tenantsToFlush = append(tenantsToFlush, tenantID)
		}
	}
	kb.bufferMutex.RUnlock()

	// If no tenants have data, return early
	if len(tenantsToFlush) == 0 {
		kb.healthMetrics.updateLastFlushTime()
		return
	}

	// Use a worker pool to limit concurrent goroutines
	maxWorkers := min(len(tenantsToFlush), 10) // Cap at 10 concurrent flushes
	workChan := make(chan string, len(tenantsToFlush))
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for tenantID := range workChan {
				kb.bufferMutex.RLock()
				buffer := kb.tenantBuffers[tenantID]
				kb.bufferMutex.RUnlock()

				if buffer != nil {
					if err := kb.flushTenantBuffer(ctx, tenantID, buffer); err != nil {
						logger.Error("Failed to flush tenant buffer",
							zap.String("tenant", tenantID),
							zap.Error(err))
					}
				}
			}
		}()
	}

	// Send work to workers
	for _, tenantID := range tenantsToFlush {
		workChan <- tenantID
	}
	close(workChan)

	wg.Wait()
	kb.healthMetrics.updateLastFlushTime()
}

func (kb *KinesisBuffer) flushTenantBuffer(ctx context.Context, tenantID string, buffer *TenantBuffer) error {
	events := buffer.getEvents()
	if len(events) == 0 {
		return nil
	}

	startTime := time.Now()

	// Process events in batches
	for i := 0; i < len(events); i += kb.maxBatchSize {
		end := i + kb.maxBatchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]

		if err := kb.processBatch(ctx, batch, tenantID); err != nil {
			// Send failed events to DLQ
			for _, event := range batch {
				if dlqErr := kb.deadLetterQueue.SendToDLQ(ctx, event, err, tenantID, "kinesis_buffer", "batch_processing_failed"); dlqErr != nil {
					logger.Error("Failed to send failed event to DLQ", zap.Error(dlqErr))
				}
			}
			kb.healthMetrics.incrementBatchFailed()
			return err
		}

		kb.healthMetrics.incrementBatchSent()
		kb.healthMetrics.addProcessedEvents(int64(len(batch)))
	}

	processingTime := time.Since(startTime)
	kb.healthMetrics.updateProcessingTime(processingTime)

	logger.Debug("Flushed tenant buffer",
		zap.String("tenant", tenantID),
		zap.Int("events", len(events)),
		zap.Duration("processing_time", processingTime))

	return nil
}

func (kb *KinesisBuffer) processBatch(ctx context.Context, events []streaming.TelemetryEvent, tenantID string) error {
	return kb.circuitBreaker.Call(func() error {
		// Group events by type for efficient processing
		traceEvents := make([]*streaming.TraceTelemetryEvent, 0)
		metricEvents := make([]*streaming.MetricsTelemetryEvent, 0)
		logEvents := make([]*streaming.LogsTelemetryEvent, 0)
		legacyEvents := make([]*streaming.LegacyTelemetryEvent, 0)

		for _, event := range events {
			switch e := event.(type) {
			case *streaming.TraceTelemetryEvent:
				traceEvents = append(traceEvents, e)
			case *streaming.MetricsTelemetryEvent:
				metricEvents = append(metricEvents, e)
			case *streaming.LogsTelemetryEvent:
				logEvents = append(logEvents, e)
			case *streaming.LegacyTelemetryEvent:
				legacyEvents = append(legacyEvents, e)
			}
		}

		// Process each type in parallel using semaphore-controlled concurrency
		// This maintains parallel processing benefits while bounding goroutine count
		var wg sync.WaitGroup
		var errors []error
		var errorsMutex sync.Mutex

		// Helper function to safely add errors
		addError := func(err error) {
			errorsMutex.Lock()
			errors = append(errors, err)
			errorsMutex.Unlock()
		}

		// Process traces with semaphore control
		if len(traceEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Acquire semaphore
				kb.batchSemaphore <- struct{}{}
				defer func() { <-kb.batchSemaphore }()

				// Use batch processing for efficiency
				for _, event := range traceEvents {
					if err := kb.kinesisClient.PublishTraceProtobuf(ctx, event.ResourceSpans,
						event.ServiceName, event.TraceID, event.Metadata.SourceIP); err != nil {
						addError(fmt.Errorf("failed to publish trace: %w", err))
					}
				}
			}()
		}

		// Process metrics with semaphore control
		if len(metricEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Acquire semaphore
				kb.batchSemaphore <- struct{}{}
				defer func() { <-kb.batchSemaphore }()

				for _, event := range metricEvents {
					if err := kb.kinesisClient.PublishMetricsProtobuf(ctx, event.ResourceMetrics,
						event.ServiceName, event.Metadata.SourceIP); err != nil {
						addError(fmt.Errorf("failed to publish metrics: %w", err))
					}
				}
			}()
		}

		// Process logs with semaphore control
		if len(logEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Acquire semaphore
				kb.batchSemaphore <- struct{}{}
				defer func() { <-kb.batchSemaphore }()

				for _, event := range logEvents {
					if err := kb.kinesisClient.PublishLogsProtobuf(ctx, event.ResourceLogs,
						event.ServiceName, event.TraceID, event.Metadata.SourceIP); err != nil {
						addError(fmt.Errorf("failed to publish logs: %w", err))
					}
				}
			}()
		}

		// Process legacy events with semaphore control and batch API
		if len(legacyEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Acquire semaphore
				kb.batchSemaphore <- struct{}{}
				defer func() { <-kb.batchSemaphore }()

				// Group legacy events by type for batch processing
				traceEventsLegacy := make([]streaming.LegacyTelemetryEvent, 0)
				metricEventsLegacy := make([]streaming.LegacyTelemetryEvent, 0)
				logEventsLegacy := make([]streaming.LegacyTelemetryEvent, 0)

				for _, event := range legacyEvents {
					switch event.Type {
					case "traces":
						traceEventsLegacy = append(traceEventsLegacy, *event)
					case "metrics":
						metricEventsLegacy = append(metricEventsLegacy, *event)
					case "logs":
						logEventsLegacy = append(logEventsLegacy, *event)
					}
				}

				// Use batch APIs for efficiency
				if len(traceEventsLegacy) > 0 {
					if err := kb.kinesisClient.PublishBatch(ctx, "traces", traceEventsLegacy); err != nil {
						addError(fmt.Errorf("failed to publish legacy trace batch: %w", err))
					}
				}
				if len(metricEventsLegacy) > 0 {
					if err := kb.kinesisClient.PublishBatch(ctx, "metrics", metricEventsLegacy); err != nil {
						addError(fmt.Errorf("failed to publish legacy metrics batch: %w", err))
					}
				}
				if len(logEventsLegacy) > 0 {
					if err := kb.kinesisClient.PublishBatch(ctx, "logs", logEventsLegacy); err != nil {
						addError(fmt.Errorf("failed to publish legacy logs batch: %w", err))
					}
				}
			}()
		}

		wg.Wait()

		if len(errors) > 0 {
			return fmt.Errorf("batch processing failed with %d errors: %v", len(errors), errors[0])
		}

		return nil
	})
}

// Close gracefully shuts down the buffer
func (kb *KinesisBuffer) Close(ctx context.Context) error {
	// Atomically check and set closed flag
	if !atomic.CompareAndSwapInt32(&kb.closed, 0, 1) {
		return nil // Already closed
	}

	// Ensure channel is closed only once using sync.Once
	kb.closeOnce.Do(func() {
		close(kb.closeChan)
	})

	// Wait for flush goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		kb.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Kinesis buffer closed gracefully")
	case <-time.After(30 * time.Second):
		logger.Warn("Kinesis buffer close timed out")
	}

	return nil
}

// Health check methods
func (bhm *BufferHealthMetrics) incrementBuffered() {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	bhm.totalEventsBuffered++
}

func (bhm *BufferHealthMetrics) incrementDropped() {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	bhm.totalEventsDropped++
}

func (bhm *BufferHealthMetrics) incrementBatchSent() {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	bhm.totalBatchesSent++
}

func (bhm *BufferHealthMetrics) incrementBatchFailed() {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	bhm.totalBatchesFailed++
}

func (bhm *BufferHealthMetrics) addProcessedEvents(count int64) {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	bhm.totalEventsProcessed += count
}

func (bhm *BufferHealthMetrics) updateProcessingTime(duration time.Duration) {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	// Simple moving average
	bhm.avgProcessingTime = (bhm.avgProcessingTime + duration) / 2
}

func (bhm *BufferHealthMetrics) updateLastFlushTime() {
	bhm.mutex.Lock()
	defer bhm.mutex.Unlock()
	bhm.lastFlushTime = time.Now()
}

// GetHealthMetrics returns current health metrics
func (kb *KinesisBuffer) GetHealthMetrics() *BufferHealthMetrics {
	kb.healthMetrics.mutex.RLock()
	defer kb.healthMetrics.mutex.RUnlock()

	// Return a copy
	return &BufferHealthMetrics{
		totalEventsBuffered:  kb.healthMetrics.totalEventsBuffered,
		totalEventsProcessed: kb.healthMetrics.totalEventsProcessed,
		totalEventsDropped:   kb.healthMetrics.totalEventsDropped,
		totalEventsFailed:    kb.healthMetrics.totalEventsFailed,
		totalBatchesSent:     kb.healthMetrics.totalBatchesSent,
		totalBatchesFailed:   kb.healthMetrics.totalBatchesFailed,
		avgBatchSize:         kb.healthMetrics.avgBatchSize,
		avgProcessingTime:    kb.healthMetrics.avgProcessingTime,
		lastFlushTime:        kb.healthMetrics.lastFlushTime,
	}
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
