package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEventPublisher is a mock implementation for testing
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishTrace(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	args := m.Called(ctx, data, serviceName, traceID, sourceIP, userAgent)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishMetrics(ctx context.Context, data json.RawMessage, serviceName, sourceIP, userAgent string) error {
	args := m.Called(ctx, data, serviceName, sourceIP, userAgent)
	return args.Error(0)
}

func (m *MockEventPublisher) PublishLogs(ctx context.Context, data json.RawMessage, serviceName, traceID, sourceIP, userAgent string) error {
	args := m.Called(ctx, data, serviceName, traceID, sourceIP, userAgent)
	return args.Error(0)
}

func (m *MockEventPublisher) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockTelemetryStore is a mock implementation for testing
type MockTelemetryStore struct {
	mock.Mock
}

func (m *MockTelemetryStore) InsertTrace(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockTelemetryStore) InsertMetric(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockTelemetryStore) InsertLog(ctx context.Context, event interface{}) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockTelemetryStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestEventPublisherInterface tests the EventPublisher interface
func TestEventPublisherInterface(t *testing.T) {
	mockPublisher := &MockEventPublisher{}
	ctx := context.Background()

	// Test PublishTrace
	traceData := json.RawMessage(`{"test": "trace"}`)
	mockPublisher.On("PublishTrace", ctx, traceData, "test-service", "trace-123", "127.0.0.1", "test-agent").Return(nil)

	err := mockPublisher.PublishTrace(ctx, traceData, "test-service", "trace-123", "127.0.0.1", "test-agent")
	assert.NoError(t, err)

	// Test PublishMetrics
	metricsData := json.RawMessage(`{"test": "metrics"}`)
	mockPublisher.On("PublishMetrics", ctx, metricsData, "test-service", "127.0.0.1", "test-agent").Return(nil)

	err = mockPublisher.PublishMetrics(ctx, metricsData, "test-service", "127.0.0.1", "test-agent")
	assert.NoError(t, err)

	// Test PublishLogs
	logsData := json.RawMessage(`{"test": "logs"}`)
	mockPublisher.On("PublishLogs", ctx, logsData, "test-service", "trace-123", "127.0.0.1", "test-agent").Return(nil)

	err = mockPublisher.PublishLogs(ctx, logsData, "test-service", "trace-123", "127.0.0.1", "test-agent")
	assert.NoError(t, err)

	// Test Close
	mockPublisher.On("Close").Return(nil)
	err = mockPublisher.Close()
	assert.NoError(t, err)

	mockPublisher.AssertExpectations(t)
}

// TestTelemetryStoreInterface tests the TelemetryStore interface
func TestTelemetryStoreInterface(t *testing.T) {
	mockStore := &MockTelemetryStore{}
	ctx := context.Background()

	// Test InsertTrace
	traceEvent := map[string]interface{}{"type": "trace", "data": "test"}
	mockStore.On("InsertTrace", ctx, traceEvent).Return(nil)

	err := mockStore.InsertTrace(ctx, traceEvent)
	assert.NoError(t, err)

	// Test InsertMetric
	metricEvent := map[string]interface{}{"type": "metric", "data": "test"}
	mockStore.On("InsertMetric", ctx, metricEvent).Return(nil)

	err = mockStore.InsertMetric(ctx, metricEvent)
	assert.NoError(t, err)

	// Test InsertLog
	logEvent := map[string]interface{}{"type": "log", "data": "test"}
	mockStore.On("InsertLog", ctx, logEvent).Return(nil)

	err = mockStore.InsertLog(ctx, logEvent)
	assert.NoError(t, err)

	// Test Close
	mockStore.On("Close").Return(nil)
	err = mockStore.Close()
	assert.NoError(t, err)

	mockStore.AssertExpectations(t)
}

// TestErrorHandling tests error scenarios with the interfaces
func TestErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockEventPublisher)
		operation     func(*MockEventPublisher) error
		expectedError string
	}{
		{
			name: "trace publish error",
			setupMock: func(m *MockEventPublisher) {
				m.On("PublishTrace", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("connection failed"))
			},
			operation: func(m *MockEventPublisher) error {
				return m.PublishTrace(context.Background(), json.RawMessage(`{}`), "service", "trace", "ip", "agent")
			},
			expectedError: "connection failed",
		},
		{
			name: "metrics publish timeout",
			setupMock: func(m *MockEventPublisher) {
				m.On("PublishMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(context.DeadlineExceeded)
			},
			operation: func(m *MockEventPublisher) error {
				return m.PublishMetrics(context.Background(), json.RawMessage(`{}`), "service", "ip", "agent")
			},
			expectedError: "context deadline exceeded",
		},
		{
			name: "logs publish network error",
			setupMock: func(m *MockEventPublisher) {
				m.On("PublishLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("network unreachable"))
			},
			operation: func(m *MockEventPublisher) error {
				return m.PublishLogs(context.Background(), json.RawMessage(`{}`), "service", "trace", "ip", "agent")
			},
			expectedError: "network unreachable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPublisher := &MockEventPublisher{}
			tt.setupMock(mockPublisher)

			err := tt.operation(mockPublisher)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			mockPublisher.AssertExpectations(t)
		})
	}
}

// TestInterfaceCompliance ensures our mock implementations comply with interfaces
func TestInterfaceCompliance(t *testing.T) {
	var publisher EventPublisher = &MockEventPublisher{}
	var store TelemetryStore = &MockTelemetryStore{}

	assert.NotNil(t, publisher)
	assert.NotNil(t, store)

	// Test that interfaces can be used polymorphically
	testPublisher := func(p EventPublisher) bool {
		return p != nil
	}

	testStore := func(s TelemetryStore) bool {
		return s != nil
	}

	assert.True(t, testPublisher(publisher))
	assert.True(t, testStore(store))
}
