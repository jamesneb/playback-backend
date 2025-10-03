package chaos

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Concrete chaos experiments implementing the ChaosExperiment interface

// DatabaseLatencyExperiment injects latency into database operations
type DatabaseLatencyExperiment struct {
	latencyMs int
	duration  time.Duration
}

func NewDatabaseLatencyExperiment(latencyMs int, duration time.Duration) *DatabaseLatencyExperiment {
	return &DatabaseLatencyExperiment{
		latencyMs: latencyMs,
		duration:  duration,
	}
}

func (dle *DatabaseLatencyExperiment) Name() string {
	return "database-latency-injection"
}

func (dle *DatabaseLatencyExperiment) Description() string {
	return fmt.Sprintf("Inject %dms latency into database queries for %v", dle.latencyMs, dle.duration)
}

func (dle *DatabaseLatencyExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: dle.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting database latency experiment",
		zap.Int("latency_ms", dle.latencyMs),
		zap.Duration("duration", dle.duration))

	// Apply latency to ClickHouse
	clickhouse := target.GetClickHouseConnection()
	if clickhouse != nil {
		if err := clickhouse.SlowDownQueries(dle.latencyMs, dle.duration); err != nil {
			return nil, fmt.Errorf("failed to inject ClickHouse latency: %w", err)
		}
		result.Observations = append(result.Observations,
			fmt.Sprintf("Injected %dms latency into ClickHouse queries", dle.latencyMs))
	}

	// Apply latency to Redis
	redis := target.GetRedisConnection()
	if redis != nil {
		if err := redis.SlowDownOperations(dle.latencyMs/2, dle.duration); err != nil {
			return nil, fmt.Errorf("failed to inject Redis latency: %w", err)
		}
		result.Observations = append(result.Observations,
			fmt.Sprintf("Injected %dms latency into Redis operations", dle.latencyMs/2))
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(dle.duration):
		result.Success = true
		result.Observations = append(result.Observations, "Database latency experiment completed successfully")
	}

	return result, nil
}

func (dle *DatabaseLatencyExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	// Restore normal operation
	if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
		if err := clickhouse.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore ClickHouse normal operation: %v\n", err)
		}
	}
	if redis := target.GetRedisConnection(); redis != nil {
		if err := redis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Redis normal operation: %v\n", err)
		}
	}
	return nil
}

// DatabaseDisconnectionExperiment simulates database connection failures
type DatabaseDisconnectionExperiment struct {
	duration time.Duration
}

func NewDatabaseDisconnectionExperiment(duration time.Duration) *DatabaseDisconnectionExperiment {
	return &DatabaseDisconnectionExperiment{
		duration: duration,
	}
}

func (dde *DatabaseDisconnectionExperiment) Name() string {
	return "database-disconnection"
}

func (dde *DatabaseDisconnectionExperiment) Description() string {
	return fmt.Sprintf("Simulate database connection failures for %v", dde.duration)
}

func (dde *DatabaseDisconnectionExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: dde.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting database disconnection experiment",
		zap.Duration("duration", dde.duration))

	// Disconnect ClickHouse temporarily
	clickhouse := target.GetClickHouseConnection()
	if clickhouse != nil {
		if err := clickhouse.DisconnectTemporarily(dde.duration); err != nil {
			return nil, fmt.Errorf("failed to disconnect ClickHouse: %w", err)
		}
		result.Observations = append(result.Observations, "Disconnected ClickHouse temporarily")
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(dde.duration):
		result.Success = true
		result.Observations = append(result.Observations, "Database disconnection experiment completed")
	}

	return result, nil
}

func (dde *DatabaseDisconnectionExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	// Ensure normal operation is restored
	if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
		if err := clickhouse.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore ClickHouse normal operation: %v\n", err)
		}
	}
	return nil
}

// CacheEvictionExperiment simulates cache failures and evictions
type CacheEvictionExperiment struct {
	duration time.Duration
}

func NewCacheEvictionExperiment(duration time.Duration) *CacheEvictionExperiment {
	return &CacheEvictionExperiment{
		duration: duration,
	}
}

func (cee *CacheEvictionExperiment) Name() string {
	return "cache-eviction"
}

func (cee *CacheEvictionExperiment) Description() string {
	return fmt.Sprintf("Simulate cache failures and evictions for %v", cee.duration)
}

func (cee *CacheEvictionExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: cee.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting cache eviction experiment",
		zap.Duration("duration", cee.duration))

	redis := target.GetRedisConnection()
	if redis != nil {
		// First flush the database to simulate total cache loss
		if err := redis.FlushDatabase(); err != nil {
			return nil, fmt.Errorf("failed to flush Redis: %w", err)
		}
		result.Observations = append(result.Observations, "Flushed Redis cache")

		// Then simulate intermittent connection issues
		if err := redis.InjectOperationErrors(0.1, cee.duration); err != nil {
			return nil, fmt.Errorf("failed to inject Redis errors: %w", err)
		}
		result.Observations = append(result.Observations, "Injected 10% error rate into Redis operations")
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(cee.duration):
		result.Success = true
		result.Observations = append(result.Observations, "Cache eviction experiment completed")
	}

	return result, nil
}

