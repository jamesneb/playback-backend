package features

import (
	"time"

	"github.com/jamesneb/playback-backend/internal/config/base"
)

const (
	FEATURES_PREFIX = "FEATURES_"
)

// Time period constants
const (
	ONE_HOUR = 1 * time.Hour
	ONE_DAY  = 24 * time.Hour
)

// Replay: Maximum duration of event history that can be replayed
const (
	MIN_REPLAY_DURATION     = 1 * time.Minute
	MAX_REPLAY_DURATION     = ONE_DAY
	DEFAULT_REPLAY_DURATION = ONE_HOUR
)

// Replay: Size of in-memory buffer for replay events
const (
	MIN_REPLAY_BUFFER_SIZE     = base.Byte(base.KILO * 1)   // 1KB
	MAX_REPLAY_BUFFER_SIZE     = base.Byte(base.MEGA * 100) // 100MB
	DEFAULT_REPLAY_BUFFER_SIZE = base.Byte(base.MEGA * 10)  // 10MB
)

// System map: How often to refresh the service dependency graph
const (
	MIN_MAP_REFRESH_INTERVAL     = 10 * time.Second
	MAX_MAP_REFRESH_INTERVAL     = 1 * time.Hour
	DEFAULT_MAP_REFRESH_INTERVAL = 5 * time.Minute
)

// System map: Maximum number of service nodes in dependency graph
const (
	MIN_MAP_NODES     = 10
	MAX_MAP_NODES     = 100_000
	DEFAULT_MAP_NODES = 1_000
)

// System map: Maximum number of connections between services
const (
	MIN_MAP_EDGES     = 10
	MAX_MAP_EDGES     = 1_000_000
	DEFAULT_MAP_EDGES = 10_000
)

// Data export: Maximum size of exported data file
const (
	MIN_EXPORT_SIZE     = base.Byte(base.MEGA * 1)
	MAX_EXPORT_SIZE     = base.Byte(base.MEGA * 1000) // 1GB
	DEFAULT_EXPORT_SIZE = base.Byte(base.MEGA * 100)  // 100MB
)

// Default values
const (
	DEFAULT_ENABLE_REPLAY      = false
	DEFAULT_ENABLE_SYSTEM_MAP  = false
	DEFAULT_ENABLE_DATA_EXPORT = false
)

// Data export format defaults
var (
	DEFAULT_EXPORT_FORMATS = []base.DataExportFormat{
		base.DATA_EXPORT_JSON,
		base.DATA_EXPORT_CSV,
		base.DATA_EXPORT_PARQUET,
	}
)
