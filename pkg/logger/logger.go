// Package logger provides structured logging capabilities for the application.
package logger

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Fields is a type alias for a map of field names to values that can be logged.
// It's used for structured logging to add context to log messages.
type Fields map[string]interface{}

// Logger defines the interface for all logging operations.
// It provides methods for different log levels and structured logging capabilities.
type Logger interface {
	// Debug logs a message at debug level. Only shown when verbosity >= 1
	Debug(msg string)

	// Info logs a message at info level. Always shown.
	Info(msg string)

	// Warn logs a message at warn level. Always shown.
	Warn(msg string)

	// Error logs a message at error level. Always shown.
	Error(msg string)

	// Trace logs a message at trace level. Only shown when verbosity >= 2
	Trace(msg string)

	// WithFields returns a new Logger with the given fields added to its context.
	// Fields are included in all subsequent log messages until cleared.
	WithFields(fields Fields) Logger
}

// Config holds the configuration for creating a new logger instance.
type Config struct {
	// Verbosity determines the logging level:
	// 0: Info, Warn, Error (default)
	// 1: Debug + Level 0
	// 2: Trace + Level 1
	Verbosity int

	// Output specifies where logs should be written.
	// If nil, defaults to os.Stderr
	Output io.Writer
}

type logger struct {
	zap       *zap.Logger
	verbosity int
}

// NewLogger creates a new Logger instance with the given configuration.
// If no output is specified in the config, os.Stderr will be used.
//
// Example:
//
//	logger := NewLogger(Config{
//	    Verbosity: 1,
//	})
//
//	logger.WithFields(Fields{
//	    "component": "scanner",
//	}).Info("Scan started")
func NewLogger(config Config) Logger {
	// Default output to stderr if not specified
	if config.Output == nil {
		config.Output = os.Stderr
	}

	// Create encoder configuration
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(config.Output),
		getLogLevel(config.Verbosity),
	)

	// Create logger
	zapLogger := zap.New(core)

	return &logger{
		zap:       zapLogger,
		verbosity: config.Verbosity,
	}
}

func getLogLevel(verbosity int) zapcore.LevelEnabler {
	switch verbosity {
	case 0:
		return zapcore.InfoLevel
	case 1:
		return zapcore.DebugLevel
	default:
		return zapcore.DebugLevel
	}
}

// Implementation of Logger interface
func (l *logger) Debug(msg string) {
	l.zap.Debug(msg)
}

func (l *logger) Info(msg string) {
	l.zap.Info(msg)
}

func (l *logger) Warn(msg string) {
	l.zap.Warn(msg)
}

func (l *logger) Error(msg string) {
	l.zap.Error(msg)
}

func (l *logger) Trace(msg string) {
	if l.verbosity >= 2 {
		l.zap.Debug("TRACE: " + msg)
	}
}

func (l *logger) WithFields(fields Fields) Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}

	return &logger{
		zap:       l.zap.With(zapFields...),
		verbosity: l.verbosity,
	}
}
