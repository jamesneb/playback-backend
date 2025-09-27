package rest

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Dependencies holds all the dependencies needed by the REST API
type Dependencies struct {
	Config               *config.Config
	KinesisClient        *streaming.KinesisClient
	ClickHouseClient     *storage.ClickHouseClient
	S3Client             *s3.Client
	Endpoints            *api.EndpointCollection
	ResilienceComponents *interfaces.ResilienceComponents
}

// APIHandlers holds all HTTP handlers for the REST API
type APIHandlers struct {
	Trace   *handlers.TraceHandler
	Metrics *handlers.MetricsHandler
	Logs    *handlers.LogsHandler
	Replay  *handlers.ReplayHandler
}

// handlersSingleton manages singleton handler instances
type handlersSingleton struct {
	mu       sync.RWMutex
	handlers map[string]*APIHandlers
}

var (
	handlersInstance *handlersSingleton
	handlersOnce     sync.Once
)

// getHandlersSingleton returns the singleton instance
func getHandlersSingleton() *handlersSingleton {
	handlersOnce.Do(func() {
		handlersInstance = &handlersSingleton{
			handlers: make(map[string]*APIHandlers),
		}
	})
	return handlersInstance
}

// getOrCreateHandlers returns cached handlers or creates new ones (thread-safe)
func (hs *handlersSingleton) getOrCreateHandlers(deps *Dependencies) (*APIHandlers, error) {
	// Create a unique key based on dependency configuration
	key, err := createDependencyKeyWithTimeout(deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependency key: %w", err)
	}

	// Try to get existing handlers with read lock
	hs.mu.RLock()
	if handlers, exists := hs.handlers[key]; exists {
		hs.mu.RUnlock()
		return handlers, nil
	}
	hs.mu.RUnlock()

	// Handlers don't exist, create them with write lock
	hs.mu.Lock()
	defer hs.mu.Unlock()

	// Double-check after acquiring write lock (still protected by write lock)
	if handlers, exists := hs.handlers[key]; exists {
		return handlers, nil
	}

	// Create new handlers
	newHandlers, err := createHandlers(deps)
	if err != nil {
		return nil, fmt.Errorf("failed to create handlers: %w", err)
	}

	hs.handlers[key] = newHandlers
	logger.Debug("Created new handler set",
		zap.String("key", key),
		zap.Int("total_handler_sets", len(hs.handlers)))
	return newHandlers, nil
}

// createDependencyKeyWithTimeout creates a unique key with timeout protection
func createDependencyKeyWithTimeout(deps *Dependencies) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), HASH_COMPUTATION_TIMEOUT)
	defer cancel()

	done := make(chan string, CHANNEL_BUFFER_SIZE)
	errCh := make(chan error, CHANNEL_BUFFER_SIZE)

	go func() {
		key, err := computeDependencyKey(deps)
		if err != nil {
			errCh <- err
			return
		}
		done <- key
	}()

	select {
	case key := <-done:
		return key, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", errors.New(ERROR_DEPENDENCY_KEY_TIMEOUT)
	}
}

// computeDependencyKey performs the actual key computation
func computeDependencyKey(deps *Dependencies) (string, error) {
	hasher := sha256.New()

	// Hash configuration that affects handler behavior
	if deps.KinesisClient != nil {
		hasher.Write([]byte(HASH_COMPONENT_KINESIS))
	}
	if deps.S3Client != nil {
		hasher.Write([]byte(HASH_COMPONENT_S3))
	}
	if deps.ResilienceComponents != nil {
		hasher.Write([]byte(HASH_COMPONENT_RESILIENCE))
	}
	if deps.ClickHouseClient != nil {
		hasher.Write([]byte(HASH_COMPONENT_CLICKHOUSE))
	}

	// Add timestamp-based component to prevent stale handlers
	if _, err := fmt.Fprintf(hasher, TIMESTAMP_HASH_FORMAT, time.Now().Unix()/SECONDS_PER_HOUR); err != nil {
		return "", fmt.Errorf("failed to write timestamp to hasher: %w", err)
	} // Hour precision

	return fmt.Sprintf("%x", hasher.Sum(nil))[:DEPENDENCY_KEY_LENGTH], nil
}

// createHandlers creates a new set of API handlers
func createHandlers(deps *Dependencies) (*APIHandlers, error) {
	traceHandler := handlers.NewTraceHandler(deps.KinesisClient, deps.ResilienceComponents)
	if traceHandler == nil {
		return nil, errors.New(ERROR_TRACE_HANDLER_CREATION)
	}

	metricsHandler := handlers.NewMetricsHandler(deps.KinesisClient)
	if metricsHandler == nil {
		return nil, errors.New(ERROR_METRICS_HANDLER_CREATION)
	}

	logsHandler := handlers.NewLogsHandler(deps.KinesisClient)
	if logsHandler == nil {
		return nil, errors.New(ERROR_LOGS_HANDLER_CREATION)
	}

	replayHandler := handlers.NewReplayHandler(deps.S3Client, REPLAY_S3_BUCKET_NAME)
	if replayHandler == nil {
		return nil, errors.New(ERROR_REPLAY_HANDLER_CREATION)
	}

	return &APIHandlers{
		Trace:   traceHandler,
		Metrics: metricsHandler,
		Logs:    logsHandler,
		Replay:  replayHandler,
	}, nil
}