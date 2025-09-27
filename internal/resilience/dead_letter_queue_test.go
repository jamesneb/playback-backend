package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/stretchr/testify/assert"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
)

func TestNewDeadLetterQueue(t *testing.T) {
	config := DLQConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
		LocalBufferSize: 1000,
		MaxRetries:      3,
		RetryBaseDelay:  time.Second,
		RetryMaxDelay:   time.Minute,
		RetryMultiplier: 2.0,
	}

	awsConfig := aws.Config{
		Region: "us-east-1",
	}

	dlq := NewDeadLetterQueue(awsConfig, config)

	assert.NotNil(t, dlq)
	assert.Equal(t, config.QueueURL, dlq.queueURL)
	assert.Equal(t, config.MaxRetries, dlq.maxRetries)
	assert.Equal(t, config.LocalBufferSize, dlq.localBufferSize)
}

func TestDeadLetterQueueDefaults(t *testing.T) {
	config := DLQConfig{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
		// Leave other fields as zero to test defaults
	}

	awsConfig := aws.Config{
		Region: "us-east-1",
	}

	dlq := NewDeadLetterQueue(awsConfig, config)

	assert.NotNil(t, dlq)
	assert.Equal(t, 1000, dlq.localBufferSize)         // Default local buffer size
	assert.Equal(t, 3, dlq.maxRetries)                 // Default max retries
	assert.Equal(t, 5*time.Second, dlq.retryBaseDelay) // Default base delay
	assert.Equal(t, 5*time.Minute, dlq.retryMaxDelay)  // Default max delay
	assert.Equal(t, 2.0, dlq.retryMultiplier)          // Default multiplier
}

func TestDeadLetterQueueSendToDLQ(t *testing.T) {
	config := DLQConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
		LocalBufferSize: 10,
		MaxRetries:      3,
	}

	awsConfig := aws.Config{
		Region: "us-east-1",
	}

	dlq := NewDeadLetterQueue(awsConfig, config)

	// Create a test event using LogsTelemetryEvent as concrete implementation
	testEvent := &streaming.LogsTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeLogs,
			ServiceName: "test-service",
			TraceID:     "test-trace-id",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   "127.0.0.1",
			},
		},
		ResourceLogs: &logspb.ResourceLogs{
			ScopeLogs: []*logspb.ScopeLogs{
				{
					LogRecords: []*logspb.LogRecord{
						{
							Body: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test log message",
								},
							},
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	originalError := errors.New("test error")

	// Since we can't easily mock SQS in this context, we'll test that the method
	// doesn't panic and handles the basic structure correctly
	// The DLQ implementation falls back to local buffer when SQS fails
	err := dlq.SendToDLQ(ctx, testEvent, originalError, "test-tenant", "grpc", "connection failed")

	// The DLQ should gracefully handle SQS failures by using local buffer
	// So this should succeed (return nil error) even without AWS credentials
	assert.NoError(t, err, "DLQ should gracefully fall back to local buffer when SQS fails")
}

func TestFailedEventStructure(t *testing.T) {
	testEvent := &streaming.LogsTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeLogs,
			ServiceName: "test-service",
			TraceID:     "test-trace-id",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   "127.0.0.1",
			},
		},
		ResourceLogs: &logspb.ResourceLogs{
			ScopeLogs: []*logspb.ScopeLogs{
				{
					LogRecords: []*logspb.LogRecord{
						{
							Body: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "test trace data",
								},
							},
						},
					},
				},
			},
		},
	}

	failedEvent := &FailedEvent{
		Event:          testEvent,
		OriginalError:  "connection timeout",
		FailureTime:    time.Now(),
		RetryCount:     1,
		LastRetryTime:  time.Now(),
		TenantID:       "tenant-123",
		FailureReason:  "network error",
		SourceEndpoint: "grpc",
		Metadata: map[string]interface{}{
			"attempt": 1,
			"region":  "us-east-1",
		},
	}

	assert.Equal(t, testEvent, failedEvent.Event)
	assert.Equal(t, "connection timeout", failedEvent.OriginalError)
	assert.Equal(t, 1, failedEvent.RetryCount)
	assert.Equal(t, "tenant-123", failedEvent.TenantID)
	assert.Equal(t, "network error", failedEvent.FailureReason)
	assert.Equal(t, "grpc", failedEvent.SourceEndpoint)
	assert.NotNil(t, failedEvent.Metadata)
}

