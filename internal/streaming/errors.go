package streaming

import "errors"

// Validation errors for telemetry events
var (
	ErrInvalidTraceData    = errors.New("invalid trace data: resource spans is nil")
	ErrInvalidMetricsData  = errors.New("invalid metrics data: resource metrics is nil")
	ErrInvalidLogsData     = errors.New("invalid logs data: resource logs is nil")
	ErrMissingServiceName  = errors.New("missing service name")
	ErrEmptySpanData       = errors.New("empty span data: no scope spans found")
	ErrEmptyMetricsData    = errors.New("empty metrics data: no scope metrics found")
	ErrEmptyLogsData       = errors.New("empty logs data: no scope logs found")
	ErrUnsupportedEventType = errors.New("unsupported telemetry event type")
)

// Processing errors
var (
	ErrHandlerNotFound     = errors.New("handler not found for event type")
	ErrKinesisPublishFailed = errors.New("failed to publish to Kinesis")
	ErrClickHouseWriteFailed = errors.New("failed to write to ClickHouse")
	ErrSerializationFailed = errors.New("failed to serialize event data")
)