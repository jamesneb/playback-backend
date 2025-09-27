package resilience

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// DeadLetterQueue handles failed telemetry events for retry processing
type DeadLetterQueue struct {
	sqsClient *sqs.Client
	queueURL  string
	
	// Local buffer for when SQS itself fails
	localBuffer     []*FailedEvent
	localBufferSize int
	mutex           sync.RWMutex
	
	// Retry settings
	maxRetries       int
	retryBaseDelay   time.Duration
	retryMaxDelay    time.Duration
	retryMultiplier  float64
}

// FailedEvent represents an event that failed to process
type FailedEvent struct {
	Event            streaming.TelemetryEvent `json:"event"`
	OriginalError    string                   `json:"original_error"`
	FailureTime      time.Time                `json:"failure_time"`
	RetryCount       int                      `json:"retry_count"`
	LastRetryTime    time.Time                `json:"last_retry_time,omitempty"`
	TenantID         string                   `json:"tenant_id"`
	FailureReason    string                   `json:"failure_reason"`
	SourceEndpoint   string                   `json:"source_endpoint"` // grpc or http
	Metadata         map[string]interface{}   `json:"metadata,omitempty"`
}

// DLQConfig holds configuration for the dead letter queue
type DLQConfig struct {
	QueueURL         string
	LocalBufferSize  int
	MaxRetries       int
	RetryBaseDelay   time.Duration
	RetryMaxDelay    time.Duration
	RetryMultiplier  float64
}

// NewDeadLetterQueue creates a new dead letter queue manager
func NewDeadLetterQueue(awsConfig aws.Config, config DLQConfig) *DeadLetterQueue {
	if config.LocalBufferSize == 0 {
		config.LocalBufferSize = 1000
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryBaseDelay == 0 {
		config.RetryBaseDelay = 5 * time.Second
	}
	if config.RetryMaxDelay == 0 {
		config.RetryMaxDelay = 5 * time.Minute
	}
	if config.RetryMultiplier == 0 {
		config.RetryMultiplier = 2.0
	}

	dlq := &DeadLetterQueue{
		sqsClient:       sqs.NewFromConfig(awsConfig),
		queueURL:        config.QueueURL,
		localBuffer:     make([]*FailedEvent, 0, config.LocalBufferSize),
		localBufferSize: config.LocalBufferSize,
		maxRetries:      config.MaxRetries,
		retryBaseDelay:  config.RetryBaseDelay,
		retryMaxDelay:   config.RetryMaxDelay,
		retryMultiplier: config.RetryMultiplier,
	}

	return dlq
}

// SendToDLQ sends a failed event to the dead letter queue
func (dlq *DeadLetterQueue) SendToDLQ(ctx context.Context, event streaming.TelemetryEvent, 
	originalError error, tenantID, sourceEndpoint, reason string) error {
	
	failedEvent := &FailedEvent{
		Event:          event,
		OriginalError:  originalError.Error(),
		FailureTime:    time.Now(),
		RetryCount:     0,
		TenantID:       tenantID,
		FailureReason:  reason,
		SourceEndpoint: sourceEndpoint,
		Metadata: map[string]interface{}{
			"service_name": event.GetServiceName(),
			"event_type":   event.GetType(),
			"trace_id":     event.GetTraceID(),
		},
	}

	// Try to send to SQS first
	if err := dlq.sendToSQS(ctx, failedEvent); err != nil {
		logger.Warn("Failed to send to SQS DLQ, using local buffer",
			zap.Error(err),
			zap.String("tenant", tenantID))
		
		// Fallback to local buffer
		return dlq.addToLocalBuffer(failedEvent)
	}

	logger.Info("Sent failed event to DLQ",
		zap.String("tenant", tenantID),
		zap.String("reason", reason),
		zap.String("endpoint", sourceEndpoint))

	return nil
}

// ProcessRetries processes events from the DLQ for retry
func (dlq *DeadLetterQueue) ProcessRetries(ctx context.Context, processor func(streaming.TelemetryEvent) error) error {
	// Process local buffer first
	dlq.processLocalBuffer(ctx, processor)
	
	// Then process SQS messages
	return dlq.processSQSMessages(ctx, processor)
}

func (dlq *DeadLetterQueue) sendToSQS(ctx context.Context, failedEvent *FailedEvent) error {
	messageBody, err := json.Marshal(failedEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal failed event: %w", err)
	}

	// Calculate delay for retry (exponential backoff)
	delay := dlq.calculateRetryDelay(failedEvent.RetryCount)

	input := &sqs.SendMessageInput{
		QueueUrl:     &dlq.queueURL,
		MessageBody:  aws.String(string(messageBody)),
		DelaySeconds: int32(delay.Seconds()),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"TenantID": {
				DataType:    aws.String("String"),
				StringValue: &failedEvent.TenantID,
			},
			"SourceEndpoint": {
				DataType:    aws.String("String"),
				StringValue: &failedEvent.SourceEndpoint,
			},
			"RetryCount": {
				DataType:    aws.String("Number"),
				StringValue: aws.String(fmt.Sprintf("%d", failedEvent.RetryCount)),
			},
			"EventType": {
				DataType:    aws.String("String"),
				StringValue: aws.String(string(failedEvent.Event.GetType())),
			},
		},
	}

	_, err = dlq.sqsClient.SendMessage(ctx, input)
	return err
}

