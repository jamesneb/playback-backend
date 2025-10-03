package chaos

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// Infrastructure adapters for chaos experiments

// ClickHouseChaosAdapter provides chaos testing operations on ClickHouse
type ClickHouseChaosAdapter interface {
	// Connection management
	DisconnectTemporarily(duration time.Duration) error
	SlowDownQueries(latencyMs int, duration time.Duration) error
	InjectQueryErrors(errorRate float64, duration time.Duration) error
	FillDiskSpace(percentage int) error
	RestoreNormalOperation() error

	// Monitoring
	IsHealthy(ctx context.Context) bool
	GetConnectionCount() (int, error)
	GetQueryLatency() (time.Duration, error)
}

// RedisChaosAdapter provides chaos testing operations on Redis
type RedisChaosAdapter interface {
	// Connection chaos
	DisconnectTemporarily(duration time.Duration) error
	SlowDownOperations(latencyMs int, duration time.Duration) error
	InjectOperationErrors(errorRate float64, duration time.Duration) error
	FlushDatabase() error
	FillMemory(percentage int) error
	RestoreNormalOperation() error

	// Monitoring
	IsHealthy(ctx context.Context) bool
	GetMemoryUsage() (int64, error)
	GetConnectionCount() (int, error)
}

// KinesisChaosAdapter provides chaos testing operations on Kinesis
type KinesisChaosAdapter interface {
	// Stream chaos
	ThrottleWrites(percentage int, duration time.Duration) error
	InjectWriteErrors(errorRate float64, duration time.Duration) error
	DelayDelivery(latencyMs int, duration time.Duration) error
	TemporaryUnavailable(duration time.Duration) error
	RestoreNormalOperation() error

	// Monitoring
	IsHealthy(ctx context.Context) bool
	GetThroughput() (int64, error)
	GetErrorRate() (float64, error)
}

// BaselineLoadTester establishes system baseline performance
type BaselineLoadTester struct {
	target       ChaosTarget
	requestCount int
	rps          int
	httpClient   *http.Client
}

// Run executes baseline load test
func (blt *BaselineLoadTester) Run(ctx context.Context) (*BaselineMetrics, error) {
	if blt.httpClient == nil {
		blt.httpClient = &http.Client{
			Timeout: 10 * time.Second,
		}
	}

	var responseTimes []time.Duration
	var failedRequests int64

	interval := time.Second / time.Duration(blt.rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	startTime := time.Now()
	requestsCompleted := 0

	for requestsCompleted < blt.requestCount {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			reqStart := time.Now()

			// Make request to health endpoint
			resp, err := blt.httpClient.Get(blt.target.GetHealthEndpoint())
			responseTime := time.Since(reqStart)

			if err != nil || (resp != nil && resp.StatusCode >= 400) {
				failedRequests++
			}

			if resp != nil {
				if err := resp.Body.Close(); err != nil {
					fmt.Printf("Failed to close response body: %v\n", err)
				}
			}

			responseTimes = append(responseTimes, responseTime)
			requestsCompleted++
		}
	}

	// Calculate metrics
	var totalTime time.Duration
	for _, rt := range responseTimes {
		totalTime += rt
	}

	sortDurations(responseTimes)

	return &BaselineMetrics{
		AvgResponseTime: totalTime / time.Duration(len(responseTimes)),
		P99ResponseTime: percentile(responseTimes, 0.99),
		ErrorRate:       float64(failedRequests) / float64(len(responseTimes)),
		RequestCount:    int64(len(responseTimes)),
		Timestamp:       startTime,
	}, nil
}

// HealthTester provides system health checking capabilities
type HealthTester struct {
	target     ChaosTarget
	httpClient *http.Client
}

