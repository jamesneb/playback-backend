package logger

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerConfig holds configuration for logger creation
type LoggerConfig struct {
	Level        zapcore.Level
	OutputPaths  []string
	ErrorPaths   []string
	Development  bool
	DisableCaller bool
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:        zapcore.InfoLevel,
		OutputPaths:  []string{"stdout"},
		ErrorPaths:   []string{"stderr"},
		Development:  false,
		DisableCaller: false,
	}
}

// DevelopmentConfig returns configuration suitable for development
func DevelopmentConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:        zapcore.DebugLevel,
		OutputPaths:  []string{"stdout"},
		ErrorPaths:   []string{"stderr"},
		Development:  true,
		DisableCaller: false,
	}
}

// ProductionConfig returns configuration suitable for production
func ProductionConfig() *LoggerConfig {
	return &LoggerConfig{
		Level:        zapcore.InfoLevel,
		OutputPaths:  []string{"stdout"},
		ErrorPaths:   []string{"stderr"},
		Development:  false,
		DisableCaller: false,
	}
}

// NewLogger creates a new logger with the given configuration
// This replaces the global init() function with explicit construction
func NewLogger(config *LoggerConfig) (*zap.Logger, error) {
	if config == nil {
		return nil, fmt.Errorf("logger configuration cannot be nil")
	}

	// Validate output paths exist and are writable
	if err := validateOutputPaths(config.OutputPaths, config.ErrorPaths); err != nil {
		return nil, fmt.Errorf("output path validation failed: %w", err)
	}

	return buildLogger(config)
}

// NewDefaultLogger creates a logger with default configuration
func NewDefaultLogger() (*zap.Logger, error) {
	return NewLogger(DefaultConfig())
}

// MustNewLogger creates a logger or panics - use sparingly and only at application startup
func MustNewLogger(config *LoggerConfig) *zap.Logger {
	logger, err := NewLogger(config)
	if err != nil {
		log.Panicf("Failed to create logger: %v", err)
	}
	return logger
}

// NewLoggerWithFallback creates a logger with automatic fallback on failure
func NewLoggerWithFallback(config *LoggerConfig) *zap.Logger {
	logger, err := NewLogger(config)
	if err != nil {
		// Use standard library logger as fallback
		log.Printf("WARNING: Failed to initialize zap logger: %v. Using fallback logger.", err)
		return createFallbackLogger()
	}
	return logger
}

// buildLogger creates and configures the zap logger with error handling
func buildLogger(config *LoggerConfig) (*zap.Logger, error) {
	var zapConfig zap.Config

	if config.Development {
		zapConfig = zap.NewDevelopmentConfig()
	} else {
		zapConfig = zap.NewProductionConfig()
		// Disable sampling to ensure all log messages are output
		zapConfig.Sampling = nil
	}

	// Apply configuration
	zapConfig.Level = zap.NewAtomicLevelAt(config.Level)
	zapConfig.OutputPaths = config.OutputPaths
	zapConfig.ErrorOutputPaths = config.ErrorPaths

	// Build options
	options := []zap.Option{
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	if !config.DisableCaller {
		options = append(options, zap.AddCaller())
	}

	logger, err := zapConfig.Build(options...)
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

// Logger is a backwards compatibility type that wraps *zap.Logger
// This allows gradual migration from global logger usage
type Logger struct {
	*zap.Logger
}

// NewLoggerFromZap creates a Logger from a *zap.Logger
func NewLoggerFromZap(zapLogger *zap.Logger) *Logger {
	return &Logger{Logger: zapLogger}
}

// Sync safely syncs the underlying logger
func (l *Logger) Sync() error {
	if l.Logger == nil {
		return fmt.Errorf("logger is nil")
	}

	if err := l.Logger.Sync(); err != nil {
		// Can't use the logger here since it might be failing
		// Use standard library to log this error
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
		return err
	}
	return nil
}

// Global logger instance for backwards compatibility
// Deprecated: Use dependency injection with NewLogger instead
var globalLogger *Logger

// InitGlobalLogger initializes the global logger instance
// Deprecated: Use dependency injection with NewLogger instead
func InitGlobalLogger(config *LoggerConfig) error {
	zapLogger, err := NewLogger(config)
	if err != nil {
		return err
	}
	globalLogger = NewLoggerFromZap(zapLogger)
	return nil
}

// GetGlobalLogger returns the global logger instance
// Deprecated: Use dependency injection with NewLogger instead
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		// Create a fallback logger if none has been initialized
		globalLogger = NewLoggerFromZap(createFallbackLogger())
	}
	return globalLogger
}

// SetGlobalLogger replaces the global logger (useful for testing)
func SetGlobalLogger(logger *Logger) {
	globalLogger = logger
}

// Initialize initializes the global logger with the specified configuration
func Initialize(serviceName, level, format string) error {
	config := DefaultConfig()

	// Set log level
	switch level {
	case "debug":
		config.Level = zapcore.DebugLevel
	case "info":
		config.Level = zapcore.InfoLevel
	case "warn":
		config.Level = zapcore.WarnLevel
	case "error":
		config.Level = zapcore.ErrorLevel
	default:
		config.Level = zapcore.InfoLevel
	}

	// Set development mode for console format
	if format == "console" {
		config.Development = true
	}

	zapLogger, err := NewLogger(config)
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}

	logger := NewLoggerFromZap(zapLogger)
	SetGlobalLogger(logger)
	return nil
}

// Global convenience functions for backwards compatibility
// Deprecated: Use dependency injection with NewLogger instead
func Info(msg string, fields ...zap.Field) {
	GetGlobalLogger().Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	GetGlobalLogger().Error(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetGlobalLogger().Warn(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
	GetGlobalLogger().Debug(msg, fields...)
}

func Sync() {
	if err := GetGlobalLogger().Sync(); err != nil {
		// Can't use logger to log this error since we're syncing
		// Write directly to stderr as a fallback
		_, _ = os.Stderr.WriteString("Failed to sync global logger: " + err.Error() + "\n")
	}
}
