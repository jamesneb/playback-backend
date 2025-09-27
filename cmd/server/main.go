package main

import (
	"log"

	_ "github.com/jamesneb/playback-backend/docs" // Import generated docs
	"github.com/jamesneb/playback-backend/internal/app"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
)

// @title Playback Backend API
// @version {{.Version}}
// @description Distributed systems event replay backend
// @host {{.Host}}
// @BasePath /api/v1
func main() {
	// Load configuration
	cfg, err := config.Load("")
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
