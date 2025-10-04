// Package testing defines configuration for test environment behavior.
//
// This package provides configuration for controlling test-specific behavior,
// such as mocking external services and adjusting timeouts for test execution.
//
// # Test Behavior
//
// Mock settings:
//   - MockExternalServices: Replace real external services with mocks
//
// # Environment Variable Overrides
//
// All configuration values can be overridden via environment variables with the
// TESTING_ prefix:
//
//	TESTING_MOCK_EXTERNAL_SERVICES=true
//
// # Files in This Package
//
// constants.go:
//   - TESTING_PREFIX for environment variable namespacing
//   - Default values for test behavior flags
//
// section.go:
//   - Config struct with testing parameters
//   - Defaults() for baseline configuration
//   - FromResolver() for loading from config providers
//   - Validate() for correctness checks
package testing
