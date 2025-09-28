package rest

import (
	"testing"

	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/jamesneb/playback-backend/pkg/config/server"
)

// BenchmarkNewAPIHandlers benchmarks the handler creation performance
func BenchmarkNewAPIHandlers(b *testing.B) {
	// Create mock dependencies
	deps := &Dependencies{
		Config:               &config.Config{},
		KinesisClient:        &streaming.KinesisClient{}, // Mock client
		ResilienceComponents: &interfaces.ResilienceComponents{},
		Endpoints:            api.NewEndpointCollection(""),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		handlers, err := NewAPIHandlers(deps)
		if err != nil {
			b.Fatalf("Failed to create handlers: %v", err)
		}
		_ = handlers
	}
}

// BenchmarkNewAPIHandlersParallel benchmarks handler creation under concurrent load
func BenchmarkNewAPIHandlersParallel(b *testing.B) {
	deps := &Dependencies{
		Config:               &config.Config{},
		KinesisClient:        &streaming.KinesisClient{},
		ResilienceComponents: &interfaces.ResilienceComponents{},
		Endpoints:            api.NewEndpointCollection(""),
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			handlers, err := NewAPIHandlers(deps)
			if err != nil {
				b.Fatalf("Failed to create handlers: %v", err)
			}
			_ = handlers
		}
	})
}

// BenchmarkDependencyValidation benchmarks dependency validation
func BenchmarkDependencyValidation(b *testing.B) {
	tests := []struct {
		name string
		deps *Dependencies
	}{
		{
			name: "valid_deps",
			deps: &Dependencies{
				Config:               &config.Config{Server: server.ServerConfig{Mode: "release"}},
				KinesisClient:        &streaming.KinesisClient{},
				ResilienceComponents: &interfaces.ResilienceComponents{},
				Endpoints:            api.NewEndpointCollection(""),
			},
		},
		{
			name: "nil_deps",
			deps: nil,
		},
		{
			name: "missing_config",
			deps: &Dependencies{
				KinesisClient: &streaming.KinesisClient{},
				Endpoints:     api.NewEndpointCollection(""),
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = validateDependencies(tt.deps)
			}
		})
	}
}

// BenchmarkStringBuilding benchmarks string building operations
func BenchmarkStringBuilding(b *testing.B) {
	tests := []struct {
		name      string
		operation func() string
	}{
		{
			name: "simple_concat",
			operation: func() string {
				return "prefix" + "middle" + "suffix"
			},
		},
		{
			name: "sprintf",
			operation: func() string {
				return "prefix" + "middle" + "suffix"
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				result := tt.operation()
				_ = result
			}
		})
	}
}