// IsHealthy checks if the system is healthy
func (ht *HealthTester) IsHealthy(ctx context.Context) (bool, int) {
	if ht.httpClient == nil {
		ht.httpClient = &http.Client{
			Timeout: 5 * time.Second,
		}
	}

	// Check primary health endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", ht.target.GetHealthEndpoint(), nil)
	if err != nil {
		return false, 0
	}

	resp, err := ht.httpClient.Do(req)
	if err != nil {
		return false, 0
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Failed to close response body: %v\n", err)
		}
	}()

	// Consider 2xx status codes as healthy
	healthy := resp.StatusCode >= 200 && resp.StatusCode < 300
	return healthy, resp.StatusCode
}

// DefaultChaosTarget provides a default implementation of ChaosTarget
type DefaultChaosTarget struct {
	BaseURL           string
	HealthEndpoint    string
	MetricsEndpoint   string
	ServiceEndpoints  []string
	ClickHouseAdapter ClickHouseChaosAdapter
	RedisAdapter      RedisChaosAdapter
	KinesisAdapter    KinesisChaosAdapter
}

func (dct *DefaultChaosTarget) GetHealthEndpoint() string {
	if dct.HealthEndpoint != "" {
		return dct.BaseURL + dct.HealthEndpoint
	}
	return dct.BaseURL + "/health"
}

func (dct *DefaultChaosTarget) GetMetricsEndpoint() string {
	if dct.MetricsEndpoint != "" {
		return dct.BaseURL + dct.MetricsEndpoint
	}
	return dct.BaseURL + "/metrics"
}

func (dct *DefaultChaosTarget) GetServiceEndpoints() []string {
	if len(dct.ServiceEndpoints) > 0 {
		endpoints := make([]string, len(dct.ServiceEndpoints))
		for i, endpoint := range dct.ServiceEndpoints {
			endpoints[i] = dct.BaseURL + endpoint
		}
		return endpoints
	}
	return []string{
		dct.BaseURL + "/api/v1/traces",
		dct.BaseURL + "/api/v1/metrics",
		dct.BaseURL + "/api/v1/logs",
	}
}

func (dct *DefaultChaosTarget) GetClickHouseConnection() ClickHouseChaosAdapter {
	return dct.ClickHouseAdapter
}

func (dct *DefaultChaosTarget) GetRedisConnection() RedisChaosAdapter {
	return dct.RedisAdapter
}

func (dct *DefaultChaosTarget) GetKinesisConnection() KinesisChaosAdapter {
	return dct.KinesisAdapter
}

// MockClickHouseAdapter provides a mock implementation for testing
type MockClickHouseAdapter struct {
	healthy       bool
	latency       time.Duration
	errorRate     float64
	connectionCnt int
}

func NewMockClickHouseAdapter() *MockClickHouseAdapter {
	return &MockClickHouseAdapter{
		healthy:       true,
		latency:       50 * time.Millisecond,
		errorRate:     0.0,
		connectionCnt: 10,
	}
}

func (mca *MockClickHouseAdapter) DisconnectTemporarily(duration time.Duration) error {
	mca.healthy = false
	go func() {
		time.Sleep(duration)
		mca.healthy = true
	}()
	return nil
}

func (mca *MockClickHouseAdapter) SlowDownQueries(latencyMs int, duration time.Duration) error {
	originalLatency := mca.latency
	mca.latency = time.Duration(latencyMs) * time.Millisecond
	go func() {
		time.Sleep(duration)
		mca.latency = originalLatency
	}()
	return nil
}

func (mca *MockClickHouseAdapter) InjectQueryErrors(errorRate float64, duration time.Duration) error {
	originalErrorRate := mca.errorRate
	mca.errorRate = errorRate
	go func() {
		time.Sleep(duration)
		mca.errorRate = originalErrorRate
	}()
	return nil
}

func (mca *MockClickHouseAdapter) FillDiskSpace(percentage int) error {
	// Mock implementation - in reality this would affect disk space
	if percentage > 90 {
		mca.healthy = false
	}
	return nil
}

func (mca *MockClickHouseAdapter) RestoreNormalOperation() error {
	mca.healthy = true
	mca.latency = 50 * time.Millisecond
	mca.errorRate = 0.0
	return nil
}

