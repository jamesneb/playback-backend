package resilience

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// TenantRateLimiter manages per-tenant rate limiting
type TenantRateLimiter struct {
	limiters map[string]*rate.Limiter
	mutex    sync.RWMutex

	// Default settings
	defaultRate  rate.Limit
	defaultBurst int

	// Per-tenant overrides
	tenantConfigs map[string]TenantConfig

	// Cleanup settings
	cleanupInterval time.Duration
	lastUsed        map[string]time.Time
	maxIdleTime     time.Duration

	// Lifecycle management
	stopCh chan struct{}
	done   chan struct{}
}

// TenantConfig holds rate limiting configuration for a specific tenant
type TenantConfig struct {
	Rate  rate.Limit // requests per second
	Burst int        // burst capacity
}

// NewTenantRateLimiter creates a new tenant-aware rate limiter
func NewTenantRateLimiter(defaultRate rate.Limit, defaultBurst int) *TenantRateLimiter {
	trl := &TenantRateLimiter{
		limiters:        make(map[string]*rate.Limiter),
		defaultRate:     defaultRate,
		defaultBurst:    defaultBurst,
		tenantConfigs:   make(map[string]TenantConfig),
		cleanupInterval: 10 * time.Minute,
		lastUsed:        make(map[string]time.Time),
		maxIdleTime:     30 * time.Minute,
		stopCh:          make(chan struct{}),
		done:            make(chan struct{}),
	}

	// Start cleanup goroutine
	go trl.cleanup()

	return trl
}

// SetTenantConfig sets custom rate limiting for a specific tenant
func (trl *TenantRateLimiter) SetTenantConfig(tenantID string, config TenantConfig) {
	trl.mutex.Lock()
	defer trl.mutex.Unlock()
	
	trl.tenantConfigs[tenantID] = config
	
	// Update existing limiter if it exists
	if limiter, exists := trl.limiters[tenantID]; exists {
		limiter.SetLimit(config.Rate)
		limiter.SetBurst(config.Burst)
	}
	
	logger.Info("Updated tenant rate limit config",
		zap.String("tenant", tenantID),
		zap.Float64("rate", float64(config.Rate)),
		zap.Int("burst", config.Burst))
}

// Allow checks if a request is allowed for the given tenant
func (trl *TenantRateLimiter) Allow(tenantID string) bool {
	limiter := trl.getLimiter(tenantID)
	allowed := limiter.Allow()
	
	if !allowed {
		logger.Warn("Rate limit exceeded",
			zap.String("tenant", tenantID),
			zap.Float64("rate", float64(limiter.Limit())),
			zap.Int("burst", limiter.Burst()))
	}
	
	return allowed
}

// AllowN checks if n requests are allowed for the given tenant
func (trl *TenantRateLimiter) AllowN(tenantID string, n int) bool {
	limiter := trl.getLimiter(tenantID)
	return limiter.AllowN(time.Now(), n)
}

// Wait blocks until a request is allowed for the given tenant
func (trl *TenantRateLimiter) Wait(ctx context.Context, tenantID string) error {
	limiter := trl.getLimiter(tenantID)
	return limiter.Wait(ctx)
}

// Reserve returns a reservation for the given tenant
func (trl *TenantRateLimiter) Reserve(tenantID string) *rate.Reservation {
	limiter := trl.getLimiter(tenantID)
	return limiter.Reserve()
}

func (trl *TenantRateLimiter) getLimiter(tenantID string) *rate.Limiter {
	trl.mutex.Lock()
	defer trl.mutex.Unlock()
	
	// Update last used time
	trl.lastUsed[tenantID] = time.Now()
	
	// Get existing limiter
	if limiter, exists := trl.limiters[tenantID]; exists {
		return limiter
	}
	
	// Create new limiter
	config, hasCustomConfig := trl.tenantConfigs[tenantID]
	if !hasCustomConfig {
		config = TenantConfig{
			Rate:  trl.defaultRate,
			Burst: trl.defaultBurst,
		}
	}
	
	limiter := rate.NewLimiter(config.Rate, config.Burst)
	trl.limiters[tenantID] = limiter
	
	logger.Debug("Created new rate limiter for tenant",
		zap.String("tenant", tenantID),
		zap.Float64("rate", float64(config.Rate)),
		zap.Int("burst", config.Burst))
	
	return limiter
}

// cleanup removes unused rate limiters periodically
func (trl *TenantRateLimiter) cleanup() {
	ticker := time.NewTicker(trl.cleanupInterval)
	defer ticker.Stop()
	defer close(trl.done)

	for {
		select {
		case <-trl.stopCh:
			return
		case <-ticker.C:
			trl.mutex.Lock()
			now := time.Now()

			for tenantID, lastUsed := range trl.lastUsed {
				if now.Sub(lastUsed) > trl.maxIdleTime {
					delete(trl.limiters, tenantID)
					delete(trl.lastUsed, tenantID)
					logger.Debug("Cleaned up unused rate limiter",
						zap.String("tenant", tenantID))
				}
			}

			trl.mutex.Unlock()
		}
	}
}

