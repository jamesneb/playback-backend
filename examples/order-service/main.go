package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
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
	
	// Metrics
	orderCounter       metric.Int64Counter
	orderDuration      metric.Float64Histogram
	inventoryCounter   metric.Int64Counter
	paymentCounter     metric.Int64Counter
	errorCounter       metric.Int64Counter
)

func initObservability() {
	// Initialize all three pillars
	initTracing()
	initMetrics() 
	initLogging()
}

func initTracing() {
	// Create OTLP HTTP exporter
	exporter, err := otlptracehttp.New(context.Background(),
		otlptracehttp.WithEndpoint("http://localhost:8080"), // Your playback-backend
		otlptracehttp.WithURLPath("/api/v1/traces"),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create OTLP exporter:", err)
	}

	// Create resource
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("order-service"),
		semconv.ServiceVersionKey.String("1.0.0"),
		semconv.DeploymentEnvironmentKey.String("development"),
	)

	// Create trace provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	tracer = otel.Tracer("order-service")
}

func initMetrics() {
	// Create OTLP HTTP exporter for metrics
	exporter, err := otlpmetrichttp.New(context.Background(),
		otlpmetrichttp.WithEndpoint("http://localhost:8080"),
		otlpmetrichttp.WithURLPath("/api/v1/metrics"),
		otlpmetrichttp.WithInsecure(),
	)
	if err != nil {
		log.Fatal("Failed to create OTLP metrics exporter:", err)
	}

	// Create resource
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("order-service"),
		semconv.ServiceVersionKey.String("1.0.0"),
	)

	// Create metric provider
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(5*time.Second))),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)
	meter = otel.Meter("order-service")

	// Create metrics instruments
	var err error
	orderCounter, err = meter.Int64Counter("orders_total", metric.WithDescription("Total number of orders"))
	if err != nil {
		log.Fatal("Failed to create order counter:", err)
	}

	orderDuration, err = meter.Float64Histogram("order_duration_seconds", metric.WithDescription("Order processing duration"))
	if err != nil {
		log.Fatal("Failed to create order duration histogram:", err)
	}

	inventoryCounter, err = meter.Int64Counter("inventory_checks_total", metric.WithDescription("Total inventory checks"))
	if err != nil {
		log.Fatal("Failed to create inventory counter:", err)
	}

	paymentCounter, err = meter.Int64Counter("payments_total", metric.WithDescription("Total payment attempts"))
	if err != nil {
		log.Fatal("Failed to create payment counter:", err)
	}

	errorCounter, err = meter.Int64Counter("errors_total", metric.WithDescription("Total errors"))
	if err != nil {
		log.Fatal("Failed to create error counter:", err)
	}
}

func initLogging() {
	// Create stdout exporter for logs (since OTLP logs are experimental)
	exporter, err := stdoutlog.New()
	if err != nil {
		log.Fatal("Failed to create stdout logs exporter:", err)
	}

	// Create resource
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("order-service"),
		semconv.ServiceVersionKey.String("1.0.0"),
	)

	// Create log provider
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	otel.SetLoggerProvider(lp)

	// Create structured logger with OTel integration
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
	)
	
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
	)

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

	// Simulate HTTP call to inventory service
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)+20))

	// Mock response - 90% success rate
	available := rand.Float32() < 0.9
	currentStock := rand.Intn(100) + 10
	
	span.SetAttributes(
		attribute.Bool("inventory.available", available),
		attribute.Int("inventory.current_stock", currentStock),
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

	// Simulate HTTP call to payment service
	time.Sleep(time.Millisecond * time.Duration(rand.Intn(200)+100))

	// Mock response - 95% success rate
	success := rand.Float32() < 0.95
	
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