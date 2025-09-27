package constants

import (
	"compress/gzip"
	"regexp"
	"time"
)

// Type aliases for application-level constants
type BufferSize int

// Application constants
const (
	LocalHost                string        = "localhost"
	ReplayS3BucketName       string        = "replays"
	DefaultCompressionLevel  int           = gzip.DefaultCompression
	CompressionMinSize       int           = 1024      // Don't compress responses smaller than 1KB
	MaxCompressionBufferSize int           = 64 * 1024 // 64KB max buffer
	MaxMultipartMemory       int64         = 32 << 20  // 32 MB
	StandardTimeFormat       string        = "2006-01-02 15:04:05"
	MaxRouteSearchIterations int           = 1000
	HashComputationTimeout   time.Duration = 100 * time.Millisecond
)

// Protocol and Response constants
const (
	ProtocolHTTPJSON = "HTTP/JSON"
	ProtocolGRPCOTLP = "gRPC/OTLP"

	MetricsPlaceholderContent = "# Metrics endpoint placeholder\n# Prometheus metrics would be served here\n"
	PprofPlaceholderContent   = "# pprof endpoint placeholder\n# Performance profiling would be served here\n"
)

// Numeric constants
const (
	SecondsPerHour        = 3600
	DependencyKeyLength   = 16
	MaxPanicMessageLength = 200
	HourPrecisionDivisor  = 3600
	ChannelBufferSize     = 1
	BytesPerMegabyte      = 1024 * 1024
)

// Dependency Hash Component constants
const (
	HashComponentKinesis    = "kinesis:present"
	HashComponentS3         = "s3:present"
	HashComponentResilience = "resilience:present"
	HashComponentClickHouse = "clickhouse:present"
	TimestampHashFormat     = "ts:%d"
)

var (
	// Precompiled regex for version sanitization
	VersionSanitizeRegex = regexp.MustCompile(`[^a-zA-Z0-9.-]`)
)
