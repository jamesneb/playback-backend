package consumer

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
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type KinesisConsumer struct {
	client      *kinesis.Client
	clickhouse  *storage.ClickHouseClient
	cfg         *ConsumerConfig // Add config field
	streams     map[string]string
	shardStates map[string]string // streamName -> shardIterator
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
}

type ConsumerConfig struct {
	Region          string
	EndpointURL     string
	AccessKeyID     string
	SecretAccessKey string
	Streams         map[string]string
	PollInterval    time.Duration
}

func NewKinesisConsumer(ctx context.Context, cfg *ConsumerConfig, clickhouse *storage.ClickHouseClient) (*KinesisConsumer, error) {
	// Load AWS configuration
	var awsCfg aws.Config
	var err error

	if cfg.EndpointURL != "" {
		// LocalStack configuration
		awsCfg, err = awsconfig.LoadDefaultConfig(
			ctx,
			awsconfig.WithRegion(cfg.Region),
			awsconfig.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
				return aws.Credentials{
					AccessKeyID:     cfg.AccessKeyID,
					SecretAccessKey: cfg.SecretAccessKey,
				}, nil
			})),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
	} else {
		// AWS configuration
		awsCfg, err = awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.Region))
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}
	}

	// Create Kinesis client with custom endpoint for LocalStack
	client := kinesis.NewFromConfig(awsCfg, func(o *kinesis.Options) {
		if cfg.EndpointURL != "" {
			o.BaseEndpoint = aws.String(cfg.EndpointURL)
		}
	})

	return &KinesisConsumer{
		client:      client,
		clickhouse:  clickhouse,
		cfg:         cfg, // Store config
		streams:     cfg.Streams,
		shardStates: make(map[string]string),
		stopChan:    make(chan struct{}),
	}, nil
}

func (kc *KinesisConsumer) Start(ctx context.Context) error {
	logger.Info("Starting Kinesis consumer", zap.Int("streams", len(kc.streams)))

	// Initialize shard iterators for all streams
	for streamType, streamName := range kc.streams {
		if err := kc.initializeShardIterator(ctx, streamType, streamName); err != nil {
			logger.Error("Failed to initialize shard iterator",
				zap.String("stream", streamName),
				zap.Error(err))
			return err
		}
	}

	// Start consumer goroutines for each stream
	for streamType, streamName := range kc.streams {
		kc.wg.Add(1)
		go kc.consumeStream(ctx, streamType, streamName)
	}

	logger.Info("Kinesis consumer started successfully")
	return nil
}

func (kc *KinesisConsumer) Stop() {
	logger.Info("Stopping Kinesis consumer")
	close(kc.stopChan)
	kc.wg.Wait()
	logger.Info("Kinesis consumer stopped")
}

func (kc *KinesisConsumer) initializeShardIterator(ctx context.Context, streamType, streamName string) error {
	// Get stream description to find shards
	describeResp, err := kc.client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
		StreamName: aws.String(streamName),
	})
	if err != nil {
		return fmt.Errorf("failed to describe stream %s: %w", streamName, err)
	}

	// For simplicity, we'll just use the first shard
	// In production, you'd handle multiple shards
	if len(describeResp.StreamDescription.Shards) == 0 {
		return fmt.Errorf("no shards found for stream %s", streamName)
	}

	shard := describeResp.StreamDescription.Shards[0]

	// Get shard iterator starting from TRIM_HORIZON (beginning of stream)
	iteratorResp, err := kc.client.GetShardIterator(ctx, &kinesis.GetShardIteratorInput{
		StreamName:        aws.String(streamName),
		ShardId:           shard.ShardId,
		ShardIteratorType: types.ShardIteratorTypeTrimHorizon,
	})
	if err != nil {
		return fmt.Errorf("failed to get shard iterator for stream %s: %w", streamName, err)
	}

	kc.mu.Lock()
	kc.shardStates[streamName] = *iteratorResp.ShardIterator
	kc.mu.Unlock()

	logger.Info("Initialized shard iterator",
		zap.String("stream", streamName),
		zap.String("shard", *shard.ShardId))

	return nil
}

