package streaming

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
)

// StreamManager handles Kinesis stream operations and verification
type StreamManager struct {
	client  *kinesis.Client
	streams map[string]string
}

// NewStreamManager creates a new stream manager
func NewStreamManager(client *kinesis.Client, streams map[string]string) *StreamManager {
	return &StreamManager{
		client:  client,
		streams: streams,
	}
}

// GetStreamName returns the stream name for the given stream type
func (sm *StreamManager) GetStreamName(streamType string) (string, error) {
	streamName, exists := sm.streams[streamType]
	if !exists {
		return "", fmt.Errorf("unknown stream type: %s", streamType)
	}
	if streamName == "" {
		return "", fmt.Errorf("stream name not configured for type: %s", streamType)
	}
	return streamName, nil
}

// VerifyStreams checks that all configured streams exist and are accessible
func (sm *StreamManager) VerifyStreams(ctx context.Context) error {
	for streamType, streamName := range sm.streams {
		if streamName == "" {
			continue // Skip unconfigured streams
		}

		_, err := sm.client.DescribeStream(ctx, &kinesis.DescribeStreamInput{
			StreamName: aws.String(streamName),
		})
		if err != nil {
			return fmt.Errorf("failed to verify stream %s (%s): %w", streamType, streamName, err)
		}
	}
	return nil
}

// GetStreams returns a copy of the streams map
func (sm *StreamManager) GetStreams() map[string]string {
	streamsCopy := make(map[string]string)
	for k, v := range sm.streams {
		streamsCopy[k] = v
	}
	return streamsCopy
}