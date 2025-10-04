package monitoring

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	MONITORING_PREFIX = "MONITORING_"
)

// Port constants
const (
	MIN_METRICS_PORT     base.Port = 1024
	MAX_METRICS_PORT     base.Port = 65535
	DEFAULT_METRICS_PORT base.Port = 9090
)

// Jaeger sampling rate: Percentage of traces to sample (0-100)
var (
	MIN_JAEGER_SAMPLING_RATE, _     = base.NewPercentage(0)
	MAX_JAEGER_SAMPLING_RATE, _     = base.NewPercentage(100)
	DEFAULT_JAEGER_SAMPLING_RATE, _ = base.NewPercentage(10) // 10% of traces
)

// Jaeger flush interval: How often to batch and send traces to collector
const (
	MIN_JAEGER_FLUSH_INTERVAL     = 1 * time.Second
	MAX_JAEGER_FLUSH_INTERVAL     = 1 * time.Minute
	DEFAULT_JAEGER_FLUSH_INTERVAL = 5 * time.Second
)

// Default values
const (
	DEFAULT_ENABLE_METRICS              = true
	DEFAULT_METRICS_PATH      base.Path = "/metrics"
	DEFAULT_ENABLE_TRACING              = true
	DEFAULT_HEALTH_CHECK_PATH base.Path = "/health"
	DEFAULT_ENABLE_JAEGER               = false
)
