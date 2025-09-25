package rest

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// setupSwagger configures Swagger documentation if enabled
func setupSwagger(r *gin.Engine, cfg *config.Config, endpoints *api.EndpointCollection) error {
	if !cfg.Swagger.Enabled {
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
	if deps.KinesisClient == nil {
		return errors.New("kinesis client is required for API routes")
	}
	if deps.S3Client == nil {
		return errors.New("S3 client is required for API routes")
	}

	// Get singleton handlers
	singleton := getHandlersSingleton()
	apiHandlers, err := singleton.getOrCreateHandlers(deps)
	if err != nil {
		return fmt.Errorf("failed to get API handlers: %w", err)
	}

	// API routes using centralized endpoints
	api := r.Group(deps.Endpoints.BasePath())
	{
		// Health check
		api.GET("/health", healthHandler(deps.Config))

		// OpenTelemetry HTTP endpoints (legacy)
		setupTraceRoutes(api, apiHandlers.Trace, deps.Endpoints)
		setupMetricsRoutes(api, apiHandlers.Metrics, deps.Endpoints)
		setupLogsRoutes(api, apiHandlers.Logs, deps.Endpoints)

		// Replay endpoints
		setupReplayRoutes(api, apiHandlers.Replay, deps.Endpoints)
	}

	// Add monitoring endpoints if enabled
	if deps.Config.Monitoring.EnableMetrics {
		if err := setupMonitoringRoutes(r, deps.Config); err != nil {
			return fmt.Errorf("Failed to set up monitoring routes: %w", err)
		}
	}

	logger.Debug("API Routes configured successfully")
	return nil
}

func verifyServerSetup(r *gin.Engine, deps *Dependencies) error {
	// Verify engine has expected routes
	routes := r.Routes()
	if len(routes) == 0 {
		return errors.New("no routes registered, server setup may have failed")
	}

	// Verify critical routes using efficient lookup
	expectedRoutes := []string{"/health", deps.Endpoints.TracesRelative()}
	routeSet := make(map[string]bool, len(routes))

	// Build route lookup set for O(1) access
	for _, route := range routes {
		routeSet[route.Path] = true
	}

	// Check expected routes efficiently
	for _, expectedRoute := range expectedRoutes {
		found := false

		// Direct match first
		if routeSet[expectedRoute] {
			found = true
		} else {
			// Fallback to substring check (limited iterations)
			iterations := 0
			for routePath := range routeSet {
				if iterations >= MAX_ROUTE_SEARCH_ITERATIONS {
					logger.Warn("Route verification iteration limit reached",
						zap.Int("max_iterations", MAX_ROUTE_SEARCH_ITERATIONS))
					break
				}
				if strings.Contains(routePath, expectedRoute) {
					found = true
					break
				}
				iterations++
			}
		}

		if !found {
			logger.Warn("Expected route not found", zap.String("route", expectedRoute))
		}
	}

	logger.Info("Server setup verified", zap.Int("total_routes", len(routes)), zap.String("gin_mode", gin.Mode()))
	return nil
}

// healthHandler returns the health check handler
func healthHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use actual runtime version with sanitization
		runtimeVersion := getRuntimeVersion()
		appVersion := cfg.App.Version
		if appVersion == "" {
			appVersion = UNKNOWN_VERSION
		}

		c.JSON(int(StatusOK), gin.H{
			"status":         "ok",
			"mode":           cfg.Server.Mode,
			"app_version":    appVersion,
			"runtime_version": runtimeVersion,
			"protocols":      []string{PROTOCOL_HTTP_JSON, PROTOCOL_GRPC_OTLP},
			"timestamp":      time.Now().Format(STANDARD_TIME_FORMAT),
		})
	}
}

// getRuntimeVersion returns the sanitized Go runtime version
func getRuntimeVersion() string {
	version := runtime.Version()
	// Sanitize version string to remove any potential sensitive info
	sanitized := versionSanitizeRegex.ReplaceAllString(version, "")
	if sanitized == "" {
		return UNKNOWN_VERSION
	}
	return sanitized
}

