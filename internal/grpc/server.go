package grpc

import (
	"context"
	"net"
	"strings"

	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/logger"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	grpcServer *grpc.Server
	addr       string
}

func NewServer(addr string, streamHandler *streaming.KinesisHandler, clickhouseHandler streaming.Handler) *Server {
	// Create gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(4*1024*1024), // 4MB max message size for large traces
		grpc.MaxSendMsgSize(4*1024*1024),
	)

	// Register OTLP services
	traceService := NewTraceService(streamHandler, clickhouseHandler)
	metricsService := NewMetricsService(streamHandler, clickhouseHandler)
	logsService := NewLogsService(streamHandler, clickhouseHandler)

	tracecollectorpb.RegisterTraceServiceServer(grpcServer, traceService)
	metricscollectorpb.RegisterMetricsServiceServer(grpcServer, metricsService)
	logscollectorpb.RegisterLogsServiceServer(grpcServer, logsService)

	// Enable gRPC reflection for debugging/tooling
	reflection.Register(grpcServer)

	return &Server{
		grpcServer: grpcServer,
		addr:       addr,
	}
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	logger.Info("Starting gRPC server", 
		zap.String("address", s.addr),
		zap.String("protocols", "OTLP/gRPC"))

	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	logger.Info("Stopping gRPC server")
	s.grpcServer.GracefulStop()
}

// ExtractClientIP extracts the client IP address from gRPC context
func ExtractClientIP(ctx context.Context) string {
	// Extract peer information from gRPC context
	if p, ok := peer.FromContext(ctx); ok {
		if tcpAddr, ok := p.Addr.(*net.TCPAddr); ok && tcpAddr.IP != nil {
			ip := tcpAddr.IP.String()
			// Handle IPv6 loopback and convert to IPv4
			if ip == "::1" {
				return "127.0.0.1"
			}
			// Extract IPv4 from IPv6-mapped addresses
			if strings.HasPrefix(ip, "::ffff:") {
				return strings.TrimPrefix(ip, "::ffff:")
			}
			return ip
		}
		// Fallback: parse address string
		addr := p.Addr.String()
		if host, _, err := net.SplitHostPort(addr); err == nil {
			if net.ParseIP(host) != nil {
				return host
			}
		}
	}
	// Default fallback for local connections
	return "127.0.0.1"
}