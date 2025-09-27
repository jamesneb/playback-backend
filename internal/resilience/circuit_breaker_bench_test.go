package resilience

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// BenchmarkCircuitBreakerCreation benchmarks circuit breaker creation
func BenchmarkCircuitBreakerCreation(b *testing.B) {
	settings := Settings{
		Name:        "test-cb",
		MaxRequests: 10,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		cb := NewCircuitBreaker(settings)
		_ = cb
	}
}

// BenchmarkCircuitBreakerExecution benchmarks circuit breaker execution
func BenchmarkCircuitBreakerExecution(b *testing.B) {
	cb := NewCircuitBreaker(Settings{
		Name:        "test-cb",
		MaxRequests: 100,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second,
	})

	successfulFunc := func() (interface{}, error) {
		return "success", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := cb.Execute(successfulFunc)
		_ = result
		_ = err
	}
}

// BenchmarkCircuitBreakerExecutionParallel benchmarks circuit breaker under concurrent load
func BenchmarkCircuitBreakerExecutionParallel(b *testing.B) {
	cb := NewCircuitBreaker(Settings{
		Name:        "test-cb",
		MaxRequests: 100,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second,
	})

	successfulFunc := func() (interface{}, error) {
		return "success", nil
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := cb.Execute(successfulFunc)
			_ = result
			_ = err
		}
	})
}

// BenchmarkCircuitBreakerFailures benchmarks circuit breaker with failures
func BenchmarkCircuitBreakerFailures(b *testing.B) {
	cb := NewCircuitBreaker(Settings{
		Name:        "test-cb",
		MaxRequests: 100,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second,
	})

	var failureCount int64
	failingFunc := func() (interface{}, error) {
		count := atomic.AddInt64(&failureCount, 1)
		if count%2 == 0 {
			return "success", nil
		}
		return nil, errors.New("simulated failure")
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := cb.Execute(failingFunc)
		_ = result
		_ = err
	}
}

// BenchmarkCircuitBreakerMetrics benchmarks metrics collection
func BenchmarkCircuitBreakerMetrics(b *testing.B) {
	cb := NewCircuitBreaker(Settings{
		Name:        "test-cb",
		MaxRequests: 100,
		Interval:    10 * time.Second,
		Timeout:     60 * time.Second,
	})

	// Execute some operations to generate metrics
	successfulFunc := func() (interface{}, error) {
		return "success", nil
	}

	for i := 0; i < 100; i++ {
		cb.Execute(successfulFunc)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		metrics := cb.Metrics()
		_ = metrics
	}
}

// BenchmarkRateLimiterCreation benchmarks rate limiter creation
func BenchmarkRateLimiterCreation(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		limiter := NewTenantRateLimiter(rate.Every(100*time.Millisecond), 10)
		_ = limiter
	}
}

// BenchmarkRateLimiterAllow benchmarks rate limiter allow operations
func BenchmarkRateLimiterAllow(b *testing.B) {
	limiter := NewTenantRateLimiter(rate.Every(time.Nanosecond), 1000) // Very permissive for benchmark

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		allowed := limiter.Allow("test-tenant")
		_ = allowed
	}
}

// BenchmarkRateLimiterAllowParallel benchmarks rate limiter under concurrent load
func BenchmarkRateLimiterAllowParallel(b *testing.B) {
	limiter := NewTenantRateLimiter(rate.Every(time.Nanosecond), 1000)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			allowed := limiter.Allow("test-tenant")
			_ = allowed
		}
	})
}

// BenchmarkRateLimiterMultipleTenants benchmarks rate limiter with multiple tenants
func BenchmarkRateLimiterMultipleTenants(b *testing.B) {
	limiter := NewTenantRateLimiter(rate.Every(time.Nanosecond), 1000)
	tenants := []string{"tenant1", "tenant2", "tenant3", "tenant4", "tenant5"}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var tenantIndex int64
		for pb.Next() {
			index := atomic.AddInt64(&tenantIndex, 1)
			tenant := tenants[index%int64(len(tenants))]
			allowed := limiter.Allow(tenant)
			_ = allowed
		}
	})
}

// BenchmarkStateTransitions benchmarks circuit breaker state transitions
func BenchmarkStateTransitions(b *testing.B) {
	cb := NewCircuitBreaker(Settings{
		Name:        "test-cb",
		MaxRequests: 5,
		Interval:    1 * time.Millisecond,
		Timeout:     1 * time.Millisecond,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures > 2
		},
	})

	var requestCount int64
	mixedFunc := func() (interface{}, error) {
		count := atomic.AddInt64(&requestCount, 1)
		if count%10 < 3 { // 30% failure rate
			return nil, errors.New("simulated failure")
		}
		return "success", nil
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := cb.Execute(mixedFunc)
		_ = result
		_ = err

		// Add small delay to allow state transitions
		if i%100 == 0 {
			time.Sleep(2 * time.Millisecond)
		}
	}
}
