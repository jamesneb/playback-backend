package rest

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/jamesneb/playback-backend/internal/handlers"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestDependencies_Structure(t *testing.T) {
	// Test that Dependencies struct can be created and has expected fields
	deps := &Dependencies{
		Config:               &config.Config{},
		KinesisClient:        &streaming.KinesisClient{},
		ClickHouseClient:     &storage.ClickHouseClient{},
		S3Client:             &s3.Client{},
		Endpoints:            &api.EndpointCollection{},
		ResilienceComponents: &interfaces.ResilienceComponents{},
	}

	// Verify all fields are set
	assert.NotNil(t, deps.Config)
	assert.NotNil(t, deps.KinesisClient)
	assert.NotNil(t, deps.ClickHouseClient)
	assert.NotNil(t, deps.S3Client)
	assert.NotNil(t, deps.Endpoints)
	assert.NotNil(t, deps.ResilienceComponents)
}

func TestDependencies_ZeroValues(t *testing.T) {
	// Test that Dependencies struct has expected zero values
	deps := &Dependencies{}

	assert.Nil(t, deps.Config)
	assert.Nil(t, deps.KinesisClient)
	assert.Nil(t, deps.ClickHouseClient)
	assert.Nil(t, deps.S3Client)
	assert.Nil(t, deps.Endpoints)
	assert.Nil(t, deps.ResilienceComponents)
}

func TestAPIHandlers_Structure(t *testing.T) {
	// Test that APIHandlers struct can be created and has expected fields
	apiHandlers := &APIHandlers{
		Trace:   &handlers.TraceHandler{},
		Metrics: &handlers.MetricsHandler{},
		Logs:    &handlers.LogsHandler{},
		Replay:  &handlers.ReplayHandler{},
	}

	// Verify all fields are set
	assert.NotNil(t, apiHandlers.Trace)
	assert.NotNil(t, apiHandlers.Metrics)
	assert.NotNil(t, apiHandlers.Logs)
	assert.NotNil(t, apiHandlers.Replay)
}

func TestAPIHandlers_ZeroValues(t *testing.T) {
	// Test that APIHandlers struct has expected zero values
	apiHandlers := &APIHandlers{}

	assert.Nil(t, apiHandlers.Trace)
	assert.Nil(t, apiHandlers.Metrics)
	assert.Nil(t, apiHandlers.Logs)
	assert.Nil(t, apiHandlers.Replay)
}

func TestDependencies_FieldTypes(t *testing.T) {
	// Test that Dependencies fields have correct types
	deps := &Dependencies{}

	// Test field types by attempting assignment
	deps.Config = &config.Config{}
	deps.KinesisClient = &streaming.KinesisClient{}
	deps.ClickHouseClient = &storage.ClickHouseClient{}
	deps.S3Client = &s3.Client{}
	deps.Endpoints = &api.EndpointCollection{}
	deps.ResilienceComponents = &interfaces.ResilienceComponents{}

	// If we get here without compilation errors, the types are correct
	assert.True(t, true, "All field types are compatible")
}

func TestAPIHandlers_FieldTypes(t *testing.T) {
	// Test that APIHandlers fields have correct types
	apiHandlers := &APIHandlers{}

	// Test field types by attempting assignment
	apiHandlers.Trace = &handlers.TraceHandler{}
	apiHandlers.Metrics = &handlers.MetricsHandler{}
	apiHandlers.Logs = &handlers.LogsHandler{}
	apiHandlers.Replay = &handlers.ReplayHandler{}

	// If we get here without compilation errors, the types are correct
	assert.True(t, true, "All field types are compatible")
}


func TestDependenciesWithFullConfiguration(t *testing.T) {
	// Test Dependencies with more complete configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host: "localhost",
			Port: 8080,
		},
		API: config.APIConfig{
			EnableCORS: true,
		},
	}

	endpoints := &api.EndpointCollection{}

	deps := &Dependencies{
		Config:               cfg,
		KinesisClient:        &streaming.KinesisClient{},
		ClickHouseClient:     &storage.ClickHouseClient{},
		S3Client:             &s3.Client{},
		Endpoints:            endpoints,
		ResilienceComponents: &interfaces.ResilienceComponents{},
	}

	// Verify configuration is properly set
	assert.Equal(t, "localhost", deps.Config.Server.Host)
	assert.Equal(t, 8080, deps.Config.Server.Port)
	assert.True(t, deps.Config.API.EnableCORS)
}

func TestStructInstantiation(t *testing.T) {
	// Test that both structs can be instantiated without issues
	deps := Dependencies{}
	handlers := APIHandlers{}

	// Should not panic or cause issues
	assert.IsType(t, Dependencies{}, deps)
	assert.IsType(t, APIHandlers{}, handlers)
}

func TestStructComparison(t *testing.T) {
	// Test struct equality
	deps1 := &Dependencies{}
	deps2 := &Dependencies{}

	// Two empty structs should be equal
	assert.Equal(t, deps1, deps2)

	handlers1 := &APIHandlers{}
	handlers2 := &APIHandlers{}

	// Two empty structs should be equal
	assert.Equal(t, handlers1, handlers2)
}