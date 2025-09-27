package grpcapi

import (
	"context"
	"testing"

	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

func TestNewServiceCollection_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		deps          *ServiceDependencies
		expectedError string
	}{
		{
			name:          "nil_dependencies",
			deps:          nil,
			expectedError: ErrServiceDepsNil,
		},
		{
			name: "nil_kinesis_client",
			deps: &ServiceDependencies{
				Config:           &config.Config{Server: config.ServerConfig{Host: "localhost", Port: 8080}},
				KinesisClient:    nil,
				ClickHouseClient: &storage.ClickHouseClient{},
			},
			expectedError: ErrKinesisClientNil,
		},
		{
			name: "nil_clickhouse_client",
			deps: &ServiceDependencies{
				Config:           &config.Config{Server: config.ServerConfig{Host: "localhost", Port: 8080}},
				KinesisClient:    &streaming.KinesisClient{},
				ClickHouseClient: nil,
			},
			expectedError: ErrClickHouseClientNil,
		},
		{
			name: "nil_config",
			deps: &ServiceDependencies{
				Config:           nil,
				KinesisClient:    &streaming.KinesisClient{},
				ClickHouseClient: &storage.ClickHouseClient{},
			},
			expectedError: ErrConfigFieldsNil,
		},
		{
			name: "empty_server_host",
			deps: &ServiceDependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Host: "", Port: 8080},
				},
				KinesisClient:        &streaming.KinesisClient{},
				ClickHouseClient:     &storage.ClickHouseClient{},
				ResilienceComponents: &interfaces.ResilienceComponents{},
			},
			expectedError: ErrServerHostEmpty,
		},
		{
			name: "invalid_server_port",
			deps: &ServiceDependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Host: "localhost", Port: 0},
				},
				KinesisClient:        &streaming.KinesisClient{},
				ClickHouseClient:     &storage.ClickHouseClient{},
				ResilienceComponents: &interfaces.ResilienceComponents{},
			},
			expectedError: ErrServerPortInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, err := NewServiceCollection(tt.deps)

			assert.Error(t, err)
			assert.Nil(t, services)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestServiceCollection_Lifecycle(t *testing.T) {
	sc := &ServiceCollection{}
	ctx := context.Background()

	// Test Start
	err := sc.Start(ctx)
	assert.NoError(t, err)

	// Test Stop
	err = sc.Stop(ctx)
	assert.NoError(t, err)

	// Test Cleanup
	err = sc.Cleanup()
	assert.NoError(t, err)
}

func TestServiceCollection_Lifecycle_NilCollection(t *testing.T) {
	var sc *ServiceCollection
	ctx := context.Background()

	// Test Start with nil collection
	err := sc.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ErrServiceCollectionNil)

	// Test Cleanup with nil collection
	err = sc.Cleanup()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ErrServiceCollectionNil)
}

func TestServiceCollection_RegisterServices_Validation(t *testing.T) {
	tests := []struct {
		name          string
		services      *ServiceCollection
		grpcServer    *grpc.Server
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil_grpc_server",
			services:      &ServiceCollection{},
			grpcServer:    nil,
			expectError:   true,
			errorContains: ErrGRPCServerNil,
		},
		{
			name:          "nil_service_collection",
			services:      nil,
			grpcServer:    grpc.NewServer(),
			expectError:   true,
			errorContains: ErrServiceCollectionNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.services.RegisterServices(tt.grpcServer)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateGRPCServer_Validation(t *testing.T) {
	tests := []struct {
		name          string
		serverConfig  *ServerConfig
		services      *ServiceCollection
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil_server_config",
			serverConfig:  nil,
			services:      &ServiceCollection{},
			expectError:   true,
			errorContains: ErrServerConfigNil,
		},
		{
			name: "nil_services",
			serverConfig: &ServerConfig{
				Address:        ":8080",
				MaxRecvMsgSize: DefaultMaxMessageSize,
				MaxSendMsgSize: DefaultMaxMessageSize,
			},
			services:      nil,
			expectError:   true,
			errorContains: ErrServicesRegistrationNil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			grpcServer, err := CreateGRPCServer(tt.serverConfig, tt.services)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, grpcServer)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, grpcServer)
			}
		})
	}
}

func TestNewServerConfig_Validation(t *testing.T) {
	// Test normal case
	config, err := NewServerConfig(":8080")
	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, ":8080", config.Address)

	// Test error case
	config, err = NewServerConfig("")
	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), ErrServerAddressEmpty)
}

func TestServiceDependencies_Structure(t *testing.T) {
	// Test that ServiceDependencies struct has expected fields
	deps := &ServiceDependencies{}

	// Verify struct can be instantiated and has expected zero values
	assert.Nil(t, deps.Config)
	assert.Nil(t, deps.KinesisClient)
	assert.Nil(t, deps.ClickHouseClient)
	assert.Nil(t, deps.ResilienceComponents)
	assert.Nil(t, deps.StreamHandler)
	assert.Nil(t, deps.ClickhouseHandler)
}

func TestServiceCollection_Structure(t *testing.T) {
	// Test that ServiceCollection struct has expected fields
	sc := &ServiceCollection{}

	// Verify struct can be instantiated and has expected zero values
	assert.Nil(t, sc.TraceService)
	assert.Nil(t, sc.MetricsService)
	assert.Nil(t, sc.LogsService)
}

func TestServerConfig_Structure(t *testing.T) {
	config := &ServerConfig{
		Address:        ":8080",
		MaxRecvMsgSize: DefaultMaxMessageSize,
		MaxSendMsgSize: MaxMessageSize,
	}

	assert.Equal(t, ":8080", config.Address)
	assert.Equal(t, DefaultMaxMessageSize, config.MaxRecvMsgSize)
	assert.Equal(t, MaxMessageSize, config.MaxSendMsgSize)
}

func TestErrorConstants_Completeness(t *testing.T) {
	// Verify that all error constants are defined and non-empty
	errorConstants := []string{
		ErrServiceDepsNil,
		ErrKinesisClientNil,
		ErrClickHouseClientNil,
		ErrConfigFieldsNil,
		ErrResilienceCompNil,
		ErrServerHostEmpty,
		ErrServerPortInvalid,
		ErrGRPCServerNil,
		ErrServiceCollectionNil,
		ErrServerConfigNil,
		ErrServicesRegistrationNil,
		ErrFailedCreateHandler,
	}

	for _, errorConstant := range errorConstants {
		assert.NotEmpty(t, errorConstant, "Error constant should not be empty")
		assert.Greater(t, len(errorConstant), 5, "Error constant should be descriptive")
	}
}
