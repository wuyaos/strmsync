package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NormalizePath 标准化路径（转换为绝对路径，清理多余的分隔符）
// Author: STRMSync Team
func NormalizePath(path string) string {
	// 先替换反斜杠为正斜杠
	path = strings.ReplaceAll(path, "\\", "/")
	// 清理路径
	cleaned := filepath.Clean(path)
	// 转换为正斜杠（跨平台一致性）
	return filepath.ToSlash(cleaned)
}

// RelativePath 计算相对路径
// Author: STRMSync Team
func RelativePath(basePath, targetPath string) (string, error) {
	base := NormalizePath(basePath)
	target := NormalizePath(targetPath)

	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", fmt.Errorf("failed to calculate relative path: %w", err)
	}

	return filepath.ToSlash(rel), nil
}

// JoinPath 拼接路径
// Author: STRMSync Team
func JoinPath(parts ...string) string {
	joined := filepath.Join(parts...)
	return filepath.ToSlash(joined)
}

// IsSubPath 检查 child 是否是 parent 的子路径
// Author: STRMSync Team
func IsSubPath(parent, child string) bool {
	parent = NormalizePath(parent)
	child = NormalizePath(child)

	// 确保 parent 以分隔符结尾
	if !strings.HasSuffix(parent, "/") {
		parent += "/"
	}

	return strings.HasPrefix(child, parent)
}

// GetFileExtension 获取文件扩展名（小写，不含点）
// Author: STRMSync Team
func GetFileExtension(filename string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		ext = strings.ToLower(ext[1:]) // 移除点并转小写
	}
	return ext
}

// IsVideoFile 判断是否为视频文件
// Author: STRMSync Team
func IsVideoFile(filename string) bool {
	videoExts := map[string]bool{
		"mkv":  true,
		"mp4":  true,
		"avi":  true,
		"mov":  true,
		"wmv":  true,
		"flv":  true,
		"m4v":  true,
		"ts":   true,
		"m2ts": true,
		"webm": true,
		"rmvb": true,
	}

	ext := GetFileExtension(filename)
	return videoExts[ext]
}

// IsMetadataFile 判断是否为元数据文件
// Author: STRMSync Team
func IsMetadataFile(filename string) bool {
	ext := GetFileExtension(filename)

	// NFO 文件
	if ext == "nfo" {
		return true
	}

	// 图片文件（海报、背景等）
	name := strings.ToLower(filename)
	imageExts := []string{"jpg", "jpeg", "png", "webp"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			// 检查是否包含特定关键词
			if strings.Contains(name, "poster") ||
				strings.Contains(name, "fanart") ||
				strings.Contains(name, "banner") ||
				strings.Contains(name, "thumb") {
				return true
			}
		}
	}

	// 字幕文件
	subtitleExts := map[string]bool{
		"srt": true,
		"ass": true,
		"ssa": true,
		"sub": true,
		"vtt": true,
	}
	if subtitleExts[ext] {
		return true
	}

	return false
}

// EnsureDir 确保目录存在，不存在则创建
// Author: STRMSync Team
func EnsureDir(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("%s exists but is not a directory", dirPath)
		}
		return nil
	}

	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}
