package app

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// InitializeClickHouseClient creates and configures ClickHouse client
func InitializeClickHouseClient(cfg *config.Config) (*storage.ClickHouseClient, error) {
	clickhouseClient, err := storage.NewClickHouseClient(&storage.ClickHouseConfig{
		Host:               cfg.Database.ClickHouse.Host,
		Database:           cfg.Database.ClickHouse.Database,
		Username:           cfg.Database.ClickHouse.Username,
		Password:           cfg.Database.ClickHouse.Password,
		MaxConnections:     cfg.Database.ClickHouse.MaxConnections,
		MaxIdleConnections: cfg.Database.ClickHouse.MaxIdleConnections,
	})
	if err != nil {
		return nil, fmt.Errorf(ErrClickHouseInit, err)
	}
	return clickhouseClient, nil
}

// InitializeKinesisClient creates and configures Kinesis client
func InitializeKinesisClient(cfg *config.Config) (*streaming.KinesisClient, error) {
	kinesisClient, err := streaming.NewKinesisClient(&cfg.Streaming.Kinesis)
	if err != nil {
		return nil, fmt.Errorf(ErrKinesisClientInit, err)
	}
	return kinesisClient, nil
}

// InitializeS3Client creates and configures S3 client
func InitializeS3Client(cfg *config.Config) (*s3.Client, error) {
	s3Client, err := storage.NewS3Client(&storage.S3Config{
		Region:          cfg.Streaming.S3.Region,
		EndpointURL:     cfg.Streaming.S3.EndpointURL,
		AccessKeyID:     cfg.Streaming.S3.AccessKeyID,
		SecretAccessKey: cfg.Streaming.S3.SecretAccessKey,
		Bucket:          cfg.Streaming.S3.Bucket,
		ForcePathStyle:  cfg.Streaming.S3.ForcePathStyle,
	})
	if err != nil {
		return nil, fmt.Errorf(ErrS3ClientInit, err)
	}
	return s3Client, nil
}