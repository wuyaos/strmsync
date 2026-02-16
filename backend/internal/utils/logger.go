// Package logger provides a global, structured logger based on zap.
// It supports console + file output, log rotation, levels, and field enrichment.
package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	mu       sync.RWMutex
	instance *zap.Logger
)

// Init initializes the global logger.
// level: debug|info|warn|error (case-insensitive).
// dir: directory for log files; a file named "app.log" will be created.
func Init(level string, dir string) error {
	cfg, err := buildConfig(level, dir)
	if err != nil {
		return err
	}

	l, err := cfg.Build()
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}

	mu.Lock()
	instance = l
	mu.Unlock()

	return nil
}

// buildConfig creates a zap.Config with sane defaults for production usage.
func buildConfig(level string, dir string) (zap.Config, error) {
	parsed, err := parseLevel(level)
	if err != nil {
		return zap.Config{}, err
	}

	// Ensure log directory exists when file output is requested.
	if strings.TrimSpace(dir) == "" {
		return zap.Config{}, errors.New("log directory is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return zap.Config{}, fmt.Errorf("create log directory: %w", err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	// Log to console and rotating file.
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			parsed,
		),
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(&lumberjack.Logger{
				Filename:   filepath.Join(dir, "app.log"),
				MaxSize:    100, // MB
				MaxBackups: 7,
				MaxAge:     30,  // days
				Compress:   true,
			}),
			parsed,
		),
	)

	return zap.Config{
		Level:       zap.NewAtomicLevelAt(parsed),
		Development: false,
		Sampling:    nil,
		Encoding:    "json",
		EncoderConfig: encoderCfg,
		OutputPaths:      []string{},
		ErrorOutputPaths: []string{},
		InitialFields:    map[string]any{},
		DisableCaller:    false,
		DisableStacktrace: false,
	}, nil
}

// parseLevel normalizes and validates the log level.
func parseLevel(level string) (zapcore.Level, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return zapcore.DebugLevel, nil
	case "info":
		return zapcore.InfoLevel, nil
	case "warn", "warning":
		return zapcore.WarnLevel, nil
	case "error":
		return zapcore.ErrorLevel, nil
	default:
		return zapcore.InfoLevel, fmt.Errorf("invalid log level: %q", level)
	}
}

// L returns the global logger. It falls back to a no-op logger if not initialized.
func L() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if instance != nil {
		return instance
	}
	return zap.NewNop()
}

// With returns a child logger with additional fields.
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

// Debug logs a debug message with optional fields.
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// Info logs an info message with optional fields.
func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// Warn logs a warning message with optional fields.
func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// Error logs an error message with optional fields.
func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// Sync flushes any buffered log entries.
// It is safe to call multiple times.
func Sync() error {
	mu.RLock()
	defer mu.RUnlock()
	if instance == nil {
		return nil
	}
	if err := instance.Sync(); err != nil {
		// zap may return an error on stdout sync on some platforms; treat as warning.
		return fmt.Errorf("logger sync: %w", err)
	}
	return nil
}
