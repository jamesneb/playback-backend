package rest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	// Create API handlers directly (no caching complexity)
	apiHandlers, err := NewAPIHandlers(deps)
	if err != nil {
		return fmt.Errorf("failed to create API handlers: %w", err)
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

		// Replay endpoints (only if S3 is available)
		if deps.S3Client != nil && apiHandlers.Replay != nil {
			setupReplayRoutes(api, apiHandlers.Replay, deps.Endpoints)
		}
	}

	// Add monitoring endpoints if enabled
	if deps.Config.Monitoring.EnableMetrics {
		if err := setupMonitoringRoutes(r, deps.Config); err != nil {
			return fmt.Errorf("failed to set up monitoring routes: %w", err)
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
				if iterations >= constants.MaxRouteSearchIterations {
					logger.Warn("Route verification iteration limit reached",
						zap.Int("max_iterations", constants.MaxRouteSearchIterations))
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

// checkClickHouseHealthAsync performs an asynchronous health check against ClickHouse database using native TCP protocol
func checkClickHouseHealthAsync(ctx context.Context, cfg *config.Config) error {
	if cfg.Database.ClickHouse.Host == "" {
		return errors.New("ClickHouse host not configured")
	}

	// Use native ClickHouse driver with TCP protocol (same as production client)
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Database.ClickHouse.Host},
		Auth: clickhouse.Auth{
			Database: cfg.Database.ClickHouse.Database,
			Username: cfg.Database.ClickHouse.Username,
			Password: cfg.Database.ClickHouse.Password,
		},
		DialTimeout: constants.HealthCheckTimeout,
	})
	if err != nil {
		return fmt.Errorf("ClickHouse connection failed: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			logger.Error("Failed to close ClickHouse connection", zap.Error(closeErr))
		}
	}()

	// Ping to verify connectivity using context
	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ClickHouse ping failed: %w", err)
	}

	return nil
}

// checkKinesisHealthAsync performs an asynchronous health check against AWS Kinesis
func checkKinesisHealthAsync(ctx context.Context, cfg *config.Config) error {
	if len(cfg.Streaming.Kinesis.Streams) == 0 {
		return errors.New("no Kinesis streams configured")
	}

	// Create AWS session and Kinesis client with context
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.Streaming.Kinesis.Region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	kinesisClient := kinesis.NewFromConfig(awsCfg)

	// Check if at least one stream exists and is active
	for streamName := range cfg.Streaming.Kinesis.Streams {
		input := &kinesis.DescribeStreamInput{
			StreamName: &streamName,
		}

		output, err := kinesisClient.DescribeStream(ctx, input)
		if err != nil {
			return fmt.Errorf("kinesis stream '%s' health check failed: %w", streamName, err)
		}

		if output.StreamDescription == nil || output.StreamDescription.StreamStatus != kinesistypes.StreamStatusActive {
			return fmt.Errorf("kinesis stream '%s' is not active", streamName)
		}

		// Only check the first stream for basic connectivity
		break
	}

	return nil
}

// healthResult represents the result of an asynchronous health check
type healthResult struct {
	dependencyName string
	status         gin.H
}

