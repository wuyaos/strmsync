// Package config provides centralized configuration management for STRMSync.
// It loads YAML config files, allows environment variable overrides, validates
// required fields, and exposes a singleton accessor.
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// Config is the root configuration structure.
// It is intentionally explicit to keep configuration stable and auditable.
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
}

// ServerConfig defines HTTP server settings.
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// DatabaseConfig defines database settings.
type DatabaseConfig struct {
	Path string `mapstructure:"path"`
}

// LogConfig defines logging settings.
type LogConfig struct {
	Level string `mapstructure:"level"`
	Path  string `mapstructure:"path"`
}

// SecurityConfig defines security-related settings.
type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"`
}

var (
	once       sync.Once
	cachedCfg  *Config
	cachedErr  error
	defaultCfgPath = filepath.FromSlash("config/config.yaml")
)

// Get returns the singleton configuration instance, loading it on first call.
// It uses the default config path: config/config.yaml.
func Get() (*Config, error) {
	once.Do(func() {
		cachedCfg, cachedErr = LoadFromFile(defaultCfgPath)
	})
	return cachedCfg, cachedErr
}

// LoadFromFile loads configuration from a YAML file and validates it.
// It does not cache results, making it suitable for tests or custom paths.
func LoadFromFile(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("config path is empty")
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Environment overrides:
	// - Prefix: STRMSYNC
	// - Key mapping: server.port -> STRMSYNC_SERVER_PORT
	v.SetEnvPrefix("STRMSYNC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Validate performs structural and semantic checks.
// It returns a descriptive error for the first failure it encounters.
func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}

	// Server validation
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be 1-65535, got %d", cfg.Server.Port)
	}
	if strings.TrimSpace(cfg.Server.Host) == "" {
		return errors.New("server.host is required")
	}

	// Database validation
	if strings.TrimSpace(cfg.Database.Path) == "" {
		return errors.New("database.path is required")
	}

	// Log validation
	if strings.TrimSpace(cfg.Log.Path) == "" {
		return errors.New("log.path is required")
	}
	level := strings.ToLower(strings.TrimSpace(cfg.Log.Level))
	switch level {
	case "debug", "info", "warn", "error":
		// ok
	default:
		return fmt.Errorf("log.level must be one of debug|info|warn|error, got %q", cfg.Log.Level)
	}

	// Security validation
	if strings.TrimSpace(cfg.Security.EncryptionKey) == "" {
		return errors.New("security.encryption_key is required (set via config or STRMSYNC_SECURITY_ENCRYPTION_KEY)")
	}

	return nil
}
