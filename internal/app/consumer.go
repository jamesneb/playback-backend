package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamesneb/playback-backend/internal/consumer"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// ConsumerApp manages the Kinesis consumer application lifecycle
type ConsumerApp struct {
	cfg              *config.Config
	services         *Services
	kinesisConsumer  *consumer.KinesisConsumer
	ctx              context.Context
	cancel           context.CancelFunc
}

// NewConsumerApp creates a new consumer application instance
func NewConsumerApp(cfg *config.Config, services *Services) *ConsumerApp {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ConsumerApp{
		cfg:      cfg,
		services: services,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Initialize sets up the Kinesis consumer with proper configuration
func (app *ConsumerApp) Initialize() error {
	// Initialize Kinesis consumer with configuration
	kinesisConsumer, err := consumer.NewKinesisConsumer(&consumer.ConsumerConfig{
		Region:          app.cfg.Streaming.Kinesis.Region,
		EndpointURL:     app.cfg.Streaming.Kinesis.EndpointURL,
		AccessKeyID:     app.cfg.Streaming.Kinesis.AccessKeyID,
		SecretAccessKey: app.cfg.Streaming.Kinesis.SecretAccessKey,
		Streams:         app.cfg.Streaming.Kinesis.Streams,
		PollInterval:    time.Second, // Could be made configurable
	}, app.services.ClickHouseClient)
	
	if err != nil {
		return fmt.Errorf("failed to initialize Kinesis consumer: %w", err)
	}
	
	app.kinesisConsumer = kinesisConsumer
	return nil
}

// Start starts the consumer application
func (app *ConsumerApp) Start() error {
	// Start the consumer
	logger.Info("Starting Kinesis consumer service",
		zap.String("version", app.cfg.App.Version),
		zap.Int("streams", len(app.cfg.Streaming.Kinesis.Streams)))

	if err := app.kinesisConsumer.Start(app.ctx); err != nil {
		return fmt.Errorf("failed to start Kinesis consumer: %w", err)
	}

	// Wait for shutdown signal
	app.waitForShutdown()
	
	// Graceful shutdown
	logger.Info("Shutdown signal received, stopping consumer...")
	app.shutdown()
	
	logger.Info("Kinesis consumer stopped successfully")
	return nil
}

// waitForShutdown waits for shutdown signal
func (app *ConsumerApp) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Kinesis consumer is running. Press Ctrl+C to stop.")

	// Block until we receive a signal
	<-sigChan
}

// shutdown gracefully shuts down the consumer
func (app *ConsumerApp) shutdown() {
	// Cancel context to stop all goroutines
	app.cancel()

	// Stop the consumer gracefully
	if app.kinesisConsumer != nil {
		app.kinesisConsumer.Stop()
	}
}

// Close cleans up resources
func (app *ConsumerApp) Close() error {
	app.cancel()
	return nil
}