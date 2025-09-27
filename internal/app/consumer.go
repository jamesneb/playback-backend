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
	services         *ConsumerServices
	kinesisConsumer  *consumer.KinesisConsumer
	ctx              context.Context
	cancel           context.CancelFunc
}


// NewConsumerApp creates a new consumer application instance
func NewConsumerApp(cfg *config.Config, services *ConsumerServices) *ConsumerApp {
	ctx, cancel := context.WithCancel(context.Background())

	return &ConsumerApp{
		cfg:      cfg,
		services: services,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Initialize sets up the Kinesis consumer with configuration
func (app *ConsumerApp) Initialize() error {
	kinesisConsumer, err := consumer.NewKinesisConsumer(&consumer.ConsumerConfig{
		Region:          app.cfg.Streaming.Kinesis.Region,
		EndpointURL:     app.cfg.Streaming.Kinesis.EndpointURL,
		AccessKeyID:     app.cfg.Streaming.Kinesis.AccessKeyID,
		SecretAccessKey: app.cfg.Streaming.Kinesis.SecretAccessKey,
		Streams:         app.cfg.Streaming.Kinesis.Streams,
		PollInterval:    time.Duration(app.cfg.Streaming.Kinesis.PollInterval) * time.Second,
	}, app.services.ClickHouseClient)

	if err != nil {
		return fmt.Errorf(ErrKinesisConsumerInit, err)
	}

	app.kinesisConsumer = kinesisConsumer
	return nil
}

// Start starts the consumer application
func (app *ConsumerApp) Start() error {
	// Start the consumer
	if app.kinesisConsumer == nil {
		return fmt.Errorf("%s", ErrConsumerNotInitialized)
	}
	logger.Info(MsgStartingConsumerService,
		zap.String("version", app.cfg.App.Version),
		zap.Int("streams", len(app.cfg.Streaming.Kinesis.Streams)))

	if err := app.kinesisConsumer.Start(app.ctx); err != nil {
		return fmt.Errorf(ErrKinesisConsumerStart, err)
	}

	// Wait for shutdown signal
	app.waitForShutdown()

	// Graceful shutdown
	logger.Info(fmt.Sprintf(MsgShutdownSignalReceived, "consumer"))
	app.shutdown()

	logger.Info(MsgConsumerStoppedSuccess)
	return nil
}

// waitForShutdown waits for shutdown signal
func (app *ConsumerApp) waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	logger.Info(MsgConsumerRunning)

	// Block until we receive a signal
	<-sigChan
}

// shutdown gracefully shuts down the consumer
func (app *ConsumerApp) shutdown() {

	// Stop the consumer gracefully
	if app.kinesisConsumer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), KinesisShutdownTimeout)
		defer cancel()
		done := make(chan struct{})
		go func() {
			app.kinesisConsumer.Stop()
			close(done)
		}()

		select {
			case <-done:
				logger.Info(MsgConsumerStoppedGraceful)
			case <-ctx.Done():
				logger.Warn(MsgConsumerShutdownTimeout)
		}
	}
	if err := app.Close(); err != nil {
		logger.Error("Failed to close consumer app", zap.Error(err))
	}
}

// Close cleans up resources
func (app *ConsumerApp) Close() error {
	if app.cancel != nil {
		app.cancel()
		app.cancel = nil
	}
	return nil
}
