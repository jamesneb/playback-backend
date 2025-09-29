package rest

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/api/rest/middleware"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/services"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/docs"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// setupSwagger configures Swagger documentation if enabled
func setupSwagger(r *gin.Engine, cfg *config.ConsolidatedConfig, endpoints *api.EndpointCollection) error {
	if !cfg.Network.HTTP.EnableSwagger {
		return nil // Disabled
	}

	swaggerPath := endpoints.SwaggerUI()
	if swaggerPath == "" {
		return errors.New("swagger enabled but no swagger path configured")

	}

	// Set up an API route for swagger
	r.GET(swaggerPath, ginSwagger.WrapHandler(swaggerFiles.Handler))

	logger.Info("Swagger documentation enabled", zap.String("path", swaggerPath))
	return nil
}

// setupAPIRoutes configures all API routes
func setupAPIRoutes(r *gin.Engine, deps *Dependencies) error {
	// Allow optional Kinesis client for local development
	if deps.KinesisClient == nil {
		logger.Warn("Kinesis client not available, using stub for local development",
			zap.String("mode", "development"))
	}

	// Create API handlers directly (no caching complexity)
	apiHandlers, err := NewAPIHandlers(deps)
	if err != nil {
		return fmt.Errorf("failed to create API handlers: %w", err)
	}

	// API routes using centralized endpoints with distributed tracing
	api := r.Group(deps.Endpoints.BasePath())
	api.Use(middleware.DistributedTracingMiddleware()) // Add W3C-compliant distributed tracing
	{
		// Health check
		api.GET("/health", healthHandler(deps.Config))

		// OpenTelemetry HTTP endpoints (legacy)
		setupTraceRoutes(api, apiHandlers.Trace, deps.Endpoints)
		setupMetricsRoutes(api, apiHandlers.Metrics, deps.Endpoints)
		setupLogsRoutes(api, apiHandlers.Logs, deps.Endpoints)

		// Replay endpoints (only if S3 is available)
		if deps.S3Client != nil && apiHandlers.Replay != nil {
			setupReplayRoutes(api, apiHandlers.Replay, deps.Endpoints)
		}
	}

	// Add monitoring endpoints if enabled
	if deps.Config.Operations.EnableMetrics {
		if err := setupMonitoringRoutes(r, deps.Config); err != nil {
			return fmt.Errorf("failed to set up monitoring routes: %w", err)
		}
	}

	// Add comprehensive API documentation
	if err := setupDocumentationRoutes(r, deps.Config, deps.Endpoints); err != nil {
		return fmt.Errorf("failed to set up documentation routes: %w", err)
	}

	logger.Debug("API Routes configured successfully")
	return nil
}

func verifyServerSetup(r *gin.Engine, deps *Dependencies) error {
	// Use dedicated route verification service
	verificationService := services.NewRouteVerificationService(deps.Endpoints)

	result, err := verificationService.VerifyServerRoutes(r)
	if err != nil {
		return fmt.Errorf("server route verification failed: %w", err)
	}

	// Log route statistics
	routeStats := verificationService.GetRouteStats(r)
	logger.Info("Server setup verified",
		zap.Any("route_stats", routeStats),
		zap.Bool("verification_ok", result.VerificationOK))

	return nil
}

// healthHandler returns a comprehensive health check handler using the dedicated health service
func healthHandler(cfg *config.ConsolidatedConfig) gin.HandlerFunc {
	healthService := services.NewHealthService(cfg)

	return func(c *gin.Context) {
		response, statusCode := healthService.PerformHealthCheck(c.Request.Context())
		c.JSON(statusCode, response)
	}
}


// setupTraceRoutes configures trace-related routes with rate limiting and payload limits
func setupTraceRoutes(api *gin.RouterGroup, handler *handlers.TraceHandler, endpoints *api.EndpointCollection) {
	// Add path-specific rate limiting for trace ingestion (high volume)
	api.POST(endpoints.TracesRelative(),
		middleware.TracePayloadLimit(),          // 25 MB limit for traces
		PathSpecificRateLimitMiddleware(50, 100), // 50 RPS, 100 burst
		SizeBasedRateLimitMiddleware(),           // Different limits for large payloads
		handler.CreateTrace)

	// Lower rate limit for trace queries
	api.GET(endpoints.TraceByIDRelative(),
		PathSpecificRateLimitMiddleware(20, 40), // 20 RPS, 40 burst
		handler.GetTrace)
}

// setupMetricsRoutes configures metrics-related routes with rate limiting and payload limits
func setupMetricsRoutes(api *gin.RouterGroup, handler *handlers.MetricsHandler, endpoints *api.EndpointCollection) {
	// Metrics have moderate volume, stricter limits than traces
	api.POST(endpoints.MetricsRelative(),
		middleware.MetricsPayloadLimit(),        // 10 MB limit for metrics
		PathSpecificRateLimitMiddleware(30, 60), // 30 RPS, 60 burst
		SizeBasedRateLimitMiddleware(),          // Size-based limits
		handler.CreateMetrics)

	api.GET(endpoints.MetricsRelative(),
		PathSpecificRateLimitMiddleware(15, 30), // 15 RPS, 30 burst
		handler.GetMetrics)
}

