// Package utils 提供文件系统路径处理工具
package path

import (
	"errors"
	"path/filepath"
	"strings"

	appconfig "github.com/strmsync/strmsync/internal/config"
)

// NormalizePath 清理路径，使其成为绝对路径，并尽可能解析符号链接
// 如果EvalSymlinks失败（例如路径不存在），返回绝对清理后的路径
//
// 示例：
//
//	p, err := NormalizePath("../data/../logs")
func NormalizePath(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("路径为空")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	return abs, nil
}

// ReplacePrefix 如果path以sourcePrefix开头，则将其替换为targetPrefix
// 保留路径分隔符，避免部分段替换
//
// 示例：
//
//	ReplacePrefix("/a/b/c.mkv", "/a", "/x") -> "/x/b/c.mkv"
func ReplacePrefix(path string, sourcePrefix string, targetPrefix string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("路径为空")
	}
	if strings.TrimSpace(sourcePrefix) == "" {
		return "", errors.New("源前缀为空")
	}
	if strings.TrimSpace(targetPrefix) == "" {
		return "", errors.New("目标前缀为空")
	}

	cleanPath := filepath.Clean(path)
	cleanSource := filepath.Clean(sourcePrefix)
	cleanTarget := filepath.Clean(targetPrefix)

	// 确保前缀匹配在路径段边界上
	rel, err := filepath.Rel(cleanSource, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." && cleanPath != cleanSource {
		return "", errors.New("路径不以源前缀开头")
	}

	return filepath.Join(cleanTarget, rel), nil
}

// IsVideoFile 判断路径是否具有已知的视频扩展名
// 检查不区分大小写
//
// 示例：
//
//	IsVideoFile("/movies/a.MKV") -> true
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return appconfig.IsKnownVideoExtension(ext)
}

// IsSafePath 确保target在base内部，防止路径遍历攻击
// 清理后如果target在base内则返回true
//
// 示例：
//
//	IsSafePath("/data", "/data/a/b.txt") -> true
//	IsSafePath("/data", "/data/../etc/passwd") -> false
func IsSafePath(base string, target string) (bool, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(target) == "" {
		return false, errors.New("base或target为空")
	}

	cleanBase, err := NormalizePath(base)
	if err != nil {
		return false, err
	}
	cleanTarget, err := NormalizePath(target)
	if err != nil {
		return false, err
	}

	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return false, err
	}
	if rel == "." {
		return true, nil
	}
	return !strings.HasPrefix(rel, ".."), nil
}
