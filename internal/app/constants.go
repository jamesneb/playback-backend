package app

import "time"

// Timeouts and durations
const (
	KinesisShutdownTimeout time.Duration = 30 * time.Second
)

// Common error messages
const (
	ErrClickHouseInit       = "failed to initialize ClickHouse client: %w"
	ErrKinesisClientInit    = "failed to initialize Kinesis client: %w"
	ErrS3ClientInit         = "failed to initialize S3 client: %w"
	ErrResilienceInit       = "failed to initialize resilience components: %w"
	ErrClickHouseClose      = "ClickHouse close error: %w"
	ErrKinesisClose         = "kinesis close error: %w"
	ErrServiceClose         = "service close errors: %v"
	ErrConsumerServiceClose = "consumer service close errors: %v"
)

// Consumer-specific error messages
const (
	ErrKinesisConsumerInit    = "failed to initialize Kinesis consumer: %w"
	ErrConsumerNotInitialized = "Consumer not initialized, call Initialize() first"
	ErrKinesisConsumerStart   = "failed to start Kinesis consumer: %w"
)

// Server-specific error messages
const (
	ErrHTTPServerStart       = "failed to start HTTP server: %w"
	ErrGRPCServerStart       = "failed to start gRPC server: %w"
	ErrRESTServerCreate      = "failed to create REST server: %w"
	ErrGRPCServiceCollection = "failed to create gRPC service collection: %w"
	ErrGRPCServerCreate      = "failed to create gRPC server: %w"
)

// Common log messages
const (
	MsgShutdownSignalReceived = "Shutdown signal received, stopping %s..."
)

// Consumer-specific log messages
const (
	MsgStartingConsumerService = "Starting Kinesis consumer service"
	MsgConsumerStoppedSuccess  = "Kinesis consumer stopped successfully"
	MsgConsumerRunning         = "Kinesis consumer is running. Press Ctrl+C to stop."
	MsgConsumerStoppedGraceful = "Consumer stopped gracefully"
	MsgConsumerShutdownTimeout = "Consumer shutdown timed out"
)

// Server-specific log messages
const (
	MsgBackendStartedSuccess    = "Playback backend started successfully"
	MsgAllServersStoppedSuccess = "All servers stopped successfully"
	MsgStartingHTTPServer       = "Starting HTTP server"
	MsgHTTPServerFailed         = "HTTP server failed"
	MsgGRPCServerFailed         = "gRPC server failed"
	MsgHTTPServerShutdownError  = "HTTP server shutdown error"
)

// Protocol strings
const (
	ProtocolHTTPJSON = "HTTP/JSON"
)
