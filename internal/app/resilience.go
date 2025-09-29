package app

import (
	"context"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/pkg/config"
	"golang.org/x/time/rate"
)

// isDLQConfigured checks if DLQ configuration is complete and valid
func isDLQConfigured(cfg *config.ConsolidatedConfig) bool {
	return cfg.Operations.DLQEnabled && cfg.Operations.DLQQueueURL != ""
}

// InitializeResilienceComponents creates and configures all resilience components
func InitializeResilienceComponents(cfg *config.ConsolidatedConfig, services *Services) (*interfaces.ResilienceComponents, *resilience.CircuitBreaker, error) {
	// Initialize tenant rate limiter from HTTP rate limiting config
	if cfg.Network.HTTP.RateLimitRPS <= 0 {
		return nil, nil, fmt.Errorf("rate limiter requests_per_second must be greater than 0, got: %d", cfg.Network.HTTP.RateLimitRPS)
	}
	rpsLimit := time.Second / time.Duration(cfg.Network.HTTP.RateLimitRPS)
	rateLimiter := resilience.NewTenantRateLimiter(
		rate.Every(rpsLimit),
		cfg.Network.HTTP.RateLimitBurst,
	)

	// Initialize circuit breaker from Operations config
	circuitBreaker := resilience.NewCircuitBreaker(resilience.Settings{
		Name:        "api-circuit-breaker",
		MaxRequests: 100, // Sensible default
		Interval:    30 * time.Second, // Sensible default
		Timeout:     cfg.Operations.CircuitBreakerTimeout,
		ReadyToTrip: func(counts resilience.Counts) bool {
			// Trip if failure rate exceeds configured threshold
			// Guard against division by zero during bootstrap
			if counts.Requests == 0 {
				return false
			}
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRate > cfg.Operations.CircuitBreakerThreshold
		},
	})

	// Initialize dead letter queue only if SQS is properly configured
	var dlq *resilience.DeadLetterQueue
	if isDLQConfigured(cfg) {
		// Load AWS configuration for DLQ
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(cfg.Operations.DLQRegion))
		if err != nil {
			return nil, nil, fmt.Errorf("failed to load AWS config for DLQ: %w", err)
		}

		// Create DLQ configuration
		dlqConfig := resilience.DLQConfig{
			QueueURL:        cfg.Operations.DLQQueueURL,
			LocalBufferSize: cfg.Operations.DLQLocalBufferSize,
			MaxRetries:      cfg.Operations.DLQMaxRetries,
			RetryBaseDelay:  cfg.Operations.DLQRetryBaseDelay,
			RetryMaxDelay:   cfg.Operations.DLQRetryMaxDelay,
			RetryMultiplier: 2.0, // Standard exponential backoff
		}

		dlq = resilience.NewDeadLetterQueue(awsCfg, dlqConfig)
	}

	// Initialize Kinesis buffer with data processing config
	kinesisBuffer := resilience.NewKinesisBuffer(
		services.KinesisClient,
		rateLimiter,
		circuitBreaker,
		dlq,
		resilience.BufferConfig{
			MaxBatchSize:    cfg.Data.Kinesis.BatchSize,
			MaxBatchWait:    cfg.Data.Kinesis.FlushInterval,
			FlushInterval:   cfg.Data.FlushInterval,
			MaxTenantBuffer: cfg.Data.MaxQueueSize,
		},
	)

	return &interfaces.ResilienceComponents{
		KinesisBuffer:   kinesisBuffer,
		RateLimiter:     rateLimiter,
		DeadLetterQueue: dlq,
	}, circuitBreaker, nil
}
