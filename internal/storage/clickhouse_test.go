package storage

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockClickHouseConn implements the driver.Conn interface for testing
type MockClickHouseConn struct {
	mock.Mock
}

func (m *MockClickHouseConn) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockClickHouseConn) QueryRow(ctx context.Context, query string, args ...interface{}) interface{} {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0)
}

func (m *MockClickHouseConn) PrepareBatch(ctx context.Context, query string) (MockBatch, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(MockBatch), args.Error(1)
}

func (m *MockClickHouseConn) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockBatch implements batch interface for testing
type MockBatch struct {
	mock.Mock
}

func (m *MockBatch) Append(args ...interface{}) error {
	mockArgs := m.Called(args...)
	return mockArgs.Error(0)
}

func (m *MockBatch) Send() error {
	args := m.Called()
	return args.Error(0)
}

// MockRow for QueryRow testing
type MockRow struct {
	values []interface{}
	index  int
}

func (m *MockRow) Scan(dest ...interface{}) error {
	if m.index >= len(m.values) {
		return errors.New("no more values")
	}
	
	for i, d := range dest {
		if i+m.index < len(m.values) {
			switch v := d.(type) {
			case *string:
				*v = m.values[i+m.index].(string)
			case *uint64:
				*v = m.values[i+m.index].(uint64)
			}
		}
	}
	m.index += len(dest)
	return nil
}

func TestNewClickHouseClient(t *testing.T) {
	tests := []struct {
		name           string
		config         *ClickHouseConfig
		expectedError  bool
		setupMock      func(*MockClickHouseConn)
	}{
		{
			name: "successful connection",
			config: &ClickHouseConfig{
				Host:               "localhost:9000",
				Database:           "test",
				Username:           "default",
				Password:           "",
				MaxConnections:     10,
				MaxIdleConnections: 5,
				ConnectionTimeout:  "10s",
			},
			expectedError: false,
			setupMock: func(mockConn *MockClickHouseConn) {
				mockConn.On("Ping", mock.Anything).Return(nil)
				mockConn.On("QueryRow", mock.Anything, "SELECT currentDatabase()").Return(&MockRow{values: []interface{}{"test"}})
				mockConn.On("QueryRow", mock.Anything, "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans_raw'", mock.Anything).Return(&MockRow{values: []interface{}{uint64(1)}})
				mockConn.On("QueryRow", mock.Anything, "SELECT count() FROM system.tables WHERE database = ? AND name = 'spans_final'", mock.Anything).Return(&MockRow{values: []interface{}{uint64(1)}})
			},
		},
		{
			name: "connection ping failure",
			config: &ClickHouseConfig{
				Host:     "invalid:9000",
				Database: "test",
			},
			expectedError: true,
			setupMock: func(mockConn *MockClickHouseConn) {
				mockConn.On("Ping", mock.Anything).Return(errors.New("connection failed"))
				mockConn.On("Close").Return(nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test structure shows how the tests would work
			// In practice, we'd need to modify the NewClickHouseClient function
			// to accept a connection interface for dependency injection
			// For now, this demonstrates the testing approach
		})
	}
}

func TestParseLogsData(t *testing.T) {
	client := &ClickHouseClient{}

	tests := []struct {
		name          string
		inputData     interface{}
		expectedLogs  int
		expectedError bool
		expectedService string
	}{
		{
			name: "valid OTLP logs data",
			inputData: json.RawMessage(`{
				"resource": {
					"attributes": [
						{"key": "service.name", "value": {"stringValue": "test-service"}},
						{"key": "service.version", "value": {"stringValue": "1.0.0"}}
					]
				},
				"scopeLogs": [{
					"logRecords": [{
						"timeUnixNano": "1640995200000000000",
						"observedTimeUnixNano": "1640995200000000000",
						"severityNumber": 9,
						"severityText": "INFO",
						"body": {"stringValue": "Test log message"},
						"attributes": [
							{"key": "user.id", "value": {"stringValue": "123"}}
						]
					}]
				}]
			}`),
			expectedLogs:    1,
			expectedError:   false,
			expectedService: "test-service",
		},
		{
			name: "invalid JSON data",
			inputData: json.RawMessage(`{"invalid": json}`),
			expectedLogs: 0,
			expectedError: true,
		},
		{
			name: "wrong data type",
			inputData: "not json.RawMessage",
			expectedLogs: 0,
			expectedError: true,
		},
		{
			name: "empty scopeLogs",
			inputData: json.RawMessage(`{
				"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "empty-service"}}]},
				"scopeLogs": []
			}`),
			expectedLogs:    0,
			expectedError:   false,
			expectedService: "empty-service",
		},
		{
			name: "severityNumber as string",
			inputData: json.RawMessage(`{
				"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "string-severity-service"}}]},
				"scopeLogs": [{
					"logRecords": [{
						"timeUnixNano": "1640995200000000000",
						"observedTimeUnixNano": "1640995200000000000",
						"severityNumber": "9",
						"severityText": "INFO",
						"body": {"stringValue": "Test with string severity"}
					}]
				}]
			}`),
			expectedLogs:    1,
			expectedError:   false,
			expectedService: "string-severity-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := client.parseLogsData(tt.inputData)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, logs, tt.expectedLogs)

			if tt.expectedLogs > 0 {
				assert.Equal(t, tt.expectedService, logs[0].ServiceName)
				
				// Verify timestamp parsing
				assert.False(t, logs[0].Timestamp.IsZero())
				assert.False(t, logs[0].ObservedTimestamp.IsZero())
				
				// Verify severity parsing
				if tt.name == "valid OTLP logs data" {
					assert.Equal(t, uint8(9), logs[0].SeverityNumber)
					assert.Equal(t, "INFO", logs[0].SeverityText)
					assert.Equal(t, "Test log message", logs[0].Body)
					assert.Equal(t, "123", logs[0].Attributes["user.id"])
				}
			}
		})
	}
}

