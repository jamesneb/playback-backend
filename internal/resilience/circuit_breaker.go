package resilience

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// CircuitBreakerState represents the current state of the circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
	ErrTooManyRequests    = errors.New("too many requests")
)

// CircuitBreaker implements a circuit breaker pattern with metrics
type CircuitBreaker struct {
	name          string
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	readyToTrip   func(counts Counts) bool
	onStateChange func(name string, from, to CircuitBreakerState)

	mutex      sync.Mutex
	state      CircuitBreakerState
	generation uint64
	counts     Counts
	expiry     time.Time

	// Metrics (atomic counters for thread-safe metrics collection)
	totalRequestsCount  int64 // Total requests processed
	totalSuccessCount   int64 // Total successful requests
	totalFailureCount   int64 // Total failed requests
	totalRejectedCount  int64 // Total requests rejected due to open circuit
	lastStateChangeTime int64 // Unix timestamp of last state change
	timeSpentClosed     int64 // Total time spent in closed state (nanoseconds)
	timeSpentOpen       int64 // Total time spent in open state (nanoseconds)
	timeSpentHalfOpen   int64 // Total time spent in half-open state (nanoseconds)
}

// Counts holds the numbers of requests and their successes/failures
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// Settings for circuit breaker configuration
type Settings struct {
	Name          string
	MaxRequests   uint32        // Max requests allowed to pass through when half-open
	Interval      time.Duration // Cyclic period in closed state for clearing counts
	Timeout       time.Duration // Period of open state
	ReadyToTrip   func(counts Counts) bool
	OnStateChange func(name string, from, to CircuitBreakerState)
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(st Settings) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:          st.Name,
		maxRequests:   st.MaxRequests,
		interval:      st.Interval,
		timeout:       st.Timeout,
		readyToTrip:   st.ReadyToTrip,
		onStateChange: st.OnStateChange,
	}

	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}

	if cb.interval <= 0 {
		cb.interval = time.Duration(0)
	}

	if cb.timeout <= 0 {
		cb.timeout = 60 * time.Second
	}

	if cb.readyToTrip == nil {
		cb.readyToTrip = func(counts Counts) bool {
			return counts.ConsecutiveFailures > 5
		}
	}

	if cb.onStateChange == nil {
		cb.onStateChange = func(name string, from, to CircuitBreakerState) {
			logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", cb.stateString(from)),
				zap.String("to", cb.stateString(to)))
		}
	}

	cb.toNewGeneration(time.Now())

	// Initialize metrics
	atomic.StoreInt64(&cb.lastStateChangeTime, time.Now().UnixNano())

	return cb
}

