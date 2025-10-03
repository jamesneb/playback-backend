package chaos

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// ChaosExperiment represents a single chaos engineering experiment
type ChaosExperiment interface {
	Name() string
	Description() string
	Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error)
	Cleanup(ctx context.Context, target ChaosTarget) error
}

// ChaosTarget represents the system under test
type ChaosTarget interface {
	// Service endpoints
	GetHealthEndpoint() string
	GetMetricsEndpoint() string
	GetServiceEndpoints() []string

	// Infrastructure access
	GetClickHouseConnection() ClickHouseChaosAdapter
	GetRedisConnection() RedisChaosAdapter
	GetKinesisConnection() KinesisChaosAdapter
}

// ChaosResult contains the results of a chaos experiment
type ChaosResult struct {
	ExperimentName  string            `json:"experiment_name"`
	Duration        time.Duration     `json:"duration"`
	StartTime       time.Time         `json:"start_time"`
	EndTime         time.Time         `json:"end_time"`
	Success         bool              `json:"success"`
	ErrorRate       float64           `json:"error_rate"`
	AvgResponseTime time.Duration     `json:"avg_response_time"`
	P99ResponseTime time.Duration     `json:"p99_response_time"`
	RequestCount    int64             `json:"request_count"`
	FailedRequests  int64             `json:"failed_requests"`
	RecoveryTime    time.Duration     `json:"recovery_time"`
	SteadyStateOK   bool              `json:"steady_state_ok"`
	Observations    []string          `json:"observations"`
	Metrics         map[string]string `json:"metrics"`
}

// BaselineMetrics stores system baseline performance
type BaselineMetrics struct {
	AvgResponseTime time.Duration
	P99ResponseTime time.Duration
	ErrorRate       float64
	RequestCount    int64
	Timestamp       time.Time
}

// ChaosRunner orchestrates chaos engineering experiments
type ChaosRunner struct {
	target      ChaosTarget
	experiments []ChaosExperiment

	// Configuration
	experimentInterval time.Duration
	steadyStateTimeout time.Duration
	maxExperimentTime  time.Duration
	baselineRequests   int
	baselineRPS        int

	// State tracking
	baselineMetrics *BaselineMetrics
	results         []*ChaosResult
	isRunning       int32

	// Synchronization
	mu sync.RWMutex
}

// NewChaosRunner creates a new chaos engineering test runner
func NewChaosRunner(target ChaosTarget) *ChaosRunner {
	return &ChaosRunner{
		target:             target,
		experiments:        make([]ChaosExperiment, 0, 10),
		experimentInterval: 2 * time.Minute,
		steadyStateTimeout: 30 * time.Second,
		maxExperimentTime:  5 * time.Minute,
		baselineRequests:   1000,
		baselineRPS:        20,
		results:            make([]*ChaosResult, 0),
	}
}

// AddExperiment adds a chaos experiment to the test suite
func (cr *ChaosRunner) AddExperiment(experiment ChaosExperiment) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.experiments = append(cr.experiments, experiment)
}

// RunAllExperiments executes all registered chaos experiments
func (cr *ChaosRunner) RunAllExperiments(ctx context.Context) ([]*ChaosResult, error) {
	if !atomic.CompareAndSwapInt32(&cr.isRunning, 0, 1) {
		return nil, fmt.Errorf("chaos experiments already running")
	}
	defer atomic.StoreInt32(&cr.isRunning, 0)

	logger.Info("Starting chaos engineering experiments",
		zap.Int("experiment_count", len(cr.experiments)),
		zap.Duration("interval", cr.experimentInterval))

	// Establish baseline metrics
	if err := cr.establishBaseline(ctx); err != nil {
		return nil, fmt.Errorf("failed to establish baseline: %w", err)
	}

	// Run experiments sequentially with recovery periods
	for i, experiment := range cr.experiments {
		logger.Info("Running chaos experiment",
			zap.String("name", experiment.Name()),
			zap.Int("sequence", i+1),
			zap.Int("total", len(cr.experiments)))

		result, err := cr.runSingleExperiment(ctx, experiment)
		if err != nil {
			logger.Error("Chaos experiment failed",
				zap.String("name", experiment.Name()),
				zap.Error(err))

			result = &ChaosResult{
				ExperimentName: experiment.Name(),
				Success:        false,
				Observations:   []string{fmt.Sprintf("Experiment failed: %v", err)},
			}
		}

		cr.mu.Lock()
		cr.results = append(cr.results, result)
		cr.mu.Unlock()

		// Recovery period between experiments
		if i < len(cr.experiments)-1 {
			logger.Info("Recovery period between experiments",
				zap.Duration("duration", cr.experimentInterval))
			time.Sleep(cr.experimentInterval)
		}
	}

	cr.mu.RLock()
	results := make([]*ChaosResult, len(cr.results))
	copy(results, cr.results)
	cr.mu.RUnlock()

	logger.Info("Completed chaos engineering experiments",
		zap.Int("total_experiments", len(results)),
		zap.Int("successful", cr.countSuccessfulExperiments(results)))

	return results, nil
}

