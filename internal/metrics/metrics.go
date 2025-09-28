package metrics

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Registry holds application-specific Prometheus metrics
type Registry struct {
	// HTTP request metrics
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec

	// Telemetry processing metrics
	telemetryProcessed *prometheus.CounterVec
	processingDuration *prometheus.HistogramVec

	// System metrics
	systemInfo prometheus.Gauge
}

// NewRegistry creates a new metrics registry with standard application metrics
func NewRegistry() *Registry {
	return &Registry{
		httpRequests: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests by method and status",
			},
			[]string{"method", "status"},
		),

		httpDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method"},
		),

		telemetryProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "telemetry_processed_total",
				Help: "Total telemetry data processed by type and service",
			},
			[]string{"type", "service", "status"},
		),

		processingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "telemetry_processing_duration_seconds",
				Help:    "Telemetry processing duration by type",
				Buckets: []float64{0.001, 0.01, 0.1, 1.0, 10.0},
			},
			[]string{"type"},
		),

		systemInfo: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "system_info",
				Help: "System information (constant 1 with version labels)",
			},
		),
	}
}

// RecordHTTPRequest records metrics for an HTTP request
func (r *Registry) RecordHTTPRequest(method, status string, duration float64) {
	r.httpRequests.WithLabelValues(method, status).Inc()
	r.httpDuration.WithLabelValues(method).Observe(duration)
}

// RecordTelemetryProcessing records metrics for telemetry data processing
func (r *Registry) RecordTelemetryProcessing(telemetryType, service, status string, duration float64) {
	r.telemetryProcessed.WithLabelValues(telemetryType, service, status).Inc()
	r.processingDuration.WithLabelValues(telemetryType).Observe(duration)
}

// UpdateSystemInfo updates system information metrics
func (r *Registry) UpdateSystemInfo() {
	r.systemInfo.Set(1)
}

// GetSystemStats returns current system statistics
func GetSystemStats() (goroutines int, memoryMB float64) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return runtime.NumGoroutine(), float64(m.Alloc) / 1024 / 1024
}
