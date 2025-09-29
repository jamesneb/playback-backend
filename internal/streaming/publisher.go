package streaming

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/jamesneb/playback-backend/internal/validation"
	"github.com/jamesneb/playback-backend/pkg/logger"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
)

// Publisher handles individual event publishing to Kinesis streams
type Publisher struct {
	client        *kinesis.Client
	streamManager *StreamManager
	serializer    *ProtobufSerializer
	validator     *validation.ProtobufValidator
}

// NewPublisher creates a new publisher
func NewPublisher(client *kinesis.Client, streamManager *StreamManager, serializer *ProtobufSerializer, validator *validation.ProtobufValidator) *Publisher {
	return &Publisher{
		client:        client,
		streamManager: streamManager,
		serializer:    serializer,
		validator:     validator,
	}
}

// PublishTrace publishes a trace event
func (p *Publisher) PublishTrace(ctx context.Context, traceData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	event := LegacyTelemetryEvent{
		Type:        StreamTypeTraces,
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

	partitionKey := traceID
	if partitionKey == "" {
		partitionKey = serviceName
	}

	return p.publishLegacyEvent(ctx, StreamTypeTraces, event, partitionKey)
}

// PublishMetrics publishes a metrics event
func (p *Publisher) PublishMetrics(ctx context.Context, metricsData json.RawMessage, serviceName, sourceIP, userAgent string) error {
	event := LegacyTelemetryEvent{
		Type:        StreamTypeMetrics,
		ServiceName: serviceName,
		Data:        metricsData,
		Metadata: LegacyTelemetryMetadata{
			IngestedAt: time.Now(),
			SourceIP:   sourceIP,
			UserAgent:  userAgent,
			Version:    "1.0",
		},
	}

	partitionKey := serviceName
	return p.publishLegacyEvent(ctx, StreamTypeMetrics, event, partitionKey)
}

// PublishLogs publishes a logs event
func (p *Publisher) PublishLogs(ctx context.Context, logsData json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	event := LegacyTelemetryEvent{
		Type:        StreamTypeLogs,
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

	partitionKey := traceID
	if partitionKey == "" {
		partitionKey = serviceName
	}

	return p.publishLegacyEvent(ctx, StreamTypeLogs, event, partitionKey)
}

// PublishBatch publishes multiple events in a single batch operation
func (p *Publisher) PublishBatch(ctx context.Context, streamType string, events []LegacyTelemetryEvent) error {
	streamName, err := p.streamManager.GetStreamName(streamType)
	if err != nil {
		return err
	}

	// Convert events to Kinesis records
	var records []types.PutRecordsRequestEntry
	for i, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("failed to marshal event %d: %w", i, err)
		}

		partitionKey := event.TraceID
		if partitionKey == "" {
			partitionKey = event.ServiceName
		}

		records = append(records, types.PutRecordsRequestEntry{
			Data:         data,
			PartitionKey: aws.String(partitionKey),
		})
	}

	// Send batch to Kinesis
	_, err = p.client.PutRecords(ctx, &kinesis.PutRecordsInput{
		StreamName: aws.String(streamName),
		Records:    records,
	})
	if err != nil {
		return fmt.Errorf("failed to put records to stream %s: %w", streamName, err)
	}

	logger.Debug("Successfully published batch", zap.Int("event_count", len(events)), zap.String("stream_type", streamType))
	return nil
}

// PublishTraceProtobuf publishes a protobuf trace event
func (p *Publisher) PublishTraceProtobuf(ctx context.Context, resourceSpans *tracepb.ResourceSpans, serviceName, traceID, sourceIP string) error {
	logger.Debug("Publishing trace protobuf", zap.String("service_name", serviceName), zap.String("trace_id", traceID))

	// Validate protobuf size before marshaling
	if err := p.validator.ValidateProtobufSize(resourceSpans, validation.MaxProtobufTraceSize, "trace"); err != nil {
		return fmt.Errorf("trace protobuf validation failed: %w", err)
	}

	// Serialize OTLP protobuf directly
	data, err := p.serializer.SerializeTraceData(resourceSpans)
	if err != nil {
		return fmt.Errorf("failed to serialize trace data: %w", err)
	}

	return p.publishRawProtobuf(ctx, StreamTypeTraces, data, "", serviceName, traceID, sourceIP)
}

