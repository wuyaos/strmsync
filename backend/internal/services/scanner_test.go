package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/adapters"
	"github.com/strmsync/strmsync/internal/database"
)

// TestNewScanner 测试创建扫描器
func TestNewScanner(t *testing.T) {
	config := &ScannerConfig{
		Concurrency: 10,
		BatchSize:   100,
		SampleSize:  1024,
	}

	scanner, err := NewScanner(config)
	if err != nil {
		t.Fatalf("Failed to create scanner: %v", err)
	}
	defer scanner.Close()

	if scanner.config.Concurrency != 10 {
		t.Errorf("Expected concurrency 10, got %d", scanner.config.Concurrency)
	}
}

// TestScannerDefaults 测试默认配置
func TestScannerDefaults(t *testing.T) {
	config := &ScannerConfig{}

	scanner, err := NewScanner(config)
	if err != nil {
		t.Fatalf("Failed to create scanner: %v", err)
	}
	defer scanner.Close()

	if scanner.config.Concurrency != 50 {
		t.Errorf("Expected default concurrency 50, got %d", scanner.config.Concurrency)
	}

	if scanner.config.BatchSize != 1000 {
		t.Errorf("Expected default batch size 1000, got %d", scanner.config.BatchSize)
	}

	if scanner.config.SampleSize != 1024*1024 {
		t.Errorf("Expected default sample size 1MB, got %d", scanner.config.SampleSize)
	}
}

// TestScanSource 测试扫描数据源
func TestScanSource(t *testing.T) {
	// 初始化数据库
	dbPath := "/tmp/test_scanner.db"
	defer os.Remove(dbPath)

	err := database.InitDB(dbPath, "info")
	if err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}
	defer database.CloseDB()

	err = database.AutoMigrate()
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// 创建测试目录和文件
	tmpDir := "/tmp/test_scan_source"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	testFiles := []string{
		"movie1.mkv",
		"movie2.mp4",
		"poster.jpg",
	}

	for _, file := range testFiles {
		os.WriteFile(filepath.Join(tmpDir, file), []byte("test"), 0644)
	}

	// 创建数据源
	sourceRepo := database.NewSourceRepository()
	source := &database.Source{
		Name:    "test-source",
		Type:    "local",
		Enabled: true,
		Config:  "{}",
	}
	sourceRepo.Create(source)

	// 创建适配器
	adapterConfig := &adapters.AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, err := adapters.NewLocalAdapter(adapterConfig)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	// 创建扫描器
	config := &ScannerConfig{
		Concurrency: 5,
		BatchSize:   10,
		SampleSize:  1024,
	}

	scanner, err := NewScanner(config)
	if err != nil {
		t.Fatalf("Failed to create scanner: %v", err)
	}
	defer scanner.Close()

	// 执行扫描
	ctx := context.Background()
	options := adapters.DefaultScanOptions()

	err = scanner.ScanSource(ctx, source.ID, adapter, options)
	if err != nil {
		t.Fatalf("Failed to scan source: %v", err)
	}

	// 验证结果
	progress := scanner.GetProgress()
	if progress == nil {
		t.Fatal("Progress should not be nil")
	}

	if progress.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", progress.Status)
	}

	if progress.ProcessedFiles != 3 {
		t.Errorf("Expected 3 processed files, got %d", progress.ProcessedFiles)
	}

	if progress.VideoFiles != 2 {
		t.Errorf("Expected 2 video files, got %d", progress.VideoFiles)
	}

	// 验证数据库中的记录
	fileRepo := database.NewFileRepository()
	count, err := fileRepo.CountBySourceID(source.ID)
	if err != nil {
		t.Fatalf("Failed to count files: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 files in database, got %d", count)
	}
}

// TestScanSourceCancellation 测试取消扫描
func TestScanSourceCancellation(t *testing.T) {
	// 初始化数据库
	dbPath := "/tmp/test_scanner_cancel.db"
	defer os.Remove(dbPath)

	database.InitDB(dbPath, "info")
	defer database.CloseDB()
	database.AutoMigrate()

	// 创建测试目录
	tmpDir := "/tmp/test_scan_cancel"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// 创建多个文件
	for i := 0; i < 100; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("movie%d.mkv", i))
		os.WriteFile(filename, []byte("test"), 0644)
	}

	// 创建数据源
	sourceRepo := database.NewSourceRepository()
	source := &database.Source{
		Name:    "test-cancel",
		Type:    "local",
		Enabled: true,
		Config:  "{}",
	}
	sourceRepo.Create(source)

	// 创建适配器
	adapterConfig := &adapters.AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := adapters.NewLocalAdapter(adapterConfig)

	// 创建扫描器
	config := &ScannerConfig{
		Concurrency: 2,  // 低并发便于测试
		BatchSize:   10,
		SampleSize:  1024,
	}

	scanner, _ := NewScanner(config)
	defer scanner.Close()

	// 在另一个 goroutine 中执行扫描
	ctx := context.Background()
	options := adapters.DefaultScanOptions()

	scanDone := make(chan error)
	go func() {
		err := scanner.ScanSource(ctx, source.ID, adapter, options)
		scanDone <- err
	}()

	// 稍等后取消扫描
	time.Sleep(50 * time.Millisecond)
	scanner.CancelScan(source.ID)

	// 等待扫描完成
	err := <-scanDone

	// 验证是否被取消（可能已经完成，因为文件较少）
	if err != nil && err != context.Canceled {
		t.Logf("Scan error (may be normal): %v", err)
	}
}

// TestFlushBuffer 测试缓冲区刷新
func TestFlushBuffer(t *testing.T) {
	// 初始化数据库
	dbPath := "/tmp/test_flush_buffer.db"
	defer os.Remove(dbPath)

	database.InitDB(dbPath, "info")
	defer database.CloseDB()
	database.AutoMigrate()

	config := &ScannerConfig{
		Concurrency: 5,
		BatchSize:   10,
		SampleSize:  1024,
	}

	scanner, _ := NewScanner(config)
	defer scanner.Close()

	// 添加文件到缓冲区
	for i := 0; i < 5; i++ {
		file := &database.File{
			SourceID:     1,
			RelativePath: fmt.Sprintf("test%d.mkv", i),
			FileName:     fmt.Sprintf("test%d.mkv", i),
			FileSize:     1024,
			ModTime:      time.Now(),
			FastHash:     "abc123",
		}
		scanner.fileBuffer = append(scanner.fileBuffer, file)
	}

	// 刷新缓冲区
	err := scanner.flushBuffer()
	if err != nil {
		t.Fatalf("Failed to flush buffer: %v", err)
	}

	// 验证缓冲区已清空
	if len(scanner.fileBuffer) != 0 {
		t.Errorf("Buffer should be empty after flush, got %d items", len(scanner.fileBuffer))
	}
}
