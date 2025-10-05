// Package circuitbreaker defines configuration for circuit breaker resilience patterns.
//
// Circuit breakers protect downstream services from cascading failures by detecting failures
// and preventing requests when a failure threshold is exceeded. This prevents resource exhaustion
// and allows failing services time to recover.
//
// # Circuit Breaker Pattern
//
// The circuit breaker pattern is a resilience pattern that prevents an application from
// repeatedly attempting an operation that is likely to fail, allowing it to detect failures
// and encapsulate the logic of preventing failures from constantly recurring.
//
// Learn more: https://martinfowler.com/bliki/CircuitBreaker.html
// Microsoft patterns: https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker
//
// # Circuit Breaker States
//
// The circuit breaker operates as a state machine with three states:
//
// CLOSED (Normal Operation):
//   - Requests pass through normally
//   - Failures are counted
//   - When failure threshold exceeded → transition to OPEN
//   - Success counters reset on successful requests
//
// OPEN (Failure State):
//   - All requests fail fast without attempting operation
//   - No requests reach downstream service
//   - After sleep window expires → transition to HALF-OPEN
//   - Gives failing service time to recover
//
// HALF-OPEN (Recovery Testing):
//   - Limited requests allowed through to test service
//   - Successful requests → increment success counter
//   - When consecutive successes reached → transition to CLOSED
//   - Any failure → transition back to OPEN
//   - Tests if service has recovered
//
// State transition diagram:
//
//	            failure rate
//	           threshold exceeded
//	[CLOSED] ─────────────────────> [OPEN]
//	    ^                              │
//	    │                              │ sleep window
//	    │                              │ expires
//	    │                              v
//	    └────────────────────── [HALF-OPEN]
//	     consecutive successes
//	     threshold reached
//
// # Configuration
//
// Timeout settings:
//   - Timeout: Maximum duration for a single request (default: 5s, range: 100ms-1m)
//
// Failure detection:
//   - RequestThreshold: Minimum requests before circuit can open (default: 10, range: 1-1000)
//   - FailureRateThreshold: Percentage of failures to open circuit (default: 50%, range: 0-100%)
//   - MaxConcurrentRequests: Maximum concurrent requests (default: 100, range: 1-10000)
//
// Recovery settings:
//   - SleepWindow: Duration to wait before entering half-open (default: 10s, range: 1s-5m)
//   - HalfOpenMaxRequests: Max requests to test during half-open (default: 5, range: 1-100)
//   - ConsecutiveSuccesses: Successes needed to close circuit (default: 2, range: 1-100)
//
// # Configuration Examples
//
// Strict circuit breaker (quick to open, slow to close):
//
//	CIRCUIT_BREAKER_ENABLED=true
//	CIRCUIT_BREAKER_REQUEST_THRESHOLD=5          # Open after few requests
//	CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=25    # 25% failure rate triggers
//	CIRCUIT_BREAKER_CONSECUTIVE_SUCCESSES=5      # Require many successes
//	CIRCUIT_BREAKER_SLEEP_WINDOW=30s             # Long recovery period
//
// Lenient circuit breaker (slow to open, quick to close):
//
//	CIRCUIT_BREAKER_ENABLED=true
//	CIRCUIT_BREAKER_REQUEST_THRESHOLD=50         # Require many requests
//	CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=75    # 75% failure rate triggers
//	CIRCUIT_BREAKER_CONSECUTIVE_SUCCESSES=1      # One success closes
//	CIRCUIT_BREAKER_SLEEP_WINDOW=5s              # Short recovery period
//
// Balanced circuit breaker (recommended default):
//
//	CIRCUIT_BREAKER_ENABLED=true
//	CIRCUIT_BREAKER_REQUEST_THRESHOLD=10
//	CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=50
//	CIRCUIT_BREAKER_CONSECUTIVE_SUCCESSES=2
//	CIRCUIT_BREAKER_SLEEP_WINDOW=10s
//
// # Tuning Guidelines
//
// RequestThreshold:
//   - Too low: Opens on transient errors
//   - Too high: Slow to detect failures
//   - Recommendation: 10-20 for most services
//
// FailureRateThreshold:
//   - Too low: Opens unnecessarily
//   - Too high: Continues hammering failing service
//   - Recommendation: 40-60% for balanced behavior
//
// SleepWindow:
//   - Too short: Doesn't give service time to recover
//   - Too long: Delays recovery detection
//   - Recommendation: 5-30s based on recovery expectations
//
// ConsecutiveSuccesses:
//   - Too low: May close on fluke success
//   - Too high: Slow to resume normal operation
//   - Recommendation: 2-5 for safety
//
// # Use Cases
//
// External API calls:
//
//	CIRCUIT_BREAKER_ENABLED=true
//	CIRCUIT_BREAKER_TIMEOUT=10s                  # API may be slow
//	CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=50
//	CIRCUIT_BREAKER_SLEEP_WINDOW=30s             # Give API time to recover
//
// Database connections:
//
//	CIRCUIT_BREAKER_ENABLED=true
//	CIRCUIT_BREAKER_TIMEOUT=5s
//	CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=25    # Strict for DB
//	CIRCUIT_BREAKER_SLEEP_WINDOW=15s
//
// Microservice communication:
//
//	CIRCUIT_BREAKER_ENABLED=true
//	CIRCUIT_BREAKER_TIMEOUT=3s                   # Low latency expected
//	CIRCUIT_BREAKER_FAILURE_RATE_THRESHOLD=60
//	CIRCUIT_BREAKER_SLEEP_WINDOW=10s
//
// # Best Practices
//
// Circuit breaker placement:
//   - Place at service boundaries
//   - One circuit breaker per downstream dependency
//   - Independent breakers for different endpoints
//   - Avoid shared circuit breakers across services
//
// Error classification:
//   - Include: Timeouts, connection errors, 5xx errors
//   - Exclude: 4xx client errors, validation errors
//   - Include transient failures only
//   - Don't trip on expected errors
//
// Monitoring:
//   - Track circuit state changes
//   - Alert on OPEN state
//   - Monitor failure rates
//   - Track half-open success/failure
//   - Measure impact on error rates
//
// Fallback strategies:
//   - Return cached data
//   - Return degraded functionality
//   - Return default values
//   - Queue requests for later
//   - Redirect to backup service
//
// # Example Usage
//
//	// Load configuration
//	cfg, err := circuitbreaker.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Create circuit breaker (using github.com/sony/gobreaker)
//	settings := gobreaker.Settings{
//	    Name:        "api-circuit",
//	    MaxRequests: uint32(cfg.HalfOpenMaxRequests),
//	    Interval:    0, // Use failure rate instead of count
//	    Timeout:     cfg.SleepWindow,
//	    ReadyToTrip: func(counts gobreaker.Counts) bool {
//	        if counts.Requests < uint32(cfg.RequestThreshold) {
//	            return false
//	        }
//	        failureRate := float64(counts.TotalFailures) / float64(counts.Requests)
//	        return failureRate >= float64(cfg.FailureRateThreshold.Value())/100
//	    },
//	    OnStateChange: func(name string, from, to gobreaker.State) {
//	        log.Printf("Circuit breaker %s: %s -> %s", name, from, to)
//	    },
//	}
//	cb := gobreaker.NewCircuitBreaker(settings)
//
//	// Use circuit breaker
//	result, err := cb.Execute(func() (interface{}, error) {
//	    ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
//	    defer cancel()
//	    return apiClient.Call(ctx)
//	})
//
// # Troubleshooting
//
// Circuit opens too frequently:
//
//	Problem: Circuit breaker opening on transient errors
//	Fix: Increase REQUEST_THRESHOLD or FAILURE_RATE_THRESHOLD
//
// Circuit doesn't open when it should:
//
//	Problem: Service continues to hammer failing dependency
//	Fix: Decrease REQUEST_THRESHOLD or FAILURE_RATE_THRESHOLD
//
// Slow recovery:
//
//	Problem: Circuit stays open too long
//	Fix: Decrease SLEEP_WINDOW or CONSECUTIVE_SUCCESSES
//
// Premature recovery:
//
//	Problem: Circuit closes before service fully recovered
//	Fix: Increase CONSECUTIVE_SUCCESSES or SLEEP_WINDOW
//
// # Cross-References
//
// Related packages:
//   - [base.Validator] - Validation framework
//   - [base.Percentage] - Percentage type for failure thresholds
//   - [dlq] - Dead letter queue for failed requests
//
// Related documentation:
//   - Circuit Breaker Pattern: https://martinfowler.com/bliki/CircuitBreaker.html
//   - Azure Patterns: https://learn.microsoft.com/en-us/azure/architecture/patterns/circuit-breaker
//   - Resilience4j: https://resilience4j.readme.io/docs/circuitbreaker
//   - Netflix Hystrix: https://github.com/Netflix/Hystrix/wiki/How-it-Works#CircuitBreaker
//
// # Files in This Package
//
// constants.go:
//   - CIRCUIT_BREAKER_PREFIX for environment variables
//   - Default values and thresholds
//   - Min/max bounds for validation
//
// section.go:
//   - [Config] struct with circuit breaker parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading configuration
//   - [Config.Validate] for validation
package circuitbreaker
