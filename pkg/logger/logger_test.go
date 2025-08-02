package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLoggerInitialization(t *testing.T) {
	// Logger should be initialized automatically via init()
	assert.NotNil(t, Logger)
}

func TestLoggerFunctions(t *testing.T) {
	// Create an observer core to capture log entries
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)
	
	// Temporarily replace the global logger for testing
	originalLogger := Logger
	Logger = observedLogger
	defer func() { Logger = originalLogger }()

	tests := []struct {
		name           string
		logFunc        func(string, ...zap.Field)
		message        string
		fields         []zap.Field
		expectedLevel  zapcore.Level
		expectedCount  int
	}{
		{
			name:          "info logging",
			logFunc:       Info,
			message:       "test info message",
			fields:        []zap.Field{zap.String("key", "value")},
			expectedLevel: zap.InfoLevel,
			expectedCount: 1,
		},
		{
			name:          "error logging",
			logFunc:       Error,
			message:       "test error message",
			fields:        []zap.Field{zap.Int("count", 42)},
			expectedLevel: zap.ErrorLevel,
			expectedCount: 1,
		},
		{
			name:          "warn logging",
			logFunc:       Warn,
			message:       "test warn message",
			fields:        []zap.Field{zap.Bool("flag", true)},
			expectedLevel: zap.WarnLevel,
			expectedCount: 1,
		},
		{
			name:          "debug logging (should be filtered out)",
			logFunc:       Debug,
			message:       "test debug message",
			fields:        []zap.Field{zap.String("debug", "info")},
			expectedLevel: zap.DebugLevel,
			expectedCount: 0, // Debug level is below Info, so won't be logged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear previous log entries
			observedLogs.TakeAll()
			
			// Call the logging function
			tt.logFunc(tt.message, tt.fields...)
			
			// Check the logged entries
			entries := observedLogs.All()
			assert.Len(t, entries, tt.expectedCount)
			
			if tt.expectedCount > 0 {
				entry := entries[0]
				assert.Equal(t, tt.expectedLevel, entry.Level)
				assert.Equal(t, tt.message, entry.Message)
				
				// Verify fields were logged
				if len(tt.fields) > 0 {
					assert.True(t, len(entry.Context) > 0, "Expected context fields to be present")
				}
			}
		})
	}
}

func TestLoggerFields(t *testing.T) {
	// Create an observer core to capture log entries
	observedZapCore, observedLogs := observer.New(zap.InfoLevel)
	observedLogger := zap.New(observedZapCore)
	
	// Temporarily replace the global logger for testing
	originalLogger := Logger
	Logger = observedLogger
	defer func() { Logger = originalLogger }()

	// Test logging with various field types
	Info("test message with fields",
		zap.String("service", "test-service"),
		zap.Int("port", 8080),
		zap.Bool("enabled", true),
		zap.Duration("timeout", 30),
	)

	entries := observedLogs.All()
	assert.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "test message with fields", entry.Message)
	assert.Len(t, entry.Context, 4)

	// Check specific fields
	fields := make(map[string]interface{})
	for _, field := range entry.Context {
		switch field.Type {
		case zapcore.StringType:
			fields[field.Key] = field.String
		case zapcore.Int64Type:
			fields[field.Key] = field.Integer
		case zapcore.BoolType:
			fields[field.Key] = field.Integer == 1
		case zapcore.DurationType:
			fields[field.Key] = field.Integer
		}
	}

	assert.Equal(t, "test-service", fields["service"])
	assert.Equal(t, int64(8080), fields["port"])
	assert.Equal(t, true, fields["enabled"])
	assert.Equal(t, int64(30), fields["timeout"])
}

func TestSyncFunction(t *testing.T) {
	// Test that Sync() doesn't panic
	assert.NotPanics(t, func() {
		Sync()
	})
}

func TestLoggerConfiguration(t *testing.T) {
	// Test logger configuration by checking it's not nil and has expected properties
	assert.NotNil(t, Logger)
	
	// Test that we can create log entries without panic
	assert.NotPanics(t, func() {
		Info("configuration test")
		Error("error test") 
		Warn("warn test")
		Debug("debug test")
	})
}

// Benchmark tests for logger performance
func BenchmarkInfoLogging(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Info("benchmark message", zap.Int("iteration", i))
	}
}

func BenchmarkErrorLogging(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Error("benchmark error", zap.String("error", "test error"))
	}
}

func BenchmarkComplexLogging(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Info("complex benchmark message",
			zap.String("service", "benchmark-service"),
			zap.Int("iteration", i),
			zap.Bool("enabled", true),
			zap.Float64("rate", 99.99),
		)
	}
}