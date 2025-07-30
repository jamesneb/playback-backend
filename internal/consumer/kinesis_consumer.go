package consumer

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
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

type KinesisConsumer struct {
	client      *kinesis.Client
	clickhouse  *storage.ClickHouseClient
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

func NewKinesisConsumer(cfg *ConsumerConfig, clickhouse *storage.ClickHouseClient) (*KinesisConsumer, error) {
	// Load AWS configuration
	var awsCfg aws.Config
	var err error

	if cfg.EndpointURL != "" {
		// LocalStack configuration
		awsCfg, err = awsconfig.LoadDefaultConfig(
			context.TODO(),
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
		awsCfg, err = awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion(cfg.Region))
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

	ticker := time.NewTicker(1 * time.Second) // Poll every second
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

	// Process records
	if len(resp.Records) > 0 {
		logger.Info("Processing records", 
			zap.String("stream", streamName), 
			zap.Int("count", len(resp.Records)))

		for _, record := range resp.Records {
			if err := kc.processRecord(ctx, streamType, record); err != nil {
				logger.Error("Failed to process record", 
					zap.String("stream", streamName),
					zap.String("sequenceNumber", *record.SequenceNumber),
					zap.Error(err))
			}
		}
	}

	return nil
}

func (kc *KinesisConsumer) processRecord(ctx context.Context, streamType string, record types.Record) error {
	// Parse the telemetry event
	var event streaming.TelemetryEvent
	if err := json.Unmarshal(record.Data, &event); err != nil {
		return fmt.Errorf("failed to unmarshal telemetry event: %w", err)
	}

	// Insert into ClickHouse based on event type
	switch streamType {
	case "traces":
		return kc.clickhouse.InsertTrace(ctx, &event)
	case "metrics":
		return kc.clickhouse.InsertMetric(ctx, &event)
	case "logs":
		return kc.clickhouse.InsertLog(ctx, &event)
	default:
		return fmt.Errorf("unknown stream type: %s", streamType)
	}
}