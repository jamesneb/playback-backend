package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/gin-gonic/gin"
	_ "github.com/jamesneb/playback-backend/docs" // Import generated docs
	grpcserver "github.com/jamesneb/playback-backend/internal/grpc"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/handlers/realtime"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// @title Playback Backend API
// @version 1.0
// @description Distributed systems event replay backend
// @host localhost:8080
// @BasePath /api/v1
func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode from config
	gin.SetMode(cfg.Server.Mode)

	// Initialize ClickHouse client for real-time path
	clickhouseClient, err := storage.NewClickHouseClient(&storage.ClickHouseConfig{
		Host:               cfg.Database.ClickHouse.Host,
		Database:           cfg.Database.ClickHouse.Database,
		Username:           cfg.Database.ClickHouse.Username,
		Password:           cfg.Database.ClickHouse.Password,
		MaxConnections:     10,
		MaxIdleConnections: 5,
	})
	if err != nil {
		log.Fatalf("Failed to initialize ClickHouse client: %v", err)
	}
	defer clickhouseClient.Close()

	// Initialize Kinesis client
	kinesisClient, err := streaming.NewKinesisClient(&cfg.Streaming.Kinesis)
	if err != nil {
		log.Fatalf("Failed to initialize Kinesis client: %v", err)
	}
	defer kinesisClient.Close()

	// Initialize S3 client for replay files
	s3Client, err := storage.NewS3Client(&storage.S3Config{
		Region:          cfg.Streaming.Kinesis.Region, // Reuse Kinesis config
		EndpointURL:     cfg.Streaming.Kinesis.EndpointURL,
		AccessKeyID:     cfg.Streaming.Kinesis.AccessKeyID,
		SecretAccessKey: cfg.Streaming.Kinesis.SecretAccessKey,
		Bucket:          "replays", // Match S3 init bucket name
		ForcePathStyle:  true,               // For LocalStack compatibility
	})
	if err != nil {
		log.Fatalf("Failed to initialize S3 client: %v", err)
	}

	// Create handlers
	kinesisHandler := streaming.NewKinesisHandler(kinesisClient)
	clickhouseHandler := realtime.NewClickHouseHandler(clickhouseClient)

	// Create HTTP server
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	
	// Add CORS middleware for frontend communication
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, Cache-Control, Pragma, Expires")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// Set trusted proxies
	if len(cfg.Server.TrustedProxies) > 0 {
		r.SetTrustedProxies(cfg.Server.TrustedProxies)
	} else {
		r.SetTrustedProxies(nil)
	}

	// Swagger endpoint
	if cfg.Swagger.Enabled {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Initialize HTTP handlers with Kinesis client (legacy compatibility)
	traceHandler := handlers.NewTraceHandler(kinesisClient)
	metricsHandler := handlers.NewMetricsHandler(kinesisClient)
	logsHandler := handlers.NewLogsHandler(kinesisClient)
	replayHandler := handlers.NewReplayHandler(s3Client, "replays")

	// API routes
	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status":    "ok",
				"mode":      cfg.Server.Mode,
				"version":   cfg.App.Version,
				"protocols": []string{"HTTP/JSON", "gRPC/OTLP"},
			})
		})

		// OpenTelemetry HTTP endpoints (legacy)
		api.POST("/traces", traceHandler.CreateTrace)
		api.GET("/traces/:id", traceHandler.GetTrace)

		api.POST("/metrics", metricsHandler.CreateMetrics)
		api.GET("/metrics", metricsHandler.GetMetrics)

		api.POST("/logs", logsHandler.CreateLogs)
		api.GET("/logs", logsHandler.GetLogs)

		// Replay endpoints
		api.GET("/replays/list", replayHandler.ListReplays)
		api.POST("/replays/download", replayHandler.DownloadReplay)
	}

	// Create gRPC server
	grpcAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, 4317) // Standard OTLP gRPC port
	grpcSrv := grpcserver.NewServer(grpcAddr, kinesisHandler, clickhouseHandler)

	// Setup graceful shutdown
	// Note: ctx is used for potential future cancellation context

	var wg sync.WaitGroup

	// Start HTTP server
	httpAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Starting HTTP server",
			zap.String("address", httpAddr),
			zap.String("protocols", "HTTP/JSON"))
		if err := r.Run(httpAddr); err != nil {
			logger.Error("HTTP server failed", zap.Error(err))
		}
	}()

	// Start gRPC server
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Starting gRPC server",
			zap.String("address", grpcAddr),
			zap.String("protocols", "OTLP/gRPC"))
		if err := grpcSrv.Start(); err != nil {
			logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Playback backend started successfully",
		zap.String("http_address", httpAddr),
		zap.String("grpc_address", grpcAddr),
		zap.String("version", cfg.App.Version))

	<-sigCh
	logger.Info("Shutdown signal received, stopping servers...")

	// Stop gRPC server
	grpcSrv.Stop()

	// Wait for goroutines to finish
	wg.Wait()
	logger.Info("All servers stopped successfully")

	defer logger.Sync()
}