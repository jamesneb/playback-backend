// Package monitoring defines the configuration for observability and monitoring.
//
// # Overview
//
// This package provides comprehensive observability configuration including:
//
//   - Metrics: Prometheus-compatible metrics endpoint
//   - Tracing: OpenTelemetry distributed tracing
//   - Jaeger: Jaeger-specific tracing configuration
//   - Health checks: Liveness and readiness endpoints
//
// # Configuration Keys
//
// All settings use the MONITORING_ prefix:
//
// Metrics configuration:
//
//	MONITORING_ENABLE_METRICS - Enable Prometheus metrics (default: true)
//	MONITORING_METRICS_PORT   - Metrics server port (default: 9090, range: 1024-65535)
//	MONITORING_METRICS_PATH   - Metrics endpoint path (default: /metrics)
//
// Tracing configuration:
//
//	MONITORING_ENABLE_TRACING   - Enable distributed tracing (default: true)
//	MONITORING_TRACING_ENDPOINT - OTLP endpoint (required if tracing enabled)
//
// Jaeger configuration:
//
//	MONITORING_ENABLE_JAEGER         - Enable Jaeger tracing (default: false)
//	MONITORING_JAEGER_SERVICE_NAME   - Service name in traces (default: playback-backend)
//	MONITORING_JAEGER_SAMPLING_RATE  - Percentage of traces to sample (default: 10%, range: 0-100%)
//	MONITORING_JAEGER_FLUSH_INTERVAL - Batch send interval (default: 5s, range: 1s-1m)
//
// Health check configuration:
//
//	MONITORING_HEALTH_CHECK_PATH - Health endpoint path (default: /health)
//
// # Example Usage
//
//	// Get monitoring config from manager
//	snapshot := mgr.Snapshot()
//	monCfg := snapshot.Monitoring
//
//	// Configure metrics server
//	if monCfg.EnableMetrics {
//	    metricsServer := &http.Server{
//	        Addr:    fmt.Sprintf(":%d", monCfg.MetricsPort),
//	        Handler: promhttp.Handler(),
//	    }
//	    http.Handle(string(monCfg.MetricsPath), promhttp.Handler())
//	}
//
//	// Configure tracing
//	if monCfg.EnableTracing {
//	    exporter, _ := otlptrace.New(
//	        context.Background(),
//	        otlptracegrpc.NewClient(
//	            otlptracegrpc.WithEndpoint(monCfg.TracingEndpoint),
//	        ),
//	    )
//	    tp := trace.NewTracerProvider(trace.WithBatcher(exporter))
//	    otel.SetTracerProvider(tp)
//	}
//
//	// Configure Jaeger
//	if monCfg.EnableJaeger {
//	    jaegerExporter, _ := jaeger.New(
//	        jaeger.WithCollectorEndpoint(),
//	    )
//	    tp := trace.NewTracerProvider(
//	        trace.WithBatcher(jaegerExporter,
//	            trace.WithBatchTimeout(monCfg.JaegerFlushInterval),
//	        ),
//	        trace.WithSampler(trace.TraceIDRatioBased(
//	            float64(monCfg.JaegerSamplingRate.Value()) / 100.0,
//	        )),
//	    )
//	}
//
//	// Register health check
//	http.HandleFunc(string(monCfg.HealthCheckPath), healthCheckHandler)
//
// # Validation
//
// The configuration is validated on load with:
//
// Metrics validation (only when enabled):
//
//   - MetricsPort in valid range (1024-65535)
//   - MetricsPath not empty
//
// Tracing validation (only when enabled):
//
//   - TracingEndpoint not empty
//
// Jaeger validation (only when enabled):
//
//   - JaegerServiceName not empty
//   - JaegerSamplingRate within bounds (0-100%)
//   - JaegerFlushInterval within bounds (1s-1m)
//
// Health checks:
//
//   - HealthCheckPath not empty (always validated)
//
// # Metrics Endpoint
//
// The metrics endpoint exposes Prometheus-compatible metrics:
//
//   - Runs on a separate port from main API
//   - Should not be publicly exposed in production
//   - Prometheus scrapes at /metrics by default
//   - Metrics include: request counts, latencies, error rates, resource usage
//
// Example Prometheus configuration:
//
//	scrape_configs:
//	  - job_name: 'playback-backend'
//	    static_configs:
//	      - targets: ['localhost:9090']
//	    metrics_path: '/metrics'
//	    scrape_interval: 15s
//
// Common metrics exposed:
//
//   - http_requests_total: Total HTTP requests by method/path/status
//   - http_request_duration_seconds: Request latency histogram
//   - grpc_server_handled_total: Total gRPC requests by method/code
//   - process_cpu_seconds_total: CPU usage
//   - process_resident_memory_bytes: Memory usage
//
// # Distributed Tracing
//
// Distributed tracing follows OpenTelemetry standards:
//
//   - Traces span multiple services
//   - Automatic context propagation via headers
//   - TracingEndpoint receives OTLP over gRPC
//   - Compatible with Jaeger, Zipkin, and cloud providers
//
// Trace data includes:
//
//   - Span IDs and parent relationships
//   - Operation names and durations
//   - Tags and logs for context
//   - Error flags and messages
//
// # Jaeger Configuration
//
// Jaeger is a popular distributed tracing backend:
//
//   - EnableJaeger can work alongside EnableTracing
//   - ServiceName appears in Jaeger UI for filtering
//   - SamplingRate controls trace volume and cost
//   - FlushInterval balances latency vs batching efficiency
//
// # Sampling Strategy
//
// Sampling reduces tracing overhead and costs:
//
// Sampling rates by environment:
//
//   - Development: 100% (capture everything)
//   - Staging: 50% (balance visibility and cost)
//   - Production: 1-10% (representative sample)
//
// Example configuration:
//
//	# Development: trace everything
//	MONITORING_JAEGER_SAMPLING_RATE=100
//
//	# Production: trace 5% of requests
//	MONITORING_JAEGER_SAMPLING_RATE=5
//
// Sampling considerations:
//
//   - Higher rates increase storage and network costs
//   - Lower rates may miss rare errors
//   - Sample all error traces regardless of rate
//   - Consider head-based vs tail-based sampling
//
// # Flush Interval Details
//
// The flush interval controls trace batching:
//
//   - Shorter intervals: Lower latency, higher network overhead
//   - Longer intervals: Higher latency, better batching efficiency
//   - Default 5s balances latency and efficiency
//
// Tuning recommendations:
//
//   - Low-latency requirements: 1-2s
//   - High-throughput systems: 5-10s
//   - Batch-processing systems: 30-60s
//
// # Health Checks
//
// The health check endpoint reports service status:
//
//   - Returns 200 OK when healthy
//   - Returns 503 Service Unavailable when unhealthy
//   - Used by load balancers and orchestrators
//   - Should check critical dependencies
//
// Example health check implementation:
//
//	func healthCheck(w http.ResponseWriter, r *http.Request) {
//	    if !databaseHealthy() || !cacheHealthy() {
//	        w.WriteHeader(503)
//	        json.NewEncoder(w).Encode(map[string]string{
//	            "status": "unhealthy",
//	            "reason": "database connection failed",
//	        })
//	        return
//	    }
//	    w.WriteHeader(200)
//	    json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
//	}
//
// # Observability Best Practices
//
// For production deployments:
//
// Metrics:
//
//   - Always enable metrics in production
//   - Secure metrics port with firewall rules
//   - Set up alerting on key metrics
//   - Monitor metrics cardinality to avoid explosion
//
// Tracing:
//
//   - Enable tracing in staging and production
//   - Use appropriate sampling rates
//   - Set up trace-based alerts for errors
//   - Correlate traces with logs using trace IDs
//
// Health checks:
//
//   - Include all critical dependencies
//   - Respond quickly (< 1s timeout)
//   - Distinguish between startup and running state
//   - Log health check failures for debugging
//
// # Integration Example
//
// Complete observability setup:
//
//	# Enable all observability features
//	MONITORING_ENABLE_METRICS=true
//	MONITORING_METRICS_PORT=9090
//	MONITORING_METRICS_PATH=/metrics
//
//	MONITORING_ENABLE_TRACING=true
//	MONITORING_TRACING_ENDPOINT=jaeger:4317
//
//	MONITORING_ENABLE_JAEGER=true
//	MONITORING_JAEGER_SERVICE_NAME=playback-backend
//	MONITORING_JAEGER_SAMPLING_RATE=10
//	MONITORING_JAEGER_FLUSH_INTERVAL=5s
//
//	MONITORING_HEALTH_CHECK_PATH=/health
//
// This configuration:
//
//   - Exposes Prometheus metrics on port 9090
//   - Sends traces to Jaeger at jaeger:4317
//   - Samples 10% of traces
//   - Flushes trace batches every 5 seconds
//   - Provides health check at /health
package monitoring
