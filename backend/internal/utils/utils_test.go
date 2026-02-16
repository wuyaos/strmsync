package utils

import (
	"io/ioutil"
	"os"
	"testing"
)

// TestCalculateFastHash 测试快速哈希
func TestCalculateFastHash(t *testing.T) {
	// 创建临时测试文件
	tmpFile, err := ioutil.TempFile("", "test_hash_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 写入测试数据
	testData := []byte("This is a test file for hash calculation")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// 测试快速哈希
	hash, err := CalculateFastHash(tmpFile.Name(), 1024*1024)
	if err != nil {
		t.Fatalf("CalculateFastHash failed: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}

	// 测试同一文件的哈希一致性
	hash2, err := CalculateFastHash(tmpFile.Name(), 1024*1024)
	if err != nil {
		t.Fatalf("CalculateFastHash failed on second call: %v", err)
	}

	if hash != hash2 {
		t.Errorf("Hash mismatch: %s != %s", hash, hash2)
	}
}

// TestCalculateFullHash 测试完整哈希
func TestCalculateFullHash(t *testing.T) {
	// 创建临时测试文件
	tmpFile, err := ioutil.TempFile("", "test_full_hash_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 写入测试数据
	testData := []byte("Test data for full hash")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// 测试完整哈希
	hash, err := CalculateFullHash(tmpFile.Name())
	if err != nil {
		t.Fatalf("CalculateFullHash failed: %v", err)
	}

	if hash == "" {
		t.Error("Hash should not be empty")
	}

	if len(hash) != 32 {
		t.Errorf("MD5 hash should be 32 characters, got %d", len(hash))
	}
}

// TestEncryptDecrypt 测试加密解密
func TestEncryptDecrypt(t *testing.T) {
	key := []byte("12345678901234567890123456789012") // 32 字节
	plaintext := "This is a secret message"

	// 测试加密
	ciphertext, err := Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if ciphertext == "" {
		t.Error("Ciphertext should not be empty")
	}

	// 测试解密
	decrypted, err := Decrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decrypted text mismatch: got %s, want %s", decrypted, plaintext)
	}
}

// TestEncryptWithInvalidKey 测试无效密钥
func TestEncryptWithInvalidKey(t *testing.T) {
	invalidKey := []byte("short") // 不是 32 字节
	plaintext := "test"

	_, err := Encrypt(plaintext, invalidKey)
	if err == nil {
		t.Error("Encrypt should fail with invalid key length")
	}
}

// TestNormalizePath 测试路径标准化
func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to//file", "/path/to/file"},
		{"path/to/./file", "path/to/file"},
		{"/path/to/../file", "/path/file"},
		{"path\\to\\file", "path/to/file"},
	}

	for _, tt := range tests {
		result := NormalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizePath(%s) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

// TestIsSubPath 测试子路径判断
func TestIsSubPath(t *testing.T) {
	tests := []struct {
		parent   string
		child    string
		expected bool
	}{
		{"/parent", "/parent/child", true},
		{"/parent", "/parent/child/file.txt", true},
		{"/parent", "/other/path", false},
		{"/parent", "/parent", false}, // 相同路径不是子路径
	}

	for _, tt := range tests {
		result := IsSubPath(tt.parent, tt.child)
		if result != tt.expected {
			t.Errorf("IsSubPath(%s, %s) = %v, want %v", tt.parent, tt.child, result, tt.expected)
		}
	}
}

// TestGetFileExtension 测试获取文件扩展名
func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"movie.mkv", "mkv"},
		{"video.MP4", "mp4"},
		{"file.tar.gz", "gz"},
		{"noext", ""},
	}

	for _, tt := range tests {
		result := GetFileExtension(tt.filename)
		if result != tt.expected {
			t.Errorf("GetFileExtension(%s) = %s, want %s", tt.filename, result, tt.expected)
		}
	}
}

// TestIsVideoFile 测试视频文件判断
func TestIsVideoFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"movie.mkv", true},
		{"video.mp4", true},
		{"Video.AVI", true},
		{"document.txt", false},
		{"image.jpg", false},
	}

	for _, tt := range tests {
		result := IsVideoFile(tt.filename)
		if result != tt.expected {
			t.Errorf("IsVideoFile(%s) = %v, want %v", tt.filename, result, tt.expected)
		}
	}
}

// TestIsMetadataFile 测试元数据文件判断
func TestIsMetadataFile(t *testing.T) {
	tests := []struct {
		filename string
		expected bool
	}{
		{"movie.nfo", true},
		{"poster.jpg", true},
		{"fanart.png", true},
		{"subtitle.srt", true},
		{"movie.mkv", false},
		{"readme.txt", false},
	}

	for _, tt := range tests {
		result := IsMetadataFile(tt.filename)
		if result != tt.expected {
			t.Errorf("IsMetadataFile(%s) = %v, want %v", tt.filename, result, tt.expected)
		}
	}
}

// TestEnsureDir 测试目录创建
func TestEnsureDir(t *testing.T) {
	// 创建临时目录
	tmpDir := "/tmp/test_ensure_dir"
	defer os.RemoveAll(tmpDir)

	// 测试创建目录
	err := EnsureDir(tmpDir)
	if err != nil {
		t.Fatalf("EnsureDir failed: %v", err)
	}

	// 验证目录存在
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Created path is not a directory")
	}

	// 测试已存在的目录
	err = EnsureDir(tmpDir)
	if err != nil {
		t.Errorf("EnsureDir should not fail for existing directory: %v", err)
	}
}
