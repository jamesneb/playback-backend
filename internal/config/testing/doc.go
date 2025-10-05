// Package testing defines configuration for test environment behavior and mocking strategies.
//
// This package provides configuration for controlling test-specific behavior including mocking
// external services, test data fixtures, integration test setup, and table-driven test patterns.
// It enables consistent testing practices across unit, integration, and end-to-end tests.
//
// # Overview
//
// Testing configuration enables:
//
//   - Mock external services (AWS, databases, APIs) for isolated testing
//   - Test fixture management for consistent test data
//   - Integration test configuration with real services
//   - Table-driven test patterns with parameterized inputs
//   - Test environment isolation and cleanup
//   - Deterministic test execution with controlled randomness
//
// # Configuration Keys
//
// All settings use the TESTING_ prefix and support environment variable overrides:
//
//	TESTING_MOCK_EXTERNAL_SERVICES - Enable service mocking (default: false)
//
// # Test Environment Types
//
// This package supports different test environments:
//
// Unit Tests:
//   - Fast, isolated tests of individual functions
//   - Mock all external dependencies
//   - No network or I/O operations
//   - Run in milliseconds
//   - TESTING_MOCK_EXTERNAL_SERVICES=true
//
// Integration Tests:
//   - Test interactions between components
//   - Use real external services (databases, queues)
//   - May use LocalStack for AWS services
//   - Run in seconds to minutes
//   - TESTING_MOCK_EXTERNAL_SERVICES=false
//
// End-to-End Tests:
//   - Test complete system functionality
//   - Use production-like services
//   - Full request/response cycles
//   - Run in minutes
//   - TESTING_MOCK_EXTERNAL_SERVICES=false
//
// # Mocking External Services
//
//	TESTING_MOCK_EXTERNAL_SERVICES=true
//
// When enabled, external services are replaced with mocks:
//
// Mocked services:
//   - AWS S3 (in-memory storage)
//   - AWS Kinesis (in-memory streams)
//   - AWS SQS (in-memory queues)
//   - Databases (in-memory or test databases)
//   - HTTP APIs (mock servers)
//   - Time (deterministic time for reproducibility)
//
// Mocking benefits:
//   - Fast test execution (no network I/O)
//   - Deterministic results (no external variability)
//   - No external dependencies (offline testing)
//   - Easy failure simulation (network errors, timeouts)
//   - Parallel test execution (no shared state)
//   - Cost-free (no AWS charges)
//
// Mocking tradeoffs:
//   - May not catch integration issues
//   - Mocks may diverge from real service behavior
//   - Requires mock maintenance
//   - May miss edge cases specific to real services
//
// # Test Fixtures
//
// Test fixtures provide consistent test data across tests:
//
// Fixture types:
//
// Static fixtures (JSON files):
//
//	testdata/
//	├── traces/
//	│   ├── valid_trace.json
//	│   ├── invalid_trace.json
//	│   └── large_trace.json
//	├── metrics/
//	│   └── sample_metrics.json
//	└── logs/
//	    └── sample_logs.json
//
// Dynamic fixtures (generated):
//
//	func NewTestTrace(opts ...Option) *Trace {
//	    trace := &Trace{
//	        TraceID: uuid.New().String(),
//	        Spans:   []Span{NewTestSpan()},
//	    }
//	    for _, opt := range opts {
//	        opt(trace)
//	    }
//	    return trace
//	}
//
// Fixture best practices:
//   - Use small, focused fixtures for specific test cases
//   - Create builder functions for complex fixtures
//   - Use option patterns for fixture customization
//   - Version fixtures with test code
//   - Document fixture purpose and structure
//
// # Mocking Patterns
//
// Interface-based mocking (recommended):
//
//	// Production code uses interface
//	type S3Client interface {
//	    PutObject(context.Context, *s3.PutObjectInput) error
//	    GetObject(context.Context, *s3.GetObjectInput) (*s3.GetObjectOutput, error)
//	}
//
//	// Mock implementation for tests
//	type MockS3Client struct {
//	    PutObjectFunc func(context.Context, *s3.PutObjectInput) error
//	    GetObjectFunc func(context.Context, *s3.GetObjectInput) (*s3.GetObjectOutput, error)
//	}
//
//	func (m *MockS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput) error {
//	    if m.PutObjectFunc != nil {
//	        return m.PutObjectFunc(ctx, input)
//	    }
//	    return nil
//	}
//
// Using mocks in tests:
//
//	func TestUploadTrace(t *testing.T) {
//	    mockS3 := &MockS3Client{
//	        PutObjectFunc: func(ctx context.Context, input *s3.PutObjectInput) error {
//	            // Verify inputs
//	            assert.Equal(t, "traces/trace-123.json", *input.Key)
//	            return nil
//	        },
//	    }
//
//	    uploader := NewUploader(mockS3)
//	    err := uploader.Upload(ctx, trace)
//	    assert.NoError(t, err)
//	}
//
// # Integration Testing Setup
//
// LocalStack for AWS services:
//
//	// docker-compose.test.yml
//	services:
//	  localstack:
//	    image: localstack/localstack:latest
//	    ports:
//	      - "4566:4566"
//	    environment:
//	      - SERVICES=s3,kinesis,sqs
//	      - DEBUG=1
//
// Test setup:
//
//	func TestMain(m *testing.M) {
//	    // Start LocalStack
//	    pool, resource := startLocalStack()
//	    defer pool.Purge(resource)
//
//	    // Run tests
//	    code := m.Run()
//
//	    // Cleanup
//	    os.Exit(code)
//	}
//
//	func TestS3Integration(t *testing.T) {
//	    if testing.Short() {
//	        t.Skip("skipping integration test")
//	    }
//
//	    // Create S3 client pointing to LocalStack
//	    cfg := s3.Config{
//	        EndpointURL:    "http://localhost:4566",
//	        ForcePathStyle: true,
//	        AccessKeyID:    "test",
//	        SecretAccessKey: "test",
//	    }
//
//	    client := createS3Client(cfg)
//	    // Test with real S3 operations
//	}
//
// # Table-Driven Tests
//
// Table-driven tests enable testing multiple scenarios with minimal code:
//
//	func TestValidateTrace(t *testing.T) {
//	    tests := []struct {
//	        name    string
//	        trace   *Trace
//	        wantErr bool
//	        errMsg  string
//	    }{
//	        {
//	            name: "valid trace",
//	            trace: &Trace{
//	                TraceID: "trace-123",
//	                Spans:   []Span{{SpanID: "span-1"}},
//	            },
//	            wantErr: false,
//	        },
//	        {
//	            name: "missing trace ID",
//	            trace: &Trace{
//	                Spans: []Span{{SpanID: "span-1"}},
//	            },
//	            wantErr: true,
//	            errMsg:  "trace ID required",
//	        },
//	        {
//	            name:    "nil trace",
//	            trace:   nil,
//	            wantErr: true,
//	            errMsg:  "trace cannot be nil",
//	        },
//	    }
//
//	    for _, tt := range tests {
//	        t.Run(tt.name, func(t *testing.T) {
//	            err := ValidateTrace(tt.trace)
//	            if tt.wantErr {
//	                assert.Error(t, err)
//	                if tt.errMsg != "" {
//	                    assert.Contains(t, err.Error(), tt.errMsg)
//	                }
//	            } else {
//	                assert.NoError(t, err)
//	            }
//	        })
//	    }
//	}
//
// Table-driven test benefits:
//   - Easy to add new test cases
//   - Clear test case organization
//   - Consistent test structure
//   - Parallel test execution (t.Parallel())
//   - Easy to identify failing scenarios
//
// # Test Helpers and Utilities
//
// Common test helpers:
//
//	// Assertion helpers
//	func RequireNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
//	    t.Helper()
//	    if err != nil {
//	        t.Fatalf("unexpected error: %v %v", err, msgAndArgs)
//	    }
//	}
//
//	// Cleanup helpers
//	func CleanupTest(t *testing.T, cleanup func()) {
//	    t.Helper()
//	    t.Cleanup(func() {
//	        if err := recover(); err != nil {
//	            t.Errorf("cleanup panic: %v", err)
//	        }
//	        cleanup()
//	    })
//	}
//
//	// Timeout helpers
//	func WithTimeout(t *testing.T, timeout time.Duration) context.Context {
//	    t.Helper()
//	    ctx, cancel := context.WithTimeout(context.Background(), timeout)
//	    t.Cleanup(cancel)
//	    return ctx
//	}
//
// # Example Usage
//
//	// Load testing configuration
//	cfg, err := testing.FromResolver(envProvider)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Unit test with mocks
//	func TestProcessTrace(t *testing.T) {
//	    if !cfg.MockExternalServices {
//	        t.Skip("mocking disabled")
//	    }
//
//	    mockS3 := &MockS3Client{}
//	    mockKinesis := &MockKinesisClient{}
//
//	    processor := NewProcessor(mockS3, mockKinesis)
//	    err := processor.Process(ctx, trace)
//	    assert.NoError(t, err)
//	}
//
//	// Integration test with real services
//	func TestProcessTraceIntegration(t *testing.T) {
//	    if cfg.MockExternalServices {
//	        t.Skip("integration test requires real services")
//	    }
//
//	    if testing.Short() {
//	        t.Skip("skipping integration test")
//	    }
//
//	    // Use real S3, Kinesis via LocalStack
//	    processor := NewProcessor(realS3Client, realKinesisClient)
//	    err := processor.Process(ctx, trace)
//	    assert.NoError(t, err)
//	}
//
// # Best Practices
//
// Unit test configuration:
//
//	TESTING_MOCK_EXTERNAL_SERVICES=true
//	# Run with: go test -short ./...
//
// Integration test configuration:
//
//	TESTING_MOCK_EXTERNAL_SERVICES=false
//	# Run with: go test ./...
//
// CI/CD configuration:
//
//	# Fast unit tests on every commit
//	- name: Unit Tests
//	  run: go test -short -race ./...
//	  env:
//	    TESTING_MOCK_EXTERNAL_SERVICES: true
//
//	# Integration tests on merge to main
//	- name: Integration Tests
//	  run: go test -v ./...
//	  env:
//	    TESTING_MOCK_EXTERNAL_SERVICES: false
//
// Test organization:
//   - Use _test.go suffix for test files
//   - Co-locate tests with code (same package)
//   - Use testdata/ for fixtures
//   - Separate unit and integration tests with build tags
//   - Use subtests (t.Run) for related tests
//   - Use table-driven tests for multiple scenarios
//
// Test isolation:
//   - Each test should be independent
//   - Use t.Cleanup() for resource cleanup
//   - Don't rely on test execution order
//   - Avoid shared mutable state
//   - Use unique IDs (UUIDs) for test data
//
// Test naming:
//   - Test functions: TestFunctionName
//   - Subtests: descriptive names (e.g., "missing required field")
//   - Benchmark functions: BenchmarkFunctionName
//   - Example functions: ExampleFunctionName
//
// # Troubleshooting
//
// Tests flaky with real services:
//
//	Problem: Tests pass sometimes, fail others
//	Fix: Enable TESTING_MOCK_EXTERNAL_SERVICES=true
//	Fix: Increase timeouts for integration tests
//	Fix: Add retries for transient failures
//	Fix: Check for shared state between tests
//
// Mocks out of sync with real services:
//
//	Problem: Tests pass but production fails
//	Fix: Run integration tests regularly
//	Fix: Update mocks when service APIs change
//	Fix: Use contract testing
//	Fix: Monitor production vs test behavior differences
//
// Slow test execution:
//
//	Problem: Tests take too long to run
//	Fix: Enable mocking for unit tests
//	Fix: Use t.Parallel() for independent tests
//	Fix: Run integration tests separately
//	Fix: Cache test fixtures
//	Fix: Use test databases instead of production
//
// # Cross-References
//
// Related packages:
//   - [base.Validator] - Validation framework
//   - [s3] - S3 configuration for integration tests
//   - [kinesis] - Kinesis configuration for integration tests
//   - [dlq] - DLQ configuration for integration tests
//
// Testing resources:
//   - Go Testing: https://pkg.go.dev/testing
//   - Table-Driven Tests: https://go.dev/wiki/TableDrivenTests
//   - Testify: https://github.com/stretchr/testify
//   - LocalStack: https://localstack.cloud/
//
// # Files in This Package
//
// constants.go:
//   - TESTING_PREFIX for environment variable namespacing
//   - Default values for test behavior flags
//
// section.go:
//   - [Config] struct with testing parameters
//   - [Defaults] for baseline configuration
//   - [FromResolver] for loading from config providers
//   - [Config.Validate] for correctness checks
package testing