// setupTraceRoutes configures trace-related routes
func setupTraceRoutes(api *gin.RouterGroup, handler *handlers.TraceHandler, endpoints *api.EndpointCollection) {
	api.POST(endpoints.TracesRelative(), handler.CreateTrace)
	api.GET(endpoints.TraceByIDRelative(), handler.GetTrace)
}

// setupMetricsRoutes configures metrics-related routes
func setupMetricsRoutes(api *gin.RouterGroup, handler *handlers.MetricsHandler, endpoints *api.EndpointCollection) {
	api.POST(endpoints.MetricsRelative(), handler.CreateMetrics)
	api.GET(endpoints.MetricsRelative(), handler.GetMetrics)
}

// setupLogsRoutes configures logs-related routes
func setupLogsRoutes(api *gin.RouterGroup, handler *handlers.LogsHandler, endpoints *api.EndpointCollection) {
	api.POST(endpoints.LogsRelative(), handler.CreateLogs)
	api.GET(endpoints.LogsRelative(), handler.GetLogs)
}

// setupReplayRoutes configures replay-related routes
func setupReplayRoutes(api *gin.RouterGroup, handler *handlers.ReplayHandler, endpoints *api.EndpointCollection) {
	api.GET(endpoints.ReplaysListRelative(), handler.ListReplays)
	api.POST(endpoints.ReplaysDownloadRelative(), handler.DownloadReplay)
}

// setupMonitoringRoutes configures monitoring endpoints
func setupMonitoringRoutes(r *gin.Engine, cfg *config.Config) error {
	if cfg.Monitoring.MetricsEndpoint != "" {
		// Validate the endpoint path
		if !strings.HasPrefix(cfg.Monitoring.MetricsEndpoint, "/") {
			return fmt.Errorf("metrics endpoint must start with '/': %s", cfg.Monitoring.MetricsEndpoint)
		}
		if strings.Contains(cfg.Monitoring.MetricsEndpoint, " ") {
			return fmt.Errorf("metrics endpoint cannot contain spaces: %s", cfg.Monitoring.MetricsEndpoint)
		}

		// Register metrics endpoint
		r.GET(cfg.Monitoring.MetricsEndpoint, metricsHandler())
		logger.Info("Metrics endpoint configured",
			zap.String("path", cfg.Monitoring.MetricsEndpoint))
	}

	if cfg.Development.EnableDebugEndpoints {
		// Validate debug endpoints are only enabled in development
		if cfg.Server.Mode == gin.ReleaseMode {
			return errors.New("debug endpoints cannot be enabled in release mode")
		}

		// Register debug endpoints
		debugGroup := r.Group(string(DEBUG_PATH_PREFIX))
		{
			debugGroup.GET("/pprof/*any", pprofHandler())
			debugGroup.GET("/vars", varsHandler())
		}
		logger.Info("Debug endpoints enabled",
			zap.String("mode", cfg.Server.Mode))
	}

	return nil
}

// metricsHandler provides Prometheus metrics endpoint
func metricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(string(HEADER_CONTENT_TYPE), string(CONTENT_TYPE_PROMETHEUS_METRICS))
		c.String(int(StatusOK), METRICS_PLACEHOLDER_CONTENT)
	}
}

// pprofHandler provides pprof debugging endpoint
func pprofHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(string(HEADER_CONTENT_TYPE), string(CONTENT_TYPE_TEXT_PLAIN))
		c.String(int(StatusOK), PPROF_PLACEHOLDER_CONTENT)
	}
}

// varsHandler provides runtime variables endpoint
func varsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header(string(HEADER_CONTENT_TYPE), string(CONTENT_TYPE_APPLICATION_JSON))
		c.JSON(int(StatusOK), gin.H{
			"runtime": gin.H{
				"version":    getRuntimeVersion(),
				"goroutines": runtime.NumGoroutine(),
				"memory":     getMemoryStats(),
				"status":     "ok",
				"timestamp":  time.Now().Format(STANDARD_TIME_FORMAT),
			},
		})
	}
}

// getMemoryStats returns basic memory statistics
func getMemoryStats() gin.H {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return gin.H{
		"alloc":       m.Alloc,
		"sys":         m.Sys,
		"num_gc":      m.NumGC,
		"goroutines":  runtime.NumGoroutine(),
	}
}