func (mca *MockClickHouseAdapter) IsHealthy(ctx context.Context) bool {
	return mca.healthy
}

func (mca *MockClickHouseAdapter) GetConnectionCount() (int, error) {
	return mca.connectionCnt, nil
}

func (mca *MockClickHouseAdapter) GetQueryLatency() (time.Duration, error) {
	return mca.latency, nil
}

// MockRedisAdapter provides a mock implementation for testing
type MockRedisAdapter struct {
	healthy       bool
	latency       time.Duration
	errorRate     float64
	memoryUsage   int64
	connectionCnt int
}

func NewMockRedisAdapter() *MockRedisAdapter {
	return &MockRedisAdapter{
		healthy:       true,
		latency:       10 * time.Millisecond,
		errorRate:     0.0,
		memoryUsage:   1024 * 1024, // 1MB
		connectionCnt: 5,
	}
}

func (mra *MockRedisAdapter) DisconnectTemporarily(duration time.Duration) error {
	mra.healthy = false
	go func() {
		time.Sleep(duration)
		mra.healthy = true
	}()
	return nil
}

func (mra *MockRedisAdapter) SlowDownOperations(latencyMs int, duration time.Duration) error {
	originalLatency := mra.latency
	mra.latency = time.Duration(latencyMs) * time.Millisecond
	go func() {
		time.Sleep(duration)
		mra.latency = originalLatency
	}()
	return nil
}

func (mra *MockRedisAdapter) InjectOperationErrors(errorRate float64, duration time.Duration) error {
	originalErrorRate := mra.errorRate
	mra.errorRate = errorRate
	go func() {
		time.Sleep(duration)
		mra.errorRate = originalErrorRate
	}()
	return nil
}

func (mra *MockRedisAdapter) FlushDatabase() error {
	// Mock implementation
	return nil
}

func (mra *MockRedisAdapter) FillMemory(percentage int) error {
	// Mock implementation
	if percentage > 95 {
		mra.healthy = false
	}
	return nil
}

func (mra *MockRedisAdapter) RestoreNormalOperation() error {
	mra.healthy = true
	mra.latency = 10 * time.Millisecond
	mra.errorRate = 0.0
	return nil
}

func (mra *MockRedisAdapter) IsHealthy(ctx context.Context) bool {
	return mra.healthy
}

func (mra *MockRedisAdapter) GetMemoryUsage() (int64, error) {
	return mra.memoryUsage, nil
}

func (mra *MockRedisAdapter) GetConnectionCount() (int, error) {
	return mra.connectionCnt, nil
}

// MockKinesisAdapter provides a mock implementation for testing
type MockKinesisAdapter struct {
	healthy    bool
	throughput int64
	errorRate  float64
}

func NewMockKinesisAdapter() *MockKinesisAdapter {
	return &MockKinesisAdapter{
		healthy:    true,
		throughput: 1000, // Records per second
		errorRate:  0.0,
	}
}

func (mka *MockKinesisAdapter) ThrottleWrites(percentage int, duration time.Duration) error {
	originalThroughput := mka.throughput
	mka.throughput = originalThroughput * int64(100-percentage) / 100
	go func() {
		time.Sleep(duration)
		mka.throughput = originalThroughput
	}()
	return nil
}

func (mka *MockKinesisAdapter) InjectWriteErrors(errorRate float64, duration time.Duration) error {
	originalErrorRate := mka.errorRate
	mka.errorRate = errorRate
	go func() {
		time.Sleep(duration)
		mka.errorRate = originalErrorRate
	}()
	return nil
}

func (mka *MockKinesisAdapter) DelayDelivery(latencyMs int, duration time.Duration) error {
	// Mock implementation for delivery delay
	return nil
}

func (mka *MockKinesisAdapter) TemporaryUnavailable(duration time.Duration) error {
	mka.healthy = false
	go func() {
		time.Sleep(duration)
		mka.healthy = true
	}()
	return nil
}

