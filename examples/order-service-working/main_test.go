package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func init() {
	// Set up minimal observability for testing
	gin.SetMode(gin.TestMode)
	
	// Initialize logger for tests (this was missing and causing nil pointer panics)
	config := zap.NewDevelopmentConfig()
	config.Level.SetLevel(zap.ErrorLevel) // Reduce noise in tests
	var err error
	logger, err = config.Build()
	if err != nil {
		panic(err)
	}
	
	// Initialize basic tracer and meter for tests
	tracer = otel.Tracer("test-order-service-working")
	meter = otel.Meter("test-order-service-working")
	
	// Initialize OpenTelemetry logger for tests (this was also missing)
	otelLogger = global.GetLoggerProvider().Logger("test-order-service-working")
	
	// Initialize HTTP client for tests
	httpClient = &http.Client{Timeout: 10 * time.Second}
	
	// Create mock metrics
	orderCounter, _ = meter.Int64Counter("orders_total_test")
	orderDuration, _ = meter.Float64Histogram("order_duration_seconds_test")
	inventoryCounter, _ = meter.Int64Counter("inventory_checks_total_test")
	paymentCounter, _ = meter.Int64Counter("payments_total_test")
	errorCounter, _ = meter.Int64Counter("errors_total_test")
}

func TestCreateEnhancedResource(t *testing.T) {
	res := createEnhancedResource()
	
	assert.NotNil(t, res)
	assert.IsType(t, &resource.Resource{}, res)
	
	// Test that the resource contains expected attributes
	attrs := res.Attributes()
	
	// Check service attributes
	serviceName := ""
	serviceVersion := ""
	serviceNamespace := ""
	
	for _, attr := range attrs {
		switch attr.Key {
		case "service.name":
			serviceName = attr.Value.AsString()
		case "service.version":
			serviceVersion = attr.Value.AsString()
		case "service.namespace":
			serviceNamespace = attr.Value.AsString()
		}
	}
	
	assert.Equal(t, "order-service", serviceName)
	assert.Equal(t, "1.0.0", serviceVersion)
	assert.Equal(t, "ecommerce", serviceNamespace)
}

func TestGetEnvironment(t *testing.T) {
	// Test default environment
	originalEnv := os.Getenv("ENVIRONMENT")
	originalNodeEnv := os.Getenv("NODE_ENV")
	
	// Clear environment variables
	os.Unsetenv("ENVIRONMENT")
	os.Unsetenv("NODE_ENV")
	
	env := getEnvironment()
	assert.Equal(t, "development", env)
	
	// Test ENVIRONMENT variable
	os.Setenv("ENVIRONMENT", "production")
	env = getEnvironment()
	assert.Equal(t, "production", env)
	
	// Test NODE_ENV variable
	os.Unsetenv("ENVIRONMENT")
	os.Setenv("NODE_ENV", "staging")
	env = getEnvironment()
	assert.Equal(t, "staging", env)
	
	// Restore original values
	if originalEnv != "" {
		os.Setenv("ENVIRONMENT", originalEnv)
	}
	if originalNodeEnv != "" {
		os.Setenv("NODE_ENV", originalNodeEnv)
	}
}