func (cee *CacheEvictionExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	if redis := target.GetRedisConnection(); redis != nil {
		if err := redis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Redis normal operation: %v\n", err)
		}
	}
	return nil
}

// StreamingThrottleExperiment simulates streaming service throttling
type StreamingThrottleExperiment struct {
	throttlePercentage int
	duration           time.Duration
}

func NewStreamingThrottleExperiment(throttlePercentage int, duration time.Duration) *StreamingThrottleExperiment {
	return &StreamingThrottleExperiment{
		throttlePercentage: throttlePercentage,
		duration:           duration,
	}
}

func (ste *StreamingThrottleExperiment) Name() string {
	return "streaming-throttle"
}

func (ste *StreamingThrottleExperiment) Description() string {
	return fmt.Sprintf("Throttle streaming throughput by %d%% for %v", ste.throttlePercentage, ste.duration)
}

func (ste *StreamingThrottleExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: ste.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting streaming throttle experiment",
		zap.Int("throttle_percentage", ste.throttlePercentage),
		zap.Duration("duration", ste.duration))

	kinesis := target.GetKinesisConnection()
	if kinesis != nil {
		if err := kinesis.ThrottleWrites(ste.throttlePercentage, ste.duration); err != nil {
			return nil, fmt.Errorf("failed to throttle Kinesis: %w", err)
		}
		result.Observations = append(result.Observations,
			fmt.Sprintf("Throttled Kinesis throughput by %d%%", ste.throttlePercentage))
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(ste.duration):
		result.Success = true
		result.Observations = append(result.Observations, "Streaming throttle experiment completed")
	}

	return result, nil
}

func (ste *StreamingThrottleExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	if kinesis := target.GetKinesisConnection(); kinesis != nil {
		if err := kinesis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Kinesis normal operation: %v\n", err)
		}
	}
	return nil
}

// MemoryPressureExperiment simulates memory pressure conditions
type MemoryPressureExperiment struct {
	memoryPercentage int
	duration         time.Duration
}

func NewMemoryPressureExperiment(memoryPercentage int, duration time.Duration) *MemoryPressureExperiment {
	return &MemoryPressureExperiment{
		memoryPercentage: memoryPercentage,
		duration:         duration,
	}
}

func (mpe *MemoryPressureExperiment) Name() string {
	return "memory-pressure"
}

func (mpe *MemoryPressureExperiment) Description() string {
	return fmt.Sprintf("Simulate %d%% memory pressure for %v", mpe.memoryPercentage, mpe.duration)
}

func (mpe *MemoryPressureExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: mpe.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting memory pressure experiment",
		zap.Int("memory_percentage", mpe.memoryPercentage),
		zap.Duration("duration", mpe.duration))

	// Apply memory pressure to Redis (cache layer)
	redis := target.GetRedisConnection()
	if redis != nil {
		if err := redis.FillMemory(mpe.memoryPercentage); err != nil {
			return nil, fmt.Errorf("failed to apply memory pressure to Redis: %w", err)
		}
		result.Observations = append(result.Observations,
			fmt.Sprintf("Applied %d%% memory pressure to Redis", mpe.memoryPercentage))
	}

	// Apply disk pressure to ClickHouse (approximate memory pressure)
	clickhouse := target.GetClickHouseConnection()
	if clickhouse != nil {
		if err := clickhouse.FillDiskSpace(mpe.memoryPercentage); err != nil {
			return nil, fmt.Errorf("failed to apply disk pressure to ClickHouse: %w", err)
		}
		result.Observations = append(result.Observations,
			fmt.Sprintf("Applied %d%% disk pressure to ClickHouse", mpe.memoryPercentage))
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(mpe.duration):
		result.Success = true
		result.Observations = append(result.Observations, "Memory pressure experiment completed")
	}

	return result, nil
}

func (mpe *MemoryPressureExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	if redis := target.GetRedisConnection(); redis != nil {
		if err := redis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Redis normal operation: %v\n", err)
		}
	}
	if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
		if err := clickhouse.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore ClickHouse normal operation: %v\n", err)
		}
	}
	return nil
}

// NetworkPartitionExperiment simulates network partitions between services
type NetworkPartitionExperiment struct {
	duration time.Duration
}

func NewNetworkPartitionExperiment(duration time.Duration) *NetworkPartitionExperiment {
	return &NetworkPartitionExperiment{
		duration: duration,
	}
}

func (npe *NetworkPartitionExperiment) Name() string {
	return "network-partition"
}

func (npe *NetworkPartitionExperiment) Description() string {
	return fmt.Sprintf("Simulate network partitions between services for %v", npe.duration)
}

