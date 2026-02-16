// Package database provides SQLite connection setup and migration utilities.
package database

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

// Init opens the SQLite database, configures the connection pool, and runs migrations.
// dbPath is the SQLite file path, e.g. "data/strmsync.db".
func Init(dbPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return errors.New("database path is empty")
	}

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	conn, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}

	// Connection pool configuration for SQLite (safe defaults).
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	if err := conn.AutoMigrate(
		&Source{},
		&File{},
		&MetadataFile{},
		&Task{},
		&Setting{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	mu.Lock()
	dbInst = conn
	mu.Unlock()

	return nil
}

// GetDB returns the global *gorm.DB instance.
func GetDB() (*gorm.DB, error) {
	mu.RLock()
	defer mu.RUnlock()
	if dbInst == nil {
		return nil, errors.New("database not initialized")
	}
	return dbInst, nil
}

// Close gracefully closes the underlying database connection.
func Close() error {
	mu.RLock()
	defer mu.RUnlock()
	if dbInst == nil {
		return nil
	}
	sqlDB, err := dbInst.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}
	return nil
}
