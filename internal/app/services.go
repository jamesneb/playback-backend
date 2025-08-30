package app

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// Services holds all initialized services/clients
type Services struct {
	KinesisClient    *streaming.KinesisClient
	ClickHouseClient *storage.ClickHouseClient
	S3Client         *s3.Client
}

// InitializeServices creates and initializes all required services
func InitializeServices(cfg *config.Config) (*Services, error) {
	services := &Services{}
	
	// Initialize ClickHouse client
	clickhouseClient, err := storage.NewClickHouseClient(&storage.ClickHouseConfig{
		Host:               cfg.Database.ClickHouse.Host,
		Database:           cfg.Database.ClickHouse.Database,
		Username:           cfg.Database.ClickHouse.Username,
		Password:           cfg.Database.ClickHouse.Password,
		MaxConnections:     cfg.Database.ClickHouse.MaxConnections,
		MaxIdleConnections: cfg.Database.ClickHouse.MaxIdleConnections,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ClickHouse client: %w", err)
	}
	services.ClickHouseClient = clickhouseClient
	
	// Initialize Kinesis client
	kinesisClient, err := streaming.NewKinesisClient(&cfg.Streaming.Kinesis)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Kinesis client: %w", err)
	}
	services.KinesisClient = kinesisClient
	
	// Initialize S3 client
	s3Client, err := storage.NewS3Client(&storage.S3Config{
		Region:          cfg.Streaming.Kinesis.Region, // Reuse Kinesis config for now
		EndpointURL:     cfg.Streaming.Kinesis.EndpointURL,
		AccessKeyID:     cfg.Streaming.Kinesis.AccessKeyID,
		SecretAccessKey: cfg.Streaming.Kinesis.SecretAccessKey,
		Bucket:          "replays", // TODO: Move to config
		ForcePathStyle:  true,      // For LocalStack compatibility
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}
	services.S3Client = s3Client
	
	return services, nil
}

// Close closes all service connections gracefully
func (s *Services) Close() error {
	var errors []error
	
	if s.ClickHouseClient != nil {
		if err := s.ClickHouseClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf("ClickHouse close error: %w", err))
		}
	}
	
	if s.KinesisClient != nil {
		if err := s.KinesisClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf("Kinesis close error: %w", err))
		}
	}
	
	// S3 client doesn't need explicit closing
	
	if len(errors) > 0 {
		return fmt.Errorf("service close errors: %v", errors)
	}
	
	return nil
}