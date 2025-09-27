package grpcapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServerConfig(t *testing.T) {
	address := ":9090"
	config, err := NewServerConfig(address)

	assert.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, address, config.Address)
	assert.Equal(t, DefaultMaxMessageSize, config.MaxRecvMsgSize)
	assert.Equal(t, DefaultMaxMessageSize, config.MaxSendMsgSize)
}

func TestNewServerConfig_EmptyAddress(t *testing.T) {
	config, err := NewServerConfig("")

	assert.Error(t, err)
	assert.Nil(t, config)
	assert.Contains(t, err.Error(), ErrServerAddressEmpty)
}

func TestServerConfig_Validation(t *testing.T) {
	tests := []struct {
		name       string
		address    string
		maxRecv    MessageSize
		maxSend    MessageSize
		shouldWork bool
	}{
		{
			name:       "valid_config",
			address:    ":8080",
			maxRecv:    DefaultMaxMessageSize,
			maxSend:    DefaultMaxMessageSize,
			shouldWork: true,
		},
		{
			name:       "custom_message_sizes",
			address:    ":9090",
			maxRecv:    MaxMessageSize,
			maxSend:    MaxMessageSize,
			shouldWork: true,
		},
		{
			name:       "localhost_address",
			address:    "localhost:8080",
			maxRecv:    DefaultMaxMessageSize,
			maxSend:    DefaultMaxMessageSize,
			shouldWork: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ServerConfig{
				Address:        tt.address,
				MaxRecvMsgSize: tt.maxRecv,
				MaxSendMsgSize: tt.maxSend,
			}

			if tt.shouldWork {
				assert.NotEmpty(t, config.Address)
				assert.Greater(t, int64(config.MaxRecvMsgSize), int64(0))
				assert.Greater(t, int64(config.MaxSendMsgSize), int64(0))
			}
		})
	}
}

func TestServerConfigDefaults(t *testing.T) {
	config, err := NewServerConfig(":8080")

	assert.NoError(t, err)
	assert.NotNil(t, config)
	// Test that defaults are set correctly
	assert.Equal(t, DefaultMaxMessageSize, config.MaxRecvMsgSize)
	assert.Equal(t, DefaultMaxMessageSize, config.MaxSendMsgSize)
	assert.NotEmpty(t, config.Address)
}

// Test validation scenarios for server creation
func TestServerValidation_Scenarios(t *testing.T) {
	tests := []struct {
		name          string
		config        *ServerConfig
		services      *ServiceCollection
		expectError   bool
		errorContains string
	}{
		{
			name:          "nil_config",
			config:        nil,
			services:      &ServiceCollection{},
			expectError:   true,
			errorContains: ErrConfigNil,
		},
		{
			name: "nil_services",
			config: &ServerConfig{
				Address: ":8080",
			},
			services:      nil,
			expectError:   true,
			errorContains: ErrServicesNil,
		},
		{
			name: "empty_address",
			config: &ServerConfig{
				Address: "",
			},
			services:      &ServiceCollection{},
			expectError:   true,
			errorContains: ErrServerAddressEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.config, tt.services)

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