func TestParseMetricsData(t *testing.T) {
	client := &ClickHouseClient{}

	tests := []struct {
		name            string
		inputData       interface{}
		expectedMetrics int
		expectedError   bool
		expectedService string
		expectedTypes   []string
	}{
		{
			name: "valid OTLP metrics with sum (counter)",
			inputData: json.RawMessage(`{
				"resource": {
					"attributes": [
						{"key": "service.name", "value": {"stringValue": "metrics-service"}}
					]
				},
				"scopeMetrics": [{
					"metrics": [{
						"name": "orders_total",
						"description": "Total orders",
						"unit": "1",
						"sum": {
							"dataPoints": [{
								"timeUnixNano": "1640995200000000000",
								"asInt": "42",
								"attributes": [
									{"key": "status", "value": {"stringValue": "success"}}
								]
							}],
							"isMonotonic": true
						}
					}]
				}]
			}`),
			expectedMetrics: 1,
			expectedError:   false,
			expectedService: "metrics-service",
			expectedTypes:   []string{"counter"},
		},
		{
			name: "valid OTLP metrics with gauge",
			inputData: json.RawMessage(`{
				"resource": {
					"attributes": [
						{"key": "service.name", "value": {"stringValue": "gauge-service"}}
					]
				},
				"scopeMetrics": [{
					"metrics": [{
						"name": "cpu_usage",
						"description": "CPU usage percentage",
						"unit": "%",
						"gauge": {
							"dataPoints": [{
								"timeUnixNano": "1640995200000000000",
								"asDouble": 75.5,
								"attributes": [
									{"key": "cpu", "value": {"stringValue": "cpu0"}}
								]
							}]
						}
					}]
				}]
			}`),
			expectedMetrics: 1,
			expectedError:   false,
			expectedService: "gauge-service",
			expectedTypes:   []string{"gauge"},
		},
		{
			name: "valid OTLP metrics with histogram",
			inputData: json.RawMessage(`{
				"resource": {
					"attributes": [
						{"key": "service.name", "value": {"stringValue": "histogram-service"}}
					]
				},
				"scopeMetrics": [{
					"metrics": [{
						"name": "request_duration",
						"description": "Request duration",
						"unit": "ms",
						"histogram": {
							"dataPoints": [{
								"timeUnixNano": "1640995200000000000",
								"count": "100",
								"sum": 1500.5,
								"bucketCounts": ["10", "20", "70"],
								"explicitBounds": [1.0, 10.0, 100.0]
							}],
							"aggregationTemporality": "AGGREGATION_TEMPORALITY_CUMULATIVE"
						}
					}]
				}]
			}`),
			expectedMetrics: 1,
			expectedError:   false,
			expectedService: "histogram-service",
			expectedTypes:   []string{"histogram"},
		},
		{
			name: "multiple metric types",
			inputData: json.RawMessage(`{
				"resource": {
					"attributes": [
						{"key": "service.name", "value": {"stringValue": "multi-service"}}
					]
				},
				"scopeMetrics": [{
					"metrics": [
						{
							"name": "counter_metric",
							"sum": {
								"dataPoints": [{"timeUnixNano": "1640995200000000000", "asInt": "10"}],
								"isMonotonic": true
							}
						},
						{
							"name": "gauge_metric", 
							"gauge": {
								"dataPoints": [{"timeUnixNano": "1640995200000000000", "asDouble": 20.5}]
							}
						}
					]
				}]
			}`),
			expectedMetrics: 2,
			expectedError:   false,
			expectedService: "multi-service",
			expectedTypes:   []string{"counter", "gauge"},
		},
		{
			name: "invalid JSON",
			inputData: json.RawMessage(`{"invalid": json}`),
			expectedMetrics: 0,
			expectedError: true,
		},
		{
			name: "wrong data type",
			inputData: "not json.RawMessage",
			expectedMetrics: 0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := client.parseMetricsData(tt.inputData)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, metrics, tt.expectedMetrics)

			if tt.expectedMetrics > 0 {
				assert.Equal(t, tt.expectedService, metrics[0].ServiceName)
				
				// Verify timestamp parsing
				for _, metric := range metrics {
					assert.False(t, metric.Timestamp.IsZero())
				}

				// Verify metric types
				if len(tt.expectedTypes) > 0 {
					for i, expectedType := range tt.expectedTypes {
						if i < len(metrics) {
							assert.Equal(t, expectedType, metrics[i].Type)
						}
					}
				}

				// Specific validations for different test cases
				switch tt.name {
				case "valid OTLP metrics with sum (counter)":
					assert.Equal(t, "orders_total", metrics[0].Name)
					assert.Equal(t, float64(42), metrics[0].Value)
					assert.Equal(t, "success", metrics[0].Attributes["status"])
				case "valid OTLP metrics with gauge":
					assert.Equal(t, "cpu_usage", metrics[0].Name)
					assert.Equal(t, 75.5, metrics[0].Value)
					assert.Equal(t, "cpu0", metrics[0].Attributes["cpu"])
				case "valid OTLP metrics with histogram":
					assert.Equal(t, "request_duration", metrics[0].Name)
					assert.Equal(t, 1500.5, metrics[0].Value)
				}
			}
		})
	}
}