func TestDLQConfigValidation(t *testing.T) {
	tests := []struct {
		name            string
		config          DLQConfig
		expectedBuffer  int
		expectedRetries int
	}{
		{
			name: "all defaults",
			config: DLQConfig{
				QueueURL: "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
			},
			expectedBuffer:  1000,
			expectedRetries: 3,
		},
		{
			name: "custom values",
			config: DLQConfig{
				QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
				LocalBufferSize: 500,
				MaxRetries:      5,
			},
			expectedBuffer:  500,
			expectedRetries: 5,
		},
	}

	awsConfig := aws.Config{Region: "us-east-1"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dlq := NewDeadLetterQueue(awsConfig, tt.config)
			assert.Equal(t, tt.expectedBuffer, dlq.localBufferSize)
			assert.Equal(t, tt.expectedRetries, dlq.maxRetries)
		})
	}
}

// TestDeadLetterQueueRetryBackoff tests the exponential backoff calculation
func TestDeadLetterQueueRetryBackoff(t *testing.T) {
	config := DLQConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
		RetryBaseDelay:  time.Second,
		RetryMaxDelay:   time.Minute,
		RetryMultiplier: 2.0,
	}

	awsConfig := aws.Config{Region: "us-east-1"}
	dlq := NewDeadLetterQueue(awsConfig, config)

	// Test that backoff settings are correctly stored
	assert.Equal(t, time.Second, dlq.retryBaseDelay)
	assert.Equal(t, time.Minute, dlq.retryMaxDelay)
	assert.Equal(t, 2.0, dlq.retryMultiplier)
}

// TestDeadLetterQueueConcurrency tests that the DLQ can handle concurrent operations
func TestDeadLetterQueueConcurrency(t *testing.T) {
	config := DLQConfig{
		QueueURL:        "https://sqs.us-east-1.amazonaws.com/123456789012/test-dlq",
		LocalBufferSize: 100,
		MaxRetries:      3,
	}

	awsConfig := aws.Config{Region: "us-east-1"}
	dlq := NewDeadLetterQueue(awsConfig, config)

	ctx := context.Background()
	testEvent := &streaming.LogsTelemetryEvent{
		BaseTelemetryEvent: streaming.BaseTelemetryEvent{
			Type:        streaming.TelemetryTypeLogs,
			ServiceName: "concurrent-test",
			TraceID:     "concurrent-trace-id",
			Metadata: streaming.TelemetryMetadata{
				IngestedAt: time.Now(),
				SourceIP:   "127.0.0.1",
			},
		},
		ResourceLogs: &logspb.ResourceLogs{
			ScopeLogs: []*logspb.ScopeLogs{
				{
					LogRecords: []*logspb.LogRecord{
						{
							Body: &commonpb.AnyValue{
								Value: &commonpb.AnyValue_StringValue{
									StringValue: "concurrent test log",
								},
							},
						},
					},
				},
			},
		},
	}

	// Run multiple goroutines trying to send to DLQ
	// This mainly tests that there are no race conditions in the structure
	numGoroutines := 10
	errorChan := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			err := dlq.SendToDLQ(ctx, testEvent,
				errors.New("test error"),
				"tenant-"+string(rune(id)), "grpc", "test")
			errorChan <- err
		}(i)
	}

	// Collect all results (should succeed due to local buffer fallback)
	for i := 0; i < numGoroutines; i++ {
		err := <-errorChan
		// DLQ should gracefully handle SQS failures with local buffer
		assert.NoError(t, err, "DLQ should handle concurrent requests gracefully with fallback")
	}
}
