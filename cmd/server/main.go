package main

import (
	"log"
	"os"

	_ "github.com/jamesneb/playback-backend/docs" // Import generated docs
	"github.com/jamesneb/playback-backend/internal/app"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
)

// getConfigPath determines the configuration file path
// Priority: CONFIG_PATH env var > default environment-based path
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
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize all services (ClickHouse, Kinesis, S3)
	services, err := app.InitializeAPIServices(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	defer func() {
		if err := services.Close(); err != nil {
			log.Printf("Failed to close services: %v", err)
		}
	}()

	// Create and start server (HTTP + gRPC)
	server := app.NewServer(cfg, services)
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}

	// Sync logger on exit
	defer logger.Sync()
}
