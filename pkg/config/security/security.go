package security

import "time"

// SecurityConfig defines security-related configuration
type SecurityConfig struct {
	JWT        JWTConfig        `yaml:"jwt"`
	TLS        TLSConfig        `yaml:"tls"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

// JWTConfig defines JSON Web Token configuration
type JWTConfig struct {
	SecretKey   string        `yaml:"secret_key"`
	ExpiryTime  time.Duration `yaml:"expiry_time"`
	RefreshTime time.Duration `yaml:"refresh_time"`
	Issuer      string        `yaml:"issuer"`
	Audience    string        `yaml:"audience"`
}

// TLSConfig defines TLS/SSL configuration
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`
	CAFile     string `yaml:"ca_file"`
	MinVersion string `yaml:"min_version"`
	MaxVersion string `yaml:"max_version"`
}

// MonitoringConfig defines monitoring and observability configuration
type MonitoringConfig struct {
	Enabled         bool             `yaml:"enabled"`
	EnableMetrics   bool             `yaml:"enable_metrics"`   // Backward compatibility
	MetricsEndpoint string           `yaml:"metrics_endpoint"` // Backward compatibility
	Jaeger          JaegerConfig     `yaml:"jaeger"`
	Prometheus      PrometheusConfig `yaml:"prometheus"`
}

// JaegerConfig defines Jaeger tracing configuration
type JaegerConfig struct {
	Enabled       bool          `yaml:"enabled"`
	Endpoint      string        `yaml:"endpoint"`
	ServiceName   string        `yaml:"service_name"`
	SamplingRate  float64       `yaml:"sampling_rate"`
	FlushInterval time.Duration `yaml:"flush_interval"`
}

// PrometheusConfig defines Prometheus metrics configuration
type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}