// healthHandler returns a comprehensive health check handler that validates system dependencies.
// This handler performs actual connectivity checks asynchronously to avoid blocking the main thread.
func healthHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use actual runtime version with sanitization
		runtimeVersion := getRuntimeVersion()
		appVersion := cfg.App.Version
		if appVersion == "" {
			appVersion = constants.ErrorUnknownVersion
		}

		// Create context with request-level timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), constants.HealthCheckTimeout)
		defer cancel()

		// Channel to collect health check results
		resultsChan := make(chan healthResult, 2)
		var activeChecks int

		// Launch ClickHouse health check asynchronously
		if cfg.Database.ClickHouse.Host != "" {
			activeChecks++
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("ClickHouse health check panic recovered", zap.Any("panic", r))
						resultsChan <- healthResult{
							dependencyName: constants.HealthDependencyDatabase,
							status: gin.H{
								constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
								constants.HealthFieldError:  "health check panic",
							},
						}
					}
				}()

				if err := checkClickHouseHealthAsync(ctx, cfg); err != nil {
					resultsChan <- healthResult{
						dependencyName: constants.HealthDependencyDatabase,
						status: gin.H{
							constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
							constants.HealthFieldError:  err.Error(),
						},
					}
				} else {
					resultsChan <- healthResult{
						dependencyName: constants.HealthDependencyDatabase,
						status: gin.H{
							constants.HealthFieldStatus: constants.HealthStatusHealthy,
						},
					}
				}
			}()
		}

		// Launch Kinesis health check asynchronously
		if len(cfg.Streaming.Kinesis.Streams) > 0 {
			activeChecks++
			go func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Error("Kinesis health check panic recovered", zap.Any("panic", r))
						resultsChan <- healthResult{
							dependencyName: constants.HealthDependencyKinesis,
							status: gin.H{
								constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
								constants.HealthFieldError:  "health check panic",
							},
						}
					}
				}()

				if err := checkKinesisHealthAsync(ctx, cfg); err != nil {
					resultsChan <- healthResult{
						dependencyName: constants.HealthDependencyKinesis,
						status: gin.H{
							constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
							constants.HealthFieldError:  err.Error(),
						},
					}
				} else {
					resultsChan <- healthResult{
						dependencyName: constants.HealthDependencyKinesis,
						status: gin.H{
							constants.HealthFieldStatus: constants.HealthStatusHealthy,
						},
					}
				}
			}()
		}

		// Collect results with timeout protection
		overallStatus := constants.HealthStatusOK
		dependencyStatus := gin.H{}

		// Set default status for unconfigured dependencies
		if cfg.Database.ClickHouse.Host == "" {
			dependencyStatus[constants.HealthDependencyDatabase] = gin.H{
				constants.HealthFieldStatus: constants.HealthStatusNotConfigured,
			}
		}
		if len(cfg.Streaming.Kinesis.Streams) == 0 {
			dependencyStatus[constants.HealthDependencyKinesis] = gin.H{
				constants.HealthFieldStatus: constants.HealthStatusNotConfigured,
			}
		}

		// Collect results from async health checks
	healthCheckLoop:
		for i := 0; i < activeChecks; i++ {
			select {
			case result := <-resultsChan:
				dependencyStatus[result.dependencyName] = result.status
				if status, exists := result.status[constants.HealthFieldStatus].(string); exists && status == constants.HealthStatusUnhealthy {
					overallStatus = constants.HealthStatusUnhealthy
				}
			case <-ctx.Done():
				// Handle timeout for remaining checks
				logger.Warn("Health check timeout occurred")
				overallStatus = constants.HealthStatusUnhealthy
				// Mark remaining dependencies as unhealthy due to timeout
				if _, exists := dependencyStatus[constants.HealthDependencyDatabase]; !exists && cfg.Database.ClickHouse.Host != "" {
					dependencyStatus[constants.HealthDependencyDatabase] = gin.H{
						constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
						constants.HealthFieldError:  "health check timeout",
					}
				}
				if _, exists := dependencyStatus[constants.HealthDependencyKinesis]; !exists && len(cfg.Streaming.Kinesis.Streams) > 0 {
					dependencyStatus[constants.HealthDependencyKinesis] = gin.H{
						constants.HealthFieldStatus: constants.HealthStatusUnhealthy,
						constants.HealthFieldError:  "health check timeout",
					}
				}
				// Break out of collection loop on timeout
				break healthCheckLoop
			}
		}

		// Health check response structure with actual dependency status
		healthStatus := gin.H{
			constants.HealthFieldStatus: overallStatus,
			"mode":                      cfg.Server.Mode,
			"app_version":               appVersion,
			"runtime_version":           runtimeVersion,
			"protocols":                 []string{constants.ProtocolHTTPJSON, constants.ProtocolGRPCOTLP},
			"timestamp":                 time.Now().Format(constants.StandardTimeFormat),
			"dependencies":              dependencyStatus,
			"system": gin.H{
				"goroutines": runtime.NumGoroutine(),
				"memory_mb":  getMemoryUsageMB(),
			},
		}

		// Return appropriate HTTP status based on actual health
		statusCode := constants.StatusOK
		if overallStatus == constants.HealthStatusUnhealthy {
			statusCode = constants.StatusServiceUnavailable
		}
		c.JSON(int(statusCode), healthStatus)
	}
}

