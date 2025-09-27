package streaming

import "time"

// StreamingConfig defines streaming and messaging configuration
type StreamingConfig struct {
	Kinesis    KinesisConfig    `yaml:"kinesis"`
	S3         S3Config         `yaml:"s3"`
	Resilience ResilienceConfig `yaml:"resilience"`
}

// KinesisConfig defines AWS Kinesis configuration
type KinesisConfig struct {
	Region             string        `yaml:"region"`
	AccessKeyID        string        `yaml:"access_key_id"`
	SecretAccessKey    string        `yaml:"secret_access_key"`
	SessionToken       string        `yaml:"session_token"`
	EndpointURL        string        `yaml:"endpoint_url"`
	TracesStreamName   string        `yaml:"traces_stream_name"`
	MetricsStreamName  string        `yaml:"metrics_stream_name"`
	LogsStreamName     string        `yaml:"logs_stream_name"`
	BatchSize          int           `yaml:"batch_size"`
	BatchTimeout       time.Duration `yaml:"batch_timeout"`
	MaxRecordSize      int           `yaml:"max_record_size"`
	MaxBatchSize       int           `yaml:"max_batch_size"`
	RetryAttempts      int           `yaml:"retry_attempts"`
	RetryDelay         time.Duration `yaml:"retry_delay"`
	EnableAsyncPublish bool          `yaml:"enable_async_publish"`
	// Legacy fields for backward compatibility
	Streams       map[string]string `yaml:"streams"`
	FlushInterval string            `yaml:"flush_interval"`
	PollInterval  time.Duration     `yaml:"poll_interval"`
}

// S3Config defines AWS S3 configuration
type S3Config struct {
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token"`
	EndpointURL     string `yaml:"endpoint_url"`
	BucketName      string `yaml:"bucket_name"`
	Bucket          string `yaml:"bucket"` // Backward compatibility
	PathPrefix      string `yaml:"path_prefix"`
	EnableSSE       bool   `yaml:"enable_sse"`
	ForcePathStyle  bool   `yaml:"force_path_style"` // Backward compatibility
}

// ResilienceConfig defines resilience patterns configuration
type ResilienceConfig struct {
	RateLimiter     RateLimiterConfig    `yaml:"rate_limiter"`
	CircuitBreaker  CircuitBreakerConfig `yaml:"circuit_breaker"`
	DeadLetterQueue DLQConfig            `yaml:"dead_letter_queue"`
	Buffer          BufferConfig         `yaml:"buffer"`
	// Legacy fields
	KinesisBuffer BufferConfig `yaml:"kinesis_buffer"` // Backward compatibility
}

// RateLimiterConfig defines rate limiting configuration
type RateLimiterConfig struct {
	RequestsPerSecond int           `yaml:"requests_per_second"`
	BurstSize         int           `yaml:"burst_size"`
	BurstCapacity     int           `yaml:"burst_capacity"` // Backward compatibility
	WindowSize        time.Duration `yaml:"window_size"`
}

// CircuitBreakerConfig defines circuit breaker configuration
type CircuitBreakerConfig struct {
	Name             string        `yaml:"name"` // Backward compatibility
	FailureRate      float64       `yaml:"failure_rate"`
	MinRequests      uint32        `yaml:"min_requests"` // Changed to uint32 for compatibility
	SuccessThreshold int           `yaml:"success_threshold"`
	Timeout          time.Duration `yaml:"timeout"`
	MaxRequests      uint32        `yaml:"max_requests"` // Changed to uint32 for compatibility
	HalfOpenRequests int           `yaml:"half_open_requests"`
	// Legacy fields
	IntervalSeconds int `yaml:"interval_seconds"` // Backward compatibility
	TimeoutSeconds  int `yaml:"timeout_seconds"`  // Backward compatibility
}

// DLQConfig defines Dead Letter Queue configuration
type DLQConfig struct {
	QueueURL         string        `yaml:"queue_url"`
	Region           string        `yaml:"region"`
	AccountID        string        `yaml:"account_id"`
	QueueName        string        `yaml:"queue_name"`
	MaxRetries       int           `yaml:"max_retries"`
	RetryBaseDelayMs int           `yaml:"retry_base_delay_ms"`
	RetryMaxDelayMs  int           `yaml:"retry_max_delay_ms"`
	MessageTTL       time.Duration `yaml:"message_ttl"`
}

// BufferConfig defines buffering configuration
type BufferConfig struct {
	MaxSize      int           `yaml:"max_size"`
	FlushTimeout time.Duration `yaml:"flush_timeout"`
	Workers      int           `yaml:"workers"`
	QueueSize    int           `yaml:"queue_size"`
	// Legacy backward compatibility
	MaxBatchSize    int `yaml:"max_batch_size"`
	MaxBatchWaitMs  int `yaml:"max_batch_wait_ms"`
	FlushIntervalMs int `yaml:"flush_interval_ms"`
	MaxTenantBuffer int `yaml:"max_tenant_buffer"`
}
