package grpcapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Server manages the gRPC server with clean separation of concerns
type Server struct {
	config   *ServerConfig
	services *ServiceCollection
	server   *grpc.Server
}

// NewServer creates a new gRPC server with all services configured
func NewServer(config *ServerConfig, services *ServiceCollection) (*Server, error) {
	if config == nil {
		return nil, errors.New("gRPC server config cannot be nil")
	}
	if services == nil {
		return nil, errors.New("gRPC service collection cannot be nil")
	}
	if config.Address == "" {
		return nil, errors.New("gRPC server address cannot be empty")
	}

	// Create gRPC server with configured options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(config.MaxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxSendMsgSize)),
	)

	// Register all services
	if err := services.RegisterServices(grpcServer); err != nil {
		return nil, fmt.Errorf("failed to register services: %w", err)
	}

	// Start service lifecycle
	if err := services.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start services: %w", err)
	}

	return &Server{
		config:   config,
		services: services,
		server:   grpcServer,
	}, nil
}

// Start starts the gRPC server
func (s *Server) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context cannot be nil")
	}
	lis, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}

	logger.Info("Starting gRPC server",
		zap.String("address", s.config.Address),
		zap.String("protocols", "OTLP/gRPC"))

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		s.server.GracefulStop()
		return ctx.Err()
	case err := <-errCh:
		return err
	}

}

// Stop gracefully stops the gRPC server with proper cleanup
func (s *Server) Stop() error {
	logger.Info("Stopping gRPC server")

	// Create timeout context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var errs []error

	// Stop services first
	if s.services != nil {
		if err := s.services.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("service shutdown failed: %w", err))
			logger.Error("Failed to stop services", zap.Error(err))
		}
	}

	// Stop the gRPC server
	if s.server != nil {
		// Graceful stop with timeout
		done := make(chan struct{})
		go func() {
			s.server.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("gRPC server stopped gracefully")
		case <-ctx.Done():
			logger.Warn("gRPC server shutdown timeout, forcing stop")
			s.server.Stop() // Force stop
			errs = append(errs, errors.New("gRPC server shutdown timeout"))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