// GetStats returns current statistics for all tenants
func (trl *TenantRateLimiter) GetStats() map[string]TenantStats {
	trl.mutex.RLock()
	defer trl.mutex.RUnlock()
	
	stats := make(map[string]TenantStats)
	now := time.Now()
	
	for tenantID, limiter := range trl.limiters {
		lastUsed := trl.lastUsed[tenantID]
		config, hasCustom := trl.tenantConfigs[tenantID]
		if !hasCustom {
			config = TenantConfig{Rate: trl.defaultRate, Burst: trl.defaultBurst}
		}
		
		stats[tenantID] = TenantStats{
			Rate:           float64(config.Rate),
			Burst:          config.Burst,
			Tokens:         limiter.Tokens(),
			LastUsed:       lastUsed,
			IdleTime:       now.Sub(lastUsed),
			HasCustomConfig: hasCustom,
		}
	}
	
	return stats
}

type TenantStats struct {
	Rate            float64
	Burst           int
	Tokens          float64
	LastUsed        time.Time
	IdleTime        time.Duration
	HasCustomConfig bool
}

// Close stops the cleanup goroutine and releases resources
func (trl *TenantRateLimiter) Close() error {
	close(trl.stopCh)
	<-trl.done // Wait for cleanup goroutine to finish
	return nil
}

// GlobalRateLimiter provides system-wide rate limiting
type GlobalRateLimiter struct {
	limiter     *rate.Limiter

	// Circuit breaker integration
	circuitBreaker *CircuitBreaker
}

// NewGlobalRateLimiter creates a system-wide rate limiter
func NewGlobalRateLimiter(rps rate.Limit, burst int) *GlobalRateLimiter {
	limiter := rate.NewLimiter(rps, burst)
	
	// Create circuit breaker for system overload protection
	cb := NewCircuitBreaker(Settings{
		Name:        "global-rate-limiter",
		MaxRequests: uint32(burst / 2),
		Interval:    30 * time.Second,
		Timeout:     10 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// Trip if more than 50% of requests fail
			// Guard against division by zero
			if counts.Requests == 0 {
				return false
			}
			failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRate > 0.5
		},
	})
	
	return &GlobalRateLimiter{
		limiter:        limiter,
		circuitBreaker: cb,
	}
}

// Allow checks if a request is allowed globally
func (grl *GlobalRateLimiter) Allow() bool {
	// Check circuit breaker first
	if grl.circuitBreaker.State() == StateOpen {
		return false
	}
	
	return grl.limiter.Allow()
}

// Execute runs a request through global rate limiting and circuit breaking
func (grl *GlobalRateLimiter) Execute(fn func() error) error {
	if !grl.Allow() {
		return fmt.Errorf("global rate limit exceeded")
	}
	
	return grl.circuitBreaker.Call(fn)
}

// AdaptiveRateLimiter adjusts rates based on system health
type AdaptiveRateLimiter struct {
	baseLimiter *TenantRateLimiter
	
	// Health metrics
	healthChecker HealthChecker
	
	// Adaptive settings
	minRateMultiplier float64
	maxRateMultiplier float64
	adjustmentFactor  float64
}

type HealthChecker interface {
	IsHealthy() bool
	GetHealthScore() float64 // 0.0 = unhealthy, 1.0 = fully healthy
}

// NewAdaptiveRateLimiter creates a rate limiter that adapts to system health
func NewAdaptiveRateLimiter(baseLimiter *TenantRateLimiter, healthChecker HealthChecker) *AdaptiveRateLimiter {
	return &AdaptiveRateLimiter{
		baseLimiter:       baseLimiter,
		healthChecker:     healthChecker,
		minRateMultiplier: 0.1,  // Reduce to 10% when unhealthy
		maxRateMultiplier: 2.0,  // Increase to 200% when very healthy
		adjustmentFactor:  0.1,  // How quickly to adjust
	}
}

// Allow checks if request is allowed with adaptive rate limiting
func (arl *AdaptiveRateLimiter) Allow(tenantID string) bool {
	healthScore := arl.healthChecker.GetHealthScore()
	
	// Adjust the effective rate based on health (for future use)
	_ = arl.minRateMultiplier + 
		(arl.maxRateMultiplier-arl.minRateMultiplier)*healthScore
	
	// For simplicity, we'll use a probabilistic approach
	// In practice, you'd want to adjust the actual rate limiter
	baseAllowed := arl.baseLimiter.Allow(tenantID)
	
	if !baseAllowed {
		return false
	}
	
	// Additional throttling based on health
	if healthScore < 0.5 {
		// When unhealthy, randomly reject additional requests
		rejectProbability := (0.5 - healthScore) * 2 // 0 to 1
		if rand.Float64() < rejectProbability {
			return false
		}
	}
	
	return true
}