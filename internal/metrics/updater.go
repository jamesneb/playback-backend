package metrics

import (
	"context"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// MetricsUpdater provides minimal overhead background metric updates
type MetricsUpdater struct {
	registry       *Registry
	serviceTracker *ServiceTracker
	interval       time.Duration
	stopCh         chan struct{}
}

// NewMetricsUpdater creates a new metrics updater with minimal overhead
func NewMetricsUpdater(interval time.Duration) *MetricsUpdater {
	return &MetricsUpdater{
		registry:       Global(),
		serviceTracker: GlobalServiceTracker(),
		interval:       interval,
		stopCh:         make(chan struct{}),
	}
}

// Start begins the minimal background metrics update process
func (mu *MetricsUpdater) Start(ctx context.Context) {
	ticker := time.NewTicker(mu.interval)
	defer ticker.Stop()

	logger.Info("Starting metrics updater",
		zap.Duration("interval", mu.interval))

	for {
		select {
		case <-ctx.Done():
			logger.Info("Metrics updater stopped by context")
			return
		case <-mu.stopCh:
			logger.Info("Metrics updater stopped")
			return
		case <-ticker.C:
			mu.updateMetrics()
		}
	}
}

// Stop stops the metrics updater
func (mu *MetricsUpdater) Stop() {
	close(mu.stopCh)
}

// updateMetrics performs minimal overhead metric updates
func (mu *MetricsUpdater) updateMetrics() {
	// Update active service counts (zero database queries)
	mu.serviceTracker.UpdateActiveServiceMetrics(mu.registry)

	// Update system availability based on service activity
	mu.updateAvailability()

	// Update system resource utilization
	mu.updateResourceMetrics()
}

// updateAvailability calculates service availability from active services
func (mu *MetricsUpdater) updateAvailability() {
	// Simple availability calculation: if we have active services, we're available
	mu.serviceTracker.mutex.RLock()
	totalTypes := len(mu.serviceTracker.services)
	activeTypes := 0
	for _, services := range mu.serviceTracker.services {
		if len(services) > 0 {
			activeTypes++
		}
	}
	mu.serviceTracker.mutex.RUnlock()

	if totalTypes == 0 {
		mu.registry.RecordAvailability(0.0)
	} else {
		availability := float64(activeTypes) / float64(totalTypes)
		mu.registry.RecordAvailability(availability)
	}
}

// updateResourceMetrics records current system resource utilization
func (mu *MetricsUpdater) updateResourceMetrics() {
	goroutines, memoryMB := GetSystemStats()

	// Convert system stats to business-relevant utilization ratios
	cpuUtilization := float64(goroutines) / 10000.0 // Normalize goroutines to 0-1 range
	if cpuUtilization > 1.0 {
		cpuUtilization = 1.0
	}

	memoryUtilization := memoryMB / (4 * 1024) // Assume 4GB max, normalize to 0-1 range
	if memoryUtilization > 1.0 {
		memoryUtilization = 1.0
	}

	mu.registry.RecordComputeUsage("cpu", "goroutines", cpuUtilization)
	mu.registry.RecordComputeUsage("memory", "heap", memoryUtilization)

	// Record system health as business metrics
	if goroutines > 5000 {
		mu.registry.RecordBusinessError("system", "goroutines", "warning")
	}
	if memoryMB > 2048 { // > 2GB
		mu.registry.RecordBusinessError("system", "memory", "warning")
	}
}
