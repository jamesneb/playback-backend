package metrics

import (
	"runtime"
	"sync"

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

	// Business metrics - Customer Usage
	activeServices        *prometheus.GaugeVec
	dataVolumeIngested    *prometheus.CounterVec
	dataVolumeQueried     *prometheus.CounterVec
	retentionDays         *prometheus.GaugeVec

	// Business metrics - Service Quality
	dataIngestionLatency  *prometheus.HistogramVec
	queryLatency         *prometheus.HistogramVec
	errorRate            *prometheus.CounterVec
	availabilityUptime   prometheus.Gauge

	// Business metrics - Resource Utilization
	storageUsage         *prometheus.GaugeVec
	throughputPeakRPS    *prometheus.GaugeVec
	concurrentConnections *prometheus.GaugeVec

	// Business metrics - Cost Optimization
	computeResources     *prometheus.GaugeVec
	networkBandwidth     *prometheus.CounterVec
	storageOperations    *prometheus.CounterVec
}

// NewRegistry creates a new metrics registry with standard application and business metrics
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

		// Business metrics - Customer Usage
		activeServices: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "business_active_services_total",
				Help: "Number of active services sending telemetry data",
			},
			[]string{"type"},
		),

		dataVolumeIngested: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "business_data_ingested_bytes_total",
				Help: "Total volume of telemetry data ingested in bytes",
			},
			[]string{"type", "service"},
		),

		dataVolumeQueried: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "business_data_queried_bytes_total",
				Help: "Total volume of data returned from queries in bytes",
			},
			[]string{"type", "service"},
		),

		retentionDays: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "business_data_retention_days",
				Help: "Data retention period configured per service in days",
			},
			[]string{"service", "type"},
		),

		// Business metrics - Service Quality
		dataIngestionLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "business_ingestion_latency_seconds",
				Help:    "End-to-end data ingestion latency from receipt to storage",
				Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
			},
			[]string{"type", "service"},
		),

		queryLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "business_query_latency_seconds",
				Help:    "Query response latency from request to result",
				Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
			},
			[]string{"type", "operation"},
		),

		errorRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "business_errors_total",
				Help: "Total business errors by category and service",
			},
			[]string{"category", "service", "severity"},
		),

		availabilityUptime: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "business_availability_uptime_ratio",
				Help: "Service availability as uptime ratio (0.0 to 1.0)",
			},
		),

		// Business metrics - Resource Utilization
		storageUsage: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "business_storage_usage_bytes",
				Help: "Current storage utilization per service and type",
			},
			[]string{"service", "type", "tier"},
		),

		throughputPeakRPS: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "business_peak_throughput_rps",
				Help: "Peak requests per second capacity and utilization",
			},
			[]string{"type", "metric"},
		),

		concurrentConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "business_concurrent_connections_total",
				Help: "Current concurrent connections per service type",
			},
			[]string{"type", "service"},
		),

		// Business metrics - Cost Optimization
		computeResources: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "business_compute_usage_ratio",
				Help: "Compute resource utilization as ratio (0.0 to 1.0)",
			},
			[]string{"resource", "component"},
		),

		networkBandwidth: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "business_network_bytes_total",
				Help: "Network bandwidth utilization in bytes",
			},
			[]string{"direction", "service"},
		),

		storageOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "business_storage_operations_total",
				Help: "Storage operations count by type and result",
			},
			[]string{"operation", "result"},
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

// Business metrics recording methods

// RecordActiveService updates the count of active services by type
func (r *Registry) RecordActiveService(telemetryType string, count float64) {
	r.activeServices.WithLabelValues(telemetryType).Set(count)
}

// RecordDataIngestion records volume of data ingested
func (r *Registry) RecordDataIngestion(telemetryType, service string, bytes float64, latencySeconds float64) {
	r.dataVolumeIngested.WithLabelValues(telemetryType, service).Add(bytes)
	r.dataIngestionLatency.WithLabelValues(telemetryType, service).Observe(latencySeconds)
}

// RecordDataQuery records volume of data queried and query latency
func (r *Registry) RecordDataQuery(telemetryType, operation string, bytes float64, latencySeconds float64) {
	r.dataVolumeQueried.WithLabelValues(telemetryType, "query").Add(bytes)
	r.queryLatency.WithLabelValues(telemetryType, operation).Observe(latencySeconds)
}

// RecordDataRetention sets the data retention period for a service
func (r *Registry) RecordDataRetention(service, telemetryType string, days float64) {
	r.retentionDays.WithLabelValues(service, telemetryType).Set(days)
}

// RecordBusinessError records business logic errors by category and severity
func (r *Registry) RecordBusinessError(category, service, severity string) {
	r.errorRate.WithLabelValues(category, service, severity).Inc()
}

// RecordAvailability records service availability as uptime ratio
func (r *Registry) RecordAvailability(uptimeRatio float64) {
	r.availabilityUptime.Set(uptimeRatio)
}

// RecordStorageUsage records current storage utilization
func (r *Registry) RecordStorageUsage(service, telemetryType, tier string, bytes float64) {
	r.storageUsage.WithLabelValues(service, telemetryType, tier).Set(bytes)
}

// RecordThroughput records peak throughput metrics
func (r *Registry) RecordThroughput(telemetryType, metric string, rps float64) {
	r.throughputPeakRPS.WithLabelValues(telemetryType, metric).Set(rps)
}

// RecordConcurrentConnections records current concurrent connections
func (r *Registry) RecordConcurrentConnections(telemetryType, service string, count float64) {
	r.concurrentConnections.WithLabelValues(telemetryType, service).Set(count)
}

// RecordComputeUsage records compute resource utilization
func (r *Registry) RecordComputeUsage(resource, component string, utilizationRatio float64) {
	r.computeResources.WithLabelValues(resource, component).Set(utilizationRatio)
}

// RecordNetworkBandwidth records network bandwidth usage
func (r *Registry) RecordNetworkBandwidth(direction, service string, bytes float64) {
	r.networkBandwidth.WithLabelValues(direction, service).Add(bytes)
}

// RecordStorageOperation records storage operations
func (r *Registry) RecordStorageOperation(operation, result string) {
	r.storageOperations.WithLabelValues(operation, result).Inc()
}

var (
	// Global registry instance - initialized once for maximum performance
	globalRegistry *Registry
	registryOnce   sync.Once
)

// Global returns the singleton metrics registry
func Global() *Registry {
	registryOnce.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}
