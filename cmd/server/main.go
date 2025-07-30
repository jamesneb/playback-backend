package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	_ "github.com/jamesneb/playback-backend/docs" // Import generated docs
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/pkg/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	// Create Gin engine without default middleware to avoid warning
	r := gin.New()

	// Add middleware manually for better control
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Set trusted proxies
	if len(cfg.Server.TrustedProxies) > 0 {
		r.SetTrustedProxies(cfg.Server.TrustedProxies)
	} else {
		// Disable all proxy trust for security
		r.SetTrustedProxies(nil)
	}

	// Swagger endpoint (only in debug mode)
	if cfg.Swagger.Enabled && cfg.Server.Mode != "release" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Initialize handlers
	traceHandler := handlers.NewTraceHandler()
	metricsHandler := handlers.NewMetricsHandler()
	logsHandler := handlers.NewLogsHandler()

	// API routes
	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "ok",
				"mode":   cfg.Server.Mode,
			})
		})

		// OpenTelemetry endpoints
		api.POST("/traces", traceHandler.CreateTrace)
		api.GET("/traces/:id", traceHandler.GetTrace)

		api.POST("/metrics", metricsHandler.CreateMetrics)
		api.GET("/metrics", metricsHandler.GetMetrics)

		api.POST("/logs", logsHandler.CreateLogs)
		api.GET("/logs", logsHandler.GetLogs)
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting server in %s mode on %s", cfg.Server.Mode, addr)
	r.Run(addr)
}
