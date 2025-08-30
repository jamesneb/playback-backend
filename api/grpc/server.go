package grpcapi

import (
	"net"

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
func NewServer(config *ServerConfig, services *ServiceCollection) *Server {
	// Create gRPC server with configured options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.MaxSendMsgSize),
	)
	
	// Register all services
	services.RegisterServices(grpcServer)
	
	return &Server{
		config:   config,
		services: services,
		server:   grpcServer,
	}
}

// Start starts the gRPC server
func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return err
	}

	logger.Info("Starting gRPC server", 
		zap.String("address", s.config.Address),
		zap.String("protocols", "OTLP/gRPC"))

	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	logger.Info("Stopping gRPC server")
	s.server.GracefulStop()
}

// GetServer returns the underlying gRPC server for advanced usage
func (s *Server) GetServer() *grpc.Server {
	return s.server
}