func (kc *KinesisConsumer) consumeStream(ctx context.Context, streamType, streamName string) {
	defer kc.wg.Done()

	logger.Info("Starting consumer for stream",
		zap.String("type", streamType),
		zap.String("stream", streamName))

	// Use configured poll interval instead of hardcoded 1 second
	pollInterval := kc.cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 1 * time.Second // Fallback to 1 second if not configured
	}

	logger.Debug("Consumer poll interval configured",
		zap.String("stream", streamName),
		zap.Duration("interval", pollInterval))

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-kc.stopChan:
			return
		case <-ticker.C:
			if err := kc.pollStream(ctx, streamType, streamName); err != nil {
				logger.Error("Error polling stream",
					zap.String("stream", streamName),
					zap.Error(err))
			}
		}
	}
}

func (kc *KinesisConsumer) pollStream(ctx context.Context, streamType, streamName string) error {
	kc.mu.RLock()
	shardIterator := kc.shardStates[streamName]
	kc.mu.RUnlock()

	if shardIterator == "" {
		return fmt.Errorf("no shard iterator for stream %s", streamName)
	}

	// Get records from Kinesis
	resp, err := kc.client.GetRecords(ctx, &kinesis.GetRecordsInput{
		ShardIterator: aws.String(shardIterator),
	})
	if err != nil {
		return fmt.Errorf("failed to get records from stream %s: %w", streamName, err)
	}

	// Update shard iterator for next poll
	if resp.NextShardIterator != nil {
		kc.mu.Lock()
		kc.shardStates[streamName] = *resp.NextShardIterator
		kc.mu.Unlock()
	}

	// Process records in batches for better performance
	if len(resp.Records) > 0 {
		logger.Info("Processing records",
			zap.String("stream", streamName),
			zap.Int("count", len(resp.Records)))

		// Collect events for batch processing
		events := make([]streaming.TelemetryEvent, 0, len(resp.Records))

		for _, record := range resp.Records {
			// Determine format based on partition key prefix
			partitionKey := ""
			if record.PartitionKey != nil {
				partitionKey = *record.PartitionKey
			}
			isProtobuf := record.PartitionKey != nil && strings.HasPrefix(*record.PartitionKey, "pb:")

			// Debug: Check if we have any protobuf records
			if isProtobuf {
				logger.Info("Found protobuf record!",
					zap.String("partition_key", partitionKey),
					zap.String("stream", streamName))
			}

			var event streaming.TelemetryEvent
			var err error

			if isProtobuf {
				// Parse raw OTLP protobuf data (gRPC path)
				event, err = kc.parseRawProtobuf(record.Data, *record.PartitionKey, streamType)
			} else {
				// Parse legacy JSON data (HTTP path)
				event, err = kc.parseJSONRecord(record.Data)
			}

			if err != nil {
				logger.Error("Failed to parse Kinesis record",
					zap.String("stream", streamName),
					zap.String("sequenceNumber", *record.SequenceNumber),
					zap.String("format", map[bool]string{true: "protobuf", false: "json"}[isProtobuf]),
					zap.Error(err))
				continue
			}

			events = append(events, event)
		}

		// Batch insert into ClickHouse
		if len(events) > 0 {
			if err := kc.processBatch(ctx, streamType, events); err != nil {
				logger.Error("Failed to process batch",
					zap.String("stream", streamName),
					zap.Int("batch_size", len(events)),
					zap.Error(err))

				// Fallback: process individually
				for _, event := range events {
					if err := kc.processRecordDirect(ctx, streamType, event); err != nil {
						logger.Error("Failed to process individual record after batch failure", zap.Error(err))
					}
				}
			}
		}
	}

	return nil
}