// getRuntimeVersion returns the sanitized Go runtime version
func getRuntimeVersion() string {
	version := runtime.Version()
	// Sanitize version string to remove any potential sensitive info
	sanitized := constants.VersionSanitizeRegex.ReplaceAllString(version, "")
	if sanitized == "" {
		return constants.ErrorUnknownVersion
	}
	return sanitized
}

// setupTraceRoutes configures trace-related routes with rate limiting
func setupTraceRoutes(api *gin.RouterGroup, handler *handlers.TraceHandler, endpoints *api.EndpointCollection) {
	// Add path-specific rate limiting for trace ingestion (high volume)
	api.POST(endpoints.TracesRelative(),
		PathSpecificRateLimitMiddleware(50, 100), // 50 RPS, 100 burst
		SizeBasedRateLimitMiddleware(),           // Different limits for large payloads
		handler.CreateTrace)

	// Lower rate limit for trace queries
	api.GET(endpoints.TraceByIDRelative(),
		PathSpecificRateLimitMiddleware(20, 40), // 20 RPS, 40 burst
		handler.GetTrace)
}

// setupMetricsRoutes configures metrics-related routes with rate limiting
func setupMetricsRoutes(api *gin.RouterGroup, handler *handlers.MetricsHandler, endpoints *api.EndpointCollection) {
	// Metrics have moderate volume, stricter limits than traces
	api.POST(endpoints.MetricsRelative(),
		PathSpecificRateLimitMiddleware(30, 60), // 30 RPS, 60 burst
		SizeBasedRateLimitMiddleware(),          // Size-based limits
		handler.CreateMetrics)

	api.GET(endpoints.MetricsRelative(),
		PathSpecificRateLimitMiddleware(15, 30), // 15 RPS, 30 burst
		handler.GetMetrics)
}

// setupLogsRoutes configures logs-related routes with rate limiting
func setupLogsRoutes(api *gin.RouterGroup, handler *handlers.LogsHandler, endpoints *api.EndpointCollection) {
	// Logs can be high volume, similar to traces but slightly more restrictive
	api.POST(endpoints.LogsRelative(),
		PathSpecificRateLimitMiddleware(40, 80), // 40 RPS, 80 burst
		SizeBasedRateLimitMiddleware(),          // Size-based limits
		handler.CreateLogs)

	api.GET(endpoints.LogsRelative(),
		PathSpecificRateLimitMiddleware(15, 30), // 15 RPS, 30 burst
		handler.GetLogs)
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
		debugGroup := r.Group(string(constants.DebugPathPrefix))
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
		c.Header(string(constants.HeaderContentType), string(constants.ContentTypeApplicationJSON))
		c.JSON(int(constants.StatusOK), gin.H{
			"runtime": gin.H{
				"version":    getRuntimeVersion(),
				"goroutines": runtime.NumGoroutine(),
				"memory":     getMemoryStats(),
				"status":     "ok",
				"timestamp":  time.Now().Format(constants.StandardTimeFormat),
			},
		})
	}
}

// getMemoryUsageMB returns memory usage in megabytes
func getMemoryUsageMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / float64(constants.BytesPerMegabyte)
}

// getMemoryStats returns basic memory statistics
func getMemoryStats() gin.H {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return gin.H{
		"alloc":      m.Alloc,
		"sys":        m.Sys,
		"num_gc":     m.NumGC,
		"goroutines": runtime.NumGoroutine(),
	}
}
