// Package database 提供SQLite连接设置和迁移工具
package db

import (
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

// InitWithConfig 打开SQLite数据库，配置连接池，并运行迁移
// 允许传入已加载/验证的日志配置，避免重复读取环境变量
// dbPath 是SQLite文件路径，例如："data.db"
// logCfg 是必填的日志配置
func InitWithConfig(dbPath string, logCfg *LogConfig) error {
	if strings.TrimSpace(dbPath) == "" {
		return errors.New("数据库路径为空")
	}
	if logCfg == nil {
		return errors.New("日志配置为空")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	// 配置 GORM Logger：适配 zap，并忽略 ErrRecordNotFound
	// 读取 SQL 日志配置（仅在初始化时读取一次，避免高频 SQL 时的性能开销）
	resolved := *logCfg
	if resolved.SQLSlowMs < 0 {
		resolved.SQLSlowMs = 0
	}
	gormLogger := logger.NewGormLogger(logger.With(zap.String("component", "gorm"))).
		WithSlowThreshold(time.Duration(resolved.SQLSlowMs) * time.Millisecond)
	var gormLog gormlogger.Interface = gormLogger
	if resolved.SQL {
		gormLog = gormLog.LogMode(gormlogger.Info)
	} else {
		gormLog = gormLog.LogMode(gormlogger.Warn)
	}

	gormConfig := &gorm.Config{
		Logger: gormLog,
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
		model.TaskRunEvent{},
		model.LogEntry{},
		model.Setting{},
	); err != nil {
		// 迁移失败时给出友好提示，可能是数据重复导致
		return fmt.Errorf("自动迁移失败: %w（如遇到唯一约束错误，请检查jobs表是否有重复的name字段）", err)
	}
	if err := backfillJobRemoteRoot(conn); err != nil {
		return fmt.Errorf("回填远程根目录失败: %w", err)
	}
	if err := dropJobRemotePathColumn(conn); err != nil {
		return fmt.Errorf("清理旧远程路径列失败: %w", err)
	}

	mu.Lock()
	dbInst = conn
	mu.Unlock()

	return nil
}

func backfillJobRemoteRoot(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}

	type tableColumn struct {
		Name string `gorm:"column:name"`
	}
	var columns []tableColumn
	if err := db.Raw("PRAGMA table_info(jobs)").Scan(&columns).Error; err != nil {
		return fmt.Errorf("query jobs table info: %w", err)
	}

	hasRemotePath := false
	hasRemoteRoot := false
	for _, col := range columns {
		switch strings.ToLower(strings.TrimSpace(col.Name)) {
		case "remote_path":
			hasRemotePath = true
		case "remote_root":
			hasRemoteRoot = true
		}
	}
	if !hasRemotePath || !hasRemoteRoot {
		return nil
	}

	res := db.Exec(`UPDATE jobs
SET remote_root = remote_path
WHERE (remote_root IS NULL OR remote_root = '')
  AND remote_path IS NOT NULL
  AND remote_path <> ''`)
	if res.Error != nil {
		return fmt.Errorf("update jobs remote_root: %w", res.Error)
	}
	return nil
}

func dropJobRemotePathColumn(db *gorm.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}

	type tableColumn struct {
		Name string `gorm:"column:name"`
	}
	var columns []tableColumn
	if err := db.Raw("PRAGMA table_info(jobs)").Scan(&columns).Error; err != nil {
		return fmt.Errorf("query jobs table info: %w", err)
	}

	hasRemotePath := false
	for _, col := range columns {
		if strings.EqualFold(strings.TrimSpace(col.Name), "remote_path") {
			hasRemotePath = true
			break
		}
	}
	if !hasRemotePath {
		return nil
	}

	if err := db.Exec("ALTER TABLE jobs DROP COLUMN remote_path").Error; err != nil {
		return fmt.Errorf("drop jobs.remote_path: %w", err)
	}
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

// customLogger 已由 zap 适配器替代