func (kc *KinesisConsumer) processBatch(ctx context.Context, streamType string, events []streaming.TelemetryEvent) error {
	logger.Debug("Processing batch",
		zap.String("stream_type", streamType),
		zap.Int("batch_size", len(events)))

	// Separate events by type (protobuf vs JSON) for optimal batch processing
	var protoTraces []*streaming.TraceTelemetryEvent
	var protoMetrics []*streaming.MetricsTelemetryEvent
	var protoLogs []*streaming.LogsTelemetryEvent
	var legacyEvents []streaming.TelemetryEvent

	// Classify events
	for _, event := range events {
		switch streamType {
		case "traces":
			if traceEvent, ok := event.(*streaming.TraceTelemetryEvent); ok {
				protoTraces = append(protoTraces, traceEvent)
			} else {
				legacyEvents = append(legacyEvents, event)
			}
		case "metrics":
			if metricEvent, ok := event.(*streaming.MetricsTelemetryEvent); ok {
				protoMetrics = append(protoMetrics, metricEvent)
			} else {
				legacyEvents = append(legacyEvents, event)
			}
		case "logs":
			if logEvent, ok := event.(*streaming.LogsTelemetryEvent); ok {
				protoLogs = append(protoLogs, logEvent)
			} else {
				legacyEvents = append(legacyEvents, event)
			}
		}
	}

	// Process protobuf events in true batches
	switch streamType {
	case "traces":
		if len(protoTraces) > 0 {
			if err := kc.clickhouse.InsertTraceProtobufBatch(ctx, protoTraces); err != nil {
				return fmt.Errorf("failed to batch insert protobuf traces: %w", err)
			}
		}
	case "metrics":
		if len(protoMetrics) > 0 {
			if err := kc.clickhouse.InsertMetricProtobufBatch(ctx, protoMetrics); err != nil {
				return fmt.Errorf("failed to batch insert protobuf metrics: %w", err)
			}
		}
	case "logs":
		if len(protoLogs) > 0 {
			if err := kc.clickhouse.InsertLogProtobufBatch(ctx, protoLogs); err != nil {
				return fmt.Errorf("failed to batch insert protobuf logs: %w", err)
			}
		}
	}

	// Process legacy JSON events individually (fallback for backward compatibility)
	for _, event := range legacyEvents {
		switch streamType {
		case "traces":
			if err := kc.clickhouse.InsertTrace(ctx, event); err != nil {
				return fmt.Errorf("failed to insert JSON trace: %w", err)
			}
		case "metrics":
			if err := kc.clickhouse.InsertMetric(ctx, event); err != nil {
				return fmt.Errorf("failed to insert JSON metric: %w", err)
			}
		case "logs":
			if err := kc.clickhouse.InsertLog(ctx, event); err != nil {
				return fmt.Errorf("failed to insert JSON log: %w", err)
			}
		}
	}

	logger.Debug("Successfully processed batch",
		zap.String("stream_type", streamType),
		zap.Int("events_processed", len(events)),
		zap.Int("protobuf_events", len(protoTraces)+len(protoMetrics)+len(protoLogs)),
		zap.Int("legacy_events", len(legacyEvents)))

	return nil
}

// processRecord method removed - replaced by dual-path parsing in batch processing

func (kc *KinesisConsumer) processRecordDirect(ctx context.Context, streamType string, event streaming.TelemetryEvent) error {
	logger.Debug("Processing Kinesis record",
		zap.String("stream_type", streamType),
		zap.String("trace_id", event.GetTraceID()),
		zap.String("service_name", event.GetServiceName()))

	// Insert raw data into ClickHouse - materialized views will handle processing
	switch streamType {
	case "traces":
		return kc.clickhouse.InsertTrace(ctx, event)
	case "metrics":
		return kc.clickhouse.InsertMetric(ctx, event)
	case "logs":
		return kc.clickhouse.InsertLog(ctx, event)
	default:
		return fmt.Errorf("unknown stream type: %s", streamType)
	}
}

