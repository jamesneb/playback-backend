package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	pkgerrors "github.com/jamesneb/playback-backend/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewTraceHandler(t *testing.T) {
	mockPublisher := &MockEventPublisher{}
	handler := NewTraceHandler(mockPublisher, &interfaces.ResilienceComponents{})

	assert.NotNil(t, handler)
	assert.Equal(t, mockPublisher, handler.eventPublisher)
}

func TestTraceHandler_CreateTrace(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		requestBody      interface{}
		contentType      string
		expectedStatus   int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "valid OTLP trace data",
			requestBody: map[string]interface{}{
				"resourceSpans": []interface{}{
					map[string]interface{}{
						"resource": map[string]interface{}{
							"attributes": []interface{}{
								map[string]interface{}{
									"key": "service.name",
									"value": map[string]interface{}{
										"stringValue": "test-service",
									},
								},
							},
						},
						"scopeSpans": []interface{}{
							map[string]interface{}{
								"spans": []interface{}{
									map[string]interface{}{
										"traceId": "0123456789abcdef0123456789abcdef", // 32-char hex trace ID
										"spanId":  "0123456789abcdef",                 // 16-char hex span ID
										"name":    "test-operation",
									},
								},
							},
						},
					},
				},
			},
			contentType:    "application/json",
			expectedStatus: http.StatusAccepted, // Expect success with proper mock
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response TraceResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.ID)
				assert.Equal(t, "0123456789abcdef0123456789abcdef", response.TraceID)
			},
		},
		{
			name:           "invalid JSON body",
			requestBody:    `{invalid json}`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response.Error, "Invalid")
			},
		},
		{
			name:           "empty request body",
			requestBody:    "",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "missing content-type header",
			requestBody: map[string]interface{}{
				"resourceSpans": []interface{}{},
			},
			contentType:    "",                              // No content type
			expectedStatus: http.StatusUnsupportedMediaType, // Proper validation now returns 415
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock Kinesis client that doesn't make real calls
			mockClient := &MockEventPublisher{}

			// Set up mock expectations for successful test case
			if tt.name == "valid OTLP trace data" {
				mockClient.On("PublishTrace", mock.Anything, mock.Anything, "test-service", "0123456789abcdef0123456789abcdef", mock.Anything, mock.Anything).Return(nil)
			}

			handler := NewTraceHandler(mockClient, &interfaces.ResilienceComponents{})

			// Create request
			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else if tt.requestBody != nil {
				var err error
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/traces", bytes.NewBuffer(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			// Create response recorder
			w := httptest.NewRecorder()

			// Create Gin context
			router := gin.New()
			router.POST("/traces", handler.CreateTrace)

			// Execute request
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}

			// Assert mock expectations if they were set
			if tt.name == "valid OTLP trace data" {
				mockClient.AssertExpectations(t)
			}
		})
	}
}

func TestTraceHandler_GetTrace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockPublisher := &MockEventPublisher{}

	// Create handler without ClickHouse - this is the expected scenario for the test
	handler := NewTraceHandler(mockPublisher, &interfaces.ResilienceComponents{})

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/traces/test-trace-id", nil)
	w := httptest.NewRecorder()

	// Create Gin context with URL parameter and error handling middleware
	router := gin.New()

	// Add error handling middleware
	zapLogger := zap.NewNop() // Use a no-op logger for tests
	errorHandler := pkgerrors.NewHandler(zapLogger)
	router.Use(errorHandler.Middleware())

	router.GET("/traces/:id", handler.GetTrace)

	// Execute request
	router.ServeHTTP(w, req)

	// Verify response - should return service unavailable since no ClickHouse is configured
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Check that it returns a proper error response structure
	errorDetails, exists := response["error"].(map[string]interface{})
	require.True(t, exists, "Response should contain error details")
	assert.Equal(t, "SERVICE_UNAVAILABLE", errorDetails["code"])
}

func TestExtractServiceNameAndTraceID(t *testing.T) {
	tests := []struct {
		name            string
		data            json.RawMessage
		expectedService string
		expectedTraceID string
	}{
		{
			name: "service name and trace ID present",
			data: json.RawMessage(`{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": {
								"stringValue": "my-service"
							}
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"traceId": "abc123def456"
						}]
					}]
				}]
			}`),
			expectedService: "my-service",
			expectedTraceID: "abc123def456",
		},
		{
			name: "no service name attribute",
			data: json.RawMessage(`{
				"resourceSpans": [{
					"resource": {
						"attributes": []
					},
					"scopeSpans": [{
						"spans": [{
							"traceId": "xyz789"
						}]
					}]
				}]
			}`),
			expectedService: "unknown",
			expectedTraceID: "xyz789",
		},
		{
			name:            "invalid JSON",
			data:            json.RawMessage(`{invalid json}`),
			expectedService: "unknown",
			expectedTraceID: "",
		},
		{
			name: "no spans",
			data: json.RawMessage(`{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": {
								"stringValue": "test-service"
							}
						}]
					},
					"scopeSpans": []
				}]
			}`),
			expectedService: "test-service",
			expectedTraceID: "",
		},
	}

	// Create a handler instance for testing the method
	handler := &TraceHandler{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, traceID := handler.extractServiceNameAndTraceID(tt.data)
			assert.Equal(t, tt.expectedService, service)
			assert.Equal(t, tt.expectedTraceID, traceID)
		})
	}
}

// TestExtractTraceID is now covered by TestExtractServiceNameAndTraceID

func TestGenerateID(t *testing.T) {
	// Test that generateID produces different IDs
	id1 := generateID()
	id2 := generateID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)

	// Test ID format (should be timestamp-based)
	assert.Regexp(t, `^\d+$`, id1)
	assert.Regexp(t, `^\d+$`, id2)
}

// Integration test for the complete trace handler flow
func TestTraceHandler_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create realistic trace data
	traceData := map[string]interface{}{
		"resourceSpans": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": []interface{}{
						map[string]interface{}{
							"key": "service.name",
							"value": map[string]interface{}{
								"stringValue": "integration-test-service",
							},
						},
						map[string]interface{}{
							"key": "service.version",
							"value": map[string]interface{}{
								"stringValue": "1.0.0",
							},
						},
					},
				},
				"scopeSpans": []interface{}{
					map[string]interface{}{
						"scope": map[string]interface{}{
							"name":    "integration-test",
							"version": "1.0.0",
						},
						"spans": []interface{}{
							map[string]interface{}{
								"traceId":           "fedcba9876543210fedcba9876543210", // 32-char hex trace ID
								"spanId":            "fedcba9876543210",                 // 16-char hex span ID
								"name":              "integration-operation",
								"kind":              1, // SPAN_KIND_INTERNAL
								"startTimeUnixNano": "1640995200000000000",
								"endTimeUnixNano":   "1640995201000000000",
								"status": map[string]interface{}{
									"code":    1, // STATUS_CODE_OK
									"message": "Success",
								},
							},
						},
					},
				},
			},
		},
	}

	// Create handler
	mockClient := &MockEventPublisher{}
	mockClient.On("PublishTrace", mock.Anything, mock.Anything, "integration-test-service", "fedcba9876543210fedcba9876543210", mock.Anything, mock.Anything).Return(nil)

	handler := NewTraceHandler(mockClient, &interfaces.ResilienceComponents{})
	router := gin.New()
	router.POST("/traces", handler.CreateTrace)

	// Create request
	body, err := json.Marshal(traceData)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/traces", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "integration-test/1.0")

	w := httptest.NewRecorder()

	// Execute request
	router.ServeHTTP(w, req)

	// Should return success with proper mock setup
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response TraceResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "fedcba9876543210fedcba9876543210", response.TraceID)

	mockClient.AssertExpectations(t)
}
