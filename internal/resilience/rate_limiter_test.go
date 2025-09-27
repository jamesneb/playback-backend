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
	rl := NewTenantRateLimiter(2, 1) // 2 requests per second, 1 burst
	tenantID := "test-tenant"

	// First request should be allowed (uses burst)
	allowed := rl.Allow(tenantID)
	assert.True(t, allowed)

	// Second request should be allowed (uses normal rate)
	allowed = rl.Allow(tenantID)
	assert.True(t, allowed)

	// Third request should be denied (rate limit exceeded)
	allowed = rl.Allow(tenantID)
	assert.False(t, allowed)

	// Wait for rate limit to replenish
	time.Sleep(time.Millisecond * 600) // Wait for more than 500ms (at 2 req/sec)

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
	rl := NewTenantRateLimiter(1, 1) // Very slow rate
	tenantID := "timeout-test-tenant"

	// Use up the burst
	allowed := rl.Allow(tenantID)
	assert.True(t, allowed)

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	// This should timeout waiting for rate limit
	start := time.Now()
	err := rl.Wait(ctx, tenantID)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
	assert.Greater(t, elapsed, time.Millisecond*90) // Should wait close to timeout
	assert.Less(t, elapsed, time.Millisecond*150)   // But not much longer
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
