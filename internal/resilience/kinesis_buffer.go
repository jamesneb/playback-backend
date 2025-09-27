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

var (
	ErrBufferFull     = errors.New("kinesis buffer is full")
	ErrBufferClosed   = errors.New("kinesis buffer is closed")
	ErrTenantThrottled = errors.New("tenant is being throttled")
)

// KinesisBuffer provides resilient buffering and batching for Kinesis
type KinesisBuffer struct {
	kinesisClient    *streaming.KinesisClient
	rateLimiter      *TenantRateLimiter
	circuitBreaker   *CircuitBreaker
	deadLetterQueue  *DeadLetterQueue
	
	// Buffer settings
	maxBatchSize     int
	maxBatchWait     time.Duration
	maxBufferSize    int
	maxTenantBuffer  int
	flushInterval    time.Duration
	
	// Per-tenant buffers
	tenantBuffers    map[string]*TenantBuffer
	bufferMutex      sync.RWMutex
	
	// Global state
	closed           int32           // Use atomic operations (0=open, 1=closed)
	closeChan        chan struct{}
	closeOnce        sync.Once       // Ensure channel is closed only once
	wg               sync.WaitGroup
	
	// Health monitoring
	healthMetrics    *BufferHealthMetrics
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
		tenantBuffers:   make(map[string]*TenantBuffer),
		closeChan:       make(chan struct{}),
		healthMetrics:   &BufferHealthMetrics{},
	}

	// Start background flush goroutine
	kb.wg.Add(1)
	go kb.flushLoop()

	return kb
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
		go func() {
			// Create background context with timeout for async flush
			// Don't inherit request context which gets cancelled when handler returns
			flushCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := kb.flushTenantBuffer(flushCtx, tenantID, buffer); err != nil {
				logger.Error("Failed to flush full buffer",
					zap.String("tenant", tenantID),
					zap.Error(err))
			}
		}()
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
		tenantID:    tenantID,
		events:      make([]streaming.TelemetryEvent, 0, kb.maxBatchSize),
		lastFlush:   time.Now(),
		maxSize:     kb.maxTenantBuffer, // Use configured per-tenant limit
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
	kb.bufferMutex.RLock()
	tenantIDs := make([]string, 0, len(kb.tenantBuffers))
	for tenantID := range kb.tenantBuffers {
		tenantIDs = append(tenantIDs, tenantID)
	}
	kb.bufferMutex.RUnlock()

	var wg sync.WaitGroup
	for _, tenantID := range tenantIDs {
		wg.Add(1)
		go func(tID string) {
			defer wg.Done()
			
			kb.bufferMutex.RLock()
			buffer := kb.tenantBuffers[tID]
			kb.bufferMutex.RUnlock()
			
			if buffer != nil {
				if err := kb.flushTenantBuffer(ctx, tID, buffer); err != nil {
					logger.Error("Failed to flush tenant buffer",
						zap.String("tenant", tID),
						zap.Error(err))
				}
			}
		}(tenantID)
	}

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

		// Process each type in parallel
		var wg sync.WaitGroup
		var errors []error
		var errorsMutex sync.Mutex

		// Process traces
		if len(traceEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, event := range traceEvents {
					if err := kb.kinesisClient.PublishTraceProtobuf(ctx, event.ResourceSpans, 
						event.ServiceName, event.TraceID, event.Metadata.SourceIP); err != nil {
						errorsMutex.Lock()
						errors = append(errors, fmt.Errorf("failed to publish trace: %w", err))
						errorsMutex.Unlock()
					}
				}
			}()
		}

		// Process metrics
		if len(metricEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, event := range metricEvents {
					if err := kb.kinesisClient.PublishMetricsProtobuf(ctx, event.ResourceMetrics,
						event.ServiceName, event.Metadata.SourceIP); err != nil {
						errorsMutex.Lock()
						errors = append(errors, fmt.Errorf("failed to publish metrics: %w", err))
						errorsMutex.Unlock()
					}
				}
			}()
		}

		// Process logs
		if len(logEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, event := range logEvents {
					if err := kb.kinesisClient.PublishLogsProtobuf(ctx, event.ResourceLogs,
						event.ServiceName, event.TraceID, event.Metadata.SourceIP); err != nil {
						errorsMutex.Lock()
						errors = append(errors, fmt.Errorf("failed to publish logs: %w", err))
						errorsMutex.Unlock()
					}
				}
			}()
		}

		// Process legacy events (HTTP JSON)
		if len(legacyEvents) > 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for _, event := range legacyEvents {
					switch event.Type {
					case "traces":
						if err := kb.kinesisClient.PublishTrace(ctx, event.Data, event.ServiceName,
							event.TraceID, event.Metadata.SourceIP, event.Metadata.UserAgent); err != nil {
							errorsMutex.Lock()
							errors = append(errors, fmt.Errorf("failed to publish legacy trace: %w", err))
							errorsMutex.Unlock()
						}
					case "metrics":
						if err := kb.kinesisClient.PublishMetrics(ctx, event.Data, event.ServiceName,
							event.Metadata.SourceIP, event.Metadata.UserAgent); err != nil {
							errorsMutex.Lock()
							errors = append(errors, fmt.Errorf("failed to publish legacy metrics: %w", err))
							errorsMutex.Unlock()
						}
					case "logs":
						if err := kb.kinesisClient.PublishLogs(ctx, event.Data, event.ServiceName,
							event.TraceID, event.Metadata.SourceIP, event.Metadata.UserAgent); err != nil {
							errorsMutex.Lock()
							errors = append(errors, fmt.Errorf("failed to publish legacy logs: %w", err))
							errorsMutex.Unlock()
						}
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