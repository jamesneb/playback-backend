package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogsHandler(t *testing.T) {
	client := &streaming.KinesisClient{}
	handler := NewLogsHandler(client)

	assert.NotNil(t, handler)
	assert.Equal(t, client, handler.kinesisClient)
}

func TestLogsHandler_CreateLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    interface{}
		contentType    string
		expectedStatus int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "valid OTLP logs data",
			requestBody: map[string]interface{}{
				"resourceLogs": []interface{}{
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
						"scopeLogs": []interface{}{
							map[string]interface{}{
								"scope": map[string]interface{}{
									"name": "test-scope",
								},
								"logRecords": []interface{}{
									map[string]interface{}{
										"traceId":      "dGVzdC10cmFjZS1pZA==",
										"spanId":       "dGVzdC1zcGFuLWlk",
										"timeUnixNano": "1640995200000000000",
										"severityNumber": 9,
										"severityText": "INFO",
										"body": map[string]interface{}{
											"stringValue": "Test log message",
										},
									},
								},
							},
						},
					},
				},
			},
			contentType:    "application/json",
			expectedStatus: http.StatusInternalServerError, // Expect failure due to unconfigured Kinesis
			validateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "Failed to process logs data", response.Error)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &streaming.KinesisClient{}
			handler := NewLogsHandler(mockClient)

			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else if tt.requestBody != nil {
				var err error
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewBuffer(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			router := gin.New()
			router.POST("/logs", handler.CreateLogs)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestLogsHandler_GetLogs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &streaming.KinesisClient{}
	handler := NewLogsHandler(mockClient)

	tests := []struct {
		name         string
		queryParams  string
		expectedCode int
		validateFunc func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:         "without query parameters",
			queryParams:  "",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LogsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Len(t, response.Logs, 2) // Mock returns 2 logs
			},
		},
		{
			name:         "with service parameter",
			queryParams:  "?service=test-service",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LogsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "test-service", response.Service)
			},
		},
		{
			name:         "with level parameter",
			queryParams:  "?level=ERROR",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LogsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "ERROR", response.Level)
			},
		},
		{
			name:         "with time range parameters",
			queryParams:  "?from=2023-01-01T00:00:00Z&to=2023-01-01T01:00:00Z",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LogsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "2023-01-01T00:00:00Z", response.TimeRange.From)
				assert.Equal(t, "2023-01-01T01:00:00Z", response.TimeRange.To)
			},
		},
		{
			name:         "with search query",
			queryParams:  "?q=error&service=order-service",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response LogsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "error", response.Query)
				assert.Equal(t, "order-service", response.Service)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/logs"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/logs", handler.GetLogs)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}
		})
	}
}

func TestExtractLogsServiceName(t *testing.T) {
	tests := []struct {
		name     string
		data     json.RawMessage
		expected string
	}{
		{
			name: "service name in resource attributes",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": {
								"stringValue": "my-logs-service"
							}
						}]
					}
				}]
			}`),
			expected: "my-logs-service",
		},
		{
			name: "no service name attribute",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"resource": {
						"attributes": [{
							"key": "service.version",
							"value": {
								"stringValue": "1.0.0"
							}
						}]
					}
				}]
			}`),
			expected: "unknown",
		},
		{
			name: "empty service name",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": {
								"stringValue": ""
							}
						}]
					}
				}]
			}`),
			expected: "",
		},
		{
			name:     "invalid JSON",
			data:     json.RawMessage(`{invalid json}`),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLogsServiceName(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractLogsTraceID(t *testing.T) {
	tests := []struct {
		name     string
		data     json.RawMessage
		expected string
	}{
		{
			name: "trace ID in log record",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": [{
							"traceId": "dGVzdC10cmFjZS1pZA=="
						}]
					}]
				}]
			}`),
			expected: "dGVzdC10cmFjZS1pZA==",
		},
		{
			name: "multiple log records, return first trace ID",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": [
							{"traceId": "Zmlyc3QtdHJhY2U="},
							{"traceId": "c2Vjb25kLXRyYWNl"}
						]
					}]
				}]
			}`),
			expected: "Zmlyc3QtdHJhY2U=",
		},
		{
			name: "no trace ID",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": [{
							"spanId": "test-span"
						}]
					}]
				}]
			}`),
			expected: "",
		},
		{
			name: "empty trace ID",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": [{
							"traceId": ""
						}]
					}]
				}]
			}`),
			expected: "",
		},
		{
			name:     "invalid JSON",
			data:     json.RawMessage(`{invalid}`),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLogsTraceID(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountLogs(t *testing.T) {
	tests := []struct {
		name     string
		data     json.RawMessage
		expected int
	}{
		{
			name: "single log record",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": [{
							"body": {"stringValue": "test log"}
						}]
					}]
				}]
			}`),
			expected: 1,
		},
		{
			name: "multiple log records",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": [
							{"body": {"stringValue": "log1"}},
							{"body": {"stringValue": "log2"}},
							{"body": {"stringValue": "log3"}}
						]
					}]
				}]
			}`),
			expected: 3,
		},
		{
			name: "no log records",
			data: json.RawMessage(`{
				"resourceLogs": [{
					"scopeLogs": [{
						"logRecords": []
					}]
				}]
			}`),
			expected: 0,
		},
		{
			name: "multiple resource logs",
			data: json.RawMessage(`{
				"resourceLogs": [
					{
						"scopeLogs": [{
							"logRecords": [
								{"body": {"stringValue": "log1"}},
								{"body": {"stringValue": "log2"}}
							]
						}]
					},
					{
						"scopeLogs": [{
							"logRecords": [
								{"body": {"stringValue": "log3"}}
							]
						}]
					}
				]
			}`),
			expected: 3,
		},
		{
			name:     "invalid JSON",
			data:     json.RawMessage(`{invalid}`),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countLogs(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogDataStructures(t *testing.T) {
	// Test OTLP logs structure creation
	request := LogsRequest{
		ResourceLogs: []ResourceLog{
			{
				Resource: Resource{
					Attributes: []Attribute{
						{
							Key: "service.name",
							Value: AttributeValue{
								StringValue: stringPtr("test-service"),
							},
						},
					},
				},
				ScopeLogs: []ScopeLog{
					{
						Scope: Scope{
							Name:    "test-scope",
							Version: "1.0.0",
						},
						LogRecords: []LogRecord{
							{
								TimeUnixNano:     1640995200000000000,
								SeverityNumber:   9,
								SeverityText:     "INFO",
								TraceID:          "test-trace",
								SpanID:           "test-span",
								Body:             LogRecordBody{StringValue: stringPtr("Test log message")},
								Attributes: []Attribute{
									{
										Key: "endpoint",
										Value: AttributeValue{
											StringValue: stringPtr("/api/test"),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Test serialization
	data, err := json.Marshal(request)
	assert.NoError(t, err)
	assert.NotEmpty(t, data)

	// Test deserialization
	var decoded LogsRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded.ResourceLogs, 1)
	assert.Equal(t, "test-service", *decoded.ResourceLogs[0].Resource.Attributes[0].Value.StringValue)
	assert.Equal(t, "Test log message", *decoded.ResourceLogs[0].ScopeLogs[0].LogRecords[0].Body.StringValue)
}

// Integration test for the complete logs handler flow
func TestLogsHandler_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	logsData := map[string]interface{}{
		"resourceLogs": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": []interface{}{
						map[string]interface{}{
							"key": "service.name",
							"value": map[string]interface{}{
								"stringValue": "integration-logs-service",
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
				"scopeLogs": []interface{}{
					map[string]interface{}{
						"scope": map[string]interface{}{
							"name":    "integration-test",
							"version": "1.0.0",
						},
						"logRecords": []interface{}{
							map[string]interface{}{
								"traceId":           "integration-trace-123",
								"spanId":            "integration-span-456",
								"timeUnixNano":      "1640995200000000000",
								"severityNumber":    9,
								"severityText":      "INFO",
								"body": map[string]interface{}{
									"stringValue": "Integration test log message",
								},
								"attributes": []interface{}{
									map[string]interface{}{
										"key": "test.type",
										"value": map[string]interface{}{
											"stringValue": "integration",
										},
									},
								},
							},
							map[string]interface{}{
								"traceId":           "integration-trace-123",
								"spanId":            "integration-span-789",
								"timeUnixNano":      "1640995201000000000",
								"severityNumber":    13,
								"severityText":      "ERROR",
								"body": map[string]interface{}{
									"stringValue": "Integration test error message",
								},
							},
						},
					},
				},
			},
		},
	}

	mockClient := &streaming.KinesisClient{}
	handler := NewLogsHandler(mockClient)
	router := gin.New()
	router.POST("/logs", handler.CreateLogs)

	body, err := json.Marshal(logsData)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "integration-test/1.0")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return error due to unconfigured Kinesis but parsing should work
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to process logs data", response.Error)
}

// Benchmark test for logs handler performance
func BenchmarkLogsHandler_CreateLogs(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	handler := NewLogsHandler(&streaming.KinesisClient{})
	
	logsData := map[string]interface{}{
		"resourceLogs": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": []interface{}{
						map[string]interface{}{
							"key": "service.name",
							"value": map[string]interface{}{
								"stringValue": "benchmark-service",
							},
						},
					},
				},
				"scopeLogs": []interface{}{
					map[string]interface{}{
						"logRecords": []interface{}{
							map[string]interface{}{
								"body": map[string]interface{}{
									"stringValue": "Benchmark log message",
								},
							},
						},
					},
				},
			},
		},
	}
	
	body, _ := json.Marshal(logsData)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/logs", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/logs", handler.CreateLogs)
		router.ServeHTTP(w, req)
	}
}