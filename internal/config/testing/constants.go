package testing

// Environment variable prefix for testing configuration.
//
// All testing configuration environment variables start with this prefix.
// Example variables:
//   - TESTING_MOCK_EXTERNAL_SERVICES
const (
	TESTING_PREFIX = "TESTING_"
)

// Default configuration values for testing.
//
// These constants define sensible defaults for test behavior.
const (
	// DEFAULT_MOCK_EXTERNAL_SERVICES controls whether external services are mocked.
	// False by default to support integration tests with real services.
	//
	// Set to true for:
	//   - Unit tests (fast, isolated, no network)
	//   - CI/CD pipelines (consistent, reproducible)
	//   - Offline development (no AWS credentials needed)
	//
	// Set to false for:
	//   - Integration tests (test real service interactions)
	//   - End-to-end tests (test complete system)
	//   - LocalStack testing (test with local AWS services)
	//
	// Enable via environment variable:
	//   export TESTING_MOCK_EXTERNAL_SERVICES=true
	DEFAULT_MOCK_EXTERNAL_SERVICES = false
)
