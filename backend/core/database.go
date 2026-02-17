// Package database 提供SQLite连接设置和迁移工具
package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	mu     sync.RWMutex
	dbInst *gorm.DB
)

// Init 打开SQLite数据库，配置连接池，并运行迁移
// dbPath 是SQLite文件路径，例如："data/strmsync.db"
func Init(dbPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return errors.New("数据库路径为空")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建数据库目录失败: %w", err)
	}

	conn, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
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
	if conn.Migrator().HasTable(&Job{}) {
		if err := checkJobNameDuplicates(conn); err != nil {
			return fmt.Errorf("数据完整性检查失败: %w", err)
		}
	}

	// 自动迁移所有表
	if err := conn.AutoMigrate(
		&DataServer{},
		&MediaServer{},
		&Job{},
		&TaskRun{},
		&LogEntry{},
		&Setting{},
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
	if err := db.Model(&Job{}).
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
