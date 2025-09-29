package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	grpcapi "github.com/jamesneb/playback-backend/api/grpc"
	"github.com/jamesneb/playback-backend/api/rest"
	"github.com/jamesneb/playback-backend/internal/metrics"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
)

// Server manages both HTTP and gRPC servers
type Server struct {
	cfg      *config.ConsolidatedConfig
	services *Services
	httpSrv  *http.Server
	grpcSrv  *grpcapi.Server
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewServer creates a new server instance with all dependencies
func NewServer(cfg *config.ConsolidatedConfig, services *Services) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		cfg:      cfg,
		services: services,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start starts both HTTP and gRPC servers
func (s *Server) Start() error {
	var wg sync.WaitGroup

	// Start HTTP server
	if err := s.startHTTPServer(&wg); err != nil {
		return fmt.Errorf(ErrHTTPServerStart, err)
	}

	// Start gRPC server
	if err := s.startGRPCServer(&wg); err != nil {
		return fmt.Errorf(ErrGRPCServerStart, err)
	}

	// Log successful startup
	logger.Info(MsgBackendStartedSuccess,
		zap.String("http_address", s.httpAddress()),
		zap.String("grpc_address", s.grpcAddress()),
		zap.String("version", s.cfg.App.Version))

	// Wait for shutdown signal
	s.waitForShutdown()

	// Graceful shutdown
	logger.Info(fmt.Sprintf(MsgShutdownSignalReceived, "servers"))
	s.shutdown()

	// Wait for servers to stop
	wg.Wait()
	logger.Info(MsgAllServersStoppedSuccess)

	return nil
}

// startHTTPServer starts the HTTP server in a goroutine
func (s *Server) startHTTPServer(wg *sync.WaitGroup) error {
	// Extract API configuration from consolidated config - optimized for performance
	apiPrefix := s.cfg.Network.HTTP.APIPrefix
	if apiPrefix == "" {
		apiPrefix = "/api/v1" // Default fallback with leading slash for optimal routing
	}

	// Extract version efficiently - single allocation
	apiVersion := "v1" // Default
	if idx := strings.LastIndex(apiPrefix, "/v"); idx != -1 && idx+2 < len(apiPrefix) {
		apiVersion = apiPrefix[idx+1:] // Zero-copy substring
	}

	restDeps := &rest.Dependencies{
		Config:               s.cfg,
		KinesisClient:        s.services.KinesisClient,
		ClickHouseClient:     s.services.ClickHouseClient,
		S3Client:             s.services.S3Client,
		Endpoints:            api.NewEndpointCollectionWithConfig("", apiVersion, apiPrefix), // Use configured API version and prefix
		ResilienceComponents: s.services.ResilienceComponents,
		MetricsRegistry:      metrics.NewRegistry(),
	}

	ginEngine, err := rest.NewServer(restDeps)

	if err != nil {
		return fmt.Errorf(ErrRESTServerCreate, err)
	}
	// Create HTTP server
	s.httpSrv = &http.Server{
		Addr:           s.httpAddress(),
		Handler:        ginEngine,
		ReadTimeout:    s.cfg.Network.HTTP.ReadTimeout,
		WriteTimeout:   s.cfg.Network.HTTP.WriteTimeout,
		IdleTimeout:    s.cfg.Network.HTTP.IdleTimeout,
		MaxHeaderBytes: s.cfg.Network.HTTP.MaxHeaderSizeKB << 10, // Bit shift for optimal KB->bytes conversion
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(MsgStartingHTTPServer,
			zap.String("address", s.httpAddress()),
			zap.String("protocols", ProtocolHTTPJSON))

		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error(MsgHTTPServerFailed, zap.Error(err))
		}
	}()

	return nil
}

// startGRPCServer starts the gRPC server in a goroutine
func (s *Server) startGRPCServer(wg *sync.WaitGroup) error {
	// Use unified ResilienceComponents with circuit breaker
	grpcResilienceComponents := s.services.ResilienceComponents
	if grpcResilienceComponents.CircuitBreaker == nil {
		grpcResilienceComponents.CircuitBreaker = s.services.CircuitBreaker
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
		return fmt.Errorf(ErrGRPCServiceCollection, err)
	}

	// Create gRPC server configuration
	grpcConfig, err := grpcapi.NewServerConfig(s.grpcAddress())
	if err != nil {
		return fmt.Errorf("failed to create gRPC server config: %w", err)
	}

	// Create gRPC server
	grpcSrv, err := grpcapi.NewServer(grpcConfig, grpcServices)
	if err != nil {
		return fmt.Errorf(ErrGRPCServerCreate, err)
	}
	s.grpcSrv = grpcSrv

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.grpcSrv.Start(s.ctx); err != nil && err != context.Canceled {
			logger.Error(MsgGRPCServerFailed, zap.Error(err))
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
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.Network.HTTP.ShutdownTimeout)
		defer cancel()

		if err := s.httpSrv.Shutdown(ctx); err != nil {
			logger.Error(MsgHTTPServerShutdownError, zap.Error(err))
		}
	}

	// Shutdown gRPC server
	if s.grpcSrv != nil {
		if err := s.grpcSrv.Stop(); err != nil {
			logger.Error("Failed to stop gRPC server", zap.Error(err))
		}
	}
}

// httpAddress returns the HTTP server address - optimized for performance
func (s *Server) httpAddress() string {
	return fmt.Sprintf("%s:%d", s.cfg.Network.HTTP.Host, s.cfg.Network.HTTP.Port)
}

// grpcAddress returns the gRPC server address - optimized for performance
func (s *Server) grpcAddress() string {
	return fmt.Sprintf("%s:%d", s.cfg.Network.HTTP.Host, s.cfg.Network.GRPC.Port)
}
