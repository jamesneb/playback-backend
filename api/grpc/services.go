package grpcapi

import (
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	grpcservices "github.com/jamesneb/playback-backend/internal/grpc"
	"github.com/jamesneb/playback-backend/internal/handlers/realtime"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ServiceDependencies holds all dependencies needed by gRPC services
type ServiceDependencies struct {
	Config           *config.Config
	KinesisClient    *streaming.KinesisClient
	ClickHouseClient *storage.ClickHouseClient
}

// ServiceCollection holds all initialized gRPC services
type ServiceCollection struct {
	TraceService   *grpcservices.TraceService
	MetricsService *grpcservices.MetricsService  
	LogsService    *grpcservices.LogsService
}

// NewServiceCollection creates all gRPC services with proper dependencies
func NewServiceCollection(deps *ServiceDependencies) *ServiceCollection {
	// Create stream handlers
	streamHandler := streaming.NewKinesisHandler(deps.KinesisClient)
	clickhouseHandler := realtime.NewClickHouseHandler(deps.ClickHouseClient)
	
	return &ServiceCollection{
		TraceService:   grpcservices.NewTraceService(streamHandler, clickhouseHandler),
		MetricsService: grpcservices.NewMetricsService(streamHandler, clickhouseHandler),
		LogsService:    grpcservices.NewLogsService(streamHandler, clickhouseHandler),
	}
}

// RegisterServices registers all OTLP services with the gRPC server
func (sc *ServiceCollection) RegisterServices(grpcServer *grpc.Server) {
	tracecollectorpb.RegisterTraceServiceServer(grpcServer, sc.TraceService)
	metricscollectorpb.RegisterMetricsServiceServer(grpcServer, sc.MetricsService)
	logscollectorpb.RegisterLogsServiceServer(grpcServer, sc.LogsService)
	
	// Enable gRPC reflection for debugging/tooling
	reflection.Register(grpcServer)
}

// ServerConfig holds gRPC server configuration
type ServerConfig struct {
	Address        string
	MaxRecvMsgSize int
	MaxSendMsgSize int
}

// NewServerConfig creates default gRPC server configuration
func NewServerConfig(address string) *ServerConfig {
	return &ServerConfig{
		Address:        address,
		MaxRecvMsgSize: 4 * 1024 * 1024, // 4MB max message size for large traces
		MaxSendMsgSize: 4 * 1024 * 1024,
	}
}

// CreateGRPCServer creates a gRPC server with services registered
func CreateGRPCServer(serverConfig *ServerConfig, services *ServiceCollection) *grpc.Server {
	// Create gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(serverConfig.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(serverConfig.MaxSendMsgSize),
	)
	
	// Register all services
	services.RegisterServices(grpcServer)
	
	return grpcServer
}