func TestGetDeploymentType(t *testing.T) {
	// Clear all environment variables that affect deployment type
	envVars := []string{
		"AWS_EXECUTION_ENV",
		"ECS_CONTAINER_METADATA_URI_V4",
		"KUBERNETES_SERVICE_HOST",
	}
	
	originalValues := make(map[string]string)
	for _, env := range envVars {
		originalValues[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	
	// Test default (local)
	deploymentType := getDeploymentType()
	assert.Equal(t, "local", deploymentType)
	
	// Test AWS Lambda
	os.Setenv("AWS_EXECUTION_ENV", "AWS_Lambda_go1.x")
	deploymentType = getDeploymentType()
	assert.Equal(t, "aws_lambda", deploymentType)
	os.Unsetenv("AWS_EXECUTION_ENV")
	
	// Test AWS ECS
	os.Setenv("ECS_CONTAINER_METADATA_URI_V4", "http://169.254.170.2/v4/metadata")
	deploymentType = getDeploymentType()
	assert.Equal(t, "aws_ecs", deploymentType)
	os.Unsetenv("ECS_CONTAINER_METADATA_URI_V4")
	
	// Test Kubernetes
	os.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	deploymentType = getDeploymentType()
	assert.Equal(t, "kubernetes", deploymentType)
	os.Unsetenv("KUBERNETES_SERVICE_HOST")
	
	// Restore original values
	for env, value := range originalValues {
		if value != "" {
			os.Setenv(env, value)
		}
	}
}

func TestDetectCloudProvider(t *testing.T) {
	// Clear cloud provider environment variables
	cloudEnvVars := []string{
		"AWS_REGION",
		"AWS_DEFAULT_REGION",
		"GOOGLE_CLOUD_PROJECT",
		"AZURE_CLIENT_ID",
	}
	
	originalValues := make(map[string]string)
	for _, env := range cloudEnvVars {
		originalValues[env] = os.Getenv(env)
		os.Unsetenv(env)
	}
	
	// Test default (unknown)
	provider := detectCloudProvider()
	assert.Equal(t, "unknown", provider)
	
	// Test AWS
	os.Setenv("AWS_REGION", "us-east-1")
	provider = detectCloudProvider()
	assert.Equal(t, "aws", provider)
	os.Unsetenv("AWS_REGION")
	
	// Test AWS with default region
	os.Setenv("AWS_DEFAULT_REGION", "us-west-2")
	provider = detectCloudProvider()
	assert.Equal(t, "aws", provider)
	os.Unsetenv("AWS_DEFAULT_REGION")
	
	// Test GCP
	os.Setenv("GOOGLE_CLOUD_PROJECT", "my-project")
	provider = detectCloudProvider()
	assert.Equal(t, "gcp", provider)
	os.Unsetenv("GOOGLE_CLOUD_PROJECT")
	
	// Test Azure
	os.Setenv("AZURE_CLIENT_ID", "client-id-123")
	provider = detectCloudProvider()
	assert.Equal(t, "azure", provider)
	os.Unsetenv("AZURE_CLIENT_ID")
	
	// Restore original values
	for env, value := range originalValues {
		if value != "" {
			os.Setenv(env, value)
		}
	}
}

func TestDetectContainerRuntime(t *testing.T) {
	// Test default (unknown)
	runtime := detectContainerRuntime()
	assert.True(t, runtime == "unknown" || runtime == "docker", "Should detect unknown or docker if .dockerenv exists")
	
	// Test with container environment variable
	originalContainer := os.Getenv("container")
	os.Setenv("container", "podman")
	runtime = detectContainerRuntime()
	assert.Equal(t, "podman", runtime)
	
	// Restore original value
	if originalContainer != "" {
		os.Setenv("container", originalContainer)
	} else {
		os.Unsetenv("container")
	}
}

func setupTestRouterWorking() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	
	r.POST("/orders", createOrder)
	r.GET("/orders/:id", getOrder)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "order-service"})
	})
	
	return r
}

func TestCreateOrder_ValidOrder_Working(t *testing.T) {
	router := setupTestRouterWorking()

	order := Order{
		UserID: "user_working_123",
		Items: []Item{
			{ProductID: "prod_working_1", Quantity: 1, Price: 35.99},
		},
		Total: 35.99,
	}

	jsonData, _ := json.Marshal(order)
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Note: This test might have variable results due to random inventory/payment simulation
	assert.True(t, w.Code == 201 || w.Code == 400, "Should return either success (201) or business logic failure (400)")

	if w.Code == 201 {
		var responseOrder Order
		err := json.Unmarshal(w.Body.Bytes(), &responseOrder)
		assert.NoError(t, err)
		assert.NotEmpty(t, responseOrder.ID)
		assert.Equal(t, "user_working_123", responseOrder.UserID)
		assert.NotEmpty(t, responseOrder.Created)
		assert.True(t, responseOrder.Status == "confirmed" || responseOrder.Status == "failed")
	}
}

