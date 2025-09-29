package streaming

import (
	"context"
	"encoding/json"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"go.uber.org/zap"
)

// StubEventPublisher is a no-op implementation for local development
type StubEventPublisher struct {
	enabled bool
}

// NewStubEventPublisher creates a stub publisher for local development
func NewStubEventPublisher() telemetry.EventPublisher {
	return &StubEventPublisher{
		enabled: true,
	}
}

// PublishTrace logs the trace event instead of sending to Kinesis
func (s *StubEventPublisher) PublishTrace(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	if !s.enabled {
		return nil
	}

	logger.Debug("Stub: Publishing trace event",
		zap.String("traceID", traceID),
		zap.String("serviceName", serviceName),
		zap.String("sourceIP", sourceIP),
		zap.Int("dataSize", len(data)))

	return nil
}

// PublishMetrics logs the metric event instead of sending to Kinesis
func (s *StubEventPublisher) PublishMetrics(ctx context.Context, data json.RawMessage, serviceName, sourceIP, userAgent string) error {
	if !s.enabled {
		return nil
	}

	logger.Debug("Stub: Publishing metric event",
		zap.String("serviceName", serviceName),
		zap.String("sourceIP", sourceIP),
		zap.Int("dataSize", len(data)))

	return nil
}

// PublishLogs logs the log event instead of sending to Kinesis
func (s *StubEventPublisher) PublishLogs(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	if !s.enabled {
		return nil
	}

	logger.Debug("Stub: Publishing log event",
		zap.String("traceID", traceID),
		zap.String("serviceName", serviceName),
		zap.String("sourceIP", sourceIP),
		zap.Int("dataSize", len(data)))

	return nil
}

// Close is a no-op for stub
func (s *StubEventPublisher) Close() error {
	s.enabled = false
	logger.Info("Stub event publisher closed")
	return nil
}