package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/resilience"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int           `yaml:"requests_per_second"`
	BurstCapacity     int           `yaml:"burst_capacity"`
	CleanupInterval   time.Duration `yaml:"cleanup_interval"`
	MaxIdleTime       time.Duration `yaml:"max_idle_time"`
}

// RateLimitingMiddleware creates a comprehensive rate limiting middleware
func RateLimitingMiddleware(cfg *config.Config) gin.HandlerFunc {
	// Extract rate limiting config from main config
	rateLimitConfig := RateLimitConfig{
		RequestsPerSecond: cfg.Resilience.RateLimiter.RequestsPerSecond,
		BurstCapacity:     cfg.Resilience.RateLimiter.BurstCapacity,
		CleanupInterval:   30 * time.Second,
		MaxIdleTime:       5 * time.Minute,
	}

	// Create rate limiter for API endpoints
	globalRateLimit := rate.Every(time.Second / time.Duration(rateLimitConfig.RequestsPerSecond))
	apiLimiter := resilience.NewTenantRateLimiter(globalRateLimit, rateLimitConfig.BurstCapacity)

	return gin.HandlerFunc(func(c *gin.Context) {
		// Extract client identifier (IP or tenant)
		clientID := extractClientIdentifier(c)

		// Apply rate limiting
		if !apiLimiter.Allow(clientID) {
			logger.Warn("Rate limit exceeded for API request",
				zap.String("client_id", clientID),
				zap.String("path", c.Request.URL.Path),
				zap.String("method", c.Request.Method))

			// Set rate limit headers
			setRateLimitHeaders(c, rateLimitConfig)

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":               "rate_limit_exceeded",
				"message":             "Too many requests. Please try again later.",
				"retry_after_seconds": 1,
			})
			c.Abort()
			return
		}

		// Add rate limit headers to successful requests
		setRateLimitHeaders(c, rateLimitConfig)

		c.Next()
	})
}

// extractClientIdentifier extracts a unique identifier for rate limiting
func extractClientIdentifier(c *gin.Context) string {
	// Try tenant ID from header first
	if tenantID := c.GetHeader("X-Tenant-ID"); tenantID != "" {
		return "tenant:" + tenantID
	}

	// Try API key
	if apiKey := c.GetHeader("X-API-Key"); apiKey != "" {
		return "api_key:" + apiKey
	}

	// Fall back to client IP
	return "ip:" + c.ClientIP()
}

// setRateLimitHeaders sets standard rate limiting headers
func setRateLimitHeaders(c *gin.Context, config RateLimitConfig) {
	c.Header("X-RateLimit-Limit", strconv.Itoa(config.RequestsPerSecond))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(config.BurstCapacity))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))
}

// PathSpecificRateLimitMiddleware creates rate limiting for specific endpoints
func PathSpecificRateLimitMiddleware(requestsPerSecond int, burstCapacity int) gin.HandlerFunc {
	pathRateLimit := rate.Every(time.Second / time.Duration(requestsPerSecond))
	pathLimiter := resilience.NewTenantRateLimiter(pathRateLimit, burstCapacity)

	return gin.HandlerFunc(func(c *gin.Context) {
		clientID := extractClientIdentifier(c)

		if !pathLimiter.Allow(clientID) {
			logger.Warn("Path-specific rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("path", c.Request.URL.Path),
				zap.Int("limit_rps", requestsPerSecond))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":               "rate_limit_exceeded",
				"message":             "Rate limit exceeded for this endpoint. Please try again later.",
				"retry_after_seconds": 1,
			})
			c.Abort()
			return
		}

		c.Next()
	})
}

// SizeBasedRateLimitMiddleware applies different rate limits based on request size
func SizeBasedRateLimitMiddleware() gin.HandlerFunc {
	// Different limits for different payload sizes
	smallPayloadLimiter := resilience.NewTenantRateLimiter(rate.Every(100*time.Millisecond), 50) // 10 RPS
	largePayloadLimiter := resilience.NewTenantRateLimiter(rate.Every(time.Second), 5)           // 1 RPS

	return gin.HandlerFunc(func(c *gin.Context) {
		clientID := extractClientIdentifier(c)
		contentLength := c.Request.ContentLength

		var limiter *resilience.TenantRateLimiter
		var limitType string

		// Apply different limits based on payload size
		if contentLength > 1024*1024 { // > 1MB
			limiter = largePayloadLimiter
			limitType = "large_payload"
		} else {
			limiter = smallPayloadLimiter
			limitType = "small_payload"
		}

		if !limiter.Allow(clientID) {
			logger.Warn("Size-based rate limit exceeded",
				zap.String("client_id", clientID),
				zap.String("limit_type", limitType),
				zap.Int64("content_length", contentLength))

			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":               "rate_limit_exceeded",
				"message":             "Rate limit exceeded based on request size. Please try again later.",
				"retry_after_seconds": 2,
			})
			c.Abort()
			return
		}

		c.Next()
	})
}
