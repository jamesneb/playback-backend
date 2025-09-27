package interfaces

import (
	"github.com/jamesneb/playback-backend/internal/resilience"
)

// ResilienceComponents groups resilience-related dependencies for both HTTP and gRPC handlers
// This interface provides a clean separation between API layers and internal implementations
type ResilienceComponents struct {
	KinesisBuffer   *resilience.KinesisBuffer
	RateLimiter     *resilience.TenantRateLimiter
	CircuitBreaker  *resilience.CircuitBreaker
	DeadLetterQueue *resilience.DeadLetterQueue
}

// NewResilienceComponents creates a new ResilienceComponents instance
func NewResilienceComponents(
	buffer *resilience.KinesisBuffer,
	rateLimiter *resilience.TenantRateLimiter,
	circuitBreaker *resilience.CircuitBreaker,
	dlq *resilience.DeadLetterQueue,
) *ResilienceComponents {
	return &ResilienceComponents{
		KinesisBuffer:   buffer,
		RateLimiter:     rateLimiter,
		CircuitBreaker:  circuitBreaker,
		DeadLetterQueue: dlq,
	}
}

// HasKinesisBuffer checks if KinesisBuffer is configured
func (rc *ResilienceComponents) HasKinesisBuffer() bool {
	return rc != nil && rc.KinesisBuffer != nil
}

// HasRateLimiter checks if RateLimiter is configured
func (rc *ResilienceComponents) HasRateLimiter() bool {
	return rc != nil && rc.RateLimiter != nil
}

// HasCircuitBreaker checks if CircuitBreaker is configured
func (rc *ResilienceComponents) HasCircuitBreaker() bool {
	return rc != nil && rc.CircuitBreaker != nil
}

// HasDeadLetterQueue checks if DeadLetterQueue is configured
func (rc *ResilienceComponents) HasDeadLetterQueue() bool {
	return rc != nil && rc.DeadLetterQueue != nil
}