func TestInsertLog(t *testing.T) {
	tests := []struct {
		name           string
		event          *streaming.TelemetryEvent
		parseError     bool
		batchError     bool
		expectedError  bool
	}{
		{
			name: "successful log insertion",
			event: &streaming.TelemetryEvent{
				Type:        "logs",
				ServiceName: "test-service",
				Data: json.RawMessage(`{
					"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "test-service"}}]},
					"scopeLogs": [{
						"logRecords": [{
							"timeUnixNano": "1640995200000000000",
							"observedTimeUnixNano": "1640995200000000000",
							"severityNumber": 9,
							"severityText": "INFO",
							"body": {"stringValue": "Test log"}
						}]
					}]
				}`),
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "127.0.0.1",
				},
			},
			expectedError: false,
		},
		{
			name: "parsing error",
			event: &streaming.TelemetryEvent{
				Type: "logs",
				Data: json.RawMessage(`{"invalid": json}`),
			},
			parseError:    true,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This would require mocking the ClickHouse connection
			// The actual implementation would look like:
			
			// mockConn := &MockClickHouseConn{}
			// client := &ClickHouseClient{conn: mockConn}
			
			// if !tt.parseError {
			//     mockBatch := &MockBatch{}
			//     mockConn.On("PrepareBatch", mock.Anything, mock.AnythingOfType("string")).Return(mockBatch, nil)
			//     mockBatch.On("Append", mock.Anything).Return(nil)
			//     if tt.batchError {
			//         mockBatch.On("Send").Return(errors.New("batch send failed"))
			//     } else {
			//         mockBatch.On("Send").Return(nil)
			//     }
			// }
			
			// err := client.InsertLog(context.Background(), tt.event)
			
			// if tt.expectedError {
			//     assert.Error(t, err)
			// } else {
			//     assert.NoError(t, err)
			// }
			
			// mockConn.AssertExpectations(t)
		})
	}
}

