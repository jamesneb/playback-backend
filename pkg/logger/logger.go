package logger

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

func init() {
	// Initialize logger with proper error handling instead of panic
	logger, err := initializeLogger()
	if err != nil {
		// Use standard library logger as fallback
		log.Printf("WARNING: Failed to initialize zap logger: %v. Using fallback logger.", err)
		Logger = createFallbackLogger()
		return
	}
	Logger = logger
}

// initializeLogger creates and configures the zap logger with error handling
func initializeLogger() (*zap.Logger, error) {
	// Validate output paths exist and are writable
	if err := validateOutputPaths([]string{"stdout"}, []string{"stderr"}); err != nil {
		return nil, fmt.Errorf("output path validation failed: %w", err)
	}

	config := zap.NewProductionConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// Disable sampling to ensure all log messages are output
	config.Sampling = nil

	// Add caller info and stack traces for errors
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1), // Skip wrapper functions
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build logger config: %w", err)
	}

	return logger, nil
}

// validateOutputPaths checks if output paths are accessible
func validateOutputPaths(outputPaths, errorPaths []string) error {
	// For stdout/stderr, no validation needed as they're always available
	// For file paths, we would check write permissions here
	for _, path := range append(outputPaths, errorPaths...) {
		if path != "stdout" && path != "stderr" {
			// Check if directory exists and is writable for file paths
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("output path %s not accessible: %w", path, err)
			}
		}
	}
	return nil
}

// createFallbackLogger creates a minimal zap logger when main initialization fails
func createFallbackLogger() *zap.Logger {
	// Create the most basic logger configuration that should never fail
	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development: false,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			MessageKey:     "msg",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		// If even the fallback fails, return a no-op logger
		return zap.NewNop()
	}
	return logger
}

func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

func Sync() {
	if err := Logger.Sync(); err != nil {
		// Can't use the logger here since it might be failing
		// Use standard library to log this error
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}