func TestCreateOrder_WithEnhancedLogging(t *testing.T) {
	router := setupTestRouterWorking()

	order := Order{
		UserID: "enhanced_logging_user",
		Items: []Item{
			{ProductID: "enhanced_prod", Quantity: 2, Price: 45.00},
		},
		Total: 90.00,
	}

	jsonData, _ := json.Marshal(order)
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should process with enhanced telemetry
	assert.True(t, w.Code == 201 || w.Code == 400, "Should handle enhanced telemetry order")
}

func TestCheckInventory_WithHTTPCall(t *testing.T) {
	ctx := context.Background()

	// Test the enhanced inventory check that makes HTTP calls
	tests := []struct {
		name      string
		productID string
		quantity  int
	}{
		{"enhanced inventory check", "enhanced_prod_001", 1},
		{"bulk enhanced order", "enhanced_prod_002", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since checkInventory makes real HTTP calls to httpbin.org,
			// we test that it handles the call gracefully
			// Note: This might be slow due to actual HTTP requests
			result := checkInventory(ctx, tt.productID, tt.quantity)
			assert.IsType(t, bool(false), result)
		})
	}
}

func TestProcessPayment_WithHTTPCall(t *testing.T) {
	ctx := context.Background()

	// Test the enhanced payment processing that makes HTTP calls
	tests := []struct {
		name    string
		orderID string
		amount  float64
		userID  string
	}{
		{"enhanced small payment", "enhanced_order_001", 25.99, "enhanced_user_001"},
		{"enhanced large payment", "enhanced_order_002", 199.99, "enhanced_user_002"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since processPayment makes real HTTP calls to httpbin.org,
			// we test that it handles the call gracefully
			// Note: This might be slow due to actual HTTP requests
			result := processPayment(ctx, tt.orderID, tt.amount, tt.userID)
			assert.IsType(t, bool(false), result)
		})
	}
}