// establishBaseline captures system baseline performance with minimal allocations
func (cr *ChaosRunner) establishBaseline(ctx context.Context) error {
	logger.Info("Establishing system baseline",
		zap.Int("requests", cr.baselineRequests),
		zap.Int("rps", cr.baselineRPS))

	loadTester := &BaselineLoadTester{
		target:       cr.target,
		requestCount: cr.baselineRequests,
		rps:          cr.baselineRPS,
	}

	baseline, err := loadTester.Run(ctx)
	if err != nil {
		return fmt.Errorf("baseline load test failed: %w", err)
	}

	cr.baselineMetrics = baseline
	logger.Info("Baseline established",
		zap.Duration("avg_response_time", baseline.AvgResponseTime),
		zap.Duration("p99_response_time", baseline.P99ResponseTime),
		zap.Float64("error_rate", baseline.ErrorRate))

	return nil
}

// runSingleExperiment executes a single chaos experiment with efficient monitoring
func (cr *ChaosRunner) runSingleExperiment(ctx context.Context, experiment ChaosExperiment) (*ChaosResult, error) {
	startTime := time.Now()

	expCtx, cancel := context.WithTimeout(ctx, cr.maxExperimentTime)
	defer cancel()

	// Fast health check before experiment
	healthTester := &HealthTester{target: cr.target}
	if healthy, _ := healthTester.IsHealthy(expCtx); !healthy {
		return nil, fmt.Errorf("system not in steady state before experiment")
	}

	// Start monitoring with buffered channel for performance
	monitorCtx, monitorCancel := context.WithCancel(expCtx)
	monitorChan := make(chan *MonitoringData, 1)

	go func() {
		defer close(monitorChan)
		monitor := &ContinuousMonitor{
			target:         cr.target,
			sampleInterval: 1 * time.Second,
		}
		monitorChan <- monitor.Run(monitorCtx)
	}()

	// Run the experiment
	result, err := experiment.Run(expCtx, cr.target)
	if err != nil {
		monitorCancel()
		return nil, fmt.Errorf("experiment execution failed: %w", err)
	}

	// Stop monitoring and collect results
	monitorCancel()
	monitoringData := <-monitorChan

	// Cleanup with error handling
	if cleanupErr := experiment.Cleanup(expCtx, cr.target); cleanupErr != nil {
		logger.Warn("Experiment cleanup failed",
			zap.String("experiment", experiment.Name()),
			zap.Error(cleanupErr))
	}

	// Wait for recovery with timeout
	recoveryStart := time.Now()
	recovered := cr.waitForRecovery(expCtx, 30*time.Second)
	if !recovered {
		if result.Observations == nil {
			result.Observations = make([]string, 0, 1)
		}
		result.Observations = append(result.Observations,
			"System did not recover to steady state within timeout")
	}

	// Enrich result with monitoring data - zero allocation where possible
	result.Duration = time.Since(startTime)
	result.StartTime = startTime
	result.EndTime = time.Now()
	result.RecoveryTime = time.Since(recoveryStart)

	if monitoringData != nil {
		result.ErrorRate = monitoringData.ErrorRate
		result.AvgResponseTime = monitoringData.AvgResponseTime
		result.P99ResponseTime = monitoringData.P99ResponseTime
		result.RequestCount = monitoringData.RequestCount
		result.FailedRequests = monitoringData.FailedRequests
	}

	return result, nil
}

// waitForRecovery waits for system recovery with efficient polling
func (cr *ChaosRunner) waitForRecovery(ctx context.Context, timeout time.Duration) bool {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	healthTester := &HealthTester{target: cr.target}

	for {
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return false
		case <-ticker.C:
			if healthy, _ := healthTester.IsHealthy(ctx); healthy {
				return true
			}
		}
	}
}

// countSuccessfulExperiments counts experiments that passed
func (cr *ChaosRunner) countSuccessfulExperiments(results []*ChaosResult) int {
	count := 0
	for _, result := range results {
		if result.Success {
			count++
		}
	}
	return count
}
