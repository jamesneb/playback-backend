package rest

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jamesneb/playback-backend/internal/interfaces"
	"github.com/jamesneb/playback-backend/internal/storage"
	"github.com/jamesneb/playback-backend/internal/streaming"
	"github.com/jamesneb/playback-backend/pkg/api"
	"github.com/jamesneb/playback-backend/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestValidateDependencies(t *testing.T) {
	tests := []struct {
		name        string
		deps        *Dependencies
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil_dependencies",
			deps:        nil,
			expectError: true,
			errorMsg:    ERROR_DEPENDENCIES_NIL,
		},
		{
			name: "nil_config",
			deps: &Dependencies{
				Config:    nil,
				Endpoints: &api.EndpointCollection{},
			},
			expectError: true,
			errorMsg:    ERROR_CONFIG_NIL,
		},
		{
			name: "nil_endpoints",
			deps: &Dependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Mode: gin.ReleaseMode},
				},
				Endpoints: nil,
			},
			expectError: true,
			errorMsg:    ERROR_ENDPOINTS_NIL,
		},
		{
			name: "invalid_server_mode",
			deps: &Dependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Mode: "invalid"},
				},
				Endpoints: &api.EndpointCollection{},
			},
			expectError: true,
			errorMsg:    "invalid server mode",
		},
		{
			name: "valid_dependencies_release_mode",
			deps: &Dependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Mode: gin.ReleaseMode},
				},
				Endpoints: &api.EndpointCollection{},
			},
			expectError: false,
		},
		{
			name: "valid_dependencies_debug_mode",
			deps: &Dependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Mode: gin.DebugMode},
				},
				Endpoints: &api.EndpointCollection{},
			},
			expectError: false,
		},
		{
			name: "valid_dependencies_test_mode",
			deps: &Dependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Mode: gin.TestMode},
				},
				Endpoints: &api.EndpointCollection{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDependencies(tt.deps)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyConfig(t *testing.T) {
	// Set initial mode
	gin.SetMode(gin.ReleaseMode)

	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
	}{
		{
			name: "set_debug_mode",
			config: &config.Config{
				Server: config.ServerConfig{Mode: gin.DebugMode},
			},
			expectError: false,
		},
		{
			name: "set_release_mode",
			config: &config.Config{
				Server: config.ServerConfig{Mode: gin.ReleaseMode},
			},
			expectError: false,
		},
		{
			name: "set_test_mode",
			config: &config.Config{
				Server: config.ServerConfig{Mode: gin.TestMode},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalMode := gin.Mode()

			err := applyConfig(tt.config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.config.Server.Mode, gin.Mode())
			}

			// Reset mode
			gin.SetMode(originalMode)
		})
	}
}

func TestSetupTrustedProxies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		trustedProxies []string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid_ip_addresses",
			trustedProxies: []string{"127.0.0.1", "192.168.1.1"},
			expectError:    false,
		},
		{
			name:           "localhost_should_be_invalid",
			trustedProxies: []string{"localhost"},
			expectError:    true,
			errorContains:  "failed to set trusted proxies",
		},
		{
			name:           "valid_cidr",
			trustedProxies: []string{"192.168.0.0/24"},
			expectError:    false,
		},
		{
			name:           "empty_proxies",
			trustedProxies: []string{},
			expectError:    false,
		},
		{
			name:           "invalid_proxy_address",
			trustedProxies: []string{"invalid-address"},
			expectError:    true,
			errorContains:  "invalid trusted proxy address",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			cfg := &config.Config{
				Server: config.ServerConfig{
					TrustedProxies: tt.trustedProxies,
				},
			}

			err := setupTrustedProxies(r, cfg)

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

func TestTimeoutMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Test that the middleware function doesn't panic when called
	middleware := timeoutMiddleware(REQUEST_TIMEOUT)
	assert.NotNil(t, middleware)

	// Test with zero timeout
	middleware = timeoutMiddleware(0)
	assert.NotNil(t, middleware)
}

func TestDependenciesStructure(t *testing.T) {
	// Test that Dependencies struct has expected fields
	deps := &Dependencies{}

	// Verify struct can be instantiated and has expected zero values
	assert.Nil(t, deps.Config)
	assert.Nil(t, deps.KinesisClient)
	assert.Nil(t, deps.ClickHouseClient)
	assert.Nil(t, deps.S3Client)
	assert.Nil(t, deps.Endpoints)
	assert.Nil(t, deps.ResilienceComponents)
}

func TestAPIHandlersStructure(t *testing.T) {
	// Test that APIHandlers struct has expected fields
	handlers := &APIHandlers{}

	// Verify struct can be instantiated and has expected zero values
	assert.Nil(t, handlers.Trace)
	assert.Nil(t, handlers.Metrics)
	assert.Nil(t, handlers.Logs)
	assert.Nil(t, handlers.Replay)
}

func TestNewServer_ValidationErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		deps          *Dependencies
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil_dependencies",
			deps:          nil,
			expectError:   true,
			errorContains: "invalid dependencies",
		},
		{
			name: "nil_config",
			deps: &Dependencies{
				Config:    nil,
				Endpoints: &api.EndpointCollection{},
			},
			expectError:   true,
			errorContains: "invalid dependencies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.deps)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, server)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
			}
		})
	}
}

func TestValidServerModes(t *testing.T) {
	validModes := []string{gin.ReleaseMode, gin.DebugMode, gin.TestMode}

	for _, mode := range validModes {
		t.Run("mode_"+mode, func(t *testing.T) {
			deps := &Dependencies{
				Config: &config.Config{
					Server: config.ServerConfig{Mode: mode},
				},
				Endpoints: &api.EndpointCollection{},
			}

			err := validateDependencies(deps)
			assert.NoError(t, err)
		})
	}
}

func TestMinimalValidDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create minimal valid dependencies
	deps := &Dependencies{
		Config: &config.Config{
			Server: config.ServerConfig{
				Mode:           gin.TestMode,
				TrustedProxies: []string{},
			},
			API: config.APIConfig{
				EnableCORS: false,
			},
		},
		Endpoints:            &api.EndpointCollection{},
		KinesisClient:        &streaming.KinesisClient{},
		ClickHouseClient:     &storage.ClickHouseClient{},
		ResilienceComponents: &interfaces.ResilienceComponents{},
	}

	// This should pass validation but may fail in later setup steps
	err := validateDependencies(deps)
	assert.NoError(t, err, "Minimal dependencies should pass validation")
}