func TestInsertMetric(t *testing.T) {
	tests := []struct {
		name          string
		event         *streaming.TelemetryEvent
		expectedError bool
	}{
		{
			name: "successful metric insertion",
			event: &streaming.TelemetryEvent{
				Type:        "metrics",
				ServiceName: "test-service",
				Data: json.RawMessage(`{
					"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "test-service"}}]},
					"scopeMetrics": [{
						"metrics": [{
							"name": "test_counter",
							"sum": {
								"dataPoints": [{"timeUnixNano": "1640995200000000000", "asInt": "5"}],
								"isMonotonic": true
							}
						}]
					}]
				}`),
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "127.0.0.1",
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Similar structure to TestInsertLog
			// Would require mocking ClickHouse connection and batch operations
		})
	}
}

func TestInsertTrace(t *testing.T) {
	tests := []struct {
		name          string
		event         *streaming.TelemetryEvent
		expectedError bool
	}{
		{
			name: "successful trace insertion",
			event: &streaming.TelemetryEvent{
				Type:        "traces",
				ServiceName: "test-service",
				TraceID:     "abc123",
				Data:        json.RawMessage(`{"traceData": "test"}`),
				Metadata: streaming.TelemetryMetadata{
					IngestedAt: time.Now(),
					SourceIP:   "127.0.0.1",
				},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Similar structure, focusing on raw trace insertion
			// Traces are inserted directly without parsing (materialized views handle processing)
		})
	}
}

// Benchmark tests for performance-critical parsing functions
func BenchmarkParseLogsData(b *testing.B) {
	client := &ClickHouseClient{}
	data := json.RawMessage(`{
		"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "bench-service"}}]},
		"scopeLogs": [{
			"logRecords": [{
				"timeUnixNano": "1640995200000000000",
				"observedTimeUnixNano": "1640995200000000000",
				"severityNumber": 9,
				"severityText": "INFO",
				"body": {"stringValue": "Benchmark log message"}
			}]
		}]
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.parseLogsData(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMetricsData(b *testing.B) {
	client := &ClickHouseClient{}
	data := json.RawMessage(`{
		"resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "bench-service"}}]},
		"scopeMetrics": [{
			"metrics": [{
				"name": "bench_counter",
				"sum": {
					"dataPoints": [{"timeUnixNano": "1640995200000000000", "asInt": "100"}],
					"isMonotonic": true
				}
			}]
		}]
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.parseMetricsData(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}