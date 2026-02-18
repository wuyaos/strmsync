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

	"github.com/strmsync/strmsync/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

var (
	mu       sync.RWMutex
	instance *zap.Logger

	// 数据库日志写入相关
	logDBMu      sync.Mutex
	logDBEnabled bool               // 是否启用数据库写入
	logDBOnce    sync.Once          // 确保worker只启动一次
	logDBChan    chan *core.LogEntry // 日志写入缓冲通道
	logDBBuffer  = 1024             // 默认缓冲区大小
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
	encoderCfg.CallerKey = "caller"
	encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder // 统一使用小写level

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

	l := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

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

// SetLogToDBEnabled 配置是否启用日志写入数据库
// enabled: 是否启用数据库写入
// buffer: 缓冲区大小，0表示使用默认值
func SetLogToDBEnabled(enabled bool, buffer int) {
	logDBMu.Lock()
	defer logDBMu.Unlock()
	logDBEnabled = enabled
	if buffer > 0 {
		logDBBuffer = buffer
	}
}

// WriteLogToDB 异步写入日志到数据库（带缓冲）
// 采用非阻塞设计，当缓冲区满时丢弃日志而非阻塞主流程
// db: 数据库连接
// entry: 日志条目（Level会被标准化为小写）
func WriteLogToDB(db *gorm.DB, entry *core.LogEntry) {
	if db == nil || entry == nil {
		return
	}

	// 检查是否启用数据库写入
	logDBMu.Lock()
	enabled := logDBEnabled
	logDBMu.Unlock()
	if !enabled {
		return
	}

	// 标准化level为小写
	entry.Level = strings.ToLower(strings.TrimSpace(entry.Level))

	// 初始化异步写入worker（只执行一次）
	logDBOnce.Do(func() {
		logDBChan = make(chan *core.LogEntry, logDBBuffer)
		go func() {
			for e := range logDBChan {
				if e == nil {
					continue
				}
				if err := db.Create(e).Error; err != nil {
					// 写入失败时记录到文件日志，避免无限递归
					L().Warn("写入日志到数据库失败", zap.Error(err))
				}
			}
		}()
	})

	// 非阻塞写入，缓冲满时丢弃（避免内存无限增长）
	select {
	case logDBChan <- entry:
		// 成功写入缓冲
	default:
		// 缓冲已满，丢弃日志并记录警告
		L().Warn("日志写入缓冲已满，丢弃日志", zap.String("message", entry.Message))
	}
}

// ShutdownLogDBWriter 优雅关闭日志写入worker
// 等待所有缓冲的日志写入完成后关闭通道
func ShutdownLogDBWriter() {
	logDBMu.Lock()
	defer logDBMu.Unlock()

	if logDBChan != nil {
		close(logDBChan)
		logDBChan = nil
	}
}