func (mka *MockKinesisAdapter) RestoreNormalOperation() error {
	mka.healthy = true
	mka.throughput = 1000
	mka.errorRate = 0.0
	return nil
}

func (mka *MockKinesisAdapter) IsHealthy(ctx context.Context) bool {
	return mka.healthy
}

func (mka *MockKinesisAdapter) GetThroughput() (int64, error) {
	return mka.throughput, nil
}

func (mka *MockKinesisAdapter) GetErrorRate() (float64, error) {
	return mka.errorRate, nil
}

// Utility functions for performance calculations

// sortDurations sorts duration slice in place
func sortDurations(durations []time.Duration) {
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
}

// percentile calculates the given percentile from sorted durations
func percentile(sortedDurations []time.Duration, p float64) time.Duration {
	if len(sortedDurations) == 0 {
		return 0
	}
	if len(sortedDurations) == 1 {
		return sortedDurations[0]
	}

	index := p * float64(len(sortedDurations)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sortedDurations) {
		return sortedDurations[len(sortedDurations)-1]
	}

	weight := index - float64(lower)
	return time.Duration(float64(sortedDurations[lower])*(1-weight) + float64(sortedDurations[upper])*weight)
}

// MonitoringData contains continuous monitoring results with zero-copy optimizations
type MonitoringData struct {
	AvgResponseTime time.Duration
	P99ResponseTime time.Duration
	ErrorRate       float64
	RequestCount    int64
	FailedRequests  int64
	Samples         []MonitoringSample
}

// MonitoringSample represents a single monitoring sample
type MonitoringSample struct {
	Timestamp    time.Time
	ResponseTime time.Duration
	Success      bool
	StatusCode   int
}

// ContinuousMonitor monitors system behavior during experiments with efficient sampling
type ContinuousMonitor struct {
	target         ChaosTarget
	sampleInterval time.Duration
}

// Run executes continuous monitoring until context cancellation
func (cm *ContinuousMonitor) Run(ctx context.Context) *MonitoringData {
	// Pre-allocate slice with estimated capacity for better performance
	samples := make([]MonitoringSample, 0, 64)
	ticker := time.NewTicker(cm.sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return cm.analyzesamples(samples)
		case <-ticker.C:
			sample := cm.takeSample(ctx)
			samples = append(samples, sample)
		}
	}
}

// takeSample takes a single monitoring sample with minimal allocations
func (cm *ContinuousMonitor) takeSample(ctx context.Context) MonitoringSample {
	start := time.Now()

	// Simple HTTP health check as monitoring probe
	healthTester := &HealthTester{target: cm.target}
	healthy, statusCode := healthTester.IsHealthy(ctx)

	return MonitoringSample{
		Timestamp:    start,
		ResponseTime: time.Since(start),
		Success:      healthy,
		StatusCode:   statusCode,
	}
}

// analyzesamples analyzes collected monitoring samples with performance optimizations
func (cm *ContinuousMonitor) analyzesamples(samples []MonitoringSample) *MonitoringData {
	if len(samples) == 0 {
		return &MonitoringData{}
	}

	var totalResponseTime time.Duration
	var failedCount int64

	// Pre-allocate response times slice
	responseTimes := make([]time.Duration, 0, len(samples))

	// Single pass through samples for efficiency
	for i := range samples {
		sample := &samples[i]
		totalResponseTime += sample.ResponseTime
		responseTimes = append(responseTimes, sample.ResponseTime)

		if !sample.Success {
			failedCount++
		}
	}

	// Sort for percentile calculation using efficient sort
	sortDurations(responseTimes)

	return &MonitoringData{
		AvgResponseTime: totalResponseTime / time.Duration(len(samples)),
		P99ResponseTime: percentile(responseTimes, 0.99),
		ErrorRate:       float64(failedCount) / float64(len(samples)),
		RequestCount:    int64(len(samples)),
		FailedRequests:  failedCount,
		Samples:         samples,
	}
}