// parseRawProtobuf parses raw OTLP protobuf data sent directly from gRPC path
func (kc *KinesisConsumer) parseRawProtobuf(data []byte, partitionKey, streamType string) (streaming.TelemetryEvent, error) {
	// Extract metadata from partition key: "pb:<service>:<trace_id>:<timestamp>"
	parts := strings.Split(partitionKey, ":")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid protobuf partition key format")
	}

	serviceName := parts[1]
	traceID := parts[2]
	// timestamp is parts[3] if present

	// Parse raw OTLP protobuf based on stream type
	switch streamType {
	case "traces":
		var resourceSpans tracepb.ResourceSpans
		if err := proto.Unmarshal(data, &resourceSpans); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw OTLP trace data: %w", err)
		}

		return &streaming.TraceTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeTraces,
				ServiceName: serviceName,
				TraceID:     traceID,
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(), // Consumer ingestion time
				},
			},
			ResourceSpans: &resourceSpans, // Pure protobuf from gRPC!
		}, nil

	case "metrics":
		var resourceMetrics metricspb.ResourceMetrics
		if err := proto.Unmarshal(data, &resourceMetrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw OTLP metrics data: %w", err)
		}

		return &streaming.MetricsTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeMetrics,
				ServiceName: serviceName,
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(), // Consumer ingestion time
				},
			},
			ResourceMetrics: &resourceMetrics, // Pure protobuf from gRPC!
		}, nil

	case "logs":
		var resourceLogs logspb.ResourceLogs
		if err := proto.Unmarshal(data, &resourceLogs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal raw OTLP logs data: %w", err)
		}

		return &streaming.LogsTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeLogs,
				ServiceName: serviceName,
				TraceID:     traceID,
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(), // Consumer ingestion time
				},
			},
			ResourceLogs: &resourceLogs, // Pure protobuf from gRPC!
		}, nil

	default:
		return nil, fmt.Errorf("unknown stream type for protobuf: %s", streamType)
	}
}

// parseJSONRecord parses legacy JSON data from HTTP API path
func (kc *KinesisConsumer) parseJSONRecord(data []byte) (streaming.TelemetryEvent, error) {
	// Unmarshal legacy JSON format
	var legacyEvent streaming.LegacyTelemetryEvent
	if err := json.Unmarshal(data, &legacyEvent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal legacy JSON: %w", err)
	}

	// Convert legacy JSON event to type-safe event (for backward compatibility)
	switch legacyEvent.Type {
	case "traces":
		return &streaming.TraceTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeTraces,
				ServiceName: legacyEvent.ServiceName,
				TraceID:     legacyEvent.TraceID,
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: legacyEvent.Metadata.IngestedAt,
					SourceIP:   legacyEvent.Metadata.SourceIP,
				},
			},
			// ResourceSpans is nil - this indicates legacy JSON data
			// The GetSerializedData method will need to handle this case
		}, nil

	case "metrics":
		return &streaming.MetricsTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeMetrics,
				ServiceName: legacyEvent.ServiceName,
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: legacyEvent.Metadata.IngestedAt,
					SourceIP:   legacyEvent.Metadata.SourceIP,
				},
			},
			// ResourceMetrics is nil - indicates legacy JSON data
		}, nil

	case "logs":
		return &streaming.LogsTelemetryEvent{
			BaseTelemetryEvent: streaming.BaseTelemetryEvent{
				Type:        streaming.TelemetryTypeLogs,
				ServiceName: legacyEvent.ServiceName,
				TraceID:     legacyEvent.TraceID,
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: legacyEvent.Metadata.IngestedAt,
					SourceIP:   legacyEvent.Metadata.SourceIP,
				},
			},
			// ResourceLogs is nil - indicates legacy JSON data
		}, nil

	default:
		return nil, fmt.Errorf("unknown legacy event type: %s", legacyEvent.Type)
	}
}