// setupLogsRoutes configures logs-related routes with rate limiting and payload limits
func setupLogsRoutes(api *gin.RouterGroup, handler *handlers.LogsHandler, endpoints *api.EndpointCollection) {
	// Logs can be high volume, similar to traces but slightly more restrictive
	api.POST(endpoints.LogsRelative(),
		middleware.LogsPayloadLimit(),           // 15 MB limit for logs
		PathSpecificRateLimitMiddleware(40, 80), // 40 RPS, 80 burst
		SizeBasedRateLimitMiddleware(),          // Size-based limits
		handler.CreateLogs)

	api.GET(endpoints.LogsRelative(),
		PathSpecificRateLimitMiddleware(15, 30), // 15 RPS, 30 burst
		handler.GetLogs)
}

// setupReplayRoutes configures replay-related routes with payload limits
func setupReplayRoutes(api *gin.RouterGroup, handler *handlers.ReplayHandler, endpoints *api.EndpointCollection) {
	api.GET(endpoints.ReplaysListRelative(), handler.ListReplays)
	api.POST(endpoints.ReplaysDownloadRelative(),
		middleware.ReplayPayloadLimit(), // 1 KB limit for replay requests (just metadata)
		handler.DownloadReplay)
}

// setupMonitoringRoutes configures monitoring endpoints
func setupMonitoringRoutes(r *gin.Engine, cfg *config.ConsolidatedConfig) error {
	if cfg.Operations.MetricsPath != "" {
		// Validate the endpoint path
		if !strings.HasPrefix(cfg.Operations.MetricsPath, "/") {
			return fmt.Errorf("metrics endpoint must start with '/': %s", cfg.Operations.MetricsPath)
		}
		if strings.Contains(cfg.Operations.MetricsPath, " ") {
			return fmt.Errorf("metrics endpoint cannot contain spaces: %s", cfg.Operations.MetricsPath)
		}

		// Register metrics endpoint
		r.GET(cfg.Operations.MetricsPath, metricsHandler())
		logger.Info("Metrics endpoint configured",
			zap.String("path", cfg.Operations.MetricsPath))
	}

	if cfg.Network.HTTP.EnableDebug {
		// Validate debug endpoints are only enabled in development
		if cfg.Network.HTTP.Mode == gin.ReleaseMode {
			return errors.New("debug endpoints cannot be enabled in release mode")
		}

		// Register debug endpoints
		debugGroup := r.Group(string(constants.DebugPathPrefix))
		{
			debugGroup.GET("/pprof/*any", pprofHandler())
			debugGroup.GET("/vars", varsHandler())
		}
		logger.Info("Debug endpoints enabled",
			zap.String("mode", cfg.Network.HTTP.Mode))
	}

	return nil
}

// metricsHandler provides Prometheus metrics endpoint
func metricsHandler() gin.HandlerFunc {
	handler := promhttp.Handler()
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// pprofHandler provides pprof debugging endpoint
func pprofHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(string(constants.HeaderContentType), string(constants.ContentTypeTextPlain))
		c.String(int(constants.StatusOK), constants.PprofPlaceholderContent)
	}
}

// varsHandler provides runtime variables endpoint
func varsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use memory stats and runtime info from a temporary service call
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		version := runtime.Version()
		sanitized := constants.VersionSanitizeRegex.ReplaceAllString(version, "")
		if sanitized == "" {
			sanitized = constants.ErrorUnknownVersion
		}

		c.Header(string(constants.HeaderContentType), string(constants.ContentTypeApplicationJSON))
		c.JSON(int(constants.StatusOK), gin.H{
			"runtime": gin.H{
				"version":    sanitized,
				"goroutines": runtime.NumGoroutine(),
				"memory": gin.H{
					"alloc":      m.Alloc,
					"sys":        m.Sys,
					"num_gc":     m.NumGC,
					"goroutines": runtime.NumGoroutine(),
				},
				"status":    "ok",
				"timestamp": time.Now().Format(constants.StandardTimeFormat),
			},
		})
	}
}

// setupDocumentationRoutes configures comprehensive API documentation routes
func setupDocumentationRoutes(r *gin.Engine, cfg *config.ConsolidatedConfig, endpoints *api.EndpointCollection) error {
	// Create documentation server instance
	docServer, err := docs.NewServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create documentation server: %w", err)
	}

	// Use the built-in route setup method
	docServer.SetupRoutes(r)

	logger.Info("Documentation routes configured",
		zap.String("base_path", "/docs"))

	return nil
}

