package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/jamesneb/playback-backend/pkg/config"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

type KinesisClient struct {
	client      *kinesis.Client
	streams     map[string]string // stream name mapping
	
	// Batching support (for legacy JSON HTTP API)
	batchChannels map[string]chan LegacyTelemetryEvent
	stopBatching  chan struct{}
	batchSize     int
	flushInterval time.Duration
}

// Note: TelemetryEvent and TelemetryMetadata are now defined in handler.go

func NewKinesisClient(cfg *config.KinesisConfig) (*KinesisClient, error) {
	// Load AWS config
	awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
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

	kc := &KinesisClient{
		client:        client,
		streams:       streams,
		batchChannels: make(map[string]chan LegacyTelemetryEvent),
		stopBatching:  make(chan struct{}),
		batchSize:     100,                // Default batch size
		flushInterval: 5 * time.Second,    // Default flush interval
	}

	// Verify streams exist
	if err := kc.verifyStreams(context.Background()); err != nil {
		log.Printf("Warning: Stream verification failed: %v", err)
		// Continue anyway for development/LocalStack scenarios
	}

	log.Printf("Kinesis client initialized with streams: %v", streams)
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

func (kc *KinesisClient) publishEvent(ctx context.Context, streamType string, event TelemetryEvent, partitionKey string) error {
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

	log.Printf("Published %s event to stream %s, shard: %s, sequence: %s", 
		streamType, streamName, *result.ShardId, *result.SequenceNumber)

	return nil
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

	log.Printf("Published %s event to stream %s, shard: %s, sequence: %s", 
		streamType, streamName, *result.ShardId, *result.SequenceNumber)

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
			log.Printf("Failed to marshal event %d: %v", i, err)
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
	log.Printf("Batch published %d records to stream %s, failed: %d", 
		len(records), streamName, *result.FailedRecordCount)

	return nil
}

// StartBatchProcessor enables high-throughput batch processing for traces
func (kc *KinesisClient) StartBatchProcessor(ctx context.Context) error {
	// Initialize batch channels for each stream type
	for streamType := range kc.streams {
		kc.batchChannels[streamType] = make(chan LegacyTelemetryEvent, kc.batchSize*2) // Buffer size
		go kc.processBatch(ctx, streamType)
	}
	
	log.Printf("Started Kinesis batch processors for %d streams (batch_size=%d, flush_interval=%v)", 
		len(kc.streams), kc.batchSize, kc.flushInterval)
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
		log.Printf("Batch channel full for %s, falling back to direct publish", streamType)
		return kc.publishLegacyEvent(context.Background(), streamType, event, event.TraceID)
	}
}

// processBatch handles batching logic for a specific stream type
func (kc *KinesisClient) processBatch(ctx context.Context, streamType string) {
	eventBuffer := make([]LegacyTelemetryEvent, 0, kc.batchSize)
	ticker := time.NewTicker(kc.flushInterval)
	defer ticker.Stop()
	
	batchChannel := kc.batchChannels[streamType]
	
	for {
		select {
		case <-ctx.Done():
			// Flush remaining events before shutting down
			if len(eventBuffer) > 0 {
				kc.flushBatch(ctx, streamType, eventBuffer)
			}
			return
			
		case <-kc.stopBatching:
			// Flush remaining events before shutting down
			if len(eventBuffer) > 0 {
				kc.flushBatch(ctx, streamType, eventBuffer)
			}
			return
			
		case event := <-batchChannel:
			eventBuffer = append(eventBuffer, event)
			
			// Flush when batch is full
			if len(eventBuffer) >= kc.batchSize {
				kc.flushBatch(ctx, streamType, eventBuffer)
				eventBuffer = eventBuffer[:0] // Reset buffer
			}
			
		case <-ticker.C:
			// Periodic flush even if batch isn't full
			if len(eventBuffer) > 0 {
				kc.flushBatch(ctx, streamType, eventBuffer)
				eventBuffer = eventBuffer[:0] // Reset buffer
			}
		}
	}
}

// flushBatch sends accumulated events to Kinesis
func (kc *KinesisClient) flushBatch(ctx context.Context, streamType string, events []LegacyTelemetryEvent) {
	if len(events) == 0 {
		return
	}
	
	if err := kc.PublishBatch(ctx, streamType, events); err != nil {
		log.Printf("Failed to flush batch for %s: %v", streamType, err)
		
		// Fallback: try individual publishes for failed batch
		for _, event := range events {
			partitionKey := event.TraceID
			if partitionKey == "" {
				partitionKey = event.ServiceName
			}
			if err := kc.publishLegacyEvent(ctx, streamType, event, partitionKey); err != nil {
				log.Printf("Failed to publish individual event after batch failure: %v", err)
			}
		}
	} else {
		log.Printf("Successfully flushed batch of %d events to %s", len(events), streamType)
	}
}

// Native protobuf publishing methods for gRPC path (sends raw OTLP protobuf to Kinesis)
func (kc *KinesisClient) PublishTraceProtobuf(ctx context.Context, resourceSpans *tracepb.ResourceSpans, serviceName, traceID, sourceIP string) error {
	log.Printf("🔥 CALLING PublishTraceProtobuf for service=%s, traceID=%s", serviceName, traceID)
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

	// Create a special partition key that indicates this is protobuf data
	// Format: "pb:<service>:<trace_id>:<timestamp>" so consumer can detect protobuf vs JSON
	protobufPartitionKey := fmt.Sprintf("pb:%s:%s:%d", serviceName, traceID, time.Now().UnixNano())

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

	log.Printf("Published raw %s protobuf to stream %s, shard: %s, sequence: %s", 
		streamType, streamName, *result.ShardId, *result.SequenceNumber)

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

func (kc *KinesisClient) Close() error {
	// Stop batch processors
	close(kc.stopBatching)
	
	// Close batch channels
	for _, ch := range kc.batchChannels {
		close(ch)
	}
	
	log.Println("Kinesis client closed with batch processors stopped")
	return nil
}