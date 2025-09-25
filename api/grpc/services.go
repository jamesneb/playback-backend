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
	"errors"
	"fmt"
	"context"
)
// Bytes represents a size in bytes
type Bytes int

func (b Bytes) String() string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

const (
	DEFAULT_MAX_MESSAGE_SIZE Bytes = 4 * 1024 * 1024
)

// ServiceDependencies holds all dependencies needed by gRPC services
type ServiceDependencies struct {
	Config                *config.Config
	KinesisClient         *streaming.KinesisClient
	ClickHouseClient      *storage.ClickHouseClient
	ResilienceComponents  *grpcservices.ResilienceComponents
	StreamHandler					streaming.Handler // Interface
	ClickhouseHandler			*realtime.ClickHouseHandler	// Interface
}

// ServiceCollection holds all initialized gRPC services
type ServiceCollection struct {
	TraceService   *grpcservices.TraceService
	MetricsService *grpcservices.MetricsService
	LogsService    *grpcservices.LogsService
}

// Cleanup gracefully shuts down all services
func (sc *ServiceCollection) Cleanup() error {
	if sc == nil {
		return errors.New("service collection is nil")
	}

	var errs []error

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// Lifecycle manages service lifecycle
type Lifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Start initializes all services
func (sc *ServiceCollection) Start(ctx context.Context) error {
	if sc == nil {
		return errors.New("service collection is nil")
	}
	return nil
}

// Stop shuts down all services
func (sc *ServiceCollection) Stop(ctx context.Context) error {
	return sc.Cleanup()
}

// NewServiceCollection creates all gRPC services with proper dependencies
func NewServiceCollection(deps *ServiceDependencies) (*ServiceCollection, error) {
	if deps == nil {
		return nil, errors.New("service dependencies cannot be nil")
	}
	if deps.KinesisClient == nil {
		return nil, errors.New("Kinesis client cannot be nil")
	}
	if deps.ClickHouseClient == nil {
		return nil, errors.New("Clickhouse client cannot be nil")
	}
	if deps.Config == nil {
		return nil, errors.New("config cannot be nil")
	}
	if deps.ResilienceComponents == nil {
		return nil, errors.New("resilience components cannot be nil")
	}
	if deps.Config.Server.Host == "" {
		return nil, errors.New("server host cannot be empty")
	}
	if deps.Config.Server.Port <= 0 {
		return nil, errors.New("server port must be positive")
	}

	var streamHandler streaming.Handler
	var clickhouseHandler realtime.ClickHouseHandler


	if deps.StreamHandler != nil {
		streamHandler = deps.StreamHandler
	} else {
		streamHandler = streaming.NewKinesisHandler(deps.KinesisClient)
		if streamHandler == nil {
			return nil, fmt.Errorf("failed to create kinesis handler for client %v", deps.KinesisClient )
		}

	}
	if deps.ClickhouseHandler != nil {
		clickhouseHandler = *deps.ClickhouseHandler
	} else {
		temp := realtime.NewClickHouseHandler(deps.ClickHouseClient)
		clickhouseHandler = *temp
	}
	return &ServiceCollection{
		TraceService:   grpcservices.NewTraceService(streamHandler.(*streaming.KinesisHandler), &clickhouseHandler, deps.ResilienceComponents),
		MetricsService: grpcservices.NewMetricsService(streamHandler.(*streaming.KinesisHandler), &clickhouseHandler),
		LogsService:    grpcservices.NewLogsService(streamHandler.(*streaming.KinesisHandler), &clickhouseHandler),
	}, nil
}

// RegisterServices registers all OTLP services with the gRPC server
func (sc *ServiceCollection) RegisterServices(grpcServer *grpc.Server) error {
	if grpcServer == nil {
		return errors.New("grpc server cannot be nil")
	}
	if sc == nil {
		return errors.New("Service collection cannot be nil")
	}
	tracecollectorpb.RegisterTraceServiceServer(grpcServer, sc.TraceService)
	metricscollectorpb.RegisterMetricsServiceServer(grpcServer, sc.MetricsService)
	logscollectorpb.RegisterLogsServiceServer(grpcServer, sc.LogsService)

	// Enable gRPC reflection for debugging/tooling
	reflection.Register(grpcServer)
	return nil
}

// ServerConfig holds gRPC server configuration
type ServerConfig struct {
	Address        string
	MaxRecvMsgSize Bytes
	MaxSendMsgSize Bytes
}

// NewServerConfig creates default gRPC server configuration
func NewServerConfig(address string) *ServerConfig {
	if address == "" {
		panic("gRPC server address cannot be empty")
	}
	return &ServerConfig{
		Address:        address,
		MaxRecvMsgSize: DEFAULT_MAX_MESSAGE_SIZE,
		MaxSendMsgSize: DEFAULT_MAX_MESSAGE_SIZE,
	}
}

// CreateGRPCServer creates a gRPC server with services registered
func CreateGRPCServer(serverConfig *ServerConfig, services *ServiceCollection) (*grpc.Server, error) {
	if serverConfig == nil {
		return nil, errors.New("server config cannot be nil")
	}
	if services == nil {
		return nil, errors.New("services cannot be nil")
	}
	// Create gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(serverConfig.MaxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(serverConfig.MaxSendMsgSize)),
	)

	// Register all services
	if err := services.RegisterServices(grpcServer); err != nil {
		return nil, fmt.Errorf("failed to register services: %w", err)
	}

	return grpcServer, nil
}