// Execute runs the given request if the circuit breaker accepts it
func (cb *CircuitBreaker) Execute(req func() (interface{}, error)) (interface{}, error) {
	generation, err := cb.beforeRequest()
	if err != nil {
		return nil, err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result, err := req()
	cb.afterRequest(generation, err == nil)
	return result, err
}

// Call is a wrapper around Execute that doesn't return a result
func (cb *CircuitBreaker) Call(fn func() error) error {
	_, err := cb.Execute(func() (interface{}, error) {
		return nil, fn()
	})
	return err
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	// Update metrics
	atomic.AddInt64(&cb.totalRequestsCount, 1)

	if state == StateOpen {
		atomic.AddInt64(&cb.totalRejectedCount, 1)
		return generation, ErrCircuitBreakerOpen
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		atomic.AddInt64(&cb.totalRejectedCount, 1)
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

func (cb *CircuitBreaker) onSuccess(state CircuitBreakerState, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveSuccesses++
	cb.counts.ConsecutiveFailures = 0

	// Update success metrics
	atomic.AddInt64(&cb.totalSuccessCount, 1)

	if state == StateHalfOpen {
		cb.setState(StateClosed, now)
	}
}

func (cb *CircuitBreaker) onFailure(state CircuitBreakerState, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveFailures++
	cb.counts.ConsecutiveSuccesses = 0

	// Update failure metrics
	atomic.AddInt64(&cb.totalFailureCount, 1)

	if cb.readyToTrip(cb.counts) {
		cb.setState(StateOpen, now)
	}
}

func (cb *CircuitBreaker) currentState(now time.Time) (CircuitBreakerState, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

func (cb *CircuitBreaker) setState(state CircuitBreakerState, now time.Time) {
	if cb.state == state {
		return
	}

	// Track time spent in previous state
	lastStateChangeTime := atomic.LoadInt64(&cb.lastStateChangeTime)
	if lastStateChangeTime > 0 {
		timeDiff := now.UnixNano() - lastStateChangeTime
		switch cb.state {
		case StateClosed:
			atomic.AddInt64(&cb.timeSpentClosed, timeDiff)
		case StateOpen:
			atomic.AddInt64(&cb.timeSpentOpen, timeDiff)
		case StateHalfOpen:
			atomic.AddInt64(&cb.timeSpentHalfOpen, timeDiff)
		}
	}

	prev := cb.state
	cb.state = state
	cb.toNewGeneration(now)

	// Update state change timestamp
	atomic.StoreInt64(&cb.lastStateChangeTime, now.UnixNano())

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}

func (cb *CircuitBreaker) stateString(state CircuitBreakerState) string {
	switch state {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns a copy of the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.counts
}

// CircuitBreakerMetrics holds comprehensive metrics for the circuit breaker
type CircuitBreakerMetrics struct {
	Name              string              `json:"name"`
	State             CircuitBreakerState `json:"state"`
	StateString       string              `json:"state_string"`
	TotalRequests     int64               `json:"total_requests"`
	TotalSuccesses    int64               `json:"total_successes"`
	TotalFailures     int64               `json:"total_failures"`
	TotalRejected     int64               `json:"total_rejected"`
	SuccessRate       float64             `json:"success_rate"`
	FailureRate       float64             `json:"failure_rate"`
	RejectionRate     float64             `json:"rejection_rate"`
	TimeSpentClosed   time.Duration       `json:"time_spent_closed_ns"`
	TimeSpentOpen     time.Duration       `json:"time_spent_open_ns"`
	TimeSpentHalfOpen time.Duration       `json:"time_spent_half_open_ns"`
	LastStateChange   time.Time           `json:"last_state_change"`
	CurrentCounts     Counts              `json:"current_counts"`
}

// Metrics returns comprehensive metrics for the circuit breaker
func (cb *CircuitBreaker) Metrics() CircuitBreakerMetrics {
	cb.mutex.Lock()
	currentState := cb.state
	currentCounts := cb.counts
	cb.mutex.Unlock()

	// Load atomic metrics
	totalRequests := atomic.LoadInt64(&cb.totalRequestsCount)
	totalSuccesses := atomic.LoadInt64(&cb.totalSuccessCount)
	totalFailures := atomic.LoadInt64(&cb.totalFailureCount)
	totalRejected := atomic.LoadInt64(&cb.totalRejectedCount)
	timeSpentClosed := atomic.LoadInt64(&cb.timeSpentClosed)
	timeSpentOpen := atomic.LoadInt64(&cb.timeSpentOpen)
	timeSpentHalfOpen := atomic.LoadInt64(&cb.timeSpentHalfOpen)
	lastStateChangeTime := atomic.LoadInt64(&cb.lastStateChangeTime)

	// Calculate rates
	var successRate, failureRate, rejectionRate float64
	if totalRequests > 0 {
		successRate = float64(totalSuccesses) / float64(totalRequests)
		failureRate = float64(totalFailures) / float64(totalRequests)
		rejectionRate = float64(totalRejected) / float64(totalRequests)
	}

	return CircuitBreakerMetrics{
		Name:              cb.name,
		State:             currentState,
		StateString:       cb.stateString(currentState),
		TotalRequests:     totalRequests,
		TotalSuccesses:    totalSuccesses,
		TotalFailures:     totalFailures,
		TotalRejected:     totalRejected,
		SuccessRate:       successRate,
		FailureRate:       failureRate,
		RejectionRate:     rejectionRate,
		TimeSpentClosed:   time.Duration(timeSpentClosed),
		TimeSpentOpen:     time.Duration(timeSpentOpen),
		TimeSpentHalfOpen: time.Duration(timeSpentHalfOpen),
		LastStateChange:   time.Unix(0, lastStateChangeTime),
		CurrentCounts:     currentCounts,
	}
}

// ResetMetrics resets all metrics counters (useful for testing or periodic resets)
func (cb *CircuitBreaker) ResetMetrics() {
	atomic.StoreInt64(&cb.totalRequestsCount, 0)
	atomic.StoreInt64(&cb.totalSuccessCount, 0)
	atomic.StoreInt64(&cb.totalFailureCount, 0)
	atomic.StoreInt64(&cb.totalRejectedCount, 0)
	atomic.StoreInt64(&cb.timeSpentClosed, 0)
	atomic.StoreInt64(&cb.timeSpentOpen, 0)
	atomic.StoreInt64(&cb.timeSpentHalfOpen, 0)
	atomic.StoreInt64(&cb.lastStateChangeTime, time.Now().UnixNano())
}
