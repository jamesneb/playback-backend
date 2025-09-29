package main

import (
	"log"
	"os"

	"github.com/jamesneb/playback-backend/internal/app"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// determineLoggerConfig creates logger configuration based on environment and app config
func determineLoggerConfig(cfg *config.Config) *logger.LoggerConfig {
	env := os.Getenv("ENV")

	// Use development config for local/dev environments
	if env == "" || env == "local" || env == "dev" || env == "development" {
		return logger.DevelopmentConfig()
	}

	// Use production config for staging/prod environments
	return logger.ProductionConfig()
}

func main() {
	// Load configuration
	cfg, err := config.Load("")
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
	zapLogger.Info("Starting Playback Backend Consumer",
		zap.String("env", os.Getenv("ENV")),
	)

	// Initialize consumer services (ClickHouse and Kinesis)
	// Use the ConsolidatedConfig embedded in the Config
	services, err := app.InitializeConsumerServices(cfg.ConsolidatedConfig)
	if err != nil {
		zapLogger.Fatal("Failed to initialize services", zap.Error(err))
	}
	defer func() {
		if err := services.Close(); err != nil {
			zapLogger.Error("Failed to close services", zap.Error(err))
		}
	}()

	// Create and initialize consumer application
	consumerApp := app.NewConsumerApp(cfg.ConsolidatedConfig, services)
	if err := consumerApp.Initialize(); err != nil {
		zapLogger.Fatal("Failed to initialize consumer", zap.Error(err))
	}
	defer func() {
		if err := consumerApp.Close(); err != nil {
			zapLogger.Error("Failed to close consumer app", zap.Error(err))
		}
	}()

	// Start consumer application
	if err := consumerApp.Start(); err != nil {
		zapLogger.Fatal("Consumer failed", zap.Error(err))
	}

	// Sync logger on exit
	defer func() {
		if err := appLogger.Sync(); err != nil {
			// Don't use logger here as we're shutting down
			_, _ = os.Stderr.WriteString("Failed to sync logger: " + err.Error() + "\n")
		}
	}()
}
