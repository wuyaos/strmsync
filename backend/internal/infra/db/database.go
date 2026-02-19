// Package database 提供SQLite连接设置和迁移工具
package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var (
	mu     sync.RWMutex
	dbInst *gorm.DB
)

// resolveLogConfig 解析 SQL 日志配置
// 优先使用传入的配置（来自已验证的 cfg），否则回退到环境变量读取
func resolveLogConfig(logCfg *LogConfig) LogConfig {
	if logCfg != nil {
		return *logCfg
	}
	// 降级逻辑：直接读取环境变量（用于向后兼容）
	return LogConfig{
		SQL:       getEnvBool("LOG_SQL", false),
		SQLSlowMs: getEnvInt("LOG_SQL_SLOW_MS", 0),
	}
}

// Init 打开SQLite数据库，配置连接池，并运行迁移
// dbPath 是SQLite文件路径，例如："data/strmsync.db"
// 保持向后兼容：内部调用 InitWithConfig
func Init(dbPath string) error {
	return InitWithConfig(dbPath, nil)
}

// InitWithConfig 打开SQLite数据库，配置连接池，并运行迁移
// 允许传入已加载/验证的日志配置，避免重复读取环境变量
// dbPath 是SQLite文件路径，例如："data/strmsync.db"
// logCfg 是可选的日志配置，nil 时回退到环境变量读取
func InitWithConfig(dbPath string, logCfg *LogConfig) error {
	if strings.TrimSpace(dbPath) == "" {
		return errors.New("数据库路径为空")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 配置 GORM Logger：忽略 ErrRecordNotFound 以避免日志污染
	// 读取 SQL 日志配置（仅在初始化时读取一次，避免高频 SQL 时的性能开销）
	resolved := resolveLogConfig(logCfg)
	// 防御式处理：避免负值导致异常（配置已验证的场景不会触发）
	if resolved.SQLSlowMs < 0 {
		resolved.SQLSlowMs = 0
	}

	gormConfig := &gorm.Config{
		Logger: &customLogger{
			logSQL:       resolved.SQL,
			sqlSlowMs:    resolved.SQLSlowMs,
			sqlSlowThres: time.Duration(resolved.SQLSlowMs) * time.Millisecond,
		},
	}

	conn, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return fmt.Errorf("打开SQLite失败: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库失败: %w", err)
	}

	// SQLite连接池配置（安全默认值）
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	// 迁移前检查：如果Job表已存在，检查是否有重复名称
	// 这样可以在AutoMigrate尝试创建uniqueIndex之前给出清晰的错误提示
	if conn.Migrator().HasTable(model.Job{}) {
		if err := checkJobNameDuplicates(conn); err != nil {
			return fmt.Errorf("数据完整性检查失败: %w", err)
		}
	}

	// 自动迁移所有表
	if err := conn.AutoMigrate(
		model.DataServer{},
		model.MediaServer{},
		model.Job{},
		model.TaskRun{},
		model.LogEntry{},
		model.Setting{},
	); err != nil {
		// 迁移失败时给出友好提示，可能是数据重复导致
		return fmt.Errorf("自动迁移失败: %w（如遇到唯一约束错误，请检查jobs表是否有重复的name字段）", err)
	}

	mu.Lock()
	dbInst = conn
	mu.Unlock()

	return nil
}

// checkJobNameDuplicates 检查Job表中是否存在重复的name字段
// 如果存在重复，返回错误并提示需要手动修复
func checkJobNameDuplicates(db *gorm.DB) error {
	var duplicates []struct {
		Name  string
		Count int64
	}

	// 查询所有重复的name
	if err := db.Model(model.Job{}).
		Select("name, COUNT(*) as count").
		Group("name").
		Having("count > 1").
		Find(&duplicates).Error; err != nil {
		return fmt.Errorf("检查重复名称失败: %w", err)
	}

	if len(duplicates) > 0 {
		// 构建错误信息
		var names []string
		for _, dup := range duplicates {
			names = append(names, fmt.Sprintf("'%s'(%d次)", dup.Name, dup.Count))
		}
		return fmt.Errorf("检测到重复的任务名称: %s。请手动修复后再启动系统", strings.Join(names, ", "))
	}

	return nil
}

// GetDB 返回全局*gorm.DB实例
func GetDB() (*gorm.DB, error) {
	mu.RLock()
	defer mu.RUnlock()
	if dbInst == nil {
		return nil, errors.New("数据库未初始化")
	}
	return dbInst, nil
}

// Close 优雅关闭底层数据库连接
func Close() error {
	mu.RLock()
	defer mu.RUnlock()
	if dbInst == nil {
		return nil
	}
	sqlDB, err := dbInst.DB()
	if err != nil {
		return fmt.Errorf("获取底层数据库失败: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("关闭数据库失败: %w", err)
	}
	return nil
}

// customLogger 自定义 GORM Logger，过滤掉 ErrRecordNotFound 等正常情况的日志
// customLogger 实现 GORM Logger 接口，过滤 ErrRecordNotFound 并支持独立 SQL 日志配置
type customLogger struct {
	logSQL       bool // 是否启用 SQL 日志
	sqlSlowMs    int  // 慢查询阈值（毫秒）
	sqlSlowThres time.Duration // 慢查询阈值（预计算）
}

// LogMode 实现 gormlogger.Interface
func (l *customLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	return l
}

// Info 实现 gormlogger.Interface
func (l *customLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	logger.LogInfo(fmt.Sprintf(msg, data...))
}

// Warn 实现 gormlogger.Interface
func (l *customLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	logger.LogWarn(fmt.Sprintf(msg, data...))
}

// Error 实现 gormlogger.Interface，但过滤掉 ErrRecordNotFound
func (l *customLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	// 忽略 record not found 错误（这是正常的查询结果）
	if strings.Contains(msg, "record not found") {
		return
	}
	logger.LogError(fmt.Sprintf(msg, data...))
}

// Trace 实现 gormlogger.Interface，记录 SQL 执行详情
func (l *customLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	elapsed := time.Since(begin)

	// 过滤 ErrRecordNotFound，有错误时才调用 fc()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		sql, rows := fc()
		logger.LogError("SQL执行失败",
			zap.Error(err),
			zap.String("sql", sql),
			zap.Int64("rows", rows),
			zap.Duration("elapsed", elapsed))
		return
	}

	// SQL 日志独立开关：默认关闭，可按慢查询阈值筛选
	if !l.logSQL {
		return
	}

	// 慢查询阈值过滤
	if l.sqlSlowMs > 0 && elapsed < l.sqlSlowThres {
		return
	}

	// 只有需要记录时才调用 fc()，避免不必要的 SQL 字符串构建
	sql, rows := fc()
	logger.Debug("SQL执行",
		zap.String("sql", sql),
		zap.Int64("rows", rows),
		zap.Duration("elapsed", elapsed))
}
