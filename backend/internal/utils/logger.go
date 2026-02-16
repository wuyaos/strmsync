// Package utils provides utility functions including logging.
// Logger supports console + file output, log rotation, levels, and field enrichment.
package utils

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

// InitLogger 初始化全局日志器
// level: debug|info|warn|error（不区分大小写）
// dir: 日志文件目录，将创建 app.log 文件
func InitLogger(level string, dir string) error {
	parsed, err := parseLevel(level)
	if err != nil {
		return err
	}

	// 确保日志目录存在
	if strings.TrimSpace(dir) == "" {
		return errors.New("日志目录为空")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder

	// 同时输出到控制台和文件
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
				MaxAge:     30,  // 天
				Compress:   true,
			}),
			parsed,
		),
	)

	l := zap.New(core)

	mu.Lock()
	instance = l
	mu.Unlock()

	return nil
}

// parseLevel 解析并验证日志级别
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
		return zapcore.InfoLevel, fmt.Errorf("无效的日志级别: %q", level)
	}
}

// L 返回全局日志器。如果未初始化，返回no-op日志器
func L() *zap.Logger {
	mu.RLock()
	defer mu.RUnlock()
	if instance != nil {
		return instance
	}
	return zap.NewNop()
}

// With 返回带有额外字段的子日志器
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

// Debug 记录debug级别日志
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// LogInfo 记录info级别日志
func LogInfo(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// LogWarn 记录warning级别日志
func LogWarn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// LogError 记录error级别日志
func LogError(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// SyncLogger 刷新缓冲的日志条目
// 可以安全地多次调用
func SyncLogger() error {
	mu.RLock()
	defer mu.RUnlock()
	if instance == nil {
		return nil
	}
	if err := instance.Sync(); err != nil {
		// zap在某些平台上同步stdout可能返回错误；视为警告
		return fmt.Errorf("日志同步失败: %w", err)
	}
	return nil
}
