// Package syncengine 提供同步引擎辅助工具
package syncengine

import (
	"path"
	"strings"
)

// NormalizeExcludeDirs 规范化排除目录（相对远端根路径）
func NormalizeExcludeDirs(dirs []string) []string {
	if len(dirs) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(dirs))
	seen := make(map[string]struct{}, len(dirs))

	for _, dir := range dirs {
		item := strings.TrimSpace(dir)
		if item == "" {
			continue
		}
		item = strings.ReplaceAll(item, "\\", "/")
		item = strings.TrimPrefix(item, "./")
		item = strings.TrimLeft(item, "/")
		item = path.Clean(item)
		item = strings.TrimPrefix(item, "/")
		item = strings.TrimSuffix(item, "/")
		if item == "" || item == "." {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// IsExcludedPath 判断路径是否命中排除目录（基于远端根路径）
func IsExcludedPath(remoteRoot, entryPath string, excludeDirs []string) bool {
	if len(excludeDirs) == 0 {
		return false
	}
	if strings.TrimSpace(remoteRoot) == "" {
		return false
	}

	rel, ok := relativeToRoot(remoteRoot, entryPath)
	if !ok {
		return false
	}
	rel = strings.TrimLeft(rel, "/")
	if rel == "" {
		return false
	}

	for _, dir := range excludeDirs {
		if rel == dir || strings.HasPrefix(rel, dir+"/") {
			return true
		}
	}
	return false
}

func relativeToRoot(remoteRoot, entryPath string) (string, bool) {
	root := normalizeRemotePath(remoteRoot)
	full := normalizeRemotePath(entryPath)
	if root == "/" {
		return strings.TrimPrefix(full, "/"), true
	}
	if full == root {
		return "", true
	}
	prefix := root + "/"
	if strings.HasPrefix(full, prefix) {
		return strings.TrimPrefix(full, prefix), true
	}
	return "", false
}

func normalizeRemotePath(p string) string {
	value := strings.TrimSpace(p)
	if value == "" {
		return "/"
	}
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Clean("/" + value)
	if value == "." {
		return "/"
	}
	return value
}