func (npe *NetworkPartitionExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: npe.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting network partition experiment",
		zap.Duration("duration", npe.duration))

	// Simulate network partition by making multiple services temporarily unavailable
	var servicesAffected int

	if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
		if err := clickhouse.DisconnectTemporarily(npe.duration); err == nil {
			servicesAffected++
			result.Observations = append(result.Observations, "ClickHouse network partitioned")
		}
	}

	if kinesis := target.GetKinesisConnection(); kinesis != nil {
		if err := kinesis.TemporaryUnavailable(npe.duration); err == nil {
			servicesAffected++
			result.Observations = append(result.Observations, "Kinesis network partitioned")
		}
	}

	if servicesAffected == 0 {
		return result, fmt.Errorf("no services could be network partitioned")
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(npe.duration):
		result.Success = true
		result.Observations = append(result.Observations,
			fmt.Sprintf("Network partition experiment completed, %d services affected", servicesAffected))
	}

	return result, nil
}

func (npe *NetworkPartitionExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	// Restore all connections
	if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
		if err := clickhouse.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore ClickHouse normal operation: %v\n", err)
		}
	}
	if kinesis := target.GetKinesisConnection(); kinesis != nil {
		if err := kinesis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Kinesis normal operation: %v\n", err)
		}
	}
	return nil
}

// RandomizedFailureExperiment injects random failures across different components
type RandomizedFailureExperiment struct {
	duration      time.Duration
	failureRate   float64
	componentMask int // Bitmask for which components to affect
}

const (
	FailureClickHouse = 1 << iota
	FailureRedis
	FailureKinesis
	FailureAll = FailureClickHouse | FailureRedis | FailureKinesis
)

func NewRandomizedFailureExperiment(duration time.Duration, failureRate float64) *RandomizedFailureExperiment {
	return &RandomizedFailureExperiment{
		duration:      duration,
		failureRate:   failureRate,
		componentMask: FailureAll,
	}
}

func (rfe *RandomizedFailureExperiment) Name() string {
	return "randomized-failures"
}

func (rfe *RandomizedFailureExperiment) Description() string {
	return fmt.Sprintf("Inject %.1f%% random failures across components for %v", rfe.failureRate*100, rfe.duration)
}

func (rfe *RandomizedFailureExperiment) Run(ctx context.Context, target ChaosTarget) (*ChaosResult, error) {
	result := &ChaosResult{
		ExperimentName: rfe.Name(),
		Observations:   make([]string, 0),
		Metrics:        make(map[string]string),
	}

	logger.Info("Starting randomized failure experiment",
		zap.Duration("duration", rfe.duration),
		zap.Float64("failure_rate", rfe.failureRate))

	// Apply random failures to different components
	var failuresApplied int

	if (rfe.componentMask&FailureClickHouse) != 0 && rand.Float64() < rfe.failureRate {
		if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
			if err := clickhouse.InjectQueryErrors(rfe.failureRate*0.5, rfe.duration); err == nil {
				failuresApplied++
				result.Observations = append(result.Observations,
					fmt.Sprintf("Injected %.1f%% query errors into ClickHouse", rfe.failureRate*50))
			}
		}
	}

	if (rfe.componentMask&FailureRedis) != 0 && rand.Float64() < rfe.failureRate {
		if redis := target.GetRedisConnection(); redis != nil {
			if err := redis.InjectOperationErrors(rfe.failureRate*0.3, rfe.duration); err == nil {
				failuresApplied++
				result.Observations = append(result.Observations,
					fmt.Sprintf("Injected %.1f%% operation errors into Redis", rfe.failureRate*30))
			}
		}
	}

	if (rfe.componentMask&FailureKinesis) != 0 && rand.Float64() < rfe.failureRate {
		if kinesis := target.GetKinesisConnection(); kinesis != nil {
			if err := kinesis.InjectWriteErrors(rfe.failureRate*0.4, rfe.duration); err == nil {
				failuresApplied++
				result.Observations = append(result.Observations,
					fmt.Sprintf("Injected %.1f%% write errors into Kinesis", rfe.failureRate*40))
			}
		}
	}

	// Wait for the experiment duration
	select {
	case <-ctx.Done():
		return result, ctx.Err()
	case <-time.After(rfe.duration):
		result.Success = failuresApplied > 0
		result.Observations = append(result.Observations,
			fmt.Sprintf("Randomized failure experiment completed, %d failure types applied", failuresApplied))
	}

	return result, nil
}

func (rfe *RandomizedFailureExperiment) Cleanup(ctx context.Context, target ChaosTarget) error {
	// Restore normal operation for all components
	if clickhouse := target.GetClickHouseConnection(); clickhouse != nil {
		if err := clickhouse.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore ClickHouse normal operation: %v\n", err)
		}
	}
	if redis := target.GetRedisConnection(); redis != nil {
		if err := redis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Redis normal operation: %v\n", err)
		}
	}
	if kinesis := target.GetKinesisConnection(); kinesis != nil {
		if err := kinesis.RestoreNormalOperation(); err != nil {
			fmt.Printf("Failed to restore Kinesis normal operation: %v\n", err)
		}
	}
	return nil
}
