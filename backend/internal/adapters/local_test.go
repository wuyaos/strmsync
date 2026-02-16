package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewLocalAdapter 测试创建本地适配器
func TestNewLocalAdapter(t *testing.T) {
	// 创建临时测试目录
	tmpDir := "/tmp/test_local_adapter"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test-local",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, err := NewLocalAdapter(config)
	if err != nil {
		t.Fatalf("Failed to create adapter: %v", err)
	}

	if adapter.GetType() != "local" {
		t.Errorf("Expected type 'local', got '%s'", adapter.GetType())
	}

	if adapter.GetBasePath() != tmpDir {
		t.Errorf("Expected base path '%s', got '%s'", tmpDir, adapter.GetBasePath())
	}
}

// TestNewLocalAdapterInvalidPath 测试无效路径
func TestNewLocalAdapterInvalidPath(t *testing.T) {
	config := &AdapterConfig{
		Type:     "local",
		Name:     "test-invalid",
		BasePath: "/nonexistent/path",
		Enabled:  true,
	}

	_, err := NewLocalAdapter(config)
	if err == nil {
		t.Error("Should fail with nonexistent path")
	}
}

// TestIsAvailable 测试可用性检查
func TestIsAvailable(t *testing.T) {
	tmpDir := "/tmp/test_available"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := NewLocalAdapter(config)

	err := adapter.IsAvailable()
	if err != nil {
		t.Errorf("Adapter should be available: %v", err)
	}
}

// TestListFiles 测试文件列表
func TestListFiles(t *testing.T) {
	// 创建测试目录结构
	tmpDir := "/tmp/test_list_files"
	os.RemoveAll(tmpDir) // 清理旧数据
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFiles := []string{
		"movie1.mkv",
		"movie2.mp4",
		"poster.jpg",
		"movie.nfo",
		".DS_Store", // 应该被排除
		"subdir/movie3.avi",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte("test"), 0644)
	}

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := NewLocalAdapter(config)

	// 测试递归扫描
	options := DefaultScanOptions()
	files, err := adapter.ListFiles("", options)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	// 验证结果
	if len(files) == 0 {
		t.Error("Should find files")
	}

	// 验证不包含排除的文件
	for _, file := range files {
		if file.Name == ".DS_Store" {
			t.Error("Should not include .DS_Store")
		}
	}

	// 统计视频文件
	videoCount := 0
	for _, file := range files {
		if file.IsVideo {
			videoCount++
		}
	}

	if videoCount != 3 {
		t.Errorf("Expected 3 video files, got %d", videoCount)
	}
}

// TestListFilesNonRecursive 测试非递归扫描
func TestListFilesNonRecursive(t *testing.T) {
	tmpDir := "/tmp/test_non_recursive"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// 创建文件
	os.WriteFile(filepath.Join(tmpDir, "movie1.mkv"), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir/movie2.mkv"), []byte("test"), 0644)

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := NewLocalAdapter(config)

	options := DefaultScanOptions()
	options.Recursive = false

	files, err := adapter.ListFiles("", options)
	if err != nil {
		t.Fatalf("Failed to list files: %v", err)
	}

	// 非递归应该只找到根目录的文件
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
}

// TestGetFileInfo 测试获取文件信息
func TestGetFileInfo(t *testing.T) {
	tmpDir := "/tmp/test_file_info"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.mkv")
	testData := []byte("test data")
	os.WriteFile(testFile, testData, 0644)

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := NewLocalAdapter(config)

	fileInfo, err := adapter.GetFileInfo("test.mkv")
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	if fileInfo.Name != "test.mkv" {
		t.Errorf("Expected name 'test.mkv', got '%s'", fileInfo.Name)
	}

	if !fileInfo.IsVideo {
		t.Error("Should be detected as video file")
	}

	if fileInfo.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), fileInfo.Size)
	}
}

// TestScanDirectory 测试目录扫描统计
func TestScanDirectory(t *testing.T) {
	tmpDir := "/tmp/test_scan_dir"
	os.RemoveAll(tmpDir)
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFiles := []struct {
		name string
		size int
	}{
		{"movie1.mkv", 1024},
		{"movie2.mp4", 2048},
		{"poster.jpg", 512},
	}

	for _, tf := range testFiles {
		data := make([]byte, tf.size)
		os.WriteFile(filepath.Join(tmpDir, tf.name), data, 0644)
	}

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := NewLocalAdapter(config)

	result, err := adapter.ScanDirectory("", DefaultScanOptions())
	if err != nil {
		t.Fatalf("Failed to scan directory: %v", err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", result.TotalFiles)
	}

	if result.VideoFiles != 2 {
		t.Errorf("Expected 2 video files, got %d", result.VideoFiles)
	}

	if result.Metadata != 1 {
		t.Errorf("Expected 1 metadata file, got %d", result.Metadata)
	}

	expectedSize := int64(1024 + 2048 + 512)
	if result.TotalSize != expectedSize {
		t.Errorf("Expected total size %d, got %d", expectedSize, result.TotalSize)
	}
}

// TestClose 测试关闭适配器
func TestClose(t *testing.T) {
	tmpDir := "/tmp/test_close"
	os.MkdirAll(tmpDir, 0755)
	defer os.RemoveAll(tmpDir)

	config := &AdapterConfig{
		Type:     "local",
		Name:     "test",
		BasePath: tmpDir,
		Enabled:  true,
	}

	adapter, _ := NewLocalAdapter(config)

	err := adapter.Close()
	if err != nil {
		t.Errorf("Close should not fail: %v", err)
	}
}
