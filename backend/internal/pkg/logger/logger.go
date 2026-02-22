// Package utils provides utility functions including logging.
// Logger supports console + file output, log rotation, levels, and field enrichment.
package logger

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

var (
	globalLogger atomic.Value
	globalSugar  atomic.Value
	initialized  atomic.Bool

	// 数据库日志写入相关
	logDBMu        sync.Mutex
	logDBEnabled   bool                 // 是否启用数据库写入
	logDBOnce      sync.Once            // 确保worker只启动一次
	logDBChan      chan *model.LogEntry // 日志写入缓冲通道
	logDBBuffer    = 1024               // 默认缓冲区大小
	logDBBatchSize = 100                // 默认批量写入大小
	logDBFlushTick = 1 * time.Second    // 默认批量刷新间隔
)

func init() {
	nop := zap.NewNop()
	globalLogger.Store(nop)
	globalSugar.Store(nop.Sugar())
}

// InitLogger 初始化全局日志器
// level: debug|info|warn|error（不区分大小写）
// dir: 日志文件目录，将创建 app.log 文件
func InitLogger(level string, dir string, rotate RotateConfig) error {
	parsed, err := parseLevel(level)
	if err != nil {
		return err
	}

	rotate = normalizeRotateConfig(rotate)
	logDir, logFile := ResolveLogFilePath(dir)
	if logDir == "" || logFile == "" {
		return errors.New("日志目录为空")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.CallerKey = "caller"
	encoderCfg.EncodeCaller = zapcore.ShortCallerEncoder
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder // 统一使用小写level

	// 控制台输出：更适合人类阅读的格式
	consoleCfg := zap.NewProductionEncoderConfig()
	consoleCfg.TimeKey = "time"
	consoleCfg.LevelKey = "level"
	consoleCfg.MessageKey = "message"
	consoleCfg.CallerKey = "caller"
	consoleCfg.FunctionKey = "func"
	consoleCfg.EncodeCaller = zapcore.ShortCallerEncoder
	consoleCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder // 带颜色的大写级别
	consoleCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// 同时输出到控制台和文件
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(consoleCfg),
			zapcore.AddSync(os.Stdout),
			parsed,
		),
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(&lumberjack.Logger{
				Filename:   logFile,
				MaxSize:    rotate.MaxSizeMB,
				MaxBackups: rotate.MaxBackups,
				MaxAge:     rotate.MaxAgeDays,
				Compress:   rotate.Compress,
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

	globalLogger.Store(l)
	globalSugar.Store(l.Sugar())
	initialized.Store(true)
	zap.ReplaceGlobals(l)

	return nil
}

// RotateConfig 日志分割与压缩配置
type RotateConfig struct {
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

func normalizeRotateConfig(cfg RotateConfig) RotateConfig {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 10
	}
	if cfg.MaxBackups < 0 {
		cfg.MaxBackups = 0
	}
	if cfg.MaxAgeDays < 0 {
		cfg.MaxAgeDays = 0
	}
	return cfg
}

// ResolveLogFilePath 解析日志路径，支持目录或完整文件路径
// - 传入目录：返回 <dir>/app.log
// - 传入 .log 文件路径：直接返回该文件
func ResolveLogFilePath(path string) (string, string) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", ""
	}
	if strings.EqualFold(filepath.Ext(trimmed), ".log") {
		return filepath.Dir(trimmed), trimmed
	}
	return trimmed, filepath.Join(trimmed, "app.log")
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
	if value := globalLogger.Load(); value != nil {
		if logger, ok := value.(*zap.Logger); ok && logger != nil {
			return logger
		}
	}
	return zap.NewNop()
}

// With 返回带有额外字段的子日志器
func With(fields ...zap.Field) *zap.Logger {
	return L().With(fields...)
}

// WithContext 从 context 中提取 request_id/trace_id/user_action 并注入日志字段
func WithContext(ctx context.Context) *zap.Logger {
	if ctx == nil {
		return L()
	}
	fields := []zap.Field{}
	if requestID, ok := ctx.Value("request_id").(string); ok && strings.TrimSpace(requestID) != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}
	if traceID, ok := ctx.Value("trace_id").(string); ok && strings.TrimSpace(traceID) != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}
	if userAction, ok := ctx.Value("user_action").(string); ok && strings.TrimSpace(userAction) != "" {
		fields = append(fields, zap.String("user_action", userAction))
	}
	if len(fields) == 0 {
		return L()
	}
	return L().With(fields...)
}

// WithModule 统一注入模块字段
func WithModule(module string) *zap.Logger {
	module = strings.TrimSpace(module)
	if module == "" {
		return L()
	}
	return L().With(zap.String("module", module))
}

// WithOperation 注入操作上下文字段
func WithOperation(module, action string, jobID uint) *zap.Logger {
	fields := []zap.Field{}
	if strings.TrimSpace(module) != "" {
		fields = append(fields, zap.String("module", strings.TrimSpace(module)))
	}
	if strings.TrimSpace(action) != "" {
		fields = append(fields, zap.String("user_action", strings.TrimSpace(action)))
	}
	if jobID > 0 {
		fields = append(fields, zap.Uint("job_id", jobID))
	}
	if len(fields) == 0 {
		return L()
	}
	return L().With(fields...)
}

// Debug 记录debug级别日志
func Debug(msg string, fields ...zap.Field) {
	L().Debug(msg, fields...)
}

// Info 记录info级别日志
func Info(msg string, fields ...zap.Field) {
	L().Info(msg, fields...)
}

// Warn 记录warning级别日志
func Warn(msg string, fields ...zap.Field) {
	L().Warn(msg, fields...)
}

// Error 记录error级别日志
func Error(msg string, fields ...zap.Field) {
	L().Error(msg, fields...)
}

// Debugf 记录debug级别格式化日志
func Debugf(format string, args ...any) {
	S().Debugf(format, args...)
}

// Infof 记录info级别格式化日志
func Infof(format string, args ...any) {
	S().Infof(format, args...)
}

// Warnf 记录warning级别格式化日志
func Warnf(format string, args ...any) {
	S().Warnf(format, args...)
}

// Errorf 记录error级别格式化日志
func Errorf(format string, args ...any) {
	S().Errorf(format, args...)
}

// LogInfo 兼容旧命名
func LogInfo(msg string, fields ...zap.Field) {
	Info(msg, fields...)
}

// LogWarn 兼容旧命名
func LogWarn(msg string, fields ...zap.Field) {
	Warn(msg, fields...)
}

// LogError 兼容旧命名
func LogError(msg string, fields ...zap.Field) {
	Error(msg, fields...)
}

// S 返回全局 SugaredLogger
func S() *zap.SugaredLogger {
	if value := globalSugar.Load(); value != nil {
		if sugar, ok := value.(*zap.SugaredLogger); ok && sugar != nil {
			return sugar
		}
	}
	return zap.NewNop().Sugar()
}

// SyncLogger 刷新缓冲的日志条目
// 可以安全地多次调用
func SyncLogger() error {
	logger := L()
	if err := logger.Sync(); err != nil {
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
func WriteLogToDB(db *gorm.DB, entry *model.LogEntry) {
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
		logDBChan = make(chan *model.LogEntry, logDBBuffer)
		go func() {
			buffer := make([]*model.LogEntry, 0, logDBBatchSize)
			ticker := time.NewTicker(logDBFlushTick)
			defer ticker.Stop()

			flush := func() {
				if len(buffer) == 0 {
					return
				}
				if err := db.Create(&buffer).Error; err != nil {
					logDBWarn("写入日志到数据库失败", err)
				}
				buffer = buffer[:0]
			}

			for {
				select {
				case e, ok := <-logDBChan:
					if !ok {
						flush()
						return
					}
					if e == nil {
						continue
					}
					buffer = append(buffer, e)
					if len(buffer) >= logDBBatchSize {
						flush()
					}
				case <-ticker.C:
					flush()
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
		logDBWarn("日志写入缓冲已满，丢弃日志", nil)
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

// TraceOperation 追踪操作耗时与结果
func TraceOperation(operationName string, fields ...zap.Field) func() {
	start := time.Now()
	Info("[Start] "+operationName, fields...)
	return func() {
		cost := time.Since(start)
		Info("[End] "+operationName, append(fields, zap.Duration("cost", cost))...)
	}
}

func logDBWarn(message string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "[logdb] %s: %v\n", message, err)
		return
	}
	fmt.Fprintf(os.Stderr, "[logdb] %s\n", message)
}
