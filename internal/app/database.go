package app

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// InitializeClickHouseClient creates and configures ClickHouse client
func InitializeClickHouseClient(cfg *config.ConsolidatedConfig) (*storage.ClickHouseClient, error) {
	clickhouseClient, err := storage.NewClickHouseClient(&storage.ClickHouseConfig{
		Host:               cfg.Data.ClickHouse.Host,
		Database:           cfg.Data.ClickHouse.Database,
		Username:           cfg.Data.ClickHouse.Username,
		Password:           cfg.Data.ClickHouse.Password,
		MaxConnections:     cfg.Data.ClickHouse.MaxConnections,
		MaxIdleConnections: cfg.Data.ClickHouse.MaxIdleConnections,
	})
	if err != nil {
		return nil, fmt.Errorf(ErrClickHouseInit, err)
	}
	return clickhouseClient, nil
}

// InitializeKinesisClient creates and configures Kinesis client with environment-aware stream verification
func InitializeKinesisClient(cfg *config.ConsolidatedConfig) (*streaming.KinesisClient, error) {
	// Convert ConsolidatedConfig KinesisSettings to legacy KinesisConfig
	kinesisConfig := &config.KinesisConfig{
		Region:             cfg.Data.Kinesis.Region,
		AccessKeyID:        cfg.Data.Kinesis.AccessKeyID,
		SecretAccessKey:    cfg.Data.Kinesis.SecretAccessKey,
		EndpointURL:        cfg.Data.Kinesis.EndpointURL,
		TracesStreamName:   cfg.Data.Kinesis.TracesStream,
		MetricsStreamName:  cfg.Data.Kinesis.MetricsStream,
		LogsStreamName:     cfg.Data.Kinesis.LogsStream,
		BatchSize:          cfg.Data.Kinesis.BatchSize,
		RetryAttempts:      cfg.Data.Kinesis.MaxRetries,
		RetryDelay:         cfg.Data.Kinesis.RetryDelay,
	}

	kinesisClient, err := streaming.NewKinesisClient(kinesisConfig, cfg.App.Environment)
	if err != nil {
		return nil, fmt.Errorf(ErrKinesisClientInit, err)
	}
	return kinesisClient, nil
}

// InitializeS3Client creates and configures S3 client
func InitializeS3Client(cfg *config.ConsolidatedConfig) (*s3.Client, error) {
	s3Client, err := storage.NewS3Client(&storage.S3Config{
		Region:          cfg.Data.S3.Region,
		EndpointURL:     cfg.Data.S3.EndpointURL,
		AccessKeyID:     cfg.Data.S3.AccessKeyID,
		SecretAccessKey: cfg.Data.S3.SecretAccessKey,
		Bucket:          cfg.Data.S3.Bucket,
		ForcePathStyle:  cfg.Data.S3.ForcePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf(ErrS3ClientInit, err)
	}
	return s3Client, nil
}
