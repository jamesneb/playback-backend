package main

import (
	"log"
	"os"

	_ "github.com/jamesneb/playback-backend/docs" // Import generated docs
	"github.com/jamesneb/playback-backend/internal/app"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// getConfigPath determines the configuration file path
// Priority: CONFIG_PATH env var > default environment-based path
// determineLoggerConfig creates logger configuration based on environment and app config
func determineLoggerConfig(cfg *config.ConsolidatedConfig) *logger.LoggerConfig {
	env := os.Getenv("ENV")

	// Use development config for local/dev environments
	if env == "" || env == "local" || env == "dev" || env == "development" {
		return logger.DevelopmentConfig()
	}

	// Use production config for staging/prod environments
	return logger.ProductionConfig()
}

func getConfigPath() string {
	// Check if CONFIG_PATH environment variable is set
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		return configPath
	}

	// Use default path based on environment
	env := os.Getenv("ENV")
	if env == "" {
		env = "local" // Default to local.yaml which should exist
	}

	return "config/environments/" + env + ".yaml"
}

// @title Playback Backend API
// @version {{.Version}}
// @description Distributed systems event replay backend
// @host {{.Host}}
// @BasePath /api/v1
func main() {
	// Load configuration - use default path if none specified
	configPath := getConfigPath()
	cfg, err := config.LoadConsolidatedConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger with proper dependency injection
	loggerConfig := determineLoggerConfig(cfg)
	zapLogger, err := logger.NewLogger(loggerConfig)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	appLogger := logger.NewLoggerFromZap(zapLogger)

	// Initialize global logger for backwards compatibility during migration
	if err := logger.InitGlobalLogger(loggerConfig); err != nil {
		log.Fatalf("Failed to initialize global logger: %v", err)
	}

	// Log application startup
	zapLogger.Info("Starting Playback Backend API Server",
		zap.String("config_path", configPath),
		zap.String("env", os.Getenv("ENV")),
	)

	// Initialize all services (ClickHouse, Kinesis, S3)
	services, err := app.InitializeAPIServices(cfg)
	if err != nil {
		zapLogger.Fatal("Failed to initialize services", zap.Error(err))
	}
	defer func() {
		if err := services.Close(); err != nil {
			zapLogger.Error("Failed to close services", zap.Error(err))
		}
	}()

	// Create and start server (HTTP + gRPC)
	server := app.NewServer(cfg, services)
	if err := server.Start(); err != nil {
		zapLogger.Fatal("Server failed", zap.Error(err))
	}

	// Sync logger on exit
	defer func() {
		if err := appLogger.Sync(); err != nil {
			// Don't use logger here as we're shutting down
			_, _ = os.Stderr.WriteString("Failed to sync logger: " + err.Error() + "\n")
		}
	}()
}
