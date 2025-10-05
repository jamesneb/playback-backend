// Package config centralises the primitives used to load, validate, and distribute
// configuration to the rest of the application.
//
// Core abstractions:
//   - [provider.Provider] implementations acquire raw key/value material from
//     environment variables, parameter stores, or other backing systems.
//   - [Manager] orchestrates providers, snapshots validated configuration, and
//     exposes subscription hooks for hot reloading.
//   - [propertyresolver.PropertyResolver] performs typed lookups while enforcing
//     defaults and validation rules defined by each config section.
//
// Typical workflow:
//   1. Compose one or more Provider implementations suited to the deployment
//      environment (for example, environment variables or parameter stores).
//   2. Instantiate a Manager with those providers. The Manager flattens and
//      validates configuration, emitting a strongly typed Snapshot.
//   3. Pass snapshot fragments into application components. Optionally subscribe
//      to changes to support hot reloading without restarts.
//
// Adding a new config section:
//
//   1. Create a package (such as internal/config/redis) that declares a Config
//      struct representing the section.
//   2. Provide Defaults, Validate, and FromResolver helpers so the Manager can
//      initialise and populate the section consistently.
//   3. Add a pointer to the new section in [Snapshot] and extend Manager.decode
//      to fill it from the shared resolver.
//
// Example usage:
//
//      ctx := context.Background()
//      manager, err := config.NewManager(ctx, &provider.EnvVarProvider{Prefix: "APP_"})
//      if err != nil {
//              return err
//      }
//
//      snapshot := manager.Snapshot()
//      grpcCfg := snapshot.GRPCServer
//      lis, err := net.Listen("tcp", fmt.Sprintf(":%d", grpcCfg.Port))
//      if err != nil {
//              return err
//      }
//
//      manager.Subscribe("grpc-server", func(oldSnap, newSnap config.Snapshot) {
//              if oldSnap.GRPCServer.Port != newSnap.GRPCServer.Port {
//                      log.Printf("grpc port changed: %d -> %d", oldSnap.GRPCServer.Port, newSnap.GRPCServer.Port)
//              }
//      })
//
// The package is intentionally side-effect free, making it safe to use in unit
// tests by providing alternate providers or stubbed resolvers.
package config
