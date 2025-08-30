package rest

import (
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Dependencies holds all the dependencies needed by the REST API
type Dependencies struct {
	Config            *config.Config
	KinesisClient     *streaming.KinesisClient
	ClickHouseClient  *storage.ClickHouseClient
	S3Client          *s3.Client
	Endpoints         *api.EndpointCollection
}

// NewServer creates a new Gin HTTP server with all routes configured
func NewServer(deps *Dependencies) *gin.Engine {
	// Set Gin mode from config
	gin.SetMode(deps.Config.Server.Mode)

	// Create HTTP server
	r := gin.New()
	
	// Add standard middleware
	setupMiddleware(r, deps.Config)
	
	// Set trusted proxies
	setupTrustedProxies(r, deps.Config)
	
	// Setup Swagger if enabled
	setupSwagger(r, deps.Config, deps.Endpoints)
	
	// Setup API routes
	setupAPIRoutes(r, deps)
	
	return r
}

// setupMiddleware configures standard middleware
func setupMiddleware(r *gin.Engine, cfg *config.Config) {
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	
	// CORS middleware
	if cfg.API.EnableCORS {
		r.Use(corsMiddleware(cfg.API.CORS))
	}
}

// corsMiddleware creates CORS middleware from config
func corsMiddleware(corsConfig config.CORSConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set CORS headers from config
		if len(corsConfig.AllowedOrigins) > 0 {
			origin := corsConfig.AllowedOrigins[0] // Use first origin, or implement origin matching
			if origin == "*" {
				c.Header("Access-Control-Allow-Origin", "*")
			} else {
				c.Header("Access-Control-Allow-Origin", origin)
			}
		}
		
		if len(corsConfig.AllowedMethods) > 0 {
			methods := ""
			for i, method := range corsConfig.AllowedMethods {
				if i > 0 {
					methods += ", "
				}
				methods += method
			}
			c.Header("Access-Control-Allow-Methods", methods)
		}
		
		if len(corsConfig.AllowedHeaders) > 0 {
			headers := ""
			for i, header := range corsConfig.AllowedHeaders {
				if i > 0 {
					headers += ", "
				}
				headers += header
			}
			c.Header("Access-Control-Allow-Headers", headers)
		}
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	}
}

// setupTrustedProxies configures trusted proxies
func setupTrustedProxies(r *gin.Engine, cfg *config.Config) {
	if len(cfg.Server.TrustedProxies) > 0 {
		r.SetTrustedProxies(cfg.Server.TrustedProxies)
	} else {
		r.SetTrustedProxies(nil)
	}
}

// setupSwagger configures Swagger documentation if enabled
func setupSwagger(r *gin.Engine, cfg *config.Config, endpoints *api.EndpointCollection) {
	if cfg.Swagger.Enabled {
		r.GET(endpoints.SwaggerUI(), ginSwagger.WrapHandler(swaggerFiles.Handler))
	}
}

// setupAPIRoutes configures all API routes
func setupAPIRoutes(r *gin.Engine, deps *Dependencies) {
	// Note: kinesisHandler and clickhouseHandler are created elsewhere for gRPC
	// HTTP handlers use clients directly
	
	// Initialize HTTP handlers
	traceHandler := handlers.NewTraceHandler(deps.KinesisClient)
	metricsHandler := handlers.NewMetricsHandler(deps.KinesisClient)
	logsHandler := handlers.NewLogsHandler(deps.KinesisClient)
	replayHandler := handlers.NewReplayHandler(deps.S3Client, "replays")
	
	// API routes using centralized endpoints
	api := r.Group(deps.Endpoints.BasePath())
	{
		// Health check
		api.GET("/health", healthHandler(deps.Config)) // Note: health is relative to base path
		
		// OpenTelemetry HTTP endpoints (legacy)
		setupTraceRoutes(api, traceHandler, deps.Endpoints)
		setupMetricsRoutes(api, metricsHandler, deps.Endpoints)
		setupLogsRoutes(api, logsHandler, deps.Endpoints)
		
		// Replay endpoints
		setupReplayRoutes(api, replayHandler, deps.Endpoints)
	}
	
	// Add monitoring endpoints if enabled
	if deps.Config.Monitoring.EnableMetrics {
		setupMonitoringRoutes(r, deps.Config)
	}
}

// healthHandler returns the health check handler
func healthHandler(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"mode":      cfg.Server.Mode,
			"version":   cfg.App.Version,
			"protocols": []string{"HTTP/JSON", "gRPC/OTLP"},
		})
	}
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
func setupMonitoringRoutes(r *gin.Engine, cfg *config.Config) {
	if cfg.Monitoring.MetricsEndpoint != "" {
		// Prometheus metrics endpoint would go here
		// r.GET(cfg.Monitoring.MetricsEndpoint, prometheusHandler())
	}
	
	if cfg.Development.EnableDebugEndpoints {
		// pprof endpoints would go here
		// r.GET("/debug/pprof/*any", pprofHandler())
	}
}