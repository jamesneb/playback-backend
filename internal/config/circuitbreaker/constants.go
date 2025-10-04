package circuitbreaker

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	CIRCUIT_BREAKER_PREFIX = "CIRCUIT_BREAKER_"
)

// Timeout constants
const (
	MIN_TIMEOUT     = 100 * time.Millisecond
	MAX_TIMEOUT     = 1 * time.Minute
	DEFAULT_TIMEOUT = 5 * time.Second
)

// Concurrent request constants
const (
	MIN_CONCURRENT_REQUESTS     = 1
	MAX_CONCURRENT_REQUESTS     = 10000
	DEFAULT_CONCURRENT_REQUESTS = 100
)

// Failure percentage threshold to open circuit
var (
	MIN_FAILURE_RATE_THRESHOLD, _     = base.NewPercentage(0)
	MAX_FAILURE_RATE_THRESHOLD, _     = base.NewPercentage(100)
	DEFAULT_FAILURE_RATE_THRESHOLD, _ = base.NewPercentage(50)
)

// Consecutive successes needed to close circuit from half-open
const (
	MIN_CONSECUTIVE_SUCCESSES     = 1
	MAX_CONSECUTIVE_SUCCESSES     = 100
	DEFAULT_CONSECUTIVE_SUCCESSES = 2
)

// Sleep window constants
const (
	MIN_SLEEP_WINDOW     = 1 * time.Second
	MAX_SLEEP_WINDOW     = 5 * time.Minute
	DEFAULT_SLEEP_WINDOW = 10 * time.Second
)

// Request threshold before circuit breaker activates
const (
	MIN_REQUEST_THRESHOLD     = 1
	MAX_REQUEST_THRESHOLD     = 1000
	DEFAULT_REQUEST_THRESHOLD = 10
)

// Half-open state constants
// Half-open is a recovery testing state where the circuit allows limited requests
// to determine if the downstream service has recovered
const (
	MIN_HALF_OPEN_REQUESTS     = 1
	MAX_HALF_OPEN_REQUESTS     = 100
	DEFAULT_HALF_OPEN_REQUESTS = 5
)

// Default values
const (
	DEFAULT_ENABLED = true
)
