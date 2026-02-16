// Package utils provides filesystem path helpers.
package utils

import (
	"errors"
	"path/filepath"
	"strings"
)

// NormalizePath cleans a path, makes it absolute, and resolves symlinks when possible.
// If EvalSymlinks fails (e.g., path does not exist), it returns the absolute cleaned path.
//
// Example:
//  p, err := NormalizePath("../data/../logs")
func NormalizePath(p string) (string, error) {
	if strings.TrimSpace(p) == "" {
		return "", errors.New("path is empty")
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

// ReplacePrefix replaces sourcePrefix in path with targetPrefix if path starts with sourcePrefix.
// It preserves path separators and avoids partial segment replacements.
//
// Example:
//  ReplacePrefix("/a/b/c.mkv", "/a", "/x") -> "/x/b/c.mkv"
func ReplacePrefix(path string, sourcePrefix string, targetPrefix string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("path is empty")
	}
	if strings.TrimSpace(sourcePrefix) == "" {
		return "", errors.New("source prefix is empty")
	}
	if strings.TrimSpace(targetPrefix) == "" {
		return "", errors.New("target prefix is empty")
	}

	cleanPath := filepath.Clean(path)
	cleanSource := filepath.Clean(sourcePrefix)
	cleanTarget := filepath.Clean(targetPrefix)

	// Ensure prefix match on path segment boundary.
	rel, err := filepath.Rel(cleanSource, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." && cleanPath != cleanSource {
		return "", errors.New("path does not start with source prefix")
	}

	return filepath.Join(cleanTarget, rel), nil
}

// IsVideoFile reports whether the path has a known video extension.
// The check is case-insensitive.
//
// Example:
//  IsVideoFile("/movies/a.MKV") -> true
func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".m4v", ".ts", ".m2ts", ".webm":
		return true
	default:
		return false
	}
}

// IsSafePath ensures target is within base, preventing path traversal.
// It returns true if target is inside base after cleaning.
//
// Example:
//  IsSafePath("/data", "/data/a/b.txt") -> true
//  IsSafePath("/data", "/data/../etc/passwd") -> false
func IsSafePath(base string, target string) (bool, error) {
	if strings.TrimSpace(base) == "" || strings.TrimSpace(target) == "" {
		return false, errors.New("base or target is empty")
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
