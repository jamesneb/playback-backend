package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Constants for tests
const (
	ResponseStatusSuccess = "success"
)

// Mock implementations for testing (MockEventPublisher is in test_mocks.go)

type MockMetadataExtractor struct {
	mock.Mock
}

func (m *MockMetadataExtractor) ExtractMetadata(data []byte) OTLPMetadata {
	args := m.Called(data)
	return args.Get(0).(OTLPMetadata)
}

type MockTelemetryProcessor struct {
	mock.Mock
}

func (m *MockTelemetryProcessor) ProcessTelemetryData(ctx context.Context, data []byte, metadata *OTLPMetadata) error {
	args := m.Called(ctx, data, metadata)
	return args.Error(0)
}

func TestNewBaseTelemetryHandler(t *testing.T) {
	mockPublisher := &MockEventPublisher{}
	mockExtractor := &MockMetadataExtractor{}
	mockProcessor := &MockTelemetryProcessor{}

	handler := NewBaseTelemetryHandler(
		mockPublisher,
		mockExtractor,
		mockProcessor,
		TelemetryTrace,
	)

	assert.NotNil(t, handler)
	assert.Equal(t, TelemetryTrace, handler.telemetryType)
	assert.Equal(t, mockPublisher, handler.eventPublisher)
	assert.Equal(t, mockExtractor, handler.extractor)
	assert.Equal(t, mockProcessor, handler.processor)
	assert.NotNil(t, handler.logFields)
	assert.Equal(t, 0, len(handler.logFields))
	assert.Equal(t, 8, cap(handler.logFields))
}

func TestTelemetryType_String(t *testing.T) {
	tests := []struct {
		name     string
		t        TelemetryType
		expected string
	}{
		{"trace", TelemetryTrace, "trace"},
		{"metric", TelemetryMetric, "metric"},
		{"log", TelemetryLog, "log"},
		{"unknown", TelemetryType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.t.String())
		})
	}
}

func TestBaseTelemetryHandler_HandleIngestion_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockPublisher := &MockEventPublisher{}
	mockExtractor := &MockMetadataExtractor{}
	mockProcessor := &MockTelemetryProcessor{}

	handler := NewBaseTelemetryHandler(
		mockPublisher,
		mockExtractor,
		mockProcessor,
		TelemetryTrace,
	)

	// Setup mocks
	testMetadata := OTLPMetadata{
		ServiceName: "test-service",
		TraceID:     "test-trace-id",
		Count:       5,
		DataSize:    100,
	}

	mockExtractor.On("ExtractMetadata", mock.AnythingOfType("[]uint8")).Return(testMetadata)
	mockProcessor.On("ProcessTelemetryData", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("*handlers.OTLPMetadata")).Return(nil)

	// Create test request
	testData := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"instrumentationLibrarySpans":[{"spans":[{"traceId":"test-trace-id","spanId":"test-span-id","name":"test-span"}]}]}]}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/traces", strings.NewReader(testData))
	c.Request.Header.Set("Content-Type", "application/json")

	// Execute
	handler.HandleIngestion(c)

	// Assertions
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response TraceResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, ResponseStatusSuccess, response.Status)
	assert.Contains(t, response.Message, "5 spans")
	assert.Equal(t, "test-service", response.ServiceName)
	assert.Equal(t, "test-trace-id", response.TraceID)

	mockExtractor.AssertExpectations(t)
	mockProcessor.AssertExpectations(t)
}

func TestBaseTelemetryHandler_HandleIngestion_InvalidContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewBaseTelemetryHandler(
		&MockEventPublisher{},
		&MockMetadataExtractor{},
		&MockTelemetryProcessor{},
		TelemetryTrace,
	)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/traces", strings.NewReader("{}"))
	c.Request.Header.Set("Content-Type", "text/plain")

	handler.HandleIngestion(c)

	assert.Equal(t, http.StatusUnsupportedMediaType, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid content type", response.Error)
}

func TestBaseTelemetryHandler_HandleIngestion_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewBaseTelemetryHandler(
		&MockEventPublisher{},
		&MockMetadataExtractor{},
		&MockTelemetryProcessor{},
		TelemetryTrace,
	)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/traces", strings.NewReader("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleIngestion(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Invalid OTLP trace data", response.Error)
}

