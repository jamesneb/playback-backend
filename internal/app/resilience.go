package app

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/pkg/config"
	"golang.org/x/time/rate"
)

// isDLQConfigured checks if DLQ configuration is complete and valid
func isDLQConfigured(cfg *config.Config) bool {
	dlqCfg := cfg.Resilience.DeadLetterQueue

	// Check if we have a complete URL
	if dlqCfg.QueueURL != "" {
		return true
	}

	// Check if we have all components to build a URL
	return cfg.Streaming.Kinesis.Region != "" &&
		   dlqCfg.AccountID != "" &&
		   dlqCfg.QueueName != ""
}

// InitializeResilienceComponents creates and configures all resilience components
func InitializeResilienceComponents(cfg *config.Config, services *Services) (*interfaces.ResilienceComponents, *resilience.CircuitBreaker, error) {
	// Initialize tenant rate limiter from config
	if cfg.Resilience.RateLimiter.RequestsPerSecond <= 0 {
		return nil, nil, fmt.Errorf("rate limiter requests_per_second must be greater than 0, got: %d", cfg.Resilience.RateLimiter.RequestsPerSecond)
	}
	rpsLimit := time.Second / time.Duration(cfg.Resilience.RateLimiter.RequestsPerSecond)
	rateLimiter := resilience.NewTenantRateLimiter(
		rate.Every(rpsLimit),
		cfg.Resilience.RateLimiter.BurstCapacity,
	)

	// Initialize circuit breaker from config
	circuitBreaker := resilience.NewCircuitBreaker(resilience.Settings{
		Name:        cfg.Resilience.CircuitBreaker.Name,
		MaxRequests: cfg.Resilience.CircuitBreaker.MaxRequests,
		Interval:    time.Duration(cfg.Resilience.CircuitBreaker.IntervalSeconds) * time.Second,
		Timeout:     time.Duration(cfg.Resilience.CircuitBreaker.TimeoutSeconds) * time.Second,
		ReadyToTrip: func(counts resilience.Counts) bool {
			// Trip if failure rate exceeds configured threshold
			// Guard against division by zero during bootstrap
			if counts.Requests == 0 {
				return false
			}
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= cfg.Resilience.CircuitBreaker.MinRequests && failureRate > cfg.Resilience.CircuitBreaker.FailureRate
		},
	})

	// Initialize dead letter queue only if SQS is properly configured
	var dlq *resilience.DeadLetterQueue
	if isDLQConfigured(cfg) {
		// Create AWS config with proper credential loading for DLQ
		awsConfig, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.Streaming.Kinesis.Region),
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load AWS config for DLQ: %w", err)
		}

		// Apply custom endpoint if specified
		if cfg.Streaming.Kinesis.EndpointURL != "" {
			awsConfig.BaseEndpoint = aws.String(cfg.Streaming.Kinesis.EndpointURL)
		}

		// Initialize dead letter queue using same AWS config as other services
		dlqURL := cfg.Resilience.DeadLetterQueue.QueueURL
		if dlqURL == "" {
			// If no URL provided, construct from components (for backward compatibility)
			dlqURL = fmt.Sprintf("https://sqs.%s.amazonaws.com/%s/%s",
				cfg.Streaming.Kinesis.Region,
				cfg.Resilience.DeadLetterQueue.AccountID,
				cfg.Resilience.DeadLetterQueue.QueueName)
		}

		dlq = resilience.NewDeadLetterQueue(awsConfig, resilience.DLQConfig{
			QueueURL:       dlqURL,
			MaxRetries:     cfg.Resilience.DeadLetterQueue.MaxRetries,
			RetryBaseDelay: time.Duration(cfg.Resilience.DeadLetterQueue.RetryBaseDelayMs) * time.Millisecond,
			RetryMaxDelay:  time.Duration(cfg.Resilience.DeadLetterQueue.RetryMaxDelayMs) * time.Millisecond,
		})
	}

	// Initialize Kinesis buffer from config
	kinesisBuffer := resilience.NewKinesisBuffer(
		services.KinesisClient,
		rateLimiter,
		circuitBreaker,
		dlq,
		resilience.BufferConfig{
			MaxBatchSize:    cfg.Resilience.KinesisBuffer.MaxBatchSize,
			MaxBatchWait:    time.Duration(cfg.Resilience.KinesisBuffer.MaxBatchWaitMs) * time.Millisecond,
			FlushInterval:   time.Duration(cfg.Resilience.KinesisBuffer.FlushIntervalMs) * time.Millisecond,
			MaxTenantBuffer: cfg.Resilience.KinesisBuffer.MaxTenantBuffer,
		},
	)

	return &interfaces.ResilienceComponents{
		KinesisBuffer:   kinesisBuffer,
		RateLimiter:     rateLimiter,
		DeadLetterQueue: dlq,
	}, circuitBreaker, nil
}
