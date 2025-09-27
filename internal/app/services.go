package app

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
)

// Services holds all initialized services/clients
type Services struct {
	KinesisClient        *streaming.KinesisClient
	ClickHouseClient     *storage.ClickHouseClient
	S3Client             *s3.Client
	ResilienceComponents *interfaces.ResilienceComponents
	CircuitBreaker       *resilience.CircuitBreaker
}

type ConsumerServices struct {
	KinesisClient    *streaming.KinesisClient
	ClickHouseClient *storage.ClickHouseClient
}

func InitializeConsumerServices(cfg *config.Config) (*ConsumerServices, error) {
	services := &ConsumerServices{}

	// Initialize ClickHouse client
	clickhouseClient, err := InitializeClickHouseClient(cfg)
	if err != nil {
		return nil, err
	}
	services.ClickHouseClient = clickhouseClient

	// Initialize Kinesis Client
	kinesisClient, err := InitializeKinesisClient(cfg)
	if err != nil {
		return nil, err
	}
	services.KinesisClient = kinesisClient

	return services, nil
}

// Close closes consumer service connections gracefully
func (s *ConsumerServices) Close() error {
	var errors []error

	if s.ClickHouseClient != nil {
		if err := s.ClickHouseClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf(ErrClickHouseClose, err))
		}
	}

	if s.KinesisClient != nil {
		if err := s.KinesisClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf(ErrKinesisClose, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(ErrConsumerServiceClose, errors)
	}

	return nil
}

// InitializeAPIServices creates and initializes all required services for the REST/GRPC servers
func InitializeAPIServices(cfg *config.Config) (*Services, error) {
	services := &Services{}

	// Initialize ClickHouse client
	clickhouseClient, err := InitializeClickHouseClient(cfg)
	if err != nil {
		return nil, err
	}
	services.ClickHouseClient = clickhouseClient

	// Initialize Kinesis client
	kinesisClient, err := InitializeKinesisClient(cfg)
	if err != nil {
		return nil, err
	}
	services.KinesisClient = kinesisClient

	// Initialize S3 client (optional - only if region is configured)
	if cfg.Streaming.S3.Region != "" {
		s3Client, err := InitializeS3Client(cfg)
		if err != nil {
			return nil, err
		}
		services.S3Client = s3Client
	}

	// Initialize resilience components
	resilienceComponents, circuitBreaker, err := InitializeResilienceComponents(cfg, services)
	if err != nil {
		return nil, fmt.Errorf(ErrResilienceInit, err)
	}
	services.ResilienceComponents = resilienceComponents
	services.CircuitBreaker = circuitBreaker

	return services, nil
}

// Close closes all service connections gracefully
func (s *Services) Close() error {
	var errors []error

	// Close resilience components first to stop background goroutines
	if s.ResilienceComponents != nil {
		if s.ResilienceComponents.KinesisBuffer != nil {
			if err := s.ResilienceComponents.KinesisBuffer.Close(context.Background()); err != nil {
				errors = append(errors, fmt.Errorf("failed to close KinesisBuffer: %w", err))
			}
		}
		if s.ResilienceComponents.RateLimiter != nil {
			if err := s.ResilienceComponents.RateLimiter.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close RateLimiter: %w", err))
			}
		}
	}

	if s.ClickHouseClient != nil {
		if err := s.ClickHouseClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf(ErrClickHouseClose, err))
		}
	}

	if s.KinesisClient != nil {
		if err := s.KinesisClient.Close(); err != nil {
			errors = append(errors, fmt.Errorf(ErrKinesisClose, err))
		}
	}

	// S3 client doesn't need explicit closing

	if len(errors) > 0 {
		return fmt.Errorf(ErrServiceClose, errors)
	}

	return nil
}
