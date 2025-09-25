package app

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
	"golang.org/x/time/rate"
)

// Services holds all initialized services/clients
type Services struct {
	KinesisClient        *streaming.KinesisClient
	ClickHouseClient     *storage.ClickHouseClient
	S3Client             *s3.Client
	ResilienceComponents *handlers.ResilienceComponents
	CircuitBreaker       *resilience.CircuitBreaker
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
	
	// Initialize resilience components
	resilienceComponents, circuitBreaker, err := initializeResilienceComponents(cfg, services)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize resilience components: %w", err)
	}
	services.ResilienceComponents = resilienceComponents
	services.CircuitBreaker = circuitBreaker
	
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

// initializeResilienceComponents creates and configures all resilience components
func initializeResilienceComponents(cfg *config.Config, services *Services) (*handlers.ResilienceComponents, *resilience.CircuitBreaker, error) {
	// Initialize tenant rate limiter
	rateLimiter := resilience.NewTenantRateLimiter(
		rate.Every(time.Second/100), // 100 RPS default
		200, // burst capacity
	)

	// Initialize circuit breaker for ClickHouse real-time writes
	circuitBreaker := resilience.NewCircuitBreaker(resilience.Settings{
		Name:        "clickhouse-realtime",
		MaxRequests: 10,
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts resilience.Counts) bool {
			// Trip if more than 60% of requests fail
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRate > 0.6
		},
	})

	// Create AWS config from Kinesis config
	awsConfig := aws.Config{
		Region: cfg.Streaming.Kinesis.Region,
	}
	if cfg.Streaming.Kinesis.EndpointURL != "" {
		awsConfig.EndpointResolver = aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Streaming.Kinesis.EndpointURL,
				SigningRegion:     region,
				HostnameImmutable: true,
			}, nil
		})
	}

	// Initialize dead letter queue (DLQ)
	dlq := resilience.NewDeadLetterQueue(awsConfig, resilience.DLQConfig{
		QueueURL:       "https://sqs." + cfg.Streaming.Kinesis.Region + ".amazonaws.com/000000000000/telemetry-dlq",
		MaxRetries:     3,
		RetryBaseDelay: 5 * time.Second,
		RetryMaxDelay:  5 * time.Minute,
	})

	// Initialize Kinesis buffer
	kinesisBuffer := resilience.NewKinesisBuffer(
		services.KinesisClient,
		rateLimiter,
		circuitBreaker,
		dlq,
		resilience.BufferConfig{
			MaxBatchSize:    500,
			MaxBatchWait:    1 * time.Second,
			FlushInterval:   5 * time.Second,
			MaxTenantBuffer: 1000,
		},
	)

	return &handlers.ResilienceComponents{
		KinesisBuffer:   kinesisBuffer,
		RateLimiter:     rateLimiter,
		DeadLetterQueue: dlq,
	}, circuitBreaker, nil
}