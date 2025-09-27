package processing

import "time"

// ProcessingConfig defines data processing configuration
type ProcessingConfig struct {
	BatchSize         int           `yaml:"batch_size"`
	BatchTimeout      time.Duration `yaml:"batch_timeout"`
	Workers           int           `yaml:"workers"`
	QueueSize         int           `yaml:"queue_size"`
	EnableCompression bool          `yaml:"enable_compression"`
	CompressionLevel  int           `yaml:"compression_level"`
	// Processing modes
	EnableAsync      bool `yaml:"enable_async"`
	EnableParallel   bool `yaml:"enable_parallel"`
	EnableValidation bool `yaml:"enable_validation"`
	// Performance tuning
	MaxMemoryUsage int64         `yaml:"max_memory_usage"`
	GCInterval     time.Duration `yaml:"gc_interval"`
}

// RetentionConfig defines data retention policies
type RetentionConfig struct {
	TracesRetentionDays  int           `yaml:"traces_retention_days"`
	MetricsRetentionDays int           `yaml:"metrics_retention_days"`
	LogsRetentionDays    int           `yaml:"logs_retention_days"`
	EnableAutoCleanup    bool          `yaml:"enable_auto_cleanup"`
	CleanupInterval      time.Duration `yaml:"cleanup_interval"`
}

// FeaturesConfig defines feature flags and experimental features
type FeaturesConfig struct {
	EnableReplay     bool             `yaml:"enable_replay"`
	EnableSystemMap  bool             `yaml:"enable_system_map"`
	EnableDataExport bool             `yaml:"enable_data_export"`
	Replay           ReplayConfig     `yaml:"replay"`
	SystemMap        SystemMapConfig  `yaml:"system_map"`
	DataExport       DataExportConfig `yaml:"data_export"`
}

// ReplayConfig defines replay functionality configuration
type ReplayConfig struct {
	Enabled           bool          `yaml:"enabled"`
	MaxReplayDuration time.Duration `yaml:"max_replay_duration"`
	BufferSize        int           `yaml:"buffer_size"`
}

// SystemMapConfig defines system topology mapping configuration
type SystemMapConfig struct {
	Enabled                 bool          `yaml:"enabled"`
	RefreshInterval         time.Duration `yaml:"refresh_interval"`
	MaxNodes                int           `yaml:"max_nodes"`
	MaxEdges                int           `yaml:"max_edges"`
	IncludeExternalServices bool          `yaml:"include_external_services"`
	ExcludeInternalTraffic  bool          `yaml:"exclude_internal_traffic"`
	NodeTTL                 time.Duration `yaml:"node_ttl"`
	EdgeTTL                 time.Duration `yaml:"edge_ttl"`
}

// DataExportConfig defines data export functionality configuration
type DataExportConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Formats       []string `yaml:"formats"` // json, csv, parquet
	MaxExportSize int64    `yaml:"max_export_size"`
}
