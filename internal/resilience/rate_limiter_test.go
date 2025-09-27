package resilience

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestNewTenantRateLimiter(t *testing.T) {
	rl := NewTenantRateLimiter(100, 10) // 100 requests per second, 10 burst

	assert.NotNil(t, rl)
	assert.NotNil(t, rl.limiters)
}

func TestRateLimiterAllow(t *testing.T) {
	rl := NewTenantRateLimiter(2, 2) // 2 requests per second, 2 burst capacity
	tenantID := "test-tenant"

	// First request should be allowed (uses burst)
	allowed := rl.Allow(tenantID)
	assert.True(t, allowed)

	// Second request should be allowed (uses remaining burst)
	allowed = rl.Allow(tenantID)
	assert.True(t, allowed)

	// Third request should be denied (burst exhausted, rate limit not replenished)
	allowed = rl.Allow(tenantID)
	assert.False(t, allowed)

	// Wait for rate limit to replenish (at 2 req/sec, need to wait 500ms for 1 token)
	time.Sleep(time.Millisecond * 600)

	// Should be allowed again
	allowed = rl.Allow(tenantID)
	assert.True(t, allowed)
}

func TestRateLimiterMultipleTenants(t *testing.T) {
	rl := NewTenantRateLimiter(1, 1) // 1 request per second, 1 burst

	// Test different tenants have independent rate limits
	tenant1 := "tenant-1"
	tenant2 := "tenant-2"

	// Both tenants should be allowed initially
	assert.True(t, rl.Allow(tenant1))
	assert.True(t, rl.Allow(tenant2))

	// Both should hit their rate limits
	assert.False(t, rl.Allow(tenant1))
	assert.False(t, rl.Allow(tenant2))
}

func TestRateLimiterWait(t *testing.T) {
	rl := NewTenantRateLimiter(10, 1) // 10 requests per second = 100ms per request
	tenantID := "wait-test-tenant"
	ctx := context.Background()

	// First request should not wait
	start := time.Now()
	err := rl.Wait(ctx, tenantID)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, elapsed, time.Millisecond*50) // Should be very quick

	// Second request might need to wait
	start = time.Now()
	err = rl.Wait(ctx, tenantID)
	elapsed = time.Since(start)

	assert.NoError(t, err)
	// Could be immediate if burst is available, or wait up to 100ms
	assert.Less(t, elapsed, time.Millisecond*200)
}

func TestRateLimiterWaitWithTimeout(t *testing.T) {
	// Test the underlying rate limiter behavior directly
	limiter := rate.NewLimiter(rate.Limit(0.5), 1) // 0.5 req/sec, burst of 1

	// Consume the burst token
	ok1 := limiter.Allow()
	assert.True(t, ok1, "First request should be allowed (burst token)")

	// Second request should be denied
	ok2 := limiter.Allow()
	assert.False(t, ok2, "Second request should be denied")

	// Test Wait with timeout - this should return immediately with deadline error
	// since it knows the context will timeout before a token becomes available
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx)
	elapsed := time.Since(start)

	// Should return immediately with deadline-related error
	assert.Error(t, err)
	// The error message indicates the context deadline would be exceeded
	assert.Contains(t, err.Error(), "deadline")
	// Should return very quickly (not actually wait)
	assert.Less(t, elapsed, time.Millisecond*10)

	// Now test our wrapper behaves the same way
	rl := NewTenantRateLimiter(rate.Limit(0.5), 1)
	tenantID := "timeout-test-tenant"

	// Consume burst
	allowed := rl.Allow(tenantID)
	assert.True(t, allowed)

	// Should be denied
	allowed = rl.Allow(tenantID)
	assert.False(t, allowed)

	// Test our Wait wrapper
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel2()

	start2 := time.Now()
	err2 := rl.Wait(ctx2, tenantID)
	elapsed2 := time.Since(start2)

	// Should behave like the underlying limiter
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "deadline")
	assert.Less(t, elapsed2, time.Millisecond*10)
}

func TestRateLimiterGetStats(t *testing.T) {
	rl := NewTenantRateLimiter(10, 5)
	tenantID := "stats-test-tenant"

	// Make some requests
	for i := 0; i < 3; i++ {
		rl.Allow(tenantID)
	}

	stats := rl.GetStats()
	assert.NotNil(t, stats)
	tenantStats, exists := stats[tenantID]
	assert.True(t, exists)
	assert.Equal(t, float64(10), tenantStats.Rate)
	assert.Equal(t, 5, tenantStats.Burst)
	assert.True(t, tenantStats.LastUsed.After(time.Time{})) // Should be set
}

