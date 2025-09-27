package grpcapi

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBytes_String(t *testing.T) {
	tests := []struct {
		name     string
		bytes    Bytes
		expected string
	}{
		{
			name:     "small_bytes",
			bytes:    512,
			expected: "512 B",
		},
		{
			name:     "one_kb",
			bytes:    1024,
			expected: "1.0 KB",
		},
		{
			name:     "one_mb",
			bytes:    1024 * 1024,
			expected: "1.0 MB",
		},
		{
			name:     "four_mb",
			bytes:    4 * 1024 * 1024,
			expected: "4.0 MB",
		},
		{
			name:     "16_mb",
			bytes:    16 * 1024 * 1024,
			expected: "16.0 MB",
		},
		{
			name:     "one_gb",
			bytes:    1024 * 1024 * 1024,
			expected: "1.0 GB",
		},
		{
			name:     "fractional_mb",
			bytes:    1536 * 1024, // 1.5 MB
			expected: "1.5 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bytes.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessageSizeConstants(t *testing.T) {
	assert.Equal(t, MessageSize(4*1024*1024), DefaultMaxMessageSize)
	assert.Equal(t, MessageSize(16*1024*1024), MaxMessageSize)
	assert.Equal(t, MessageSize(1024), MinMessageSize)
	assert.Equal(t, Bytes(1024), BytesInKB)
}

func TestTimeoutConstants(t *testing.T) {
	assert.Equal(t, Timeout(10*time.Second), DefaultShutdownTimeout)
	assert.Equal(t, Timeout(5*time.Second), DefaultStartTimeout)
	assert.Equal(t, Timeout(30*time.Second), MaxShutdownTimeout)
}

func TestProtocolConstants(t *testing.T) {
	assert.Equal(t, "OTLP/gRPC", ProtocolOTLPgRPC)
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants are not empty
	errorConstants := []string{
		ErrConfigNil,
		ErrServicesNil,
		ErrServerAddressEmpty,
		ErrContextNil,
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
		ErrFailedRegisterServices,
		ErrFailedStartServices,
		ErrServiceShutdownFailed,
		ErrShutdownTimeout,
		ErrCleanupErrors,
		ErrShutdownErrors,
		ErrFailedCreateHandler,
	}

	for _, errorConstant := range errorConstants {
		assert.NotEmpty(t, errorConstant, "Error constant should not be empty")
	}
}

func TestLogMessageConstants(t *testing.T) {
	// Test that log message constants are not empty
	logConstants := []string{
		MsgStartingGRPCServer,
		MsgStoppingGRPCServer,
		MsgGRPCServerStoppedGraceful,
		MsgGRPCServerShutdownTimeout,
		MsgFailedStopServices,
	}

	for _, logConstant := range logConstants {
		assert.NotEmpty(t, logConstant, "Log constant should not be empty")
	}
}

func TestSizeFormatConstants(t *testing.T) {
	assert.Equal(t, "%d B", SizeFormatBytes)
	assert.Equal(t, "%.1f %cB", SizeFormatUnit)
	assert.Equal(t, "KMGTPE", SizeUnits)
}

func TestTypeAliases(t *testing.T) {
	// Test that type aliases work correctly
	var b Bytes = 1024
	var ms MessageSize = 4096
	var to = Timeout(5 * time.Second)
	var sp ServerPort = 8080

	assert.Equal(t, int64(1024), int64(b))
	assert.Equal(t, int64(4096), int64(ms))
	assert.Equal(t, 5*time.Second, time.Duration(to))
	assert.Equal(t, 8080, int(sp))
}

func TestBytesConversion(t *testing.T) {
	// Test conversion between Bytes and MessageSize
	var b Bytes = 1024
	var ms = MessageSize(b)

	assert.Equal(t, b, Bytes(ms))
}

func TestTimeoutConversion(t *testing.T) {
	// Test conversion between time.Duration and Timeout
	duration := 10 * time.Second
	timeout := Timeout(duration)

	assert.Equal(t, duration, time.Duration(timeout))
}

func TestConstantRelationships(t *testing.T) {
	// Test that constants have expected relationships
	assert.True(t, DefaultMaxMessageSize < MaxMessageSize)
	assert.True(t, MinMessageSize < DefaultMaxMessageSize)
	assert.True(t, DefaultStartTimeout < DefaultShutdownTimeout)
	assert.True(t, DefaultShutdownTimeout < MaxShutdownTimeout)
}

func TestBytesStringFormatting_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		bytes    Bytes
		expected string
	}{
		{
			name:     "zero_bytes",
			bytes:    0,
			expected: "0 B",
		},
		{
			name:     "one_byte",
			bytes:    1,
			expected: "1 B",
		},
		{
			name:     "max_bytes_before_kb",
			bytes:    1023,
			expected: "1023 B",
		},
		{
			name:     "exactly_1kb",
			bytes:    1024,
			expected: "1.0 KB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.bytes.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultValues(t *testing.T) {
	// Test that default values are reasonable
	assert.Greater(t, int64(DefaultMaxMessageSize), int64(0))
	assert.Greater(t, int64(MaxMessageSize), int64(DefaultMaxMessageSize))
	assert.Greater(t, int64(MinMessageSize), int64(0))
	assert.Greater(t, time.Duration(DefaultShutdownTimeout), time.Duration(0))
	assert.Greater(t, time.Duration(DefaultStartTimeout), time.Duration(0))
	assert.Greater(t, time.Duration(MaxShutdownTimeout), time.Duration(DefaultShutdownTimeout))
}