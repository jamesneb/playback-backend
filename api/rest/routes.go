package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
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

// checkClickHouseHealthAsync performs an asynchronous health check against ClickHouse database
func checkClickHouseHealthAsync(ctx context.Context, cfg *config.Config) error {
	if cfg.Database.ClickHouse.Host == "" {
		return errors.New("ClickHouse host not configured")
	}

	// Construct connection string (use default port 8123 if not specified in host)
	host := cfg.Database.ClickHouse.Host
	if host == "" {
		host = "localhost:8123"
	}
	// Check if host already includes port
	if !strings.Contains(host, ":") {
		host += ":8123"
	}

	dsn := fmt.Sprintf("http://%s", host)
	if cfg.Database.ClickHouse.Database != "" {
		dsn += "/" + cfg.Database.ClickHouse.Database
	}

	// Create HTTP client with timeout that respects context
	client := &http.Client{Timeout: HEALTH_CHECK_TIMEOUT}

	// Simple ping query to check connectivity
	pingQuery := "SELECT 1 FORMAT JSONEachRow"
	req, err := http.NewRequestWithContext(ctx, "GET", dsn+"?query="+url.QueryEscape(pingQuery), nil)
	if err != nil {
		return fmt.Errorf("failed to create ClickHouse health check request: %w", err)
	}

	// Add basic auth if configured
	if cfg.Database.ClickHouse.Username != "" {
		req.SetBasicAuth(cfg.Database.ClickHouse.Username, cfg.Database.ClickHouse.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ClickHouse connection failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("Failed to close response body", zap.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ClickHouse health check failed with status: %d", resp.StatusCode)
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
			appVersion = UNKNOWN_VERSION
		}

		// Create context with request-level timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), HEALTH_CHECK_TIMEOUT)
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
							dependencyName: HEALTH_DEPENDENCY_DATABASE,
							status: gin.H{
								HEALTH_FIELD_STATUS: HEALTH_STATUS_UNHEALTHY,
								HEALTH_FIELD_ERROR:  "health check panic",
							},
						}
					}
				}()

				if err := checkClickHouseHealthAsync(ctx, cfg); err != nil {
					resultsChan <- healthResult{
						dependencyName: HEALTH_DEPENDENCY_DATABASE,
						status: gin.H{
							HEALTH_FIELD_STATUS: HEALTH_STATUS_UNHEALTHY,
							HEALTH_FIELD_ERROR:  err.Error(),
						},
					}
				} else {
					resultsChan <- healthResult{
						dependencyName: HEALTH_DEPENDENCY_DATABASE,
						status: gin.H{
							HEALTH_FIELD_STATUS: HEALTH_STATUS_HEALTHY,
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
							dependencyName: HEALTH_DEPENDENCY_KINESIS,
							status: gin.H{
								HEALTH_FIELD_STATUS: HEALTH_STATUS_UNHEALTHY,
								HEALTH_FIELD_ERROR:  "health check panic",
							},
						}
					}
				}()

				if err := checkKinesisHealthAsync(ctx, cfg); err != nil {
					resultsChan <- healthResult{
						dependencyName: HEALTH_DEPENDENCY_KINESIS,
						status: gin.H{
							HEALTH_FIELD_STATUS: HEALTH_STATUS_UNHEALTHY,
							HEALTH_FIELD_ERROR:  err.Error(),
						},
					}
				} else {
					resultsChan <- healthResult{
						dependencyName: HEALTH_DEPENDENCY_KINESIS,
						status: gin.H{
							HEALTH_FIELD_STATUS: HEALTH_STATUS_HEALTHY,
						},
					}
				}
			}()
		}

		// Collect results with timeout protection
		overallStatus := HEALTH_STATUS_OK
		dependencyStatus := gin.H{}

		// Set default status for unconfigured dependencies
		if cfg.Database.ClickHouse.Host == "" {
			dependencyStatus[HEALTH_DEPENDENCY_DATABASE] = gin.H{
				HEALTH_FIELD_STATUS: HEALTH_STATUS_NOT_CONFIGURED,
			}
		}
		if len(cfg.Streaming.Kinesis.Streams) == 0 {
			dependencyStatus[HEALTH_DEPENDENCY_KINESIS] = gin.H{
				HEALTH_FIELD_STATUS: HEALTH_STATUS_NOT_CONFIGURED,
			}
		}

		// Collect results from async health checks
	healthCheckLoop:
		for i := 0; i < activeChecks; i++ {
			select {
			case result := <-resultsChan:
				dependencyStatus[result.dependencyName] = result.status
				if status, exists := result.status[HEALTH_FIELD_STATUS].(string); exists && status == HEALTH_STATUS_UNHEALTHY {
					overallStatus = HEALTH_STATUS_UNHEALTHY
				}
			case <-ctx.Done():
				// Handle timeout for remaining checks
				logger.Warn("Health check timeout occurred")
				overallStatus = HEALTH_STATUS_UNHEALTHY
				// Mark remaining dependencies as unhealthy due to timeout
				if _, exists := dependencyStatus[HEALTH_DEPENDENCY_DATABASE]; !exists && cfg.Database.ClickHouse.Host != "" {
					dependencyStatus[HEALTH_DEPENDENCY_DATABASE] = gin.H{
						HEALTH_FIELD_STATUS: HEALTH_STATUS_UNHEALTHY,
						HEALTH_FIELD_ERROR:  "health check timeout",
					}
				}
				if _, exists := dependencyStatus[HEALTH_DEPENDENCY_KINESIS]; !exists && len(cfg.Streaming.Kinesis.Streams) > 0 {
					dependencyStatus[HEALTH_DEPENDENCY_KINESIS] = gin.H{
						HEALTH_FIELD_STATUS: HEALTH_STATUS_UNHEALTHY,
						HEALTH_FIELD_ERROR:  "health check timeout",
					}
				}
				// Break out of collection loop on timeout
				break healthCheckLoop
			}
		}

		// Health check response structure with actual dependency status
		healthStatus := gin.H{
			HEALTH_FIELD_STATUS:  overallStatus,
			"mode":               cfg.Server.Mode,
			"app_version":        appVersion,
			"runtime_version":    runtimeVersion,
			"protocols":          []string{PROTOCOL_HTTP_JSON, PROTOCOL_GRPC_OTLP},
			"timestamp":          time.Now().Format(STANDARD_TIME_FORMAT),
			"dependencies":       dependencyStatus,
			"system": gin.H{
				"goroutines": runtime.NumGoroutine(),
				"memory_mb":  getMemoryUsageMB(),
			},
		}

		// Return appropriate HTTP status based on actual health
		statusCode := StatusOK
		if overallStatus == HEALTH_STATUS_UNHEALTHY {
			statusCode = StatusServiceUnavailable
		}
		c.JSON(int(statusCode), healthStatus)
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

// getMemoryUsageMB returns memory usage in megabytes
func getMemoryUsageMB() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / float64(BYTES_PER_MEGABYTE)
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