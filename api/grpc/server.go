package grpcapi

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// rateLimitConfig implements RateLimitConfig interface
type rateLimitConfig struct {
	requestsPerSecond int
	burstCapacity     int
}

// GetRequestsPerSecond returns the requests per second limit
func (r *rateLimitConfig) GetRequestsPerSecond() int { return r.requestsPerSecond }

// GetBurstCapacity returns the burst capacity limit
func (r *rateLimitConfig) GetBurstCapacity() int { return r.burstCapacity }

// Server manages the gRPC server with clean separation of concerns
type Server struct {
	config   *ServerConfig
	services *ServiceCollection
	server   *grpc.Server
}

// NewServer creates a new gRPC server with rate limiting and all services configured
func NewServer(config *ServerConfig, services *ServiceCollection) (*Server, error) {
	if config == nil {
		return nil, fmt.Errorf("%s", ErrConfigNil)
	}
	if services == nil {
		return nil, fmt.Errorf("%s", ErrServicesNil)
	}
	if config.Address == "" {
		return nil, fmt.Errorf("%s", ErrServerAddressEmpty)
	}

	// Create default rate limiting configuration
	defaultConfig := &rateLimitConfig{
		requestsPerSecond: 100,
		burstCapacity:     200,
	}

	// Chain multiple rate limiting interceptors
	rateLimitingInterceptor := ChainGRPCInterceptors(
		GRPCRateLimitingInterceptor(defaultConfig), // Global rate limiting
		MethodSpecificRateLimitingInterceptor(),    // Method-specific limits
		SizeBasedGRPCRateLimitingInterceptor(),     // Size-based limits
	)

	// Create gRPC server with configured options including rate limiting
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(config.MaxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(config.MaxSendMsgSize)),
		grpc.UnaryInterceptor(rateLimitingInterceptor),
	)

	// Register all services
	if err := services.RegisterServices(grpcServer); err != nil {
		return nil, fmt.Errorf(ErrFailedRegisterServices, err)
	}

	// Start service lifecycle
	if err := services.Start(context.Background()); err != nil {
		return nil, fmt.Errorf(ErrFailedStartServices, err)
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
		return fmt.Errorf("%s", ErrContextNil)
	}
	lis, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}

	logger.Info(MsgStartingGRPCServer,
		zap.String("address", s.config.Address),
		zap.String("protocols", ProtocolOTLPgRPC))

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
	logger.Info(MsgStoppingGRPCServer)

	// Create timeout context for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(DefaultShutdownTimeout))
	defer cancel()

	var errs []error

	// Stop services first
	if s.services != nil {
		if err := s.services.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf(ErrServiceShutdownFailed, err))
			logger.Error(MsgFailedStopServices, zap.Error(err))
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
			logger.Info(MsgGRPCServerStoppedGraceful)
		case <-ctx.Done():
			logger.Warn(MsgGRPCServerShutdownTimeout)
			s.server.Stop() // Force stop
			errs = append(errs, fmt.Errorf("%s", ErrShutdownTimeout))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(ErrShutdownErrors, errs)
	}
	return nil
}