func (dlq *DeadLetterQueue) addToLocalBuffer(failedEvent *FailedEvent) error {
	dlq.mutex.Lock()
	defer dlq.mutex.Unlock()

	// Check if buffer is full
	if len(dlq.localBuffer) >= dlq.localBufferSize {
		// Remove oldest event (FIFO)
		dlq.localBuffer = dlq.localBuffer[1:]
		logger.Warn("Local DLQ buffer full, dropped oldest event")
	}

	dlq.localBuffer = append(dlq.localBuffer, failedEvent)
	return nil
}

func (dlq *DeadLetterQueue) processLocalBuffer(ctx context.Context, processor func(streaming.TelemetryEvent) error) {
	dlq.mutex.Lock()
	eventsToProcess := make([]*FailedEvent, len(dlq.localBuffer))
	copy(eventsToProcess, dlq.localBuffer)
	dlq.localBuffer = dlq.localBuffer[:0] // Clear the buffer
	dlq.mutex.Unlock()

	for _, failedEvent := range eventsToProcess {
		select {
		case <-ctx.Done():
			// Put unprocessed events back
			dlq.mutex.Lock()
			dlq.localBuffer = append(dlq.localBuffer, failedEvent)
			dlq.mutex.Unlock()
			return
		default:
			if err := dlq.retryEvent(ctx, failedEvent, processor); err != nil {
				logger.Error("Failed to retry event from local buffer", zap.Error(err))
			}
		}
	}
}

func (dlq *DeadLetterQueue) processSQSMessages(ctx context.Context, processor func(streaming.TelemetryEvent) error) error {
	input := &sqs.ReceiveMessageInput{
		QueueUrl:            &dlq.queueURL,
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20, // Long polling
		MessageAttributeNames: []string{"All"},
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			result, err := dlq.sqsClient.ReceiveMessage(ctx, input)
			if err != nil {
				logger.Error("Failed to receive messages from DLQ", zap.Error(err))
				// Use context-aware sleep instead of blocking time.Sleep
				select {
				case <-time.After(5 * time.Second):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}

			if len(result.Messages) == 0 {
				continue
			}

			for _, message := range result.Messages {
				if err := dlq.processMessage(ctx, message, processor); err != nil {
					logger.Error("Failed to process DLQ message", zap.Error(err))
				}
			}
		}
	}
}

func (dlq *DeadLetterQueue) processMessage(ctx context.Context, message types.Message, processor func(streaming.TelemetryEvent) error) error {
	var failedEvent FailedEvent
	if err := json.Unmarshal([]byte(*message.Body), &failedEvent); err != nil {
		// Delete invalid message
		dlq.deleteMessage(ctx, message.ReceiptHandle)
		return fmt.Errorf("failed to unmarshal DLQ message: %w", err)
	}

	// Try to retry the event
	if err := dlq.retryEvent(ctx, &failedEvent, processor); err != nil {
		// If retry failed and we haven't exceeded max retries, put it back with delay
		if failedEvent.RetryCount < dlq.maxRetries {
			failedEvent.RetryCount++
			failedEvent.LastRetryTime = time.Now()
			
			// Send back to queue with exponential backoff
			if sendErr := dlq.sendToSQS(ctx, &failedEvent); sendErr != nil {
				logger.Error("Failed to re-queue failed event", zap.Error(sendErr))
			}
		} else {
			// Max retries exceeded - log and drop
			logger.Error("Event exceeded max retries, dropping",
				zap.String("tenant", failedEvent.TenantID),
				zap.Int("retry_count", failedEvent.RetryCount),
				zap.String("original_error", failedEvent.OriginalError))
		}
		
		// Delete the message from queue
		dlq.deleteMessage(ctx, message.ReceiptHandle)
		return err
	}

	// Success - delete the message
	dlq.deleteMessage(ctx, message.ReceiptHandle)
	
	logger.Info("Successfully retried event from DLQ",
		zap.String("tenant", failedEvent.TenantID),
		zap.Int("retry_count", failedEvent.RetryCount))

	return nil
}

