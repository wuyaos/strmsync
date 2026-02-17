// Package config 提供STRMSync的集中配置管理
// 仅从环境变量加载配置（无配置文件）
// 所有配置使用 STRMSYNC_ 前缀，支持Docker部署
package core

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 根配置结构体
// 所有字段从环境变量填充
type Config struct {
	Server   ServerConfig   // 服务器配置
	Database DatabaseConfig // 数据库配置
	Log      LogConfig      // 日志配置
	Security SecurityConfig // 安全配置
	Scanner  ScannerConfig  // 扫描服务配置
	Notifier NotifierConfig // 通知服务配置
}

// ServerConfig HTTP服务器设置
type ServerConfig struct {
	Port int    // 端口号
	Host string // 监听地址
}

// DatabaseConfig 数据库设置
type DatabaseConfig struct {
	Path string // 数据库文件路径
}

// LogConfig 日志设置
type LogConfig struct {
	Level string // 日志级别
	Path  string // 日志目录
	ToDB  bool   // 是否写入数据库
}

// SecurityConfig 安全相关设置
type SecurityConfig struct {
	EncryptionKey string // 加密密钥
}

// ScannerConfig 扫描服务设置
type ScannerConfig struct {
	Concurrency int // 并发worker数量
	BatchSize   int // 批量写入数据库的批次大小
}

// NotifierConfig 通知服务设置
type NotifierConfig struct {
	Enabled         bool   // 是否启用通知
	Provider        string // Provider类型：emby/jellyfin
	BaseURL         string // 服务器地址
	Token           string // API Token
	TimeoutSeconds  int    // 请求超时（秒）
	RetryMax        int    // 最大重试次数
	RetryBaseMs     int    // 重试基础延迟（毫秒）
	DebounceSeconds int    // 去抖时间（秒）
	Scope           string // 通知范围：global/source
}

// LoadFromEnv 从环境变量加载配置
// 环境变量示例：PORT, LOG_LEVEL, DB_PATH, ENCRYPTION_KEY
func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnvInt("PORT", 6754),
			Host: getEnv("HOST", "0.0.0.0"),
		},
		Database: DatabaseConfig{
			Path: getEnv("DB_PATH", "data.db"),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
			Path:  getEnv("LOG_PATH", "logs"),
			ToDB:  getEnvBool("LOG_TO_DB", false),
		},
		Security: SecurityConfig{
			EncryptionKey: getEnv("ENCRYPTION_KEY", ""),
		},
		Scanner: ScannerConfig{
			Concurrency: getEnvInt("SCANNER_CONCURRENCY", 20),
			BatchSize:   getEnvInt("SCANNER_BATCH_SIZE", 500),
		},
		Notifier: NotifierConfig{
			Enabled:         getEnvBool("NOTIFIER_ENABLED", false),
			Provider:        getEnv("NOTIFIER_PROVIDER", ""),
			BaseURL:         getEnv("NOTIFIER_BASE_URL", ""),
			Token:           getEnv("NOTIFIER_TOKEN", ""),
			TimeoutSeconds:  getEnvInt("NOTIFIER_TIMEOUT", 10),
			RetryMax:        getEnvInt("NOTIFIER_RETRY_MAX", 3),
			RetryBaseMs:     getEnvInt("NOTIFIER_RETRY_BASE_MS", 1000),
			DebounceSeconds: getEnvInt("NOTIFIER_DEBOUNCE", 5),
			Scope:           getEnv("NOTIFIER_SCOPE", "global"),
		},
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key string, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt 获取整数类型环境变量，如果不存在或无效则返回默认值
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool 获取布尔类型环境变量，如果不存在或无效则返回默认值
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// Validate 执行结构和语义检查
// 遇到第一个失败时返回描述性错误
func Validate(cfg *Config) error {
	if cfg == nil {
		return errors.New("配置为空")
	}

	// 服务器验证
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("服务器端口必须在1-65535之间，当前值: %d", cfg.Server.Port)
	}
	if strings.TrimSpace(cfg.Server.Host) == "" {
		return errors.New("服务器地址不能为空")
	}

	// 数据库验证
	if strings.TrimSpace(cfg.Database.Path) == "" {
		return errors.New("数据库路径不能为空")
	}

	// 日志验证
	if strings.TrimSpace(cfg.Log.Path) == "" {
		return errors.New("日志路径不能为空")
	}
	level := strings.ToLower(strings.TrimSpace(cfg.Log.Level))
	switch level {
	case "debug", "info", "warn", "error":
		// 有效级别
	default:
		return fmt.Errorf("日志级别必须是 debug|info|warn|error 之一，当前值: %q", cfg.Log.Level)
	}

	// 安全验证
	if strings.TrimSpace(cfg.Security.EncryptionKey) == "" {
		return errors.New("加密密钥不能为空（通过环境变量 ENCRYPTION_KEY 设置）")
	}

	return nil
}
