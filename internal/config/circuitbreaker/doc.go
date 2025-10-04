// Package circuitbreaker defines configuration for circuit breaker resilience patterns.
//
// This package provides configuration for protecting downstream services from cascading
// failures using the circuit breaker pattern. It includes timeout, failure threshold,
// and recovery settings.
//
// # Circuit Breaker States
//
// The circuit breaker operates in three states:
//   - Closed: Normal operation, requests pass through
//   - Open: Failure threshold exceeded, requests fail fast
//   - Half-Open: Testing if service recovered, limited requests pass through
//
// # Configuration
//
// Timeout settings:
//   - Timeout: Maximum duration for a single request
//
// Failure detection:
//   - MaxConcurrentRequests: Maximum concurrent requests in half-open state
//   - ErrorThreshold: Percentage of failed requests to open circuit (0-100)
//   - SuccessThreshold: Consecutive successes needed to close circuit
//
// Recovery timing:
//   - SleepWindow: Duration to wait before entering half-open state
//   - HalfOpenMaxRequests: Max requests to test during half-open state
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// CIRCUIT_BREAKER_ prefix:
//
//	CIRCUIT_BREAKER_TIMEOUT=5s
//	CIRCUIT_BREAKER_MAX_CONCURRENT_REQUESTS=100
//	CIRCUIT_BREAKER_ERROR_THRESHOLD=50
//	CIRCUIT_BREAKER_SUCCESS_THRESHOLD=2
//	CIRCUIT_BREAKER_SLEEP_WINDOW=10s
//	CIRCUIT_BREAKER_HALF_OPEN_MAX_REQUESTS=5
//
// # Files in This Package
//
// constants.go:
//   - CIRCUIT_BREAKER_PREFIX for environment variable namespacing
//   - Default values (timeouts, thresholds, windows)
//   - Min/max bounds for validation
//
// section.go:
//   - Config struct with circuit breaker parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package circuitbreaker
