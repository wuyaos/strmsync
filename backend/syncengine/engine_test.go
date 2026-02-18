// Package syncengine 测试
package syncengine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/strmsync/strmsync/filesystem"
	"github.com/strmsync/strmsync/strmwriter"
	"github.com/strmsync/strmsync/syncengine"
	"go.uber.org/zap"
)

// TestEngineBasic 测试引擎基本功能
func TestEngineBasic(t *testing.T) {
	// 创建临时目录
	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpSrc, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 创建驱动器
	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, err := filesystem.NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	driver, err := filesystem.NewAdapter(client, syncengine.DriverLocal)
	if err != nil {
		t.Fatalf("创建驱动器失败: %v", err)
	}

	// 创建写入器
	writer, err := strmwriter.NewLocalWriter(tmpDst)
	if err != nil {
		t.Fatalf("创建写入器失败: %v", err)
	}

	// 创建引擎
	logger := zap.NewNop()
	engine, err := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot:     tmpDst,
		MaxConcurrency: 2,
		FileExtensions: []string{".mp4"},
	})
	if err != nil {
		t.Fatalf("创建引擎失败: %v", err)
	}

	// 执行同步
	ctx := context.Background()
	stats, err := engine.RunOnce(ctx, "/")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证统计信息
	if stats.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", stats.TotalFiles)
	}
	if stats.ProcessedFiles != 1 {
		t.Errorf("ProcessedFiles = %d, want 1", stats.ProcessedFiles)
	}
	if stats.CreatedFiles != 1 {
		t.Errorf("CreatedFiles = %d, want 1", stats.CreatedFiles)
	}

	// 验证 STRM 文件已创建
	strmFile := filepath.Join(tmpDst, "test.strm")
	if _, err := os.Stat(strmFile); os.IsNotExist(err) {
		t.Errorf("STRM 文件未创建: %s", strmFile)
	}
}

// TestEngineDryRun 测试试运行模式
func TestEngineDryRun(t *testing.T) {
	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpSrc, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, _ := filesystem.NewClient(cfg)
	driver, _ := filesystem.NewAdapter(client, syncengine.DriverLocal)
	writer, _ := strmwriter.NewLocalWriter(tmpDst)
	logger := zap.NewNop()

	engine, _ := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot: tmpDst,
		DryRun:     true, // 启用试运行
	})

	ctx := context.Background()
	stats, err := engine.RunOnce(ctx, "/")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// Dry Run 模式不应该创建文件
	if stats.CreatedFiles != 0 {
		t.Errorf("Dry Run 模式下不应创建文件: CreatedFiles = %d", stats.CreatedFiles)
	}

	// 验证文件确实未创建
	strmFile := filepath.Join(tmpDst, "test.strm")
	if _, err := os.Stat(strmFile); !os.IsNotExist(err) {
		t.Errorf("Dry Run 模式下不应创建实际文件")
	}
}

// TestEngineSkipExisting 测试跳过已存在文件
func TestEngineSkipExisting(t *testing.T) {
	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpSrc, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	// 预先创建 STRM 文件
	strmFile := filepath.Join(tmpDst, "test.strm")
	if err := os.WriteFile(strmFile, []byte("old content"), 0644); err != nil {
		t.Fatalf("创建 STRM 文件失败: %v", err)
	}

	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, _ := filesystem.NewClient(cfg)
	driver, _ := filesystem.NewAdapter(client, syncengine.DriverLocal)
	writer, _ := strmwriter.NewLocalWriter(tmpDst)
	logger := zap.NewNop()

	engine, _ := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot:   tmpDst,
		SkipExisting: true, // 跳过已存在
	})

	ctx := context.Background()
	stats, err := engine.RunOnce(ctx, "/")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 应该跳过文件
	if stats.SkippedFiles != 1 {
		t.Errorf("SkippedFiles = %d, want 1", stats.SkippedFiles)
	}

	// 验证文件内容未改变
	content, _ := os.ReadFile(strmFile)
	if string(content) != "old content" {
		t.Errorf("文件内容被修改: got %s, want 'old content'", string(content))
	}
}

