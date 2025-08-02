package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestNewMetricsService(t *testing.T) {
	service := NewMetricsService(nil, nil)

	assert.NotNil(t, service)
}

func TestMetricsService_Export_Basic(t *testing.T) {
	tests := []struct {
		name              string
		request           *metricscollectorpb.ExportMetricsServiceRequest
		expectedError     bool
		expectedMetricCount int
	}{
		{
			name: "successful export with single counter metric",
			request: &metricscollectorpb.ExportMetricsServiceRequest{
				ResourceMetrics: []*metricspb.ResourceMetrics{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeMetrics: []*metricspb.ScopeMetrics{
							{
								Metrics: []*metricspb.Metric{
									{
										Name:        "test_counter",
										Description: "A test counter metric",
										Unit:        "1",
										Data: &metricspb.Metric_Sum{
											Sum: &metricspb.Sum{
												DataPoints: []*metricspb.NumberDataPoint{
													{
														TimeUnixNano: 1640995200000000000,
														Value: &metricspb.NumberDataPoint_AsInt{
															AsInt: 42,
														},
													},
												},
												AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
												IsMonotonic:           true,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedError:      false,
			expectedMetricCount: 1,
		},
		{
			name: "multiple metrics with different types",
			request: &metricscollectorpb.ExportMetricsServiceRequest{
				ResourceMetrics: []*metricspb.ResourceMetrics{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "multi-metric-service"}},
								},
							},
						},
						ScopeMetrics: []*metricspb.ScopeMetrics{
							{
								Metrics: []*metricspb.Metric{
									{
										Name: "counter_metric",
										Data: &metricspb.Metric_Sum{
											Sum: &metricspb.Sum{
												DataPoints: []*metricspb.NumberDataPoint{
													{
														Value: &metricspb.NumberDataPoint_AsInt{AsInt: 10},
													},
												},
											},
										},
									},
									{
										Name: "gauge_metric",
										Data: &metricspb.Metric_Gauge{
											Gauge: &metricspb.Gauge{
												DataPoints: []*metricspb.NumberDataPoint{
													{
														Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 99.5},
													},
												},
											},
										},
									},
									{
										Name: "histogram_metric",
										Data: &metricspb.Metric_Histogram{
											Histogram: &metricspb.Histogram{
												DataPoints: []*metricspb.HistogramDataPoint{
													{
														Count:         100,
														Sum:           func() *float64 { f := 250.0; return &f }(),
														BucketCounts:  []uint64{10, 20, 30, 25, 15},
														ExplicitBounds: []float64{1, 2, 3, 4, 5},
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
			},
			expectedError:      false,
			expectedMetricCount: 3,
		},
		{
			name: "empty request",
			request: &metricscollectorpb.ExportMetricsServiceRequest{
				ResourceMetrics: []*metricspb.ResourceMetrics{},
			},
			expectedError:      false,
			expectedMetricCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewMetricsService(nil, nil)

			response, err := service.Export(context.Background(), tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, response)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, response)
				assert.Equal(t, int64(0), response.PartialSuccess.RejectedDataPoints)
			}
		})
	}
}

func TestExtractServiceNameFromMetrics(t *testing.T) {
	tests := []struct {
		name     string
		resource *metricspb.ResourceMetrics
		expected string
	}{
		{
			name: "service name present",
			resource: &metricspb.ResourceMetrics{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "my-metrics-service"}},
						},
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "1.0.0"}},
						},
					},
				},
			},
			expected: "my-metrics-service",
		},
		{
			name: "no service name attribute",
			resource: &metricspb.ResourceMetrics{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.version",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "1.0.0"}},
						},
					},
				},
			},
			expected: "unknown",
		},
		{
			name: "empty service name",
			resource: &metricspb.ResourceMetrics{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: ""}},
						},
					},
				},
			},
			expected: "unknown",
		},
		{
			name: "nil resource",
			resource: &metricspb.ResourceMetrics{
				Resource: nil,
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractServiceNameFromMetrics(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountMetrics(t *testing.T) {
	tests := []struct {
		name            string
		resourceMetrics []*metricspb.ResourceMetrics
		expected        int
	}{
		{
			name: "single resource with multiple metrics",
			resourceMetrics: []*metricspb.ResourceMetrics{
				{
					ScopeMetrics: []*metricspb.ScopeMetrics{
						{
							Metrics: []*metricspb.Metric{
								{Name: "metric1"},
								{Name: "metric2"},
								{Name: "metric3"},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "multiple resources with metrics",
			resourceMetrics: []*metricspb.ResourceMetrics{
				{
					ScopeMetrics: []*metricspb.ScopeMetrics{
						{
							Metrics: []*metricspb.Metric{
								{Name: "metric1"},
								{Name: "metric2"},
							},
						},
					},
				},
				{
					ScopeMetrics: []*metricspb.ScopeMetrics{
						{
							Metrics: []*metricspb.Metric{
								{Name: "metric3"},
							},
						},
					},
				},
			},
			expected: 3,
		},
		{
			name: "empty resource metrics",
			resourceMetrics: []*metricspb.ResourceMetrics{
				{
					ScopeMetrics: []*metricspb.ScopeMetrics{
						{
							Metrics: []*metricspb.Metric{},
						},
					},
				},
			},
			expected: 0,
		},
		{
			name:            "nil resource metrics",
			resourceMetrics: nil,
			expected:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countMetrics(tt.resourceMetrics)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertResourceMetricToMap(t *testing.T) {
	resourceMetric := &metricspb.ResourceMetrics{
		Resource: &resourcepb.Resource{
			Attributes: []*commonpb.KeyValue{
				{
					Key:   "service.name",
					Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
				},
			},
		},
		ScopeMetrics: []*metricspb.ScopeMetrics{
			{
				Metrics: []*metricspb.Metric{
					{Name: "test_metric"},
				},
			},
		},
	}

	result := convertResourceMetricToMap(resourceMetric)

	// Verify the structure
	resultMap, ok := result.(map[string]interface{})
	assert.True(t, ok, "Result should be a map")
	
	resourceMetrics, exists := resultMap["resourceMetrics"]
	assert.True(t, exists, "Should contain resourceMetrics key")
	
	resourceMetricsSlice, ok := resourceMetrics.([]interface{})
	assert.True(t, ok, "resourceMetrics should be a slice")
	assert.Len(t, resourceMetricsSlice, 1, "Should contain one resource metric")
}

// Integration test that validates the overall metrics service structure
func TestMetricsService_Integration(t *testing.T) {
	service := NewMetricsService(nil, nil)

	// Create a realistic metrics request with different metric types
	request := &metricscollectorpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "integration-metrics-service"}},
						},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Scope: &commonpb.InstrumentationScope{
							Name:    "integration-test",
							Version: "1.0.0",
						},
						Metrics: []*metricspb.Metric{
							{
								Name:        "orders_total",
								Description: "Total number of orders",
								Unit:        "1",
								Data: &metricspb.Metric_Sum{
									Sum: &metricspb.Sum{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: 1640995200000000000,
												Value: &metricspb.NumberDataPoint_AsInt{
													AsInt: 156,
												},
												Attributes: []*commonpb.KeyValue{
													{
														Key:   "status",
														Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "success"}},
													},
												},
											},
										},
										AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
										IsMonotonic:           true,
									},
								},
							},
							{
								Name:        "order_duration_seconds",
								Description: "Order processing duration in seconds",
								Unit:        "s",
								Data: &metricspb.Metric_Histogram{
									Histogram: &metricspb.Histogram{
										DataPoints: []*metricspb.HistogramDataPoint{
											{
												TimeUnixNano:   1640995200000000000,
												Count:          100,
												Sum:            func() *float64 { f := 23.5; return &f }(),
												BucketCounts:   []uint64{10, 20, 30, 25, 15},
												ExplicitBounds: []float64{0.1, 0.5, 1.0, 2.0, 5.0},
												Attributes: []*commonpb.KeyValue{
													{
														Key:   "endpoint",
														Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "/orders"}},
													},
												},
											},
										},
										AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
									},
								},
							},
							{
								Name:        "current_queue_size",
								Description: "Current number of items in processing queue",
								Unit:        "1",
								Data: &metricspb.Metric_Gauge{
									Gauge: &metricspb.Gauge{
										DataPoints: []*metricspb.NumberDataPoint{
											{
												TimeUnixNano: 1640995200000000000,
												Value: &metricspb.NumberDataPoint_AsInt{
													AsInt: 25,
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
		},
	}

	// Execute the test
	response, err := service.Export(context.Background(), request)

	// Should succeed with nil handlers
	assert.NoError(t, err)
	assert.NotNil(t, response)
	if response.PartialSuccess != nil {
		assert.Equal(t, int64(0), response.PartialSuccess.RejectedDataPoints)
	}
}

func TestMetricsService_MetricTypes(t *testing.T) {
	service := NewMetricsService(nil, nil)

	// Test different metric types
	metricTypes := []struct {
		name   string
		metric *metricspb.Metric
	}{
		{
			name: "Counter (Sum with monotonic=true)",
			metric: &metricspb.Metric{
				Name: "test_counter",
				Data: &metricspb.Metric_Sum{
					Sum: &metricspb.Sum{
						DataPoints: []*metricspb.NumberDataPoint{
							{Value: &metricspb.NumberDataPoint_AsInt{AsInt: 42}},
						},
						IsMonotonic: true,
					},
				},
			},
		},
		{
			name: "Gauge",
			metric: &metricspb.Metric{
				Name: "test_gauge",
				Data: &metricspb.Metric_Gauge{
					Gauge: &metricspb.Gauge{
						DataPoints: []*metricspb.NumberDataPoint{
							{Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 99.5}},
						},
					},
				},
			},
		},
		{
			name: "Histogram",
			metric: &metricspb.Metric{
				Name: "test_histogram",
				Data: &metricspb.Metric_Histogram{
					Histogram: &metricspb.Histogram{
						DataPoints: []*metricspb.HistogramDataPoint{
							{
								Count:          50,
								Sum:            func() *float64 { f := 125.0; return &f }(),
								BucketCounts:   []uint64{5, 10, 15, 10, 5, 5},
								ExplicitBounds: []float64{1, 2, 3, 4, 5},
							},
						},
					},
				},
			},
		},
	}

	for _, mt := range metricTypes {
		t.Run(mt.name, func(t *testing.T) {
			request := &metricscollectorpb.ExportMetricsServiceRequest{
				ResourceMetrics: []*metricspb.ResourceMetrics{
					{
						Resource: &resourcepb.Resource{
							Attributes: []*commonpb.KeyValue{
								{
									Key:   "service.name",
									Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}},
								},
							},
						},
						ScopeMetrics: []*metricspb.ScopeMetrics{
							{
								Metrics: []*metricspb.Metric{mt.metric},
							},
						},
					},
				},
			}

			response, err := service.Export(context.Background(), request)
			assert.NoError(t, err)
			assert.NotNil(t, response)
		})
	}
}

// Benchmark test for metrics service performance
func BenchmarkMetricsService_Export(b *testing.B) {
	service := NewMetricsService(nil, nil)
	
	request := &metricscollectorpb.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{
							Key:   "service.name",
							Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "benchmark-service"}},
						},
					},
				},
				ScopeMetrics: []*metricspb.ScopeMetrics{
					{
						Metrics: []*metricspb.Metric{
							{
								Name: "benchmark_counter",
								Data: &metricspb.Metric_Sum{
									Sum: &metricspb.Sum{
										DataPoints: []*metricspb.NumberDataPoint{
											{Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1}},
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Export(context.Background(), request)
		if err != nil {
			b.Fatal(err)
		}
	}
}