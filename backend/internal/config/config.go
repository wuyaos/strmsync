// Package config provides centralized configuration management for STRMSync.
// It loads configuration from environment variables only (no config files).
// All settings use STRMSYNC_ prefix and support Docker deployment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the root configuration structure.
// All fields are populated from environment variables.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
	Security SecurityConfig
}

// ServerConfig defines HTTP server settings.
type ServerConfig struct {
	Port int
	Host string
}

// DatabaseConfig defines database settings.
type DatabaseConfig struct {
	Path string
}

// LogConfig defines logging settings.
type LogConfig struct {
	Level string
	Path  string
}

// SecurityConfig defines security-related settings.
type SecurityConfig struct {
	EncryptionKey string
}

// LoadFromEnv loads configuration from environment variables.
// Environment variables use STRMSYNC_ prefix.
// Example: STRMSYNC_SERVER_PORT, STRMSYNC_LOG_LEVEL
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnvInt("PORT", 3000),
			Host: getEnv("HOST", "0.0.0.0"),
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "data/strmsync.db"),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
			Path:  getEnv("LOG_PATH", "logs"),
		},
		Security: SecurityConfig{
			EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
		},
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getEnv gets an environment variable with a default value.
func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Validate performs structural and semantic checks.
// It returns a descriptive error for the first failure it encounters.
func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	// Server validation
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be 1-65535, got %d", cfg.Server.Port)
	}
	if strings.TrimSpace(cfg.Server.Host) == "" {
		return errors.New("server host is required")
	}

	// Database validation
	if strings.TrimSpace(cfg.Database.Path) == "" {
		return errors.New("database path is required")
	}

	// Log validation
	if strings.TrimSpace(cfg.Log.Path) == "" {
		return errors.New("log path is required")
	}
	level := strings.ToLower(strings.TrimSpace(cfg.Log.Level))
	switch level {
	case "debug", "info", "warn", "error":
		// ok
	default:
		return fmt.Errorf("log level must be one of debug|info|warn|error, got %q", cfg.Log.Level)
	}

	// Security validation
	if strings.TrimSpace(cfg.Security.EncryptionKey) == "" {
		return errors.New("encryption key is required (set via ENCRYPTION_KEY environment variable)")
	}

	return nil
}
