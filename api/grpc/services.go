package grpcapi

import (
	"context"
	"fmt"

	logscollectorpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricscollectorpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollectorpb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	grpcservices "github.com/jamesneb/playback-backend/internal/grpc"
	"github.com/jamesneb/playback-backend/internal/handlers/realtime"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// ServiceDependencies holds all dependencies needed by gRPC services
type ServiceDependencies struct {
	Config                *config.Config
	KinesisClient         *streaming.KinesisClient
	ClickHouseClient      *storage.ClickHouseClient
	ResilienceComponents  *interfaces.ResilienceComponents
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
		return fmt.Errorf("%s", ErrServiceCollectionNil)
	}

	var errs []error

	if len(errs) > 0 {
		return fmt.Errorf(ErrCleanupErrors, errs)
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
		return fmt.Errorf("%s", ErrServiceCollectionNil)
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
		return nil, fmt.Errorf("%s", ErrServiceDepsNil)
	}
	if deps.KinesisClient == nil {
		return nil, fmt.Errorf("%s", ErrKinesisClientNil)
	}
	if deps.ClickHouseClient == nil {
		return nil, fmt.Errorf("%s", ErrClickHouseClientNil)
	}
	if deps.Config == nil {
		return nil, fmt.Errorf("%s", ErrConfigFieldsNil)
	}
	if deps.ResilienceComponents == nil {
		return nil, fmt.Errorf("%s", ErrResilienceCompNil)
	}
	if deps.Config.Server.Host == "" {
		return nil, fmt.Errorf("%s", ErrServerHostEmpty)
	}
	if deps.Config.Server.Port <= 0 {
		return nil, fmt.Errorf("%s", ErrServerPortInvalid)
	}

	var streamHandler streaming.Handler
	var clickhouseHandler realtime.ClickHouseHandler


	if deps.StreamHandler != nil {
		streamHandler = deps.StreamHandler
	} else {
		streamHandler = streaming.NewKinesisHandler(deps.KinesisClient)
	}
	if deps.ClickhouseHandler != nil {
		clickhouseHandler = *deps.ClickhouseHandler
	} else {
		temp := realtime.NewClickHouseHandler(deps.ClickHouseClient)
		clickhouseHandler = *temp
	}
	return &ServiceCollection{
		TraceService:   grpcservices.NewTraceService(streamHandler, &clickhouseHandler, deps.ResilienceComponents),
		MetricsService: grpcservices.NewMetricsService(streamHandler, &clickhouseHandler),
		LogsService:    grpcservices.NewLogsService(streamHandler, &clickhouseHandler),
	}, nil
}

// RegisterServices registers all OTLP services with the gRPC server
func (sc *ServiceCollection) RegisterServices(grpcServer *grpc.Server) error {
	if grpcServer == nil {
		return fmt.Errorf("%s", ErrGRPCServerNil)
	}
	if sc == nil {
		return fmt.Errorf("%s", ErrServiceCollectionNil)
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
	MaxRecvMsgSize MessageSize
	MaxSendMsgSize MessageSize
}

// NewServerConfig creates default gRPC server configuration
func NewServerConfig(address string) (*ServerConfig, error) {
	if address == "" {
		return nil, fmt.Errorf("%s", ErrServerAddressEmpty)
	}
	return &ServerConfig{
		Address:        address,
		MaxRecvMsgSize: DefaultMaxMessageSize,
		MaxSendMsgSize: DefaultMaxMessageSize,
	}, nil
}

// CreateGRPCServer creates a gRPC server with services registered
func CreateGRPCServer(serverConfig *ServerConfig, services *ServiceCollection) (*grpc.Server, error) {
	if serverConfig == nil {
		return nil, fmt.Errorf("%s", ErrServerConfigNil)
	}
	if services == nil {
		return nil, fmt.Errorf("%s", ErrServicesRegistrationNil)
	}
	// Create gRPC server with options
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(int(serverConfig.MaxRecvMsgSize)),
		grpc.MaxSendMsgSize(int(serverConfig.MaxSendMsgSize)),
	)

	// Register all services
	if err := services.RegisterServices(grpcServer); err != nil {
		return nil, fmt.Errorf(ErrFailedRegisterServices, err)
	}

	return grpcServer, nil
}
