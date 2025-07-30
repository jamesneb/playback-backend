package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesneb/playback-backend/internal/consumer"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize ClickHouse client
	clickhouseClient, err := storage.NewClickHouseClient(&storage.ClickHouseConfig{
		Host:               cfg.Database.ClickHouse.Host,
		Database:           cfg.Database.ClickHouse.Database,
		Username:           cfg.Database.ClickHouse.Username,
		Password:           cfg.Database.ClickHouse.Password,
		MaxConnections:     cfg.Database.ClickHouse.MaxConnections,
		MaxIdleConnections: cfg.Database.ClickHouse.MaxIdleConnections,
		ConnectionTimeout:  cfg.Database.ClickHouse.ConnectionTimeout,
	})
	if err != nil {
		log.Fatalf("Failed to initialize ClickHouse client: %v", err)
	}
	defer clickhouseClient.Close()

	// Initialize Kinesis consumer
	kinesisConsumer, err := consumer.NewKinesisConsumer(&consumer.ConsumerConfig{
		Region:          cfg.Streaming.Kinesis.Region,
		EndpointURL:     cfg.Streaming.Kinesis.EndpointURL,
		AccessKeyID:     cfg.Streaming.Kinesis.AccessKeyID,
		SecretAccessKey: cfg.Streaming.Kinesis.SecretAccessKey,
		Streams:         cfg.Streaming.Kinesis.Streams,
		PollInterval:    time.Second,
	}, clickhouseClient)
	if err != nil {
		log.Fatalf("Failed to initialize Kinesis consumer: %v", err)
	}

	// Create context that will be cancelled on shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the consumer
	logger.Info("Starting Kinesis consumer service",
		zap.String("version", cfg.App.Version),
		zap.Int("streams", len(cfg.Streaming.Kinesis.Streams)))

	if err := kinesisConsumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start Kinesis consumer: %v", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Kinesis consumer is running. Press Ctrl+C to stop.")

	// Block until we receive a signal
	<-sigChan

	logger.Info("Shutdown signal received, stopping consumer...")

	// Cancel context to stop all goroutines
	cancel()

	// Stop the consumer gracefully
	kinesisConsumer.Stop()

	logger.Info("Kinesis consumer stopped successfully")
}