package config

import (
	"fmt"
	"os"
	"strings"
)

// Config 主配置结构
// Author: STRMSync Team
type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Log      LogConfig      `mapstructure:"log"`
	Security SecurityConfig `mapstructure:"security"`
}

// ServerConfig HTTP 服务器配置
type ServerConfig struct {
	Port string `mapstructure:"port"`
	Mode string `mapstructure:"mode"` // debug, release
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type string `mapstructure:"type"` // sqlite
	Path string `mapstructure:"path"` // 数据库文件路径
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `mapstructure:"level"`       // debug, info, warn, error
	OutputPath string `mapstructure:"output_path"` // 日志输出路径
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EncryptionKey string `mapstructure:"encryption_key"` // AES-256 密钥（32字节）
}

// LoadConfig 加载配置，使用默认值和环境变量
// Author: STRMSync Team
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "3000"),
			Mode: getEnv("GIN_MODE", "release"),
		},
		Database: DatabaseConfig{
			Type: "sqlite",
			Path: getEnv("DB_PATH", "/app/data/strmsync.db"),
		},
		Log: LogConfig{
			Level:      strings.ToLower(getEnv("LOG_LEVEL", "info")),
			OutputPath: getEnv("LOG_PATH", "/app/logs/strmsync.log"),
		},
		Security: SecurityConfig{
			EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
		},
	}

	// 验证加密密钥
	if cfg.Security.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required")
	}
	if len(cfg.Security.EncryptionKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 characters, got %d", len(cfg.Security.EncryptionKey))
	}

	return cfg, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Validate 验证配置的有效性
// Author: STRMSync Team
func (c *Config) Validate() error {
	// 验证端口
	if c.Server.Port == "" {
		return fmt.Errorf("server port cannot be empty")
	}

	// 验证日志级别
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Log.Level] {
		return fmt.Errorf("invalid log level: %s", c.Log.Level)
	}

	// 验证数据库路径
	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	return nil
}
