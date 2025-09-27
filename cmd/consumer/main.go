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

	// Initialize consumer services (just Kinesis)
	services, err := app.InitializeConsumerServices(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}
	defer func() {
		if err := services.Close(); err != nil {
			log.Printf("Failed to close services: %v", err)
		}
	}()

	// Create and initialize consumer application
	consumerApp := app.NewConsumerApp(cfg, services)
	if err := consumerApp.Initialize(); err != nil {
		log.Fatalf("Failed to initialize consumer: %v", err)
	}
	defer func() {
		if err := consumerApp.Close(); err != nil {
			log.Printf("Failed to close consumer app: %v", err)
		}
	}()

	// Start consumer application
	if err := consumerApp.Start(); err != nil {
		log.Fatalf("Consumer failed: %v", err)
	}

	// Sync logger on exit
	defer logger.Sync()
}
