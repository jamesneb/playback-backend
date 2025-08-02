package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadTestConfig(t *testing.T) {
	config := LoadTestConfig{
		OrderServiceURL: "http://localhost:8081",
		RequestsPerSec:  10,
		DurationSec:     60,
		MaxConcurrent:   50,
	}

	assert.Equal(t, "http://localhost:8081", config.OrderServiceURL)
	assert.Equal(t, 10, config.RequestsPerSec)
	assert.Equal(t, 60, config.DurationSec)
	assert.Equal(t, 50, config.MaxConcurrent)
}

func TestGenerateRandomOrder(t *testing.T) {
	order := generateRandomOrder()

	assert.NotEmpty(t, order.UserID)
	assert.NotEmpty(t, order.Items)
	assert.True(t, len(order.Items) >= 1 && len(order.Items) <= 3, "Should have 1-3 items")
	assert.True(t, order.Total > 0, "Total should be positive")

	// Verify total calculation
	calculatedTotal := 0.0
	for _, item := range order.Items {
		assert.NotEmpty(t, item.ProductID)
		assert.True(t, item.Quantity >= 1 && item.Quantity <= 3, "Quantity should be 1-3")
		assert.True(t, item.Price >= 10.00 && item.Price <= 109.99, "Price should be in expected range")
		calculatedTotal += item.Price * float64(item.Quantity)
	}
	assert.InDelta(t, calculatedTotal, order.Total, 0.01, "Total should match sum of items")
}

func TestMakeOrderRequest_Success(t *testing.T) {
	// Create mock server that returns success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/orders", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	success := makeOrderRequest(server.URL)
	assert.True(t, success)
}

func TestMakeOrderRequest_Failure(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	success := makeOrderRequest(server.URL)
	assert.False(t, success)
}

func TestMakeOrderRequest_InvalidURL(t *testing.T) {
	success := makeOrderRequest("invalid-url")
	assert.False(t, success)
}

func TestRunLoadTest_ShortDuration(t *testing.T) {
	// Create mock server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusCreated)
		time.Sleep(10 * time.Millisecond) // Simulate processing time
	}))
	defer server.Close()

	config := LoadTestConfig{
		OrderServiceURL: server.URL,
		RequestsPerSec:  5,  // 5 requests per second
		DurationSec:     1,  // 1 second duration
		MaxConcurrent:   10,
	}

	result := runLoadTest(config)

	assert.True(t, result.TotalRequests >= 3, "Should have made at least 3 requests") // Allow some variance
	assert.True(t, result.TotalRequests <= 7, "Should not exceed expected requests")
	assert.Equal(t, result.TotalRequests, result.SuccessRequests, "All requests should succeed")
	assert.Equal(t, 0, result.FailedRequests)
	assert.True(t, result.AvgResponseTime > 0, "Should have positive average response time")
	assert.True(t, result.MinResponseTime > 0, "Should have positive min response time")
	assert.True(t, result.MaxResponseTime >= result.MinResponseTime, "Max should be >= min response time")
}

func TestRunLoadTest_MixedResults(t *testing.T) {
	// Create mock server that fails some requests
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount%3 == 0 { // Fail every 3rd request
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		time.Sleep(5 * time.Millisecond)
	}))
	defer server.Close()

	config := LoadTestConfig{
		OrderServiceURL: server.URL,
		RequestsPerSec:  10,
		DurationSec:     1,
		MaxConcurrent:   5,
	}

	result := runLoadTest(config)

	assert.True(t, result.TotalRequests > 0, "Should have made requests")
	assert.True(t, result.SuccessRequests > 0, "Should have some successful requests")
	assert.True(t, result.FailedRequests > 0, "Should have some failed requests")
	assert.Equal(t, result.TotalRequests, result.SuccessRequests+result.FailedRequests, "Total should equal success + failed")
}

func TestRunLoadTest_ConcurrencyLimit(t *testing.T) {
	// Create mock server with slow responses to test concurrency
	activeRequests := 0
	maxConcurrent := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		activeRequests++
		if activeRequests > maxConcurrent {
			maxConcurrent = activeRequests
		}
		
		time.Sleep(100 * time.Millisecond) // Hold the request for some time
		w.WriteHeader(http.StatusCreated)
		
		activeRequests--
	}))
	defer server.Close()

	config := LoadTestConfig{
		OrderServiceURL: server.URL,
		RequestsPerSec:  20, // High request rate
		DurationSec:     1,
		MaxConcurrent:   3,  // Low concurrency limit
	}

	result := runLoadTest(config)

	assert.True(t, result.TotalRequests > 0, "Should have made requests")
	assert.True(t, maxConcurrent <= config.MaxConcurrent+1, "Should respect concurrency limit (allow +1 for timing)")
}

func TestPrintResults(t *testing.T) {
	result := TestResult{
		TotalRequests:   100,
		SuccessRequests: 95,
		FailedRequests:  5,
		AvgResponseTime: 150 * time.Millisecond,
		MaxResponseTime: 500 * time.Millisecond,
		MinResponseTime: 50 * time.Millisecond,
	}

	// This test mainly ensures printResults doesn't panic
	// In a real scenario, you might capture stdout to verify output format
	assert.NotPanics(t, func() {
		printResults(result)
	})
}

func TestOrder_Structure(t *testing.T) {
	order := Order{
		UserID: "test-user",
		Items: []Item{
			{ProductID: "prod-1", Quantity: 2, Price: 25.50},
			{ProductID: "prod-2", Quantity: 1, Price: 15.00},
		},
		Total: 66.00,
	}

	assert.Equal(t, "test-user", order.UserID)
	assert.Len(t, order.Items, 2)
	assert.Equal(t, 66.00, order.Total)
	assert.Equal(t, "prod-1", order.Items[0].ProductID)
	assert.Equal(t, 2, order.Items[0].Quantity)
	assert.Equal(t, 25.50, order.Items[0].Price)
}

func TestItem_Structure(t *testing.T) {
	item := Item{
		ProductID: "test-product",
		Quantity:  3,
		Price:     19.99,
	}

	assert.Equal(t, "test-product", item.ProductID)
	assert.Equal(t, 3, item.Quantity)
	assert.Equal(t, 19.99, item.Price)
}

func TestTestResult_Calculations(t *testing.T) {
	result := TestResult{
		TotalRequests:   50,
		SuccessRequests: 45,
		FailedRequests:  5,
	}

	successRate := float64(result.SuccessRequests) / float64(result.TotalRequests) * 100
	failureRate := float64(result.FailedRequests) / float64(result.TotalRequests) * 100

	assert.Equal(t, 90.0, successRate)
	assert.Equal(t, 10.0, failureRate)
	assert.Equal(t, result.TotalRequests, result.SuccessRequests+result.FailedRequests)
}

// Benchmark tests for load testing components
func BenchmarkGenerateRandomOrder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = generateRandomOrder()
	}
}

func BenchmarkMakeOrderRequest(b *testing.B) {
	// Create a fast mock server for benchmarking
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		makeOrderRequest(server.URL)
	}
}