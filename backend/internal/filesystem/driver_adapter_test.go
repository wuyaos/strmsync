// Package filesystem 驱动适配器测试
package filesystem

import (
	"context"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/engine"
)

// TestAdapterBasicWorkflow 测试适配器基本工作流程
func TestAdapterBasicWorkflow(t *testing.T) {
	// 创建本地文件系统客户端（用于测试）
	cfg := Config{
		Type:      TypeLocal,
		MountPath: t.TempDir(), // 使用临时目录
		STRMMode:  STRMModeMount,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	// 创建适配器
	adapter, err := NewAdapter(client, syncengine.DriverLocal)
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}

	// 测试 Type 方法
	if adapter.Type() != syncengine.DriverLocal {
		t.Errorf("Type() = %v, want %v", adapter.Type(), syncengine.DriverLocal)
	}

	// 测试 Capabilities 方法
	cap := adapter.Capabilities()
	if !cap.StrmMount {
		t.Error("Local 驱动应该支持 StrmMount")
	}
	if cap.StrmHTTP {
		t.Error("Local 驱动不应该支持 StrmHTTP")
	}

	// 测试 TestConnection 方法
	ctx := context.Background()
	if err := adapter.TestConnection(ctx); err != nil {
		t.Errorf("TestConnection() 失败: %v", err)
	}
}

// TestAdapterList 测试适配器 List 方法
func TestAdapterList(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建本地文件系统客户端
	cfg := Config{
		Type:      TypeLocal,
		MountPath: tmpDir,
		STRMMode:  STRMModeMount,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	// 创建适配器
	adapter, err := NewAdapter(client, syncengine.DriverLocal)
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}

	// 测试 List 方法（空目录）
	ctx := context.Background()
	entries, err := adapter.List(ctx, "/", syncengine.ListOptions{Recursive: false})
	if err != nil {
		t.Errorf("List() 失败: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("List() 返回 %d 个条目, 期望 0", len(entries))
	}
}

// TestAdapterBuildStrmInfo 测试适配器 BuildStrmInfo 方法
func TestAdapterBuildStrmInfo(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建本地文件系统客户端
	cfg := Config{
		Type:      TypeLocal,
		MountPath: tmpDir,
		STRMMode:  STRMModeMount,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	// 创建适配器
	adapter, err := NewAdapter(client, syncengine.DriverLocal)
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}

	// 测试 BuildStrmInfo 方法
	ctx := context.Background()
	req := syncengine.BuildStrmRequest{
		ServerID:   1,
		RemotePath: "/test/file.mp4",
	}
	info, err := adapter.BuildStrmInfo(ctx, req)
	if err != nil {
		t.Errorf("BuildStrmInfo() 失败: %v", err)
	}
	if info.RawURL == "" {
		t.Error("BuildStrmInfo() 返回空 RawURL")
	}
	// 本地模式：BaseURL 可能是 nil 或空结构体（取决于是否使用降级策略）
	if info.BaseURL != nil && (info.BaseURL.Scheme != "" || info.BaseURL.Host != "") {
		t.Errorf("BuildStrmInfo() 本地模式不应该有 scheme/host, got BaseURL=%#v", info.BaseURL)
	}
	t.Logf("BuildStrmInfo 返回: RawURL=%s, Path=%s", info.RawURL, info.Path)
}

// TestAdapterCompareStrm 测试适配器 CompareStrm 方法
func TestAdapterCompareStrm(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建本地文件系统客户端
	cfg := Config{
		Type:      TypeLocal,
		MountPath: tmpDir,
		STRMMode:  STRMModeMount,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	// 创建适配器
	adapter, err := NewAdapter(client, syncengine.DriverLocal)
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}

	// 测试 CompareStrm 方法（内容一致）
	ctx := context.Background()
	expected := syncengine.StrmInfo{
		RawURL:  "/mnt/media/test.mp4",
		BaseURL: nil,
		Path:    "/mnt/media/test.mp4",
	}
	input := syncengine.CompareInput{
		Expected:  expected,
		ActualRaw: "/mnt/media/test.mp4",
	}
	result, err := adapter.CompareStrm(ctx, input)
	if err != nil {
		t.Errorf("CompareStrm() 失败: %v", err)
	}
	if !result.Equal {
		t.Errorf("CompareStrm() Equal = false, want true (reason: %s)", result.Reason)
	}

	// 测试 CompareStrm 方法（内容不同）
	input.ActualRaw = "/mnt/media/other.mp4"
	result, err = adapter.CompareStrm(ctx, input)
	if err != nil {
		t.Errorf("CompareStrm() 失败: %v", err)
	}
	if result.Equal {
		t.Error("CompareStrm() Equal = true, want false")
	}
	if !result.NeedUpdate {
		t.Error("CompareStrm() NeedUpdate = false, want true")
	}
}

// TestAdapterWatch 测试适配器 Watch 方法（不支持）
func TestAdapterWatch(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()

	// 创建本地文件系统客户端
	cfg := Config{
		Type:      TypeLocal,
		MountPath: tmpDir,
		STRMMode:  STRMModeMount,
	}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	// 创建适配器
	adapter, err := NewAdapter(client, syncengine.DriverLocal)
	if err != nil {
		t.Fatalf("创建适配器失败: %v", err)
	}

	// 测试 Watch 方法（应该返回 ErrNotSupported）
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = adapter.Watch(ctx, "/", syncengine.WatchOptions{})
	if err != syncengine.ErrNotSupported {
		t.Errorf("Watch() error = %v, want %v", err, syncengine.ErrNotSupported)
	}
}
