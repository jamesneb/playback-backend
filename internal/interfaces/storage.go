package interfaces

import (
	"context"

	"github.com/jamesneb/playback-backend/internal/streaming"
)

// ClickHouseRepository defines the interface for ClickHouse database operations
type ClickHouseRepository interface {
	// InsertTrace inserts legacy JSON trace data
	InsertTrace(ctx context.Context, event streaming.TelemetryEvent) error

	// InsertTraceProtobuf inserts protobuf trace data with structured processing
	InsertTraceProtobuf(ctx context.Context, event *streaming.TraceTelemetryEvent) error

	// InsertMetric inserts legacy JSON metric data
	InsertMetric(ctx context.Context, event streaming.TelemetryEvent) error

	// InsertMetricProtobuf inserts protobuf metric data
	InsertMetricProtobuf(ctx context.Context, event *streaming.MetricsTelemetryEvent) error

	// InsertLog inserts legacy JSON log data
	InsertLog(ctx context.Context, event streaming.TelemetryEvent) error

	// InsertLogProtobuf inserts protobuf log data
	InsertLogProtobuf(ctx context.Context, event *streaming.LogsTelemetryEvent) error

	// Close closes the database connection
	Close() error
}

// S3Repository defines the interface for S3 object storage operations
type S3Repository interface {
	// UploadObject uploads an object to S3
	UploadObject(ctx context.Context, bucket, key string, data []byte) error

	// DownloadObject downloads an object from S3
	DownloadObject(ctx context.Context, bucket, key string) ([]byte, error)

	// DeleteObject deletes an object from S3
	DeleteObject(ctx context.Context, bucket, key string) error

	// ListObjects lists objects in S3 with a given prefix
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)
}