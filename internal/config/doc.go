// Package config provides structs and functions for configuration management.
// Here you will find [provider.Provider]: The interface for anything that retrieves a set of properties
// You will also find [config.Manager]: A facade that facilitates config snapshots and hot-reloading via subscription
// And [propertyresolver.PropertyResolver]: An interface for resolving keys to values
//
// This package has been designed carefully for purity, here is a rough overview of how it is intended to be used:
// Providers are things like file parsers, SSM agents, env var loaders etc that build KV maps. If a crucial
// provider is unimplemented, create a new type that implements the Provider interface and provide a definition
// for its Load function
//
// Adding a New Config Section:
//
//  1. Create a new package (e.g., internal/config/redis/)
//  2. Define your Config struct with validation constants
//  3. Implement Defaults(), Validate(), and FromResolver() methods
//  4. Add the section to Manager's Snapshot struct as a pointer
//  5. Update Manager.decode() to populate your section
//
// Example - Application Components Using Config:
//
//	ctx := context.Background()
//	provider := &provider.EnvVarProvider{Prefix: "APP_"}
//
//	manager, err := config.NewManager(ctx, provider)
//	if err != nil {
//		return err
//	}
//
//	// Get current config snapshot (atomic, cheap)
//	cfg := manager.Snapshot()
//	grpcServer := grpc.NewServer(cfg.GRPCServer)
//
//	// Subscribe to config changes (optional, hot-reload support)
//	manager.Subscribe("grpc-server", func(old, new config.Snapshot) {
//		if old.GRPCServer.Port != new.GRPCServer.Port {
//			grpcServer.UpdatePort(new.GRPCServer.Port)
//		}
//	})
package config
