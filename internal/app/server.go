package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	grpcapi "github.com/jamesneb/playback-backend/api/grpc"
	"github.com/jamesneb/playback-backend/api/rest"
	grpcservices "github.com/jamesneb/playback-backend/internal/grpc"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Server manages both HTTP and gRPC servers
type Server struct {
	cfg      *config.Config
	services *Services
	httpSrv  *http.Server
	grpcSrv  *grpcapi.Server
	ctx 		 context.Context
	cancel 	 context.CancelFunc
}

// NewServer creates a new server instance with all dependencies
func NewServer(cfg *config.Config, services *Services) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:      cfg,
		services: services,
		ctx: ctx,
		cancel: cancel,
	}
}

// Start starts both HTTP and gRPC servers
func (s *Server) Start() error {
	var wg sync.WaitGroup

	// Start HTTP server
	if err := s.startHTTPServer(&wg); err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start gRPC server
	if err := s.startGRPCServer(&wg); err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Log successful startup
	logger.Info("Playback backend started successfully",
		zap.String("http_address", s.httpAddress()),
		zap.String("grpc_address", s.grpcAddress()),
		zap.String("version", s.cfg.App.Version))

	// Wait for shutdown signal
	s.waitForShutdown()

	// Graceful shutdown
	logger.Info("Shutdown signal received, stopping servers...")
	s.shutdown()

	// Wait for servers to stop
	wg.Wait()
	logger.Info("All servers stopped successfully")

	return nil
}

// startHTTPServer starts the HTTP server in a goroutine
func (s *Server) startHTTPServer(wg *sync.WaitGroup) error {
	// Create REST API server
	restDeps := &rest.Dependencies{
		Config:               s.cfg,
		KinesisClient:        s.services.KinesisClient,
		ClickHouseClient:     s.services.ClickHouseClient,
		S3Client:             s.services.S3Client,
		Endpoints:            api.NewEndpointCollection(""), // Base URL will be set by server
		ResilienceComponents: s.services.ResilienceComponents,
	}

	ginEngine, err := rest.NewServer(restDeps)

	if err != nil {
		return fmt.Errorf("Failed to create REST server: %w", err)
	}
	// Create HTTP server
	s.httpSrv = &http.Server{
		Addr:    s.httpAddress(),
		Handler: ginEngine,
		ReadTimeout:    s.cfg.Server.ReadTimeoutDuration,
		WriteTimeout:   s.cfg.Server.WriteTimeoutDuration,
		IdleTimeout:    s.cfg.Server.IdleTimeoutDuration,
		MaxHeaderBytes: s.cfg.Server.MaxHeaderBytes,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Starting HTTP server",
			zap.String("address", s.httpAddress()),
			zap.String("protocols", "HTTP/JSON"))

		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", zap.Error(err))
		}
	}()

	return nil
}

// startGRPCServer starts the gRPC server in a goroutine
func (s *Server) startGRPCServer(wg *sync.WaitGroup) error {
	// Convert handlers.ResilienceComponents to grpc.ResilienceComponents
	grpcResilienceComponents := &grpcservices.ResilienceComponents{
		KinesisBuffer:   s.services.ResilienceComponents.KinesisBuffer,
		RateLimiter:     s.services.ResilienceComponents.RateLimiter,
		CircuitBreaker:  s.services.CircuitBreaker,
		DeadLetterQueue: s.services.ResilienceComponents.DeadLetterQueue,
	}

	// Create gRPC service dependencies
	grpcDeps := &grpcapi.ServiceDependencies{
		Config:               s.cfg,
		KinesisClient:        s.services.KinesisClient,
		ClickHouseClient:     s.services.ClickHouseClient,
		ResilienceComponents: grpcResilienceComponents,
	}

	// Create gRPC services
	grpcServices, err := grpcapi.NewServiceCollection(grpcDeps)
	if err != nil {
		return fmt.Errorf("failed to create gRPC service collection: %w", err)
	}

	// Create gRPC server configuration
	grpcConfig := grpcapi.NewServerConfig(s.grpcAddress())

	// Create gRPC server
	grpcSrv, err := grpcapi.NewServer(grpcConfig, grpcServices)
	if err != nil {
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}
	s.grpcSrv = grpcSrv

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.grpcSrv.Start(s.ctx); err != nil && err != context.Canceled {
			logger.Error("gRPC server failed", zap.Error(err))
		}
	}()

	return nil
}

// waitForShutdown waits for shutdown signal
func (s *Server) waitForShutdown() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	<-sigCh
}

// shutdown gracefully shuts down both servers
func (s *Server) shutdown() {
	s.cancel()
	// Shutdown HTTP server with timeout
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeoutDuration)
		defer cancel()

		if err := s.httpSrv.Shutdown(ctx); err != nil {
			logger.Error("HTTP server shutdown error", zap.Error(err))
		}
	}

	// Shutdown gRPC server
	if s.grpcSrv != nil {
		s.grpcSrv.Stop()
	}
}

// httpAddress returns the HTTP server address
func (s *Server) httpAddress() string {
	return fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
}

// grpcAddress returns the gRPC server address
func (s *Server) grpcAddress() string {
	return fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.GRPCPort)
}

