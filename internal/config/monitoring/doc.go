// Package monitoring defines configuration for observability and monitoring.
//
// This package provides configuration for metrics collection, distributed tracing,
// and health check endpoints.
//
// # Metrics
//
// Prometheus-compatible metrics endpoint configuration:
//   - EnableMetrics: Toggle metrics collection
//   - MetricsPort: Port for metrics HTTP server
//   - MetricsPath: HTTP path for scraping metrics
//
// # Tracing
//
// Distributed tracing configuration:
//   - EnableTracing: Toggle trace collection
//   - TracingEndpoint: OTLP endpoint for trace export
//
// # Health Checks
//
// Health and readiness check endpoint:
//   - HealthCheckPath: HTTP path for health status
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// MONITORING_ prefix:
//
//	MONITORING_ENABLE_METRICS=true
//	MONITORING_METRICS_PORT=9090
//	MONITORING_METRICS_PATH=/metrics
//	MONITORING_ENABLE_TRACING=true
//	MONITORING_TRACING_ENDPOINT=localhost:4317
//	MONITORING_HEALTH_CHECK_PATH=/health
//
// # Files in This Package
//
// constants.go:
//   - MONITORING_PREFIX for environment variable namespacing
//   - Default values (ports, paths, enable flags)
//   - Min/max bounds for validation
//
// section.go:
//   - Config struct with monitoring parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package monitoring
