package app

import "time"

// AppConfig defines application-level configuration
type AppConfig struct {
	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Environment string `yaml:"environment"`
	Debug       bool   `yaml:"debug"`
	LogLevel    string `yaml:"log_level"`
}

// LoggingConfig defines logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	Filename   string `yaml:"filename"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
}

// DevelopmentConfig defines development-specific configuration
type DevelopmentConfig struct {
	EnableProfiling      bool          `yaml:"enable_profiling"`
	EnableDebugAPI       bool          `yaml:"enable_debug_api"`
	EnableDebugEndpoints bool          `yaml:"enable_debug_endpoints"` // Backward compatibility
	EnableHotReload      bool          `yaml:"enable_hot_reload"`
	MockExternalAPIs     bool          `yaml:"mock_external_apis"`
	SeedData             bool          `yaml:"seed_data"`
	DebugPort            int           `yaml:"debug_port"`
	ProfilerEndpoint     string        `yaml:"profiler_endpoint"`
	LiveReloadPort       int           `yaml:"live_reload_port"`
	WatchInterval        time.Duration `yaml:"watch_interval"`
}
