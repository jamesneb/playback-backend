package api

import "time"

// APIConfig defines REST API configuration
type APIConfig struct {
	EnableCORS      bool       `yaml:"enable_cors"`
	CORS            CORSConfig `yaml:"cors"`
	EnableMetrics   bool       `yaml:"enable_metrics"`
	EnableProfiling bool       `yaml:"enable_profiling"`
	EnableSwagger   bool       `yaml:"enable_swagger"`
	// Legacy backward compatibility
	Version string `yaml:"version"`
	Prefix  string `yaml:"prefix"`
}

// CORSConfig defines Cross-Origin Resource Sharing configuration
type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials"`
	MaxAge           int      `yaml:"max_age"`
}

// SwaggerConfig defines Swagger documentation configuration
type SwaggerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Host        string `yaml:"host"`
	BasePath    string `yaml:"base_path"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	Contact     struct {
		Name  string `yaml:"name"`
		Email string `yaml:"email"`
		URL   string `yaml:"url"`
	} `yaml:"contact"`
}

// PerformanceConfig defines performance-related settings
type PerformanceConfig struct {
	RequestTimeout        time.Duration `yaml:"request_timeout"`
	MaxConcurrentRequests int           `yaml:"max_concurrent_requests"`
	ResponseBufferSize    int           `yaml:"response_buffer_size"`
	CompressionLevel      int           `yaml:"compression_level"`
	CompressionMinSize    int           `yaml:"compression_min_size"`
	KeepAlive             bool          `yaml:"keep_alive"`
	KeepAliveTimeout      time.Duration `yaml:"keep_alive_timeout"`
}
