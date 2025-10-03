package rest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/api/rest/constants"
	"github.com/jamesneb/playback-backend/pkg/config"
	pkgerrors "github.com/jamesneb/playback-backend/pkg/errors"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// All constants and type aliases are now imported from constants.go

// Handler management structs and functions moved to handlers.go

// NewServer creates a new Gin HTTP server with all routes configured
func NewServer(deps *Dependencies) (*gin.Engine, error) {
	if err := validateDependencies(deps); err != nil {
		return nil, fmt.Errorf("invalid dependencies: %w", err)
	}

	if err := applyConfig(deps.Config); err != nil {
		return nil, fmt.Errorf("failed to configure Gin %w", err)
	}

	// Create HTTP server
	r := gin.New()
	if r == nil {
		return nil, errors.New(constants.ErrorGinServerCreation)
	}

	// Add default middleware
	if err := setupMiddleware(r, deps.Config); err != nil {
		return nil, fmt.Errorf("middleware setup failed: %w", err)
	}

	// Set trusted proxies
	if err := setupTrustedProxies(r, deps.Config); err != nil {
		return nil, fmt.Errorf("trusted proxy setup failed: %w", err)
	}

	// Setup Swagger if enabled
	if err := setupSwagger(r, deps.Config, deps.Endpoints); err != nil {
		return nil, fmt.Errorf("swagger setup failed: %w", err)
	}

	// Setup API routes
	if err := setupAPIRoutes(r, deps); err != nil {
		return nil, fmt.Errorf("API Route setup failed: %w", err)
	}

	if err := verifyServerSetup(r, deps); err != nil {
		return nil, fmt.Errorf("server verification failed: %w", err)

	}
	return r, nil
}

func validateDependencies(deps *Dependencies) error {
	if deps == nil {
		return errors.New(constants.ErrorDependenciesNil)
	}
	if deps.Config == nil {
		return errors.New(constants.ErrorConfigNil)
	}
	if deps.Endpoints == nil {
		return errors.New(constants.ErrorEndpointsNil)
	}

	// validate gin server mode
	mode := deps.Config.Network.HTTP.Mode
	if mode != gin.ReleaseMode && mode != gin.DebugMode && mode != gin.TestMode {
		return fmt.Errorf("invalid server mode: '%s', must be one of %s %s %s", mode, gin.ReleaseMode, gin.DebugMode, gin.TestMode)
	}
	return nil
}

func applyConfig(cfg *config.ConsolidatedConfig) error {
	originalMode := gin.Mode()

	gin.SetMode(cfg.Network.HTTP.Mode)

	if gin.Mode() != cfg.Network.HTTP.Mode {
		return fmt.Errorf("failed to set gin server mode to '%s', still '%s'", cfg.Network.HTTP.Mode, gin.Mode())

	}

	logger.Debug("Gin mode configured", zap.String("mode", cfg.Network.HTTP.Mode), zap.String("previous", originalMode))

	return nil

}

// setupMiddleware configures standard middleware
func setupMiddleware(r *gin.Engine, cfg *config.ConsolidatedConfig) error {
	if cfg == nil {
		return errors.New(constants.ErrorConfigMiddlewareNil)
	}
	if cfg.Network.HTTP.EnableCORS && len(cfg.Network.HTTP.CORS.AllowedOrigins) == 0 {
		logger.Warn(constants.CORSNoOriginsWarning)
	}

	// Request timeout middleware
	r.Use(timeoutMiddleware(constants.RequestTimeout))

	// Security headers middleware
	r.Use(securityHeadersMiddleware(cfg))

	// Rate limiting middleware (before other middleware to fail fast)
	r.Use(RateLimitingMiddleware(cfg))

	// Compression middleware
	r.Use(compressionMiddleware())

	// Logging and recovery middleware
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: customLogFormatter,
		SkipPaths: []string{string(constants.HealthEndpointPath), string(constants.MetricsEndpointPath)},
	}))
	r.Use(gin.RecoveryWithWriter(gin.DefaultWriter, customRecoveryHandler))

	// Standardized error handling middleware (must be after recovery)
	errorHandler := pkgerrors.NewHandler(logger.GetGlobalLogger().Logger)
	r.Use(errorHandler.Middleware())

	// Request size limit
	r.MaxMultipartMemory = constants.MaxMultipartMemory

	// CORS middleware
	if cfg.Network.HTTP.EnableCORS {
		corsHandler, err := createCorsMiddleware(cfg.Network.HTTP.CORS)
		if err != nil {
			return fmt.Errorf("unable to create CORS middleware: %w", err)
		}
		r.Use(corsHandler)
	}

	logger.Info("Middleware configured",
		zap.Bool("cors_enabled", cfg.Network.HTTP.EnableCORS),
		zap.Bool("compression_enabled", true),
		zap.Bool("security_headers_enabled", true),
		zap.Duration("request_timeout", constants.RequestTimeout))
	return nil
}

// timeoutMiddleware adds request timeout handling
func timeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		// Set request timeout
		ctx, cancel := c.Request.Context(), func() {}
		if timeout > 0 {
			ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
		}
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
}

// Security header middleware functions moved to security.go

// Compression middleware functions moved to compression.go

// Logging and recovery middleware functions moved to logging.go

// CORS middleware functions moved to cors.go

// setupTrustedProxies configures trusted proxies
func setupTrustedProxies(r *gin.Engine, cfg *config.ConsolidatedConfig) error {
	proxies := cfg.Network.HTTP.TrustedProxies

	for _, proxy := range proxies {
		if net.ParseIP(proxy) == nil && proxy != constants.LocalHost {
			// Try parsing as CIDR
			if _, _, err := net.ParseCIDR(proxy); err != nil {
				return fmt.Errorf("invalid trusted proxy address: %s", proxy)
			}
		}
	}

	if err := r.SetTrustedProxies(proxies); err != nil {
		return fmt.Errorf("failed to set trusted proxies %v: %w", proxies, err)
	}

	logger.Debug("Trusted proxies configured", zap.Strings("proxies", proxies))

	return nil
}
