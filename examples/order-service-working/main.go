package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"os"
	"runtime"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Order struct {
	ID       string  `json:"id"`
	UserID   string  `json:"user_id"`
	Items    []Item  `json:"items"`
	Total    float64 `json:"total"`
	Status   string  `json:"status"`
	Created  string  `json:"created"`
}

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type PaymentRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
	UserID  string  `json:"user_id"`
}

type PaymentResponse struct {
	Success       bool   `json:"success"`
	TransactionID string `json:"transaction_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

type InventoryRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type InventoryResponse struct {
	Available bool `json:"available"`
	Stock     int  `json:"stock"`
}

var (
	tracer trace.Tracer
	meter  metric.Meter
	logger *zap.Logger
	otelLogger otellog.Logger
	httpClient *http.Client
	
	// Metrics
	orderCounter       metric.Int64Counter
	orderDuration      metric.Float64Histogram
	inventoryCounter   metric.Int64Counter
	paymentCounter     metric.Int64Counter
	errorCounter       metric.Int64Counter
)

func createEnhancedResource() *resource.Resource {
	// Detect environment information
	hostname, _ := os.Hostname()
	workingDir, _ := os.Getwd()

	// Create resource with rich environment information
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			// Service information
			attribute.String("service.name", "order-service"),
			attribute.String("service.version", "1.0.0"),
			attribute.String("service.namespace", "ecommerce"),
			attribute.String("service.instance.id", fmt.Sprintf("instance-%d", time.Now().UnixNano())),

			// Deployment information  
			attribute.String("deployment.environment", getEnvironment()),
			attribute.String("deployment.type", getDeploymentType()),

			// Host information
			attribute.String("host.name", hostname),
			attribute.String("host.arch", runtime.GOARCH),
			attribute.String("host.working_dir", workingDir),

			// Runtime information
			attribute.String("telemetry.sdk.name", "opentelemetry"),
			attribute.String("telemetry.sdk.language", "go"),
			attribute.String("telemetry.sdk.version", "1.27.0"),
			attribute.String("process.runtime.name", "go"),
			attribute.String("process.runtime.version", runtime.Version()),
			attribute.Int("process.pid", os.Getpid()),

			// Cloud/container detection
			attribute.String("cloud.provider", detectCloudProvider()),
			attribute.String("container.runtime", detectContainerRuntime()),
		),
	)
	if err != nil {
		log.Fatal("Failed to create resource:", err)
	}
	
	return res
}

func getEnvironment() string {
	if env := os.Getenv("ENVIRONMENT"); env != "" {
		return env
	}
	if env := os.Getenv("NODE_ENV"); env != "" {
		return env
	}
	return "development"
}

func getDeploymentType() string {
	if os.Getenv("AWS_EXECUTION_ENV") != "" {
		return "aws_lambda"
	}
	if os.Getenv("ECS_CONTAINER_METADATA_URI_V4") != "" {
		return "aws_ecs"
	}
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return "kubernetes"
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	return "local"
}

func detectCloudProvider() string {
	if os.Getenv("AWS_REGION") != "" || os.Getenv("AWS_DEFAULT_REGION") != "" {
		return "aws"
	}
	if os.Getenv("GOOGLE_CLOUD_PROJECT") != "" {
		return "gcp"
	}
	if os.Getenv("AZURE_CLIENT_ID") != "" {
		return "azure"
	}
	return "unknown"
}

func detectContainerRuntime() string {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return "docker"
	}
	if os.Getenv("container") != "" {
		return os.Getenv("container")
	}
	return "unknown"
}

func initObservability() {
	// Initialize all three pillars
	initTracing()
	initMetrics() 
	initLogging()
	initHTTPClient()
}

func initTracing() {
	// Create OTLP gRPC exporter
	otlpExporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithEndpoint("playback-backend:4317"),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create OTLP exporter:", err)
	}

	// Create stdout exporter for console visibility
	stdoutExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		log.Fatal("Failed to create stdout trace exporter:", err)
	}

	// Create enhanced resource with environment detection
	res := createEnhancedResource()

	// Create trace provider with both exporters and shorter batch timeout
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(otlpExporter, 
			sdktrace.WithBatchTimeout(1*time.Second),  // Flush every 1 second
			sdktrace.WithMaxExportBatchSize(10),       // Smaller batch size
		),
		sdktrace.WithBatcher(stdoutExporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tracer = otel.Tracer("order-service")
}

func initMetrics() {
	// Create OTLP gRPC exporter for metrics
	otlpExporter, err := otlpmetricgrpc.New(context.Background(),
		otlpmetricgrpc.WithEndpoint("playback-backend:4317"),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create OTLP metrics exporter:", err)
	}

	// Create stdout exporter for console visibility
	stdoutExporter, err := stdoutmetric.New()
	if err != nil {
		log.Fatal("Failed to create stdout metrics exporter:", err)
	}

	// Create enhanced resource with environment detection
	res := createEnhancedResource()

	// Create metric provider with both exporters
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(otlpExporter, sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(stdoutExporter, sdkmetric.WithInterval(3*time.Second))),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)
	meter = otel.Meter("order-service")

	// Create metrics instruments
	var err2 error
	orderCounter, err2 = meter.Int64Counter("orders_total", metric.WithDescription("Total number of orders"))
	if err2 != nil {
		log.Fatal("Failed to create order counter:", err2)
	}

	orderDuration, err2 = meter.Float64Histogram("order_duration_seconds", metric.WithDescription("Order processing duration"))
	if err2 != nil {
		log.Fatal("Failed to create order duration histogram:", err2)
	}

	inventoryCounter, err2 = meter.Int64Counter("inventory_checks_total", metric.WithDescription("Total inventory checks"))
	if err2 != nil {
		log.Fatal("Failed to create inventory counter:", err2)
	}

	paymentCounter, err2 = meter.Int64Counter("payments_total", metric.WithDescription("Total payment attempts"))
	if err2 != nil {
		log.Fatal("Failed to create payment counter:", err2)
	}

	errorCounter, err2 = meter.Int64Counter("errors_total", metric.WithDescription("Total errors"))
	if err2 != nil {
		log.Fatal("Failed to create error counter:", err2)
	}
}

func initLogging() {
	// Create OTLP gRPC exporter for logs
	otlpExporter, err := otlploggrpc.New(context.Background(),
		otlploggrpc.WithEndpoint("playback-backend:4317"),
		otlploggrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create OTLP log exporter:", err)
	}

	// Create stdout exporter for console visibility
	stdoutExporter, err := stdoutlog.New()
	if err != nil {
		log.Fatal("Failed to create stdout logs exporter:", err)
	}

	// Create enhanced resource with environment detection
	res := createEnhancedResource()

	// Create log provider with simple processors for immediate export
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(otlpExporter)),
		sdklog.WithProcessor(sdklog.NewSimpleProcessor(stdoutExporter)),
		sdklog.WithResource(res),
	)
	
	// Set the global logger provider
	global.SetLoggerProvider(loggerProvider)
	
	// Initialize OTel logger
	otelLogger = global.GetLoggerProvider().Logger("order-service")

	// Test log emission on startup
	log.Printf("OTLP log exporter initialized - endpoint: playback-backend:4317")
	
	// Test OTLP log emission
	var startupLogRecord otellog.Record
	startupLogRecord.SetTimestamp(time.Now())
	startupLogRecord.SetObservedTimestamp(time.Now())
	startupLogRecord.SetSeverity(otellog.SeverityInfo)
	startupLogRecord.SetSeverityText("INFO")
	startupLogRecord.SetBody(otellog.StringValue("Order service OTLP logs initialized"))
	startupLogRecord.AddAttributes(
		otellog.String("service.startup", "true"),
		otellog.String("log.test", "startup"),
	)
	otelLogger.Emit(context.Background(), startupLogRecord)
	log.Printf("Emitted startup OTLP log record")

	// Create structured logger  
	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	
	logger, err = config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
}

func initHTTPClient() {
	// Create HTTP client with OpenTelemetry instrumentation
	httpClient = &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
		Timeout:   30 * time.Second,
	}
}

func main() {
	initObservability()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	
	// Add OpenTelemetry middleware
	r.Use(otelgin.Middleware("order-service"))
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Routes
	r.POST("/orders", createOrder)
	r.GET("/orders/:id", getOrder)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "order-service"})
	})

	log.Println("Order service starting on :8081")
	r.Run(":8081")
}

func createOrder(c *gin.Context) {
	start := time.Now()
	ctx := c.Request.Context()
	span := trace.SpanFromContext(ctx)
	
	logger.Info("Order creation started", 
		zap.String("endpoint", "/orders"),
		zap.String("method", "POST"),
		zap.String("event.type", "order.started"),
	)
	
	// Direct OTLP log emission
	var logRecord otellog.Record
	logRecord.SetTimestamp(start)
	logRecord.SetObservedTimestamp(start)
	logRecord.SetSeverity(otellog.SeverityInfo)
	logRecord.SetSeverityText("INFO")
	logRecord.SetBody(otellog.StringValue("Order creation started"))
	logRecord.AddAttributes(
		otellog.String("endpoint", "/orders"),
		otellog.String("method", "POST"),
		otellog.String("event.type", "order.started"),
	)
	otelLogger.Emit(ctx, logRecord)
	
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		span.RecordError(err)
		logger.Error("Invalid order request", 
			zap.Error(err),
			zap.String("user_agent", c.GetHeader("User-Agent")),
		)
		
		// Record error metric
		errorCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error_type", "validation"),
			attribute.String("endpoint", "/orders"),
		))
		
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Generate order ID
	order.ID = fmt.Sprintf("order_%d", time.Now().UnixNano())
	order.Created = time.Now().Format(time.RFC3339)
	order.Status = "pending"

	span.SetAttributes(
		attribute.String("order.id", order.ID),
		attribute.String("order.user_id", order.UserID),
		attribute.Int("order.item_count", len(order.Items)),
		attribute.Float64("order.total", order.Total),
	)

	logger.Info("Processing order", 
		zap.String("order_id", order.ID),
		zap.String("user_id", order.UserID),
		zap.Int("item_count", len(order.Items)),
		zap.Float64("total", order.Total),
	)

	// Check inventory for each item
	for _, item := range order.Items {
		if !checkInventory(ctx, item.ProductID, item.Quantity) {
			span.SetAttributes(attribute.String("order.failure_reason", "insufficient_inventory"))
			order.Status = "failed"
			
			logger.Warn("Order failed - insufficient inventory", 
				zap.String("order_id", order.ID),
				zap.String("product_id", item.ProductID),
				zap.Int("requested_quantity", item.Quantity),
			)
			
			// Record failure metrics
			orderCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("status", "failed"),
				attribute.String("reason", "inventory"),
			))
			errorCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("error_type", "inventory_insufficient"),
			))
			
			c.JSON(400, gin.H{"error": "insufficient inventory", "product": item.ProductID})
			return
		}
	}

	// Process payment
	if !processPayment(ctx, order.ID, order.Total, order.UserID) {
		span.SetAttributes(attribute.String("order.failure_reason", "payment_failed"))
		order.Status = "failed"
		
		logger.Error("Order failed - payment processing error", 
			zap.String("order_id", order.ID),
			zap.Float64("amount", order.Total),
		)
		
		// Record failure metrics
		orderCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("status", "failed"),
			attribute.String("reason", "payment"),
		))
		errorCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("error_type", "payment_failed"),
		))
		
		c.JSON(400, gin.H{"error": "payment failed"})
		return
	}

	// Simulate processing time
	processingTime := time.Millisecond * time.Duration(rand.Intn(100)+50)
	time.Sleep(processingTime)

	order.Status = "confirmed"
	span.SetAttributes(attribute.String("order.status", order.Status))
	
	// Record success metrics
	duration := time.Since(start).Seconds()
	orderCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", "success"),
	))
	orderDuration.Record(ctx, duration, metric.WithAttributes(
		attribute.String("status", "success"),
	))

	logger.Info("Order completed successfully", 
		zap.String("order_id", order.ID),
		zap.Duration("processing_time", time.Since(start)),
		zap.String("status", order.Status),
		zap.Float64("order_total", order.Total),
		zap.String("event.type", "order.completed"),
	)
	
	// Direct OTLP log completion record
	now := time.Now()
	var completionLogRecord otellog.Record
	completionLogRecord.SetTimestamp(now)
	completionLogRecord.SetObservedTimestamp(now)
	completionLogRecord.SetSeverity(otellog.SeverityInfo)
	completionLogRecord.SetSeverityText("INFO")
	completionLogRecord.SetBody(otellog.StringValue("Order completed successfully"))
	completionLogRecord.AddAttributes(
		otellog.String("order.id", order.ID),
		otellog.String("order.status", order.Status),
		otellog.Float64("order.total", order.Total),
		otellog.Int64("processing_time_ms", time.Since(start).Milliseconds()),
		otellog.String("event.type", "order.completed"),
	)
	otelLogger.Emit(ctx, completionLogRecord)

	c.JSON(201, order)
}

func getOrder(c *gin.Context) {
	ctx := c.Request.Context()
	span := trace.SpanFromContext(ctx)
	
	orderID := c.Param("id")
	span.SetAttributes(attribute.String("order.id", orderID))

	// Simulate database lookup
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(50)+10))

	// Mock order data
	order := Order{
		ID:      orderID,
		UserID:  "user_123",
		Status:  "confirmed",
		Total:   99.99,
		Created: time.Now().Add(-time.Hour).Format(time.RFC3339),
		Items: []Item{
			{ProductID: "prod_1", Quantity: 1, Price: 99.99},
		},
	}

	c.JSON(200, order)
}

func checkInventory(ctx context.Context, productID string, quantity int) bool {
	ctx, span := tracer.Start(ctx, "check_inventory")
	defer span.End()

	span.SetAttributes(
		attribute.String("inventory.product_id", productID),
		attribute.Int("inventory.requested_quantity", quantity),
	)

	logger.Debug("Checking inventory", 
		zap.String("product_id", productID),
		zap.Int("quantity", quantity),
	)

	// Create request payload
	reqData := map[string]interface{}{
		"product_id": productID,
		"quantity":   quantity,
	}
	
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to marshal inventory request", zap.Error(err))
		return false
	}

	// Make HTTP POST request with tracing
	req, err := http.NewRequestWithContext(ctx, "POST", "http://httpbin.org/delay/1", bytes.NewBuffer(jsonData))
	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to create inventory request", zap.Error(err))
		return false
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service", "inventory-service")

	// This will automatically create HTTP client spans
	resp, err := httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		logger.Error("Inventory service request failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	// Mock response - 90% success rate
	available := rand.Float32() < 0.9
	currentStock := rand.Intn(100) + 10
	
	span.SetAttributes(
		attribute.Bool("inventory.available", available),
		attribute.Int("inventory.current_stock", currentStock),
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("http.url", req.URL.String()),
	)

	// Record inventory check metric
	inventoryCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("product_id", productID),
		attribute.Bool("available", available),
	))

	if !available {
		span.RecordError(fmt.Errorf("insufficient inventory for product %s", productID))
		logger.Warn("Inventory check failed", 
			zap.String("product_id", productID),
			zap.Int("requested", quantity),
			zap.Int("available", currentStock),
		)
	} else {
		logger.Debug("Inventory check passed", 
			zap.String("product_id", productID),
			zap.Int("available_stock", currentStock),
		)
	}

	return available
}

func processPayment(ctx context.Context, orderID string, amount float64, userID string) bool {
	ctx, span := tracer.Start(ctx, "process_payment")
	defer span.End()

	span.SetAttributes(
		attribute.String("payment.order_id", orderID),
		attribute.Float64("payment.amount", amount),
		attribute.String("payment.user_id", userID),
	)

	logger.Info("Processing payment", 
		zap.String("order_id", orderID),
		zap.Float64("amount", amount),
		zap.String("user_id", userID),
	)

	// Create request payload
	reqData := map[string]interface{}{
		"order_id": orderID,
		"amount":   amount,
		"user_id":  userID,
	}
	
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to marshal payment request", zap.Error(err))
		return false
	}

	// Make HTTP POST request with tracing (using longer delay for payment)
	req, err := http.NewRequestWithContext(ctx, "POST", "http://httpbin.org/delay/2", bytes.NewBuffer(jsonData))
	if err != nil {
		span.RecordError(err)
		logger.Error("Failed to create payment request", zap.Error(err))
		return false
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service", "payment-service")

	// This will automatically create HTTP client spans
	resp, err := httpClient.Do(req)
	if err != nil {
		span.RecordError(err)
		logger.Error("Payment service request failed", zap.Error(err))
		return false
	}
	defer resp.Body.Close()

	// Mock response - 95% success rate
	success := rand.Float32() < 0.95
	
	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.String("http.url", req.URL.String()),
	)

	// Record payment attempt metric
	paymentCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.Bool("success", success),
		attribute.String("user_id", userID),
	))
	
	if success {
		transactionID := fmt.Sprintf("txn_%d", time.Now().UnixNano())
		span.SetAttributes(
			attribute.String("payment.transaction_id", transactionID),
			attribute.String("payment.status", "success"),
		)
		
		logger.Info("Payment processed successfully", 
			zap.String("order_id", orderID),
			zap.String("transaction_id", transactionID),
			zap.Float64("amount", amount),
		)
	} else {
		span.RecordError(fmt.Errorf("payment failed for order %s", orderID))
		span.SetAttributes(attribute.String("payment.status", "failed"))
		
		logger.Error("Payment processing failed", 
			zap.String("order_id", orderID),
			zap.Float64("amount", amount),
			zap.String("user_id", userID),
		)
	}

	return success
}