package handlers

import (
	"context"
	"encoding/json"

	"github.com/jamesneb/playback-backend/pkg/telemetry"
	"github.com/stretchr/testify/mock"
)

// MockEventPublisher is a mock implementation of telemetry.EventPublisher for testing
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

// Ensure MockEventPublisher implements telemetry.EventPublisher
var _ telemetry.EventPublisher = (*MockEventPublisher)(nil)
