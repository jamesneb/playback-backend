package streaming

import (
	"fmt"

	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// ProtobufSerializer handles serialization of OTLP protobuf messages
type ProtobufSerializer struct{}

// NewProtobufSerializer creates a new protobuf serializer
func NewProtobufSerializer() *ProtobufSerializer {
	return &ProtobufSerializer{}
}

// SerializeTraceData serializes protobuf ResourceSpans to bytes
func (s *ProtobufSerializer) SerializeTraceData(resourceSpans *tracepb.ResourceSpans) ([]byte, error) {
	if resourceSpans == nil {
		return nil, ErrInvalidTraceData
	}

	data, err := proto.Marshal(resourceSpans)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal trace data: %w", err)
	}

	return data, nil
}

// SerializeMetricsData serializes protobuf ResourceMetrics to bytes
func (s *ProtobufSerializer) SerializeMetricsData(resourceMetrics *metricspb.ResourceMetrics) ([]byte, error) {
	if resourceMetrics == nil {
		return nil, ErrInvalidMetricsData
	}

	data, err := proto.Marshal(resourceMetrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics data: %w", err)
	}

	return data, nil
}

// SerializeLogsData serializes protobuf ResourceLogs to bytes
func (s *ProtobufSerializer) SerializeLogsData(resourceLogs *logspb.ResourceLogs) ([]byte, error) {
	if resourceLogs == nil {
		return nil, ErrInvalidLogsData
	}

	data, err := proto.Marshal(resourceLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal logs data: %w", err)
	}

	return data, nil
}