package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/jamesneb/playback-backend/internal/validation"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
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

	// DefaultFlushInterval provides default flush interval for batching
	DefaultFlushInterval = 5 * time.Second
)

// Stream type constants to eliminate magic strings
const (
	StreamTypeTraces  = "traces"
	StreamTypeMetrics = "metrics"
	StreamTypeLogs    = "logs"
)

// Production environment identifiers - pre-allocated slice for O(1) lookups
var productionEnvironments = map[string]struct{}{
	"production": {},
	"prod":       {},
	"live":       {},
	"staging":    {}, // Staging treated as production for reliability
	"stage":      {},
}

// isProductionEnvironment determines if environment requires production-level reliability.
// Uses zero-allocation map lookup for O(1) performance.
func isProductionEnvironment(env string) bool {
	if env == "" {
		return false
	}

	// Single allocation string conversion with bounds check elimination
	envLower := strings.ToLower(env)
	_, isProduction := productionEnvironments[envLower]
	return isProduction
}


// KinesisClient provides thread-safe, resource-managed Kinesis streaming functionality
// with proper lifecycle management and memory leak prevention.
type KinesisClient struct {
	client        *kinesis.Client
	streamManager *StreamManager
	publisher     *Publisher
	batchProcessor *BatchProcessor

	// Resource management
	mu           sync.RWMutex
	shutdownOnce sync.Once
}

// Note: TelemetryEvent and TelemetryMetadata are now defined in handler.go

// loadAWSConfig loads AWS configuration with proper credential handling
func loadAWSConfig(cfg *config.KinesisConfig) (aws.Config, error) {
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
			return awsCfg, fmt.Errorf("failed to load AWS config with credentials: %w", err)
		}
	} else {
		// Fall back to default credential chain (environment, instance role, etc.)
		awsCfg, err = awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithRegion(cfg.Region),
		)
		if err != nil {
			return awsCfg, fmt.Errorf("failed to load AWS config: %w", err)
		}
	}

	// Override endpoint if specified (for LocalStack)
	if cfg.EndpointURL != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.EndpointURL)
	}

	return awsCfg, nil
}

// configureBatching sets up batch configuration with safe defaults and validation
func configureBatching(cfg *config.KinesisConfig) (int, time.Duration) {
	batchSize := DefaultChannelBufferSize
	flushInterval := DefaultFlushInterval

	if cfg.BatchSize > 0 && cfg.BatchSize <= MaxChannelBufferSize {
		batchSize = cfg.BatchSize
	} else if cfg.BatchSize > MaxChannelBufferSize {
		logger.Warn("Batch size exceeds maximum, using maximum",
			zap.Int("requested_batch_size", cfg.BatchSize),
			zap.Int("max_batch_size", MaxChannelBufferSize))
		batchSize = MaxChannelBufferSize
	}

	if cfg.FlushInterval != "" {
		if parsedInterval, err := time.ParseDuration(cfg.FlushInterval); err == nil {
			flushInterval = parsedInterval
		} else {
			logger.Warn("Invalid flush interval, using default",
				zap.String("invalid_interval", cfg.FlushInterval),
				zap.Duration("default_interval", DefaultFlushInterval))
		}
	}

	return batchSize, flushInterval
}

func NewKinesisClient(cfg *config.KinesisConfig, environment string) (*KinesisClient, error) {
	awsCfg, err := loadAWSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := kinesis.NewFromConfig(awsCfg)

	// Map stream types to actual stream names
	streams := map[string]string{
		StreamTypeTraces:  cfg.Streams[StreamTypeTraces],
		StreamTypeMetrics: cfg.Streams[StreamTypeMetrics],
		StreamTypeLogs:    cfg.Streams[StreamTypeLogs],
	}

	// Create components
	streamManager := NewStreamManager(client, streams)
	serializer := NewProtobufSerializer()
	validator := validation.NewProtobufValidator()
	publisher := NewPublisher(client, streamManager, serializer, validator)

	// Configure batching
	batchSize, flushInterval := configureBatching(cfg)
	batchProcessorCfg := &BatchProcessorConfig{
		BatchSize:     batchSize,
		FlushInterval: flushInterval,
	}
	batchProcessor, err := NewBatchProcessor(client, streamManager, batchProcessorCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create batch processor: %w", err)
	}

	kc := &KinesisClient{
		client:         client,
		streamManager:  streamManager,
		publisher:      publisher,
		batchProcessor: batchProcessor,
	}

	// Verify streams exist - fatal in production environments
	if err := streamManager.VerifyStreams(context.Background()); err != nil {
		// In production environments, stream verification failures are fatal
		if isProductionEnvironment(environment) {
			logger.Error("Stream verification failed in production environment",
				zap.Error(err),
				zap.String("environment", environment))
			return nil, fmt.Errorf("critical: Kinesis stream verification failed in production: %w", err)
		}

		// In non-production environments, log warning and continue
		logger.Warn("Stream verification failed - continuing for development/test environment",
			zap.Error(err),
			zap.String("environment", environment))
	} else {
		logger.Info("All Kinesis streams verified successfully",
			zap.String("environment", environment),
			zap.Any("streams", streams))
	}

	logger.Info("Kinesis client initialized", zap.Any("streams", streams))
	return kc, nil
}


func (kc *KinesisClient) PublishTrace(ctx context.Context, traceData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	return kc.publisher.PublishTrace(ctx, traceData, serviceName, traceID, sourceIP, userAgent)
}

func (kc *KinesisClient) PublishMetrics(ctx context.Context, metricsData json.RawMessage, serviceName, sourceIP, userAgent string) error {
	return kc.publisher.PublishMetrics(ctx, metricsData, serviceName, sourceIP, userAgent)
}

func (kc *KinesisClient) PublishLogs(ctx context.Context, logsData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	return kc.publisher.PublishLogs(ctx, logsData, serviceName, traceID, sourceIP, userAgent)
}


// PublishBatch publishes multiple events in a single batch operation
func (kc *KinesisClient) PublishBatch(ctx context.Context, streamType string, events []LegacyTelemetryEvent) error {
	return kc.publisher.PublishBatch(ctx, streamType, events)
}

// StartBatchProcessor enables high-throughput batch processing with proper resource management
func (kc *KinesisClient) StartBatchProcessor(ctx context.Context) error {
	return kc.batchProcessor.Start(ctx)
}

// PublishAsync sends events to batch processor for high-throughput scenarios
func (kc *KinesisClient) PublishAsync(streamType string, event LegacyTelemetryEvent) error {
	return kc.batchProcessor.PublishAsync(streamType, event)
}


// PublishTraceProtobuf publishes a protobuf trace event
func (kc *KinesisClient) PublishTraceProtobuf(ctx context.Context, resourceSpans *tracepb.ResourceSpans, serviceName, traceID, sourceIP string) error {
	return kc.publisher.PublishTraceProtobuf(ctx, resourceSpans, serviceName, traceID, sourceIP)
}

// PublishMetricsProtobuf publishes a protobuf metrics event
func (kc *KinesisClient) PublishMetricsProtobuf(ctx context.Context, resourceMetrics *metricspb.ResourceMetrics, serviceName, sourceIP string) error {
	return kc.publisher.PublishMetricsProtobuf(ctx, resourceMetrics, serviceName, sourceIP)
}

// PublishLogsProtobuf publishes a protobuf logs event
func (kc *KinesisClient) PublishLogsProtobuf(ctx context.Context, resourceLogs *logspb.ResourceLogs, serviceName, traceID, sourceIP string) error {
	return kc.publisher.PublishLogsProtobuf(ctx, resourceLogs, serviceName, traceID, sourceIP)
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

		// Stop batch processor
		if kc.batchProcessor != nil {
			if err := kc.batchProcessor.Stop(); err != nil {
				logger.Error("Failed to stop batch processor", zap.Error(err))
				shutdownError = fmt.Errorf("batch processor shutdown failed: %w", err)
			}
		}

		logger.Info("Kinesis client shutdown completed")
	})

	return shutdownError
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