// PublishMetricsProtobuf publishes a protobuf metrics event
func (p *Publisher) PublishMetricsProtobuf(ctx context.Context, resourceMetrics *metricspb.ResourceMetrics, serviceName, sourceIP string) error {
	// Validate protobuf size
	if err := p.validator.ValidateProtobufSize(resourceMetrics, validation.MaxProtobufMetricsSize, "metrics"); err != nil {
		return fmt.Errorf("metrics protobuf validation failed: %w", err)
	}

	data, err := p.serializer.SerializeMetricsData(resourceMetrics)
	if err != nil {
		return fmt.Errorf("failed to serialize metrics data: %w", err)
	}

	return p.publishRawProtobuf(ctx, StreamTypeMetrics, data, "", serviceName, "", sourceIP)
}

// PublishLogsProtobuf publishes a protobuf logs event
func (p *Publisher) PublishLogsProtobuf(ctx context.Context, resourceLogs *logspb.ResourceLogs, serviceName, traceID, sourceIP string) error {
	// Validate protobuf size
	if err := p.validator.ValidateProtobufSize(resourceLogs, validation.MaxProtobufLogsSize, "logs"); err != nil {
		return fmt.Errorf("logs protobuf validation failed: %w", err)
	}

	data, err := p.serializer.SerializeLogsData(resourceLogs)
	if err != nil {
		return fmt.Errorf("failed to serialize logs data: %w", err)
	}

	return p.publishRawProtobuf(ctx, StreamTypeLogs, data, "", serviceName, traceID, sourceIP)
}

// publishLegacyEvent handles legacy JSON events from HTTP REST API
func (p *Publisher) publishLegacyEvent(ctx context.Context, streamType string, event LegacyTelemetryEvent, partitionKey string) error {
	streamName, err := p.streamManager.GetStreamName(streamType)
	if err != nil {
		return err
	}

	// Serialize event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Send to Kinesis
	_, err = p.client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         data,
		PartitionKey: aws.String(partitionKey),
	})
	if err != nil {
		return fmt.Errorf("failed to put record to stream %s: %w", streamName, err)
	}

	logger.Debug("Successfully published legacy event", zap.String("stream_type", streamType), zap.String("partition_key", partitionKey))
	return nil
}

// publishRawProtobuf sends raw OTLP protobuf bytes directly to Kinesis
func (p *Publisher) publishRawProtobuf(ctx context.Context, streamType string, protobufData []byte, partitionKey, serviceName, traceID, sourceIP string) error {
	streamName, err := p.streamManager.GetStreamName(streamType)
	if err != nil {
		return err
	}

	// Create structured partition key that consumer expects: "pb:<service>:<trace_id>:<timestamp>"
	timestamp := time.Now().UnixNano()
	protobufPartitionKey := fmt.Sprintf("pb:%s:%s:%d", serviceName, traceID, timestamp)

	// Send raw protobuf to Kinesis
	_, err = p.client.PutRecord(ctx, &kinesis.PutRecordInput{
		StreamName:   aws.String(streamName),
		Data:         protobufData,
		PartitionKey: aws.String(protobufPartitionKey),
	})
	if err != nil {
		return fmt.Errorf("failed to put protobuf record to stream %s: %w", streamName, err)
	}

	logger.Debug("Successfully published protobuf event",
		zap.String("stream_type", streamType),
		zap.String("partition_key", protobufPartitionKey),
		zap.Int("data_size", len(protobufData)))
	return nil
}