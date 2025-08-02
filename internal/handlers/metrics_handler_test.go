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

func TestNewMetricsHandler(t *testing.T) {
	client := &streaming.KinesisClient{}
	handler := NewMetricsHandler(client)

	assert.NotNil(t, handler)
	assert.Equal(t, client, handler.kinesisClient)
}

func TestMetricsHandler_CreateMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		requestBody    interface{}
		contentType    string
		expectedStatus int
		validateResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "valid OTLP metrics data",
			requestBody: map[string]interface{}{
				"resourceMetrics": []interface{}{
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
						"scopeMetrics": []interface{}{
							map[string]interface{}{
								"scope": map[string]interface{}{
									"name": "test-scope",
								},
								"metrics": []interface{}{
									map[string]interface{}{
										"name": "test_counter",
										"sum": map[string]interface{}{
											"dataPoints": []interface{}{
												map[string]interface{}{
													"asInt": 42,
												},
											},
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
				assert.Equal(t, "Failed to process metrics data", response.Error)
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
			handler := NewMetricsHandler(mockClient)

			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else if tt.requestBody != nil {
				var err error
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(http.MethodPost, "/metrics", bytes.NewBuffer(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()
			router := gin.New()
			router.POST("/metrics", handler.CreateMetrics)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.validateResponse != nil {
				tt.validateResponse(t, w)
			}
		})
	}
}

func TestMetricsHandler_GetMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockClient := &streaming.KinesisClient{}
	handler := NewMetricsHandler(mockClient)

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
				var response MetricsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Len(t, response.Metrics, 2) // Mock returns 2 metrics
			},
		},
		{
			name:         "with service parameter",
			queryParams:  "?service=test-service",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response MetricsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "test-service", response.Service)
			},
		},
		{
			name:         "with time range parameters",
			queryParams:  "?from=2023-01-01T00:00:00Z&to=2023-01-01T01:00:00Z",
			expectedCode: http.StatusOK,
			validateFunc: func(t *testing.T, w *httptest.ResponseRecorder) {
				var response MetricsQueryResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, "2023-01-01T00:00:00Z", response.TimeRange.From)
				assert.Equal(t, "2023-01-01T01:00:00Z", response.TimeRange.To)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics"+tt.queryParams, nil)
			w := httptest.NewRecorder()

			router := gin.New()
			router.GET("/metrics", handler.GetMetrics)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedCode, w.Code)
			if tt.validateFunc != nil {
				tt.validateFunc(t, w)
			}
		})
	}
}

func TestExtractMetricsServiceName(t *testing.T) {
	tests := []struct {
		name     string
		data     json.RawMessage
		expected string
	}{
		{
			name: "service name in resource attributes",
			data: json.RawMessage(`{
				"resourceMetrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": {
								"stringValue": "my-metrics-service"
							}
						}]
					}
				}]
			}`),
			expected: "my-metrics-service",
		},
		{
			name: "no service name attribute",
			data: json.RawMessage(`{
				"resourceMetrics": [{
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
				"resourceMetrics": [{
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
			result := extractMetricsServiceName(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountMetrics(t *testing.T) {
	tests := []struct {
		name     string
		data     json.RawMessage
		expected int
	}{
		{
			name: "single metric",
			data: json.RawMessage(`{
				"resourceMetrics": [{
					"scopeMetrics": [{
						"metrics": [{
							"name": "test_metric"
						}]
					}]
				}]
			}`),
			expected: 1,
		},
		{
			name: "multiple metrics",
			data: json.RawMessage(`{
				"resourceMetrics": [{
					"scopeMetrics": [{
						"metrics": [
							{"name": "metric1"},
							{"name": "metric2"},
							{"name": "metric3"}
						]
					}]
				}]
			}`),
			expected: 3,
		},
		{
			name: "no metrics",
			data: json.RawMessage(`{
				"resourceMetrics": [{
					"scopeMetrics": [{
						"metrics": []
					}]
				}]
			}`),
			expected: 0,
		},
		{
			name: "multiple resource metrics",
			data: json.RawMessage(`{
				"resourceMetrics": [
					{
						"scopeMetrics": [{
							"metrics": [
								{"name": "metric1"},
								{"name": "metric2"}
							]
						}]
					},
					{
						"scopeMetrics": [{
							"metrics": [
								{"name": "metric3"}
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
			result := countMetrics(tt.data)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMetricDataStructures(t *testing.T) {
	// Test OTLP structure creation
	request := MetricsRequest{
		ResourceMetrics: []ResourceMetric{
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
				ScopeMetrics: []ScopeMetric{
					{
						Scope: Scope{
							Name:    "test-scope",
							Version: "1.0.0",
						},
						Metrics: []Metric{
							{
								Name:        "test_counter",
								Description: "A test counter metric",
								Unit:        "1",
								Sum: &Sum{
									DataPoints: []NumberDataPoint{
										{
											AsInt:        int64Ptr(42),
											TimeUnixNano: 1640995200000000000,
										},
									},
									AggregationTemporality: 2,
									IsMonotonic:            true,
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
	var decoded MetricsRequest
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded.ResourceMetrics, 1)
	assert.Equal(t, "test-service", *decoded.ResourceMetrics[0].Resource.Attributes[0].Value.StringValue)
}

// Integration test for the complete metrics handler flow
func TestMetricsHandler_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metricsData := map[string]interface{}{
		"resourceMetrics": []interface{}{
			map[string]interface{}{
				"resource": map[string]interface{}{
					"attributes": []interface{}{
						map[string]interface{}{
							"key": "service.name",
							"value": map[string]interface{}{
								"stringValue": "integration-metrics-service",
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
				"scopeMetrics": []interface{}{
					map[string]interface{}{
						"scope": map[string]interface{}{
							"name":    "integration-test",
							"version": "1.0.0",
						},
						"metrics": []interface{}{
							map[string]interface{}{
								"name":        "orders_total",
								"description": "Total number of orders",
								"unit":        "1",
								"sum": map[string]interface{}{
									"dataPoints": []interface{}{
										map[string]interface{}{
											"asInt":        156,
											"timeUnixNano": "1640995200000000000",
										},
									},
									"aggregationTemporality": 2,
									"isMonotonic":            true,
								},
							},
							map[string]interface{}{
								"name":        "order_duration",
								"description": "Order processing duration",
								"unit":        "s",
								"histogram": map[string]interface{}{
									"dataPoints": []interface{}{
										map[string]interface{}{
											"count":         100,
											"sum":           23.5,
											"bucketCounts":  []int{10, 20, 30, 25, 15},
											"explicitBounds": []float64{0.1, 0.5, 1.0, 2.0, 5.0},
											"timeUnixNano":  "1640995200000000000",
										},
									},
									"aggregationTemporality": 2,
								},
							},
						},
					},
				},
			},
		},
	}

	mockClient := &streaming.KinesisClient{}
	handler := NewMetricsHandler(mockClient)
	router := gin.New()
	router.POST("/metrics", handler.CreateMetrics)

	body, err := json.Marshal(metricsData)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/metrics", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "integration-test/1.0")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return error due to unconfigured Kinesis but parsing should work
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response ErrorResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Failed to process metrics data", response.Error)
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

func boolPtr(b bool) *bool {
	return &b
}

// Benchmark test for metrics handler performance
func BenchmarkMetricsHandler_CreateMetrics(b *testing.B) {
	gin.SetMode(gin.TestMode)
	
	handler := NewMetricsHandler(&streaming.KinesisClient{})
	
	metricsData := map[string]interface{}{
		"resourceMetrics": []interface{}{
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
				"scopeMetrics": []interface{}{
					map[string]interface{}{
						"metrics": []interface{}{
							map[string]interface{}{
								"name": "benchmark_metric",
								"sum": map[string]interface{}{
									"dataPoints": []interface{}{
										map[string]interface{}{
											"asInt": 1,
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
	
	body, _ := json.Marshal(metricsData)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/metrics", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		
		router := gin.New()
		router.POST("/metrics", handler.CreateMetrics)
		router.ServeHTTP(w, req)
	}
}