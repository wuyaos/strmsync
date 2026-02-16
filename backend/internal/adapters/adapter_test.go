package adapters

import (
	"testing"
)

// TestDefaultScanOptions 测试默认扫描选项
func TestDefaultScanOptions(t *testing.T) {
	opts := DefaultScanOptions()

	if !opts.Recursive {
		t.Error("Recursive should be true by default")
	}

	if !opts.IncludeVideo {
		t.Error("IncludeVideo should be true by default")
	}

	if !opts.IncludeMeta {
		t.Error("IncludeMeta should be true by default")
	}

	if opts.MaxDepth != 0 {
		t.Errorf("MaxDepth should be 0 by default, got %d", opts.MaxDepth)
	}

	if len(opts.ExcludeNames) == 0 {
		t.Error("ExcludeNames should have default values")
	}
}

// TestShouldExclude 测试排除判断
func TestShouldExclude(t *testing.T) {
	opts := DefaultScanOptions()

	tests := []struct {
		name     string
		expected bool
	}{
		{".DS_Store", true},
		{"Thumbs.db", true},
		{".tmp", true},
		{"movie.mkv", false},
		{"folder", false},
	}

	for _, tt := range tests {
		result := opts.ShouldExclude(tt.name)
		if result != tt.expected {
			t.Errorf("ShouldExclude(%s) = %v, want %v", tt.name, result, tt.expected)
		}
	}
}

// TestAdapterConfig 测试适配器配置
func TestAdapterConfig(t *testing.T) {
	config := &AdapterConfig{
		Type:     "local",
		Name:     "test-source",
		BasePath: "/media/test",
		Enabled:  true,
		Properties: map[string]string{
			"key": "value",
		},
	}

	if config.Type != "local" {
		t.Errorf("Type should be 'local', got '%s'", config.Type)
	}

	if config.Properties["key"] != "value" {
		t.Error("Properties should contain key-value pairs")
	}
}

// TestFileInfo 测试文件信息结构
func TestFileInfo(t *testing.T) {
	fileInfo := &FileInfo{
		Path:         "/test/movie.mkv",
		Name:         "movie.mkv",
		Size:         1024000,
		IsDir:        false,
		IsVideo:      true,
		IsMetadata:   false,
		RelativePath: "movies/movie.mkv",
	}

	if !fileInfo.IsVideo {
		t.Error("IsVideo should be true")
	}

	if fileInfo.IsDir {
		t.Error("IsDir should be false")
	}

	if fileInfo.Size != 1024000 {
		t.Errorf("Size should be 1024000, got %d", fileInfo.Size)
	}
}

// TestScanResult 测试扫描结果结构
func TestScanResult(t *testing.T) {
	result := &ScanResult{
		TotalFiles: 100,
		VideoFiles: 80,
		Metadata:   20,
		TotalSize:  1024000000,
		Files:      make([]*FileInfo, 0),
	}

	if result.TotalFiles != 100 {
		t.Errorf("TotalFiles should be 100, got %d", result.TotalFiles)
	}

	if result.VideoFiles != 80 {
		t.Errorf("VideoFiles should be 80, got %d", result.VideoFiles)
	}

	if len(result.Files) != 0 {
		t.Error("Files should be empty initially")
	}
}
