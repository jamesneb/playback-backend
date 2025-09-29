package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestHandlerErrorScenarios tests various error conditions and edge cases
func TestHandlerErrorScenarios(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		handler        func(*MockEventPublisher) http.Handler
		setupMock      func(*MockEventPublisher)
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "logs handler - publisher error",
			handler: func(mockPublisher *MockEventPublisher) http.Handler {
				router := gin.New()
				logsHandler := NewLogsHandler(mockPublisher, &interfaces.ResilienceComponents{})
				router.POST("/logs", logsHandler.CreateLogs)
				return router
			},
			setupMock: func(m *MockEventPublisher) {
				m.On("PublishLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("kinesis unavailable"))
			},
			requestBody: map[string]interface{}{
				"resourceLogs": []map[string]interface{}{
					{
						"resource": map[string]interface{}{
							"attributes": []map[string]interface{}{
								{"key": "service.name", "value": map[string]interface{}{"stringValue": "test-service"}},
							},
						},
						"scopeLogs": []map[string]interface{}{
							{
								"logRecords": []map[string]interface{}{
									{
										"timeUnixNano": "1640995200000000000",
										"body":         map[string]interface{}{"stringValue": "test log"},
									},
								},
							},
						},
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to process log data",
		},
		{
			name: "metrics handler - invalid JSON",
			handler: func(mockPublisher *MockEventPublisher) http.Handler {
				router := gin.New()
				metricsHandler := NewMetricsHandler(mockPublisher, &interfaces.ResilienceComponents{})
				router.POST("/metrics", metricsHandler.CreateMetrics)
				return router
			},
			setupMock: func(m *MockEventPublisher) {
				// No mock setup needed - should fail before reaching publisher
			},
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid OTLP metric data",
		},
		{
			name: "trace handler - network timeout simulation",
			handler: func(mockPublisher *MockEventPublisher) http.Handler {
				router := gin.New()
				traceHandler := NewTraceHandler(mockPublisher, nil)
				router.POST("/traces", traceHandler.CreateTrace)
				return router
			},
			setupMock: func(m *MockEventPublisher) {
				m.On("PublishTrace", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(context.DeadlineExceeded)
			},
			requestBody: map[string]interface{}{
				"resourceSpans": []map[string]interface{}{
					{
						"resource": map[string]interface{}{
							"attributes": []map[string]interface{}{
								{"key": "service.name", "value": map[string]interface{}{"stringValue": "test-service"}},
							},
						},
						"scopeSpans": []map[string]interface{}{
							{
								"spans": []map[string]interface{}{
									{
										"traceId": "0123456789abcdef0123456789abcdef",
										"spanId":  "0123456789abcdef",
										"name":    "test-span",
									},
								},
							},
						},
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "Failed to process trace data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPublisher := &MockEventPublisher{}
			tt.setupMock(mockPublisher)

			handler := tt.handler(mockPublisher)

			var body []byte
			if tt.requestBody != nil {
				if str, ok := tt.requestBody.(string); ok && str == "invalid json" {
					// Use the raw invalid JSON string
					body = []byte("invalid json")
				} else {
					var err error
					body, err = json.Marshal(tt.requestBody)
					require.NoError(t, err)
				}
			} else {
				body = []byte("invalid json")
			}

			// Determine the correct URL based on the handler type
			var url string
			if strings.Contains(tt.name, "logs") {
				url = "/logs"
			} else if strings.Contains(tt.name, "metrics") {
				url = "/metrics"
			} else if strings.Contains(tt.name, "trace") {
				url = "/traces"
			} else {
				url = "/"
			}
			req := httptest.NewRequest(http.MethodPost, url, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response["error"], tt.expectedError)
			}

			mockPublisher.AssertExpectations(t)
		})
	}
}

// TestConcurrentRequests tests handler behavior under concurrent load
func TestConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockPublisher := &MockEventPublisher{}
	mockPublisher.On("PublishLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(10) // Expect 10 concurrent requests

	router := gin.New()
	logsHandler := NewLogsHandler(mockPublisher, &interfaces.ResilienceComponents{})
	router.POST("/logs", logsHandler.CreateLogs)

	requestBody := map[string]interface{}{
		"resourceLogs": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []map[string]interface{}{
						{"key": "service.name", "value": map[string]interface{}{"stringValue": "concurrent-test"}},
					},
				},
				"scopeLogs": []map[string]interface{}{
					{
						"logRecords": []map[string]interface{}{
							{
								"timeUnixNano": "1640995200000000000",
								"body":         map[string]interface{}{"stringValue": "concurrent log"},
							},
						},
					},
				},
			},
		},
	}

	body, err := json.Marshal(requestBody)
	require.NoError(t, err)

	// Run 10 concurrent requests
	for i := 0; i < 10; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		}()
	}

	// Give goroutines time to complete
	time.Sleep(100 * time.Millisecond)
	mockPublisher.AssertExpectations(t)
}

// TestInputValidation tests various input validation scenarios
func TestInputValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		contentType    string
		body           string
		expectedStatus int
	}{
		{
			name:           "missing content type",
			contentType:    "",
			body:           `{"resourceLogs": []}`,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "wrong content type",
			contentType:    "text/plain",
			body:           `{"resourceLogs": []}`,
			expectedStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:           "empty body",
			contentType:    "application/json",
			body:           "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "malformed JSON",
			contentType:    "application/json",
			body:           `{"resourceLogs": [}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "extremely large payload",
			contentType:    "application/json",
			body:           `{"resourceLogs": [` + strings.Repeat(`{"resource": {"attributes": [{"key": "test", "value": {"stringValue": "`+strings.Repeat("x", 1000)+`"}}]}},`, 1000) + `]}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	mockPublisher := &MockEventPublisher{}
	// Set up a default mock expectation for any calls that might reach the publisher
	mockPublisher.On("PublishLogs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Maybe() // Maybe() allows the expectation to not be called

	router := gin.New()
	logsHandler := NewLogsHandler(mockPublisher, &interfaces.ResilienceComponents{})
	router.POST("/logs", logsHandler.CreateLogs)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/logs", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}