func (dlq *DeadLetterQueue) retryEvent(ctx context.Context, failedEvent *FailedEvent, processor func(streaming.TelemetryEvent) error) error {
	// Add context with retry information
	logger.Debug("Retrying failed event",
		zap.String("tenant", failedEvent.TenantID),
		zap.Int("retry_count", failedEvent.RetryCount),
		zap.String("original_failure", failedEvent.FailureReason))

	return processor(failedEvent.Event)
}

func (dlq *DeadLetterQueue) deleteMessage(ctx context.Context, receiptHandle *string) {
	_, err := dlq.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &dlq.queueURL,
		ReceiptHandle: receiptHandle,
	})
	if err != nil {
		logger.Error("Failed to delete message from DLQ", zap.Error(err))
	}
}

func (dlq *DeadLetterQueue) calculateRetryDelay(retryCount int) time.Duration {
	delay := time.Duration(float64(dlq.retryBaseDelay) * 
		pow(dlq.retryMultiplier, float64(retryCount)))
	
	if delay > dlq.retryMaxDelay {
		delay = dlq.retryMaxDelay
	}
	
	return delay
}

// Simple power function since math.Pow requires float64
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}

// GetStats returns statistics about the DLQ
func (dlq *DeadLetterQueue) GetStats(ctx context.Context) (*DLQStats, error) {
	// Get SQS queue attributes
	attrs, err := dlq.sqsClient.GetQueueAttributes(ctx, &sqs.GetQueueAttributesInput{
		QueueUrl: &dlq.queueURL,
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameApproximateNumberOfMessages,
			types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	if err != nil {
		return nil, err
	}

	dlq.mutex.RLock()
	localBufferSize := len(dlq.localBuffer)
	dlq.mutex.RUnlock()

	stats := &DLQStats{
		LocalBufferSize: localBufferSize,
		LocalBufferCap:  dlq.localBufferSize,
	}

	if approxMsgs, ok := attrs.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)]; ok {
		if _, err := fmt.Sscanf(approxMsgs, "%d", &stats.SQSVisibleMessages); err != nil {
			// Log the error but don't fail since this is just for stats
			logger.Warn("Failed to parse SQS visible messages count", zap.Error(err), zap.String("value", approxMsgs))
		}
	}
	
	if approxNotVisible, ok := attrs.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessagesNotVisible)]; ok {
		if _, err := fmt.Sscanf(approxNotVisible, "%d", &stats.SQSInFlightMessages); err != nil {
			// Log the error but don't fail since this is just for stats
			logger.Warn("Failed to parse SQS in-flight messages count", zap.Error(err), zap.String("value", approxNotVisible))
		}
	}

	return stats, nil
}

type DLQStats struct {
	LocalBufferSize     int `json:"local_buffer_size"`
	LocalBufferCap      int `json:"local_buffer_capacity"`
	SQSVisibleMessages  int `json:"sqs_visible_messages"`
	SQSInFlightMessages int `json:"sqs_inflight_messages"`
}

// StartRetryProcessor starts a background goroutine to process DLQ retries
func (dlq *DeadLetterQueue) StartRetryProcessor(ctx context.Context, processor func(streaming.TelemetryEvent) error, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Info("Starting DLQ retry processor", zap.Duration("interval", interval))

	go func() {
		for {
			select {
			case <-ctx.Done():
				logger.Info("DLQ retry processor stopped")
				return
			case <-ticker.C:
				if err := dlq.ProcessRetries(ctx, processor); err != nil {
					logger.Error("Error processing DLQ retries", zap.Error(err))
				}
			}
		}
	}()
}