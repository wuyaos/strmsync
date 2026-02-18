package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------
// Backfill 测试
// ---------------------

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	// 使用内存SQLite数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(&model.DataServer{}, &model.MediaServer{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return db
}

func TestBackfillServerUIDs_EmptyDatabase(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	stats, err := BackfillServerUIDs(context.Background(), db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	if stats.DataServersTotal != 0 {
		t.Errorf("DataServersTotal = %d, want 0", stats.DataServersTotal)
	}
	if stats.MediaServersTotal != 0 {
		t.Errorf("MediaServersTotal = %d, want 0", stats.MediaServersTotal)
	}
}

func TestBackfillServerUIDs_WithEmptyUIDs(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	// 创建没有UID的DataServer记录（绕过BeforeCreate hook）
	// 注意：不指定uid字段，让它保持NULL（uniqueIndex允许多个NULL）
	db.Exec(`INSERT INTO data_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-server", "clouddrive2", "192.168.1.100", 19798, "key", true, "{}", time.Now(), time.Now())

	// 创建没有UID的MediaServer记录
	db.Exec(`INSERT INTO media_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-emby", "emby", "192.168.1.200", 8096, "emby-key", true, "{}", time.Now(), time.Now())

	// 执行回填
	stats, err := BackfillServerUIDs(context.Background(), db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	// 验证统计信息
	if stats.DataServersTotal != 1 {
		t.Errorf("DataServersTotal = %d, want 1", stats.DataServersTotal)
	}
	if stats.DataServersUpdated != 1 {
		t.Errorf("DataServersUpdated = %d, want 1", stats.DataServersUpdated)
	}
	if stats.MediaServersTotal != 1 {
		t.Errorf("MediaServersTotal = %d, want 1", stats.MediaServersTotal)
	}
	if stats.MediaServersUpdated != 1 {
		t.Errorf("MediaServersUpdated = %d, want 1", stats.MediaServersUpdated)
	}

	// 验证UID已生成
	var dataServer model.DataServer
	if err := db.First(&dataServer, "name = ?", "test-server").Error; err != nil {
		t.Fatalf("failed to query data server: %v", err)
	}
	if dataServer.UID == "" {
		t.Error("DataServer UID not backfilled")
	}
	if len(dataServer.UID) != 64 {
		t.Errorf("DataServer UID length = %d, want 64", len(dataServer.UID))
	}

	var mediaServer model.MediaServer
	if err := db.First(&mediaServer, "name = ?", "test-emby").Error; err != nil {
		t.Fatalf("failed to query media server: %v", err)
	}
	if mediaServer.UID == "" {
		t.Error("MediaServer UID not backfilled")
	}
	if len(mediaServer.UID) != 64 {
		t.Errorf("MediaServer UID length = %d, want 64", len(mediaServer.UID))
	}
}

func TestBackfillServerUIDs_WithExistingUIDs(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	// 创建已有UID的记录（使用BeforeCreate生成UID）
	dataServer := &model.DataServer{
		Name:    "test-server",
		Type:    "clouddrive2",
		Host:    "192.168.1.100",
		Port:    19798,
		APIKey:  "key",
		Enabled: true,
		Options: "{}",
	}
	if err := db.Create(dataServer).Error; err != nil {
		t.Fatalf("failed to create data server: %v", err)
	}

	originalUID := dataServer.UID

	// 执行回填
	stats, err := BackfillServerUIDs(context.Background(), db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	// 已有UID的记录不应被处理
	if stats.DataServersTotal != 0 {
		t.Errorf("DataServersTotal = %d, want 0 (existing UIDs should not be counted)", stats.DataServersTotal)
	}

	// 验证UID未被修改
	var checkServer model.DataServer
	if err := db.First(&checkServer, dataServer.ID).Error; err != nil {
		t.Fatalf("failed to query data server: %v", err)
	}
	if checkServer.UID != originalUID {
		t.Errorf("UID was modified: original=%s, current=%s", originalUID, checkServer.UID)
	}
}

func TestBackfillServerUIDs_Conflict(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	// 创建第一个服务器（有UID）
	dataServer1 := &model.DataServer{
		Name:    "server1",
		Type:    "clouddrive2",
		Host:    "192.168.1.100",
		Port:    19798,
		APIKey:  "key",
		Enabled: true,
		Options: "{}",
	}
	if err := db.Create(dataServer1).Error; err != nil {
		t.Fatalf("failed to create data server 1: %v", err)
	}

	conflictUID := dataServer1.UID

	// 创建第二个服务器（相同连接信息，但没有UID）
	db.Exec(`INSERT INTO data_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"server2", "clouddrive2", "192.168.1.100", 19798, "key", true, "{}", time.Now(), time.Now())

	// 执行回填
	stats, err := BackfillServerUIDs(context.Background(), db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	// 应检测到冲突
	if stats.DataServersConflict != 1 {
		t.Errorf("DataServersConflict = %d, want 1", stats.DataServersConflict)
	}
	if len(stats.DataServersConflicts) != 1 {
		t.Errorf("len(DataServersConflicts) = %d, want 1", len(stats.DataServersConflicts))
	}

	// 第二个服务器的UID应保持为空（跳过更新）
	var server2 model.DataServer
	if err := db.Where("name = ?", "server2").First(&server2).Error; err != nil {
		t.Fatalf("failed to query server2: %v", err)
	}
	if server2.UID != "" {
		t.Errorf("conflicting server UID should remain empty, got %s", server2.UID)
	}

	// 验证冲突信息
	if stats.DataServersConflicts[0].Table != "data_servers" {
		t.Errorf("conflict table = %s, want data_servers", stats.DataServersConflicts[0].Table)
	}
	if stats.DataServersConflicts[0].UID != conflictUID {
		t.Errorf("conflict UID = %s, want %s", stats.DataServersConflicts[0].UID, conflictUID)
	}
}

func TestBackfillServerUIDs_ContextCancellation(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	// 创建多个没有UID的记录
	for i := 0; i < 5; i++ {
		db.Exec(`INSERT INTO data_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("server%d", i), "clouddrive2", fmt.Sprintf("192.168.1.%d", 100+i), 19798, "key", true, "{}", time.Now(), time.Now())
	}

	// 创建可取消的context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	// 执行回填（应该提前中断）
	stats, err := BackfillServerUIDs(ctx, db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	// 不应处理完所有记录（因为context已取消）
	if stats.DataServersTotal == 5 && stats.DataServersUpdated == 5 {
		t.Error("backfill should have been interrupted by context cancellation")
	}
}

func TestBackfillServerUIDs_NilDB(t *testing.T) {
	logger := zap.NewNop()

	_, err := BackfillServerUIDs(context.Background(), nil, logger)
	if err == nil {
		t.Error("BackfillServerUIDs(nil db) should return error")
	}
}

func TestBackfillServerUIDs_Stats(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	// 创建2个需要回填的DataServer
	for i := 0; i < 2; i++ {
		db.Exec(`INSERT INTO data_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("data%d", i), "clouddrive2", fmt.Sprintf("192.168.1.%d", 100+i), 19798, "key", true, "{}", time.Now(), time.Now())
	}

	// 创建3个需要回填的MediaServer
	for i := 0; i < 3; i++ {
		db.Exec(`INSERT INTO media_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			fmt.Sprintf("media%d", i), "emby", fmt.Sprintf("192.168.2.%d", 100+i), 8096, "key", true, "{}", time.Now(), time.Now())
	}

	stats, err := BackfillServerUIDs(context.Background(), db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	// 验证统计信息
	if stats.DataServersTotal != 2 {
		t.Errorf("DataServersTotal = %d, want 2", stats.DataServersTotal)
	}
	if stats.DataServersUpdated != 2 {
		t.Errorf("DataServersUpdated = %d, want 2", stats.DataServersUpdated)
	}
	if stats.MediaServersTotal != 3 {
		t.Errorf("MediaServersTotal = %d, want 3", stats.MediaServersTotal)
	}
	if stats.MediaServersUpdated != 3 {
		t.Errorf("MediaServersUpdated = %d, want 3", stats.MediaServersUpdated)
	}

	// 验证耗时记录
	if stats.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
	if stats.Elapsed == 0 {
		t.Error("Elapsed is zero")
	}
}

// ---------------------
// 冲突检测测试
// ---------------------

func TestBackfillConflictDetection(t *testing.T) {
	db := setupTestDB(t)
	logger := zap.NewNop()

	// 场景：两个不同名称的服务器，但连接信息完全相同
	// server1 有UID（已创建）
	// server2 没有UID（需要回填，但会与server1冲突）

	server1 := &model.DataServer{
		Name:    "primary-server",
		Type:    "clouddrive2",
		Host:    "192.168.1.100",
		Port:    19798,
		APIKey:  "shared-key",
		Enabled: true,
		Options: "{}",
	}
	if err := db.Create(server1).Error; err != nil {
		t.Fatalf("failed to create server1: %v", err)
	}

	// 手动插入server2（绕过BeforeCreate）
	db.Exec(`INSERT INTO data_servers (name, type, host, port, api_key, enabled, options, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"secondary-server", "clouddrive2", "192.168.1.100", 19798, "shared-key", true, "{}", time.Now(), time.Now())

	stats, err := BackfillServerUIDs(context.Background(), db, logger)
	if err != nil {
		t.Fatalf("BackfillServerUIDs() error = %v", err)
	}

	// 应检测到冲突
	if stats.DataServersConflict != 1 {
		t.Errorf("expected 1 conflict, got %d", stats.DataServersConflict)
	}

	// secondary-server的UID应保持为空
	var server2 model.DataServer
	db.Where("name = ?", "secondary-server").First(&server2)
	if server2.UID != "" {
		t.Errorf("conflicting server should have empty UID, got %s", server2.UID)
	}

	// primary-server的UID应保持不变
	var server1Check model.DataServer
	db.First(&server1Check, server1.ID)
	if server1Check.UID != server1.UID {
		t.Errorf("original server UID was modified")
	}
}
