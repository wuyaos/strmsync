package database

import (
	"os"
	"testing"
	"time"
)

// TestInitDB 测试数据库初始化
func TestInitDB(t *testing.T) {
	// 使用临时数据库文件
	dbPath := "/tmp/test_strmsync.db"
	defer os.Remove(dbPath)

	err := InitDB(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	if DB == nil {
		t.Fatal("Database instance is nil")
	}

	// 测试自动迁移
	err = AutoMigrate()
	if err != nil {
		t.Fatalf("Failed to auto migrate: %v", err)
	}

	// 测试健康检查
	err = HealthCheck()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	// 清理
	CloseDB()
}

// TestSourceRepository 测试数据源仓储
func TestSourceRepository(t *testing.T) {
	// 初始化测试数据库
	dbPath := "/tmp/test_source.db"
	defer os.Remove(dbPath)

	InitDB(dbPath, "info")
	AutoMigrate()
	defer CloseDB()

	repo := NewSourceRepository()

	// 测试创建
	source := &Source{
		Name:    "test-source",
		Type:    "local",
		Enabled: true,
		Config:  `{"path": "/test"}`,
	}

	err := repo.Create(source)
	if err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	if source.ID == 0 {
		t.Fatal("Source ID should not be 0")
	}

	// 测试查询
	found, err := repo.GetByID(source.ID)
	if err != nil {
		t.Fatalf("Failed to get source: %v", err)
	}

	if found.Name != "test-source" {
		t.Errorf("Expected name 'test-source', got '%s'", found.Name)
	}

	// 测试更新
	found.Name = "updated-source"
	err = repo.Update(found)
	if err != nil {
		t.Fatalf("Failed to update source: %v", err)
	}

	// 测试获取所有
	sources, err := repo.GetAll()
	if err != nil {
		t.Fatalf("Failed to get all sources: %v", err)
	}

	if len(sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(sources))
	}

	// 测试删除
	err = repo.Delete(source.ID)
	if err != nil {
		t.Fatalf("Failed to delete source: %v", err)
	}
}

// TestFileRepository 测试文件仓储
func TestFileRepository(t *testing.T) {
	// 初始化测试数据库
	dbPath := "/tmp/test_file.db"
	defer os.Remove(dbPath)

	InitDB(dbPath, "info")
	AutoMigrate()
	defer CloseDB()

	// 先创建数据源
	sourceRepo := NewSourceRepository()
	source := &Source{
		Name:    "test-source",
		Type:    "local",
		Enabled: true,
	}
	sourceRepo.Create(source)

	// 测试文件仓储
	fileRepo := NewFileRepository()

	file := &File{
		SourceID:     source.ID,
		RelativePath: "/movies/test.mkv",
		FileName:     "test.mkv",
		FileSize:     1024000,
		ModTime:      time.Now(),
		FastHash:     "abc123",
	}

	err := fileRepo.Create(file)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// 测试统计
	count, err := fileRepo.CountBySourceID(source.ID)
	if err != nil {
		t.Fatalf("Failed to count files: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 file, got %d", count)
	}
}

// TestTaskRepository 测试任务仓储
func TestTaskRepository(t *testing.T) {
	// 初始化测试数据库
	dbPath := "/tmp/test_task.db"
	defer os.Remove(dbPath)

	InitDB(dbPath, "info")
	AutoMigrate()
	defer CloseDB()

	repo := NewTaskRepository()

	task := &Task{
		Type:       "scan",
		Status:     "pending",
		TotalItems: 100,
		Progress:   0,
	}

	err := repo.Create(task)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// 测试更新状态
	err = repo.UpdateStatus(task.ID, "running", "Processing...")
	if err != nil {
		t.Fatalf("Failed to update task status: %v", err)
	}

	// 验证更新
	found, err := repo.GetByID(task.ID)
	if err != nil {
		t.Fatalf("Failed to get task: %v", err)
	}

	if found.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", found.Status)
	}
}

// TestSettingRepository 测试设置仓储
func TestSettingRepository(t *testing.T) {
	// 初始化测试数据库
	dbPath := "/tmp/test_setting.db"
	defer os.Remove(dbPath)

	InitDB(dbPath, "info")
	AutoMigrate()
	defer CloseDB()

	repo := NewSettingRepository()

	// 测试设置
	err := repo.Set("test_key", "test_value", "general")
	if err != nil {
		t.Fatalf("Failed to set: %v", err)
	}

	// 测试获取
	setting, err := repo.Get("test_key")
	if err != nil {
		t.Fatalf("Failed to get setting: %v", err)
	}

	if setting.Value != "test_value" {
		t.Errorf("Expected value 'test_value', got '%s'", setting.Value)
	}

	// 测试更新（再次 Set）
	err = repo.Set("test_key", "new_value", "general")
	if err != nil {
		t.Fatalf("Failed to update setting: %v", err)
	}

	setting, _ = repo.Get("test_key")
	if setting.Value != "new_value" {
		t.Errorf("Expected value 'new_value', got '%s'", setting.Value)
	}
}
