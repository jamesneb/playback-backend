package grpcapi

import (
	"fmt"
	"time"
)

// Type aliases for better readability and type safety
type (
	Bytes       int64
	MessageSize Bytes
	Timeout     time.Duration
	ServerPort  int
)

// Bytes formatting helper
func (b Bytes) String() string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf(SizeFormatBytes, b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf(SizeFormatUnit, float64(b)/float64(div), SizeUnits[exp])
}

// Size constants using type aliases
const (
	DefaultMaxMessageSize MessageSize = 4 * 1024 * 1024  // 4MB
	MaxMessageSize        MessageSize = 16 * 1024 * 1024 // 16MB
	MinMessageSize        MessageSize = 1024             // 1KB
	BytesInKB             Bytes       = 1024

	// Protobuf size limits
	MaxProtobufTraceSize   MessageSize = 8 * 1024 * 1024 // 8MB max for trace data
	MaxProtobufMetricsSize MessageSize = 4 * 1024 * 1024 // 4MB max for metrics data
	MaxProtobufLogsSize    MessageSize = 2 * 1024 * 1024 // 2MB max for logs data
	MaxProtobufSpansCount  int         = 10000           // Max spans per request
	MaxProtobufScopeCount  int         = 100             // Max scopes per resource
)

// Timeout constants using type aliases
const (
	DefaultShutdownTimeout Timeout = Timeout(10 * time.Second)
	DefaultStartTimeout    Timeout = Timeout(5 * time.Second)
	MaxShutdownTimeout     Timeout = Timeout(30 * time.Second)
)

// Protocol strings
const (
	ProtocolOTLPgRPC = "OTLP/gRPC"
)

// Error messages
const (
	ErrConfigNil               = "gRPC server config cannot be nil"
	ErrServicesNil             = "gRPC service collection cannot be nil"
	ErrServerAddressEmpty      = "gRPC server address cannot be empty"
	ErrContextNil              = "context cannot be nil"
	ErrServiceDepsNil          = "service dependencies cannot be nil"
	ErrKinesisClientNil        = "Kinesis client cannot be nil"
	ErrClickHouseClientNil     = "ClickHouse client cannot be nil"
	ErrConfigFieldsNil         = "config cannot be nil"
	ErrResilienceCompNil       = "resilience components cannot be nil"
	ErrServerHostEmpty         = "server host cannot be empty"
	ErrServerPortInvalid       = "server port must be positive"
	ErrGRPCServerNil           = "gRPC server cannot be nil"
	ErrServiceCollectionNil    = "service collection is nil"
	ErrServerConfigNil         = "server config cannot be nil"
	ErrServicesRegistrationNil = "services cannot be nil"
	ErrFailedRegisterServices  = "failed to register services: %w"
	ErrFailedStartServices     = "failed to start services: %w"
	ErrServiceShutdownFailed   = "service shutdown failed: %w"
	ErrShutdownTimeout         = "gRPC server shutdown timeout"
	ErrCleanupErrors           = "cleanup errors: %v"
	ErrShutdownErrors          = "shutdown errors: %v"
	ErrFailedCreateHandler     = "failed to create kinesis handler for client %v"

	// Protobuf validation errors
	ErrProtobufSizeTooLarge  = "protobuf message size exceeds limit"
	ErrProtobufInvalidData   = "protobuf message contains invalid data"
	ErrProtobufSpanCount     = "too many spans in trace request"
	ErrProtobufScopeCount    = "too many scopes in resource"
	ErrProtobufMarshalFailed = "failed to marshal protobuf data"
)

// Log messages
const (
	MsgStartingGRPCServer        = "Starting gRPC server"
	MsgStoppingGRPCServer        = "Stopping gRPC server"
	MsgGRPCServerStoppedGraceful = "gRPC server stopped gracefully"
	MsgGRPCServerShutdownTimeout = "gRPC server shutdown timeout, forcing stop"
	MsgFailedStopServices        = "Failed to stop services"
)

// Size formatting constants
const (
	SizeFormatBytes = "%d B"
	SizeFormatUnit  = "%.1f %cB"
	SizeUnits       = "KMGTPE"
)
