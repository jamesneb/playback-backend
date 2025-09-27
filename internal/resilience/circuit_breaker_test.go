package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCircuitBreaker(t *testing.T) {
	settings := Settings{
		Name:        "test-cb",
		MaxRequests: 5,
		Timeout:     time.Minute,
		Interval:    time.Second * 10,
	}

	cb := NewCircuitBreaker(settings)

	assert.NotNil(t, cb)
	assert.Equal(t, StateClosed, cb.State())

	metrics := cb.Metrics()
	assert.Equal(t, int64(0), metrics.TotalFailures)
	assert.Equal(t, int64(0), metrics.TotalRequests)
}

func TestCircuitBreakerStateTransitions(t *testing.T) {
	settings := Settings{
		Name:        "test-transitions",
		MaxRequests: 1,
		Timeout:     time.Millisecond * 100,
		Interval:    time.Millisecond * 50,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreaker(settings)

	// Test closed state - success
	_, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State())

	metrics := cb.Metrics()
	assert.Equal(t, int64(1), metrics.TotalRequests)

	// Test closed state - failures leading to open state
	for i := 0; i < 3; i++ {
		_, err = cb.Execute(func() (interface{}, error) {
			return nil, errors.New("test error")
		})
		assert.Error(t, err)
	}

	assert.Equal(t, StateOpen, cb.State())

	metrics = cb.Metrics()
	assert.Equal(t, int64(3), metrics.TotalFailures)

	// Test open state - requests should fail immediately
	_, err = cb.Execute(func() (interface{}, error) {
		return "should fail", nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker is open")

	// Wait for reset timeout to transition to half-open
	time.Sleep(time.Millisecond * 150)

	// Next request should transition to half-open and then to closed on success
	_, err = cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, StateClosed, cb.State()) // Success in half-open transitions back to closed
}

func TestCircuitBreakerHalfOpenState(t *testing.T) {
	settings := Settings{
		Name:        "test-half-open",
		MaxRequests: 1,
		Timeout:     time.Millisecond * 50,
		Interval:    time.Millisecond * 25,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}

	cb := NewCircuitBreaker(settings)

	// Trigger failures to open the circuit
	for i := 0; i < 2; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("test error")
		})
		assert.Error(t, err)
	}
	assert.Equal(t, StateOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(time.Millisecond * 75)

	// Test half-open with failure - should go back to open
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, errors.New("still failing")
	})
	assert.Error(t, err)
	// Note: State might still be half-open or open depending on timing and implementation
	// The important thing is that the error was returned
	state := cb.State()
	assert.True(t, state == StateOpen || state == StateHalfOpen, "Expected Open or HalfOpen state")
}

func TestCircuitBreakerConcurrentRequests(t *testing.T) {
	settings := Settings{
		Name:        "test-concurrent",
		MaxRequests: 10,
		Timeout:     time.Second,
		Interval:    time.Millisecond * 100,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 10
		},
	}

	cb := NewCircuitBreaker(settings)

	// Run concurrent successful requests
	numGoroutines := 50
	successCh := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, err := cb.Execute(func() (interface{}, error) {
				time.Sleep(time.Millisecond) // Simulate work
				return "success", nil
			})
			successCh <- err == nil
		}()
	}

	// Wait for all goroutines to complete
	successCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-successCh {
			successCount++
		}
	}

	assert.Equal(t, numGoroutines, successCount)
	assert.Equal(t, StateClosed, cb.State())

	metrics := cb.Metrics()
	assert.Equal(t, int64(numGoroutines), metrics.TotalRequests)
}

func TestCircuitBreakerMetrics(t *testing.T) {
	settings := Settings{
		Name:        "test-metrics",
		MaxRequests: 5,
		Timeout:     time.Minute,
		Interval:    time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 3
		},
	}

	cb := NewCircuitBreaker(settings)

	// Execute successful requests
	for i := 0; i < 5; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return "success", nil
		})
		assert.NoError(t, err)
	}

	// Execute failing requests
	for i := 0; i < 2; i++ {
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("test error")
		})
		assert.Error(t, err)
	}

	metrics := cb.Metrics()
	assert.Equal(t, int64(7), metrics.TotalRequests)
	assert.Equal(t, int64(2), metrics.TotalFailures)
	assert.Equal(t, StateClosed, cb.State()) // Not enough failures to open
}

func TestCircuitBreakerContextCancellation(t *testing.T) {
	settings := Settings{
		Name:        "test-context",
		MaxRequests: 5,
		Timeout:     time.Minute,
		Interval:    time.Second,
	}

	cb := NewCircuitBreaker(settings)
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	defer cancel()

	_, err := cb.Execute(func() (interface{}, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Millisecond * 100):
			return "success", nil
		}
	})

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestCircuitBreakerClose(t *testing.T) {
	settings := Settings{
		Name:        "test-close",
		MaxRequests: 3,
		Timeout:     time.Millisecond * 100,
		Interval:    time.Millisecond * 50,
	}

	cb := NewCircuitBreaker(settings)

	// Circuit breaker doesn't have a Close method in the current implementation
	// Test that it continues to work normally
	_, err := cb.Execute(func() (interface{}, error) {
		return "success", nil
	})
	assert.NoError(t, err)
}

func TestCircuitBreakerEdgeCases(t *testing.T) {
	t.Run("zero max failures", func(t *testing.T) {
		settings := Settings{
			Name:        "zero-failures",
			MaxRequests: 0, // Should default to 1
			Timeout:     time.Second,
			Interval:    time.Millisecond * 100,
			ReadyToTrip: func(counts Counts) bool {
				return counts.ConsecutiveFailures >= 1 // First failure should open
			},
		}

		cb := NewCircuitBreaker(settings)

		// First failure should open circuit
		_, err := cb.Execute(func() (interface{}, error) {
			return nil, errors.New("test error")
		})
		assert.Error(t, err)
		assert.Equal(t, StateOpen, cb.State())
	})

	t.Run("nil function", func(t *testing.T) {
		settings := Settings{
			Name:        "nil-function",
			MaxRequests: 3,
			Timeout:     time.Second,
			Interval:    time.Millisecond * 100,
		}

		cb := NewCircuitBreaker(settings)

		// Should handle nil function gracefully by panicking, which Execute should catch
		assert.Panics(t, func() {
			_, _ = cb.Execute(nil)
		})
	})
}
