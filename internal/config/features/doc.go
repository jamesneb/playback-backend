// Package features defines the configuration for optional application features.
//
// # Overview
//
// This package provides configuration for optional telemetry features including:
//
//   - Replay: Re-process historical events from storage
//   - System Map: Service dependency graph visualization
//   - Data Export: Export telemetry data to various formats
//
// Each feature can be individually enabled or disabled. When disabled, features
// are not validated, allowing for clean production configurations.
//
// # Configuration Keys
//
// All settings use the FEATURES_ prefix:
//
// Replay feature:
//
//	FEATURES_ENABLE_REPLAY      - Enable event replay (default: false)
//	FEATURES_REPLAY_DURATION    - Max history window (default: 1h, range: 1m-24h)
//	FEATURES_REPLAY_BUFFER_SIZE - In-memory buffer size (default: 10MB, range: 1KB-100MB)
//
// System map feature:
//
//	FEATURES_ENABLE_SYSTEM_MAP    - Enable service dependency graph (default: false)
//	FEATURES_MAP_REFRESH_INTERVAL - Graph refresh rate (default: 5m, range: 10s-1h)
//	FEATURES_MAP_MAX_NODES        - Maximum service nodes (default: 1000, range: 10-100000)
//	FEATURES_MAP_MAX_EDGES        - Maximum connections (default: 10000, range: 10-1000000)
//
// Data export feature:
//
//	FEATURES_ENABLE_DATA_EXPORT - Enable data export (default: false)
//	FEATURES_EXPORT_FORMATS     - Comma-separated formats (default: json,csv,parquet)
//	FEATURES_EXPORT_MAX_SIZE    - Maximum export file size (default: 100MB, range: 1MB-1GB)
//
// # Example Usage
//
//	// Get features config from manager
//	snapshot := mgr.Snapshot()
//	featuresCfg := snapshot.Features
//
//	// Check if replay is enabled
//	if featuresCfg.EnableReplay {
//	    replayService := NewReplayService(
//	        featuresCfg.ReplayDuration,
//	        featuresCfg.ReplayBufferSize,
//	    )
//	}
//
//	// Use system map configuration
//	if featuresCfg.EnableSystemMap {
//	    mapBuilder := NewSystemMapBuilder(
//	        featuresCfg.MapMaxNodes,
//	        featuresCfg.MapMaxEdges,
//	        featuresCfg.MapRefreshInterval,
//	    )
//	}
//
//	// Configure data export
//	if featuresCfg.EnableDataExport {
//	    exporter := NewDataExporter(
//	        featuresCfg.ExportFormats,
//	        featuresCfg.ExportMaxSize,
//	    )
//	}
//
// # Validation
//
// The configuration is validated on load with conditional rules:
//
// Replay validation (only when enabled):
//
//   - ReplayDuration within bounds (1m to 24h)
//   - ReplayBufferSize within bounds (1KB to 100MB)
//
// System map validation (only when enabled):
//
//   - MapRefreshInterval within bounds (10s to 1h)
//   - MapMaxNodes within bounds (10 to 100,000)
//   - MapMaxEdges within bounds (10 to 1,000,000)
//
// Data export validation (only when enabled):
//
//   - At least one export format specified
//   - ExportMaxSize within bounds (1MB to 1GB)
//
// # Replay Feature
//
// The replay feature allows re-processing historical telemetry events:
//
//   - Duration controls how far back in time to replay
//   - BufferSize sets the in-memory buffer for event batching
//   - Useful for debugging, testing, and data recovery
//   - Events are replayed in chronological order
//
// Example use cases:
//
//   - Replay last hour of events after system upgrade
//   - Re-process events with updated analysis rules
//   - Validate data pipeline changes against historical data
//
// # System Map Feature
//
// The system map builds a real-time service dependency graph:
//
//   - Visualizes service-to-service communication
//   - Automatically discovers new services and connections
//   - RefreshInterval controls graph update frequency
//   - MaxNodes caps total services to prevent memory exhaustion
//   - MaxEdges caps total connections
//
// The graph helps with:
//
//   - Understanding system architecture
//   - Identifying communication bottlenecks
//   - Detecting circular dependencies
//   - Impact analysis for service changes
//
// # Data Export Feature
//
// The data export feature supports multiple output formats:
//
//   - JSON: Human-readable, good for debugging
//   - CSV: Spreadsheet-compatible, good for analysis
//   - Parquet: Columnar format, efficient for large datasets
//
// Export considerations:
//
//   - MaxSize prevents exports from consuming excessive disk space
//   - Multiple formats can be enabled simultaneously
//   - Exports are typically triggered via API endpoints
//   - Large exports may be paginated automatically
//
// # Feature Flag Pattern
//
// All features follow a consistent enable/configure pattern:
//
//	// Check if feature is enabled before using configuration
//	if cfg.EnableReplay {
//	    // Only these fields are guaranteed valid when enabled
//	    duration := cfg.ReplayDuration
//	    buffer := cfg.ReplayBufferSize
//	}
//
// Benefits of this pattern:
//
//   - Clean separation of enabled vs disabled features
//   - Validation only runs for enabled features
//   - Default configurations work without modification
//   - Easy to enable features in specific environments
//
// # Performance Considerations
//
// When enabling features, consider resource impacts:
//
// Replay:
//
//   - ReplayBufferSize affects memory usage
//   - Longer durations increase storage I/O
//   - May impact real-time processing throughput
//
// System Map:
//
//   - More nodes/edges increase memory usage
//   - Faster refresh rates increase CPU usage
//   - Graph algorithms scale with node/edge count
//
// Data Export:
//
//   - Larger MaxSize increases disk I/O
//   - Multiple formats increase export time
//   - Consider export frequency and retention
package features