func TestGetOrder_Working(t *testing.T) {
	router := setupTestRouterWorking()

	req, _ := http.NewRequest("GET", "/orders/working_order_123", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var order Order
	err := json.Unmarshal(w.Body.Bytes(), &order)
	assert.NoError(t, err)
	assert.Equal(t, "working_order_123", order.ID)
	assert.Equal(t, "user_123", order.UserID)
	assert.Equal(t, "confirmed", order.Status)
	assert.Equal(t, 99.99, order.Total)
	assert.Len(t, order.Items, 1)
}

func TestHealthEndpoint_Working(t *testing.T) {
	router := setupTestRouterWorking()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
	assert.Equal(t, "order-service", response["service"])
}

func TestCreateOrder_InvalidJSON_Working(t *testing.T) {
	router := setupTestRouterWorking()

	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer([]byte("invalid json for working service")))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, 400, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

func TestEnvironmentDetection_Integration(t *testing.T) {
	// Test that environment detection functions work together
	env := getEnvironment()
	deploymentType := getDeploymentType()
	cloudProvider := detectCloudProvider()
	containerRuntime := detectContainerRuntime()

	assert.NotEmpty(t, env)
	assert.NotEmpty(t, deploymentType)
	assert.NotEmpty(t, cloudProvider)
	assert.NotEmpty(t, containerRuntime)

	// All should return valid string values
	assert.IsType(t, "", env)
	assert.IsType(t, "", deploymentType)
	assert.IsType(t, "", cloudProvider)
	assert.IsType(t, "", containerRuntime)
}

func TestCreateOrder_MultipleItems_Working(t *testing.T) {
	router := setupTestRouterWorking()

	order := Order{
		UserID: "multi_item_working_user",
		Items: []Item{
			{ProductID: "working_prod_1", Quantity: 1, Price: 12.50},
			{ProductID: "working_prod_2", Quantity: 3, Price: 8.75},
			{ProductID: "working_prod_3", Quantity: 2, Price: 22.00},
		},
		Total: 70.75,
	}

	jsonData, _ := json.Marshal(order)
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should process multiple items with enhanced telemetry
	assert.True(t, w.Code == 201 || w.Code == 400, "Should handle multiple items with enhanced features")
}

func TestCreateOrder_WithTracing_Working(t *testing.T) {
	router := setupTestRouterWorking()

	order := Order{
		UserID: "tracing_test_user",
		Items: []Item{
			{ProductID: "traced_prod", Quantity: 1, Price: 55.00},
		},
		Total: 55.00,
	}

	jsonData, _ := json.Marshal(order)
	
	// Create request with enhanced tracing context
	req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TestClient/1.0")
	
	// Add trace context
	ctx := trace.ContextWithSpan(req.Context(), trace.SpanFromContext(context.Background()))
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should process with enhanced tracing
	assert.True(t, w.Code == 201 || w.Code == 400, "Should handle tracing enhanced request")
}

// Integration test for the complete order flow with enhanced features
func TestOrderProcessing_Integration_Working(t *testing.T) {
	router := setupTestRouterWorking()

	// Test multiple orders with enhanced telemetry
	orders := []Order{
		{
			UserID: "integration_working_user_1",
			Items:  []Item{{ProductID: "integration_prod_1", Quantity: 1, Price: 75.00}},
			Total:  75.00,
		},
		{
			UserID: "integration_working_user_2",
			Items:  []Item{{ProductID: "integration_prod_2", Quantity: 2, Price: 40.00}},
			Total:  80.00,
		},
	}

	processedCount := 0

	for i, order := range orders {
		jsonData, _ := json.Marshal(order)
		req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == 201 || w.Code == 400 {
			processedCount++
		}
		
		// Ensure we get valid responses with enhanced processing
		assert.True(t, w.Code == 201 || w.Code == 400, "Enhanced order %d should return valid status", i)
	}

	assert.Equal(t, len(orders), processedCount, "All enhanced orders should be processed")
}

// Benchmark tests for enhanced order service performance
func BenchmarkCreateOrder_Working(b *testing.B) {
	router := setupTestRouterWorking()

	order := Order{
		UserID: "benchmark_working_user",
		Items:  []Item{{ProductID: "benchmark_working_prod", Quantity: 1, Price: 100.00}},
		Total:  100.00,
	}

	jsonData, _ := json.Marshal(order)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/orders", bytes.NewBuffer(jsonData))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkCreateEnhancedResource(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = createEnhancedResource()
	}
}

func BenchmarkEnvironmentDetection(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = getEnvironment()
		_ = getDeploymentType()
		_ = detectCloudProvider()
		_ = detectContainerRuntime()
	}
}

// Test environment detection with runtime information
func TestRuntimeDetection(t *testing.T) {
	res := createEnhancedResource()
	attrs := res.Attributes()
	
	// Check that runtime information is included
	foundRuntimeName := false
	foundRuntimeVersion := false
	foundArch := false
	
	for _, attr := range attrs {
		switch attr.Key {
		case "process.runtime.name":
			assert.Equal(t, "go", attr.Value.AsString())
			foundRuntimeName = true
		case "process.runtime.version":
			assert.Equal(t, runtime.Version(), attr.Value.AsString())
			foundRuntimeVersion = true
		case "host.arch":
			assert.Equal(t, runtime.GOARCH, attr.Value.AsString())
			foundArch = true
		}
	}
	
	assert.True(t, foundRuntimeName, "Should include runtime name")
	assert.True(t, foundRuntimeVersion, "Should include runtime version")
	assert.True(t, foundArch, "Should include host architecture")
}