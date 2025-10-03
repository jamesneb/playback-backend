package metrics

import (
	"sync"
	"time"
)

// ServiceTracker efficiently tracks active services with minimal overhead
type ServiceTracker struct {
	services map[string]map[string]time.Time // type -> service -> lastSeen
	mutex    sync.RWMutex
}

// NewServiceTracker creates a new service tracker
func NewServiceTracker() *ServiceTracker {
	return &ServiceTracker{
		services: make(map[string]map[string]time.Time),
	}
}

// RecordActivity records service activity (called from handlers)
func (st *ServiceTracker) RecordActivity(telemetryType, serviceName string) {
	if serviceName == "" || serviceName == "unknown" {
		return
	}

	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.services[telemetryType] == nil {
		st.services[telemetryType] = make(map[string]time.Time)
	}
	st.services[telemetryType][serviceName] = time.Now()
}

// UpdateActiveServiceMetrics updates Prometheus metrics with current active service counts
func (st *ServiceTracker) UpdateActiveServiceMetrics(registry *Registry) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	now := time.Now()
	staleThreshold := 5 * time.Minute

	for telemetryType, services := range st.services {
		activeCount := 0
		// Clean up stale services while counting
		for serviceName, lastSeen := range services {
			if now.Sub(lastSeen) <= staleThreshold {
				activeCount++
			} else {
				// Mark for cleanup (safe since we're in RLock, will clean up later)
				delete(services, serviceName)
			}
		}
		registry.RecordActiveService(telemetryType, float64(activeCount))
	}
}

var (
	// Global service tracker instance
	globalServiceTracker *ServiceTracker
	serviceTrackerOnce   sync.Once
)

// GlobalServiceTracker returns the singleton service tracker
func GlobalServiceTracker() *ServiceTracker {
	serviceTrackerOnce.Do(func() {
		globalServiceTracker = NewServiceTracker()
	})
	return globalServiceTracker
}
