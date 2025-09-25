package main

import (
	"log"

	"github.com/jamesneb/playback-backend/internal/app"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize all services (ClickHouse, Kinesis, S3)
	services, err := app.InitializeServices(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	defer services.Close()

	// Create and initialize consumer application
	consumerApp := app.NewConsumerApp(cfg, services)
	if err := consumerApp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize consumer: %v", err)
	}
	defer consumerApp.Close()

	// Start consumer application
	if err := consumerApp.Start(); err != nil {
		log.Fatalf("Consumer failed: %v", err)
	}

	// Sync logger on exit
	defer logger.Sync()
}