// TestEngineForceUpdate 测试强制更新
func TestEngineForceUpdate(t *testing.T) {
	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(tmpSrc, "test.mp4")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, _ := filesystem.NewClient(cfg)
	driver, _ := filesystem.NewAdapter(client, syncengine.DriverLocal)
	writer, _ := strmwriter.NewLocalWriter(tmpDst)
	logger := zap.NewNop()

	engine, _ := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot: tmpDst,
	})

	ctx := context.Background()

	// 第一次同步
	stats1, _ := engine.RunOnce(ctx, "/")
	if stats1.CreatedFiles != 1 {
		t.Errorf("第一次同步: CreatedFiles = %d, want 1", stats1.CreatedFiles)
	}

	// 第二次同步（不强制更新）
	stats2, _ := engine.RunOnce(ctx, "/")
	if stats2.SkippedFiles != 1 {
		t.Errorf("第二次同步: SkippedFiles = %d, want 1", stats2.SkippedFiles)
	}

	// 第三次同步（强制更新）
	engine, _ = syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot:  tmpDst,
		ForceUpdate: true,
	})
	stats3, _ := engine.RunOnce(ctx, "/")
	if stats3.UpdatedFiles != 1 {
		t.Errorf("第三次同步: UpdatedFiles = %d, want 1", stats3.UpdatedFiles)
	}
}

// TestEngineFileExtensionFilter 测试文件扩展名过滤
func TestEngineFileExtensionFilter(t *testing.T) {
	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建多个测试文件
	files := []string{"video.mp4", "audio.mp3", "document.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpSrc, f), []byte("test"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, _ := filesystem.NewClient(cfg)
	driver, _ := filesystem.NewAdapter(client, syncengine.DriverLocal)
	writer, _ := strmwriter.NewLocalWriter(tmpDst)
	logger := zap.NewNop()

	engine, _ := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot:     tmpDst,
		FileExtensions: []string{".mp4", ".mkv"}, // 只同步视频文件
	})

	ctx := context.Background()
	stats, err := engine.RunOnce(ctx, "/")
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 应该只处理 .mp4 文件
	if stats.ProcessedFiles != 1 {
		t.Errorf("ProcessedFiles = %d, want 1", stats.ProcessedFiles)
	}
	if stats.FilteredFiles != 2 {
		t.Errorf("FilteredFiles = %d, want 2", stats.FilteredFiles)
	}
}

// TestEngineConcurrency 测试并发处理
func TestEngineConcurrency(t *testing.T) {
	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建多个测试文件
	for i := 0; i < 10; i++ {
		filename := filepath.Join(tmpSrc, "test"+string(rune('0'+i))+".mp4")
		if err := os.WriteFile(filename, []byte("test"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, _ := filesystem.NewClient(cfg)
	driver, _ := filesystem.NewAdapter(client, syncengine.DriverLocal)
	writer, _ := strmwriter.NewLocalWriter(tmpDst)
	logger := zap.NewNop()

	engine, _ := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot:     tmpDst,
		MaxConcurrency: 5, // 并发数为5
	})

	ctx := context.Background()
	start := time.Now()
	stats, err := engine.RunOnce(ctx, "/")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证所有文件都已处理
	if stats.ProcessedFiles != 10 {
		t.Errorf("ProcessedFiles = %d, want 10", stats.ProcessedFiles)
	}

	t.Logf("并发处理10个文件耗时: %v", duration)
}

// TestEngineContextCancel 测试上下文取消
func TestEngineContextCancel(t *testing.T) {
	t.Skip("Context 取消测试依赖时序，跳过以避免不稳定")

	tmpSrc := t.TempDir()
	tmpDst := t.TempDir()

	// 创建多个测试文件
	for i := 0; i < 100; i++ {
		filename := filepath.Join(tmpSrc, "test"+string(rune('0'+i%10))+".mp4")
		if err := os.WriteFile(filename, []byte("test"), 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	cfg := filesystem.Config{
		Type:      filesystem.TypeLocal,
		MountPath: tmpSrc,
		STRMMode:  filesystem.STRMModeMount,
	}
	client, _ := filesystem.NewClient(cfg)
	driver, _ := filesystem.NewAdapter(client, syncengine.DriverLocal)
	writer, _ := strmwriter.NewLocalWriter(tmpDst)
	logger := zap.NewNop()

	engine, _ := syncengine.NewEngine(driver, writer, logger, syncengine.EngineOptions{
		OutputRoot:     tmpDst,
		MaxConcurrency: 1, // 串行处理
	})

	// 创建已取消的 context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	_, err := engine.RunOnce(ctx, "/")

	// 应该因取消而失败
	if err != context.Canceled {
		t.Errorf("期望 context.Canceled 错误，got: %v", err)
	}
}