func TestRateLimiterCleanupInactiveTenants(t *testing.T) {
	rl := NewTenantRateLimiter(10, 5)
	// Override cleanup settings for faster testing
	rl.cleanupInterval = time.Millisecond * 50 // Very frequent cleanup
	rl.maxIdleTime = time.Millisecond * 30     // Very short idle time

	// Add some tenants
	tenant1 := "active-tenant"
	tenant2 := "inactive-tenant"

	rl.Allow(tenant1)
	rl.Allow(tenant2)

	// Keep tenant1 active
	time.Sleep(time.Millisecond * 20)
	rl.Allow(tenant1)

	// Wait for cleanup to potentially remove tenant2
	time.Sleep(time.Millisecond * 100)

	// Check that cleanup doesn't break functionality
	assert.True(t, rl.Allow(tenant1))
	assert.True(t, rl.Allow(tenant2)) // Should still work even if cleaned up
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	rl := NewTenantRateLimiter(100, 10)
	tenantID := "concurrent-test-tenant"

	numGoroutines := 50
	allowedCh := make(chan bool, numGoroutines)

	// Run concurrent requests
	for i := 0; i < numGoroutines; i++ {
		go func() {
			allowed := rl.Allow(tenantID)
			allowedCh <- allowed
		}()
	}

	// Count allowed requests
	allowedCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-allowedCh {
			allowedCount++
		}
	}

	// Should allow some requests (at least burst size)
	assert.Greater(t, allowedCount, 5)

	// Stats should be consistent
	stats := rl.GetStats()
	tenantStats := stats[tenantID]
	assert.Equal(t, float64(100), tenantStats.Rate)
	assert.Equal(t, 10, tenantStats.Burst)
}

func TestRateLimiterClose(t *testing.T) {
	rl := NewTenantRateLimiter(10, 5)

	// Make some requests
	rl.Allow("test-tenant")

	// Close should not error
	err := rl.Close()
	assert.NoError(t, err)

	// Rate limiter should still function after close
	// (cleanup goroutine stops but rate limiting continues)
	assert.True(t, rl.Allow("test-tenant-after-close"))
}

func TestRateLimiterEdgeCases(t *testing.T) {
	t.Run("empty tenant ID", func(t *testing.T) {
		rl := NewTenantRateLimiter(10, 5)

		// Should handle empty tenant ID gracefully
		allowed := rl.Allow("")
		assert.True(t, allowed) // Should still work

		stats := rl.GetStats()
		tenantStats, exists := stats[""]
		assert.True(t, exists)
		assert.Equal(t, float64(10), tenantStats.Rate)
	})

	t.Run("zero rate limit", func(t *testing.T) {
		rl := NewTenantRateLimiter(0, 1) // Zero rate

		// Should still allow burst
		allowed := rl.Allow("zero-rate-tenant")
		assert.True(t, allowed)

		// Subsequent requests should be denied
		allowed = rl.Allow("zero-rate-tenant")
		assert.False(t, allowed)
	})

	t.Run("very high rate", func(t *testing.T) {
		rl := NewTenantRateLimiter(1000000, 1000) // Very high rate

		// Should allow many requests quickly
		for i := 0; i < 100; i++ {
			allowed := rl.Allow("high-rate-tenant")
			assert.True(t, allowed, "Request %d should be allowed", i)
		}
	})
}

func TestRateLimiterTenantConfig(t *testing.T) {
	rl := NewTenantRateLimiter(10, 5) // Default: 10 req/s, 5 burst

	tenantID := "custom-tenant"

	// Set custom config for tenant
	customConfig := TenantConfig{
		Rate:  rate.Limit(100), // 100 req/s
		Burst: 20,              // 20 burst
	}
	rl.SetTenantConfig(tenantID, customConfig)

	// Use the tenant to create limiter
	rl.Allow(tenantID)

	// Check stats reflect custom config
	stats := rl.GetStats()
	tenantStats := stats[tenantID]
	assert.Equal(t, float64(100), tenantStats.Rate)
	assert.Equal(t, 20, tenantStats.Burst)
	assert.True(t, tenantStats.HasCustomConfig)
}

func TestRateLimiterAllowN(t *testing.T) {
	rl := NewTenantRateLimiter(10, 5)
	tenantID := "allow-n-tenant"

	// Should allow multiple requests if within burst
	allowed := rl.AllowN(tenantID, 3)
	assert.True(t, allowed)

	// Should deny if exceeds available tokens
	allowed = rl.AllowN(tenantID, 5)
	assert.False(t, allowed)
}

func TestRateLimiterReserve(t *testing.T) {
	rl := NewTenantRateLimiter(10, 5)
	tenantID := "reserve-tenant"

	// Reserve should return a reservation
	reservation := rl.Reserve(tenantID)
	assert.NotNil(t, reservation)

	// Should be able to use the reservation
	assert.True(t, reservation.OK())
}