func TestBaseTelemetryHandler_HandleIngestion_ProcessingError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockPublisher := &MockEventPublisher{}
	mockExtractor := &MockMetadataExtractor{}
	mockProcessor := &MockTelemetryProcessor{}

	handler := NewBaseTelemetryHandler(
		mockPublisher,
		mockExtractor,
		mockProcessor,
		TelemetryTrace,
	)

	// Setup mocks
	testMetadata := OTLPMetadata{
		ServiceName: "test-service",
		TraceID:     "test-trace-id",
		Count:       5,
		DataSize:    100,
	}

	mockExtractor.On("ExtractMetadata", mock.AnythingOfType("[]uint8")).Return(testMetadata)
	mockProcessor.On("ProcessTelemetryData", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("*handlers.OTLPMetadata")).Return(assert.AnError)

	testData := `{"resourceSpans":[]}`

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/traces", strings.NewReader(testData))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.HandleIngestion(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to process trace data", response.Error)

	mockExtractor.AssertExpectations(t)
	mockProcessor.AssertExpectations(t)
}

func TestContainsJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		expected    bool
	}{
		{"valid json", "application/json", true},
		{"json with charset", "application/json; charset=utf-8", true},
		{"text plain", "text/plain", false},
		{"empty", "", false},
		{"short string", "app", false},
		{"xml", "application/xml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, containsJSON(tt.contentType))
		})
	}
}

func TestMetadataExtractors(t *testing.T) {
	t.Run("TraceMetadataExtractor", func(t *testing.T) {
		extractor := &TraceMetadataExtractor{}

		testData := []byte(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"instrumentationLibrarySpans":[{"spans":[{"traceId":"test-trace","name":"span1"},{"traceId":"test-trace","name":"span2"}]}]}]}`)

		metadata := extractor.ExtractMetadata(testData)

		assert.Equal(t, "test-service", metadata.ServiceName)
		assert.Equal(t, "test-trace", metadata.TraceID)
		assert.Equal(t, int32(2), metadata.Count)
	})

	t.Run("MetricMetadataExtractor", func(t *testing.T) {
		extractor := &MetricMetadataExtractor{}

		testData := []byte(`{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"scopeMetrics":[{"metrics":[{"name":"metric1"},{"name":"metric2"}]}]}]}`)

		metadata := extractor.ExtractMetadata(testData)

		assert.Equal(t, "test-service", metadata.ServiceName)
		assert.Equal(t, "", metadata.TraceID)
		assert.Equal(t, int32(2), metadata.Count)
	})

	t.Run("LogMetadataExtractor", func(t *testing.T) {
		extractor := &LogMetadataExtractor{}

		testData := []byte(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"scopeLogs":[{"logRecords":[{"traceId":"test-trace","body":{"stringValue":"log1"}},{"traceId":"test-trace","body":{"stringValue":"log2"}}]}]}]}`)

		metadata := extractor.ExtractMetadata(testData)

		assert.Equal(t, "test-service", metadata.ServiceName)
		assert.Equal(t, "test-trace", metadata.TraceID)
		assert.Equal(t, int32(2), metadata.Count)
	})
}

func TestStreamingTelemetryProcessor(t *testing.T) {
	mockPublisher := &MockEventPublisher{}

	processor := NewStreamingTelemetryProcessor(mockPublisher, TelemetryTrace)

	assert.NotNil(t, processor)
	assert.Equal(t, mockPublisher, processor.eventPublisher)
	assert.Equal(t, TelemetryTrace, processor.telemetryType)

	// Test ProcessTelemetryData
	testData := []byte(`{"test": "data"}`)
	metadata := &OTLPMetadata{
		ServiceName: "test-service",
		TraceID:     "test-trace-id",
		Count:       1,
		DataSize:    20,
	}

	mockPublisher.On("PublishTrace", mock.Anything, mock.AnythingOfType("json.RawMessage"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	err := processor.ProcessTelemetryData(context.Background(), testData, metadata)

	assert.NoError(t, err)
	mockPublisher.AssertExpectations(t)
}

func BenchmarkBaseTelemetryHandler_HandleIngestion(b *testing.B) {
	gin.SetMode(gin.TestMode)

	mockPublisher := &MockEventPublisher{}
	mockExtractor := &MockMetadataExtractor{}
	mockProcessor := &MockTelemetryProcessor{}

	handler := NewBaseTelemetryHandler(
		mockPublisher,
		mockExtractor,
		mockProcessor,
		TelemetryTrace,
	)

	testMetadata := OTLPMetadata{
		ServiceName: "test-service",
		TraceID:     "test-trace-id",
		Count:       5,
		DataSize:    100,
	}

	mockExtractor.On("ExtractMetadata", mock.AnythingOfType("[]uint8")).Return(testMetadata)
	mockProcessor.On("ProcessTelemetryData", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("[]uint8"), mock.AnythingOfType("*handlers.OTLPMetadata")).Return(nil)

	testData := `{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"test-service"}}]},"instrumentationLibrarySpans":[{"spans":[{"traceId":"test-trace-id","spanId":"test-span-id","name":"test-span"}]}]}]}`

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("POST", "/api/v1/traces", strings.NewReader(testData))
		c.Request.Header.Set("Content-Type", "application/json")

		handler.HandleIngestion(c)
	}
}
