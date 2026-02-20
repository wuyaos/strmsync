package sync

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"go.uber.org/zap"
)

// ---------------------
// Planner 测试
// ---------------------

func TestPlanner_CalculateTargetPath(t *testing.T) {
	tests := []struct {
		name       string
		filePath   string
		targetPath string
		expected   string
	}{
		{
			name:       "simple file conversion",
			filePath:   "movies/movie.mkv",
			targetPath: "/target",
			expected:   filepath.Join("/target", "movies", "movie.strm"),
		},
		{
			name:       "nested path",
			filePath:   "shows/season1/ep1.mp4",
			targetPath: "/mnt/media",
			expected:   filepath.Join("/mnt/media", "shows", "season1", "ep1.strm"),
		},
		{
			name:       "file at root",
			filePath:   "root.mkv",
			targetPath: "/target",
			expected:   filepath.Join("/target", "root.strm"),
		},
	}

	p := &Planner{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.calculateTargetStrmPath(tt.filePath, tt.targetPath)
			if err != nil {
				t.Fatalf("calculateTargetStrmPath: %v", err)
			}
			if result != tt.expected {
				t.Errorf("calculateTargetStrmPath = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestPlanner_BuildStrmContent(t *testing.T) {
	p := &Planner{}

	tests := []struct {
		name     string
		config   *ports.JobConfig
		event    *ports.FileEvent
		expected string
	}{
		{
			name: "local mode with mount path",
			config: &ports.JobConfig{
				STRMMode:   ports.STRMModeLocal,
				AccessPath: "/data",
				MountPath:  "/mnt",
				SourcePath: "/data/movies",
			},
			event:    &ports.FileEvent{Path: "Action/file.mkv"},
			expected: filepath.Join("/mnt", "movies", "Action", "file.mkv"),
		},
		{
			name: "local mode fallback to access path",
			config: &ports.JobConfig{
				STRMMode:   ports.STRMModeLocal,
				AccessPath: "/mnt/data",
				SourcePath: "/mnt/data",
			},
			event:    &ports.FileEvent{Path: "movie.mkv"},
			expected: filepath.Join("/mnt/data", "movie.mkv"),
		},
		{
			name: "url mode with replace rules",
			config: &ports.JobConfig{
				STRMMode:         ports.STRMModeURL,
				AccessPath:       "/remote",
				SourcePath:       "/remote/series",
				BaseURL:          "http://example.com/api",
				STRMReplaceRules: []ports.STRMReplaceRule{{From: "http://example.com", To: "https://cdn.example.com"}},
			},
			event:    &ports.FileEvent{Path: "s1/e1.mkv"},
			expected: "https://cdn.example.com/api/d/remote/series/s1/e1.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.buildStrmContent(tt.config, tt.event)
			if err != nil {
				t.Fatalf("buildStrmContent: %v", err)
			}
			if result != tt.expected {
				t.Errorf("buildStrmContent = %q, want %q", result, tt.expected)
			}
		})
	}
}

// ---------------------
// Generator 测试
// ---------------------

func TestGenerator_ValidatePath(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{targetRoot: tmpDir, logger: zap.NewNop()}

	tests := []struct {
		name      string
		path      string
		wantError bool
	}{
		{
			name:      "valid path inside root",
			path:      filepath.Join(tmpDir, "subdir", "movie.strm"),
			wantError: false,
		},
		{
			name:      "valid path at root",
			path:      filepath.Join(tmpDir, "movie.strm"),
			wantError: false,
		},
		{
			name:      "path outside root - traversal",
			path:      filepath.Join(tmpDir, "..", "outside.strm"),
			wantError: true,
		},
		{
			name:      "deeply nested outside root",
			path:      filepath.Join(tmpDir, "a", "..", "..", "outside.strm"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := g.validatePath(tt.path)
			if (err != nil) != tt.wantError {
				t.Errorf("validatePath(%q) error = %v, wantError %v", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestGenerator_CreateOrUpdateStrm(t *testing.T) {
	tmpDir := t.TempDir()
	g := &Generator{targetRoot: tmpDir, logger: zap.NewNop()}

	tests := []struct {
		name            string
		targetPath      string
		streamURL       string
		expectedContent string
	}{
		{
			name:            "creates new strm file",
			targetPath:      filepath.Join(tmpDir, "movie.strm"),
			streamURL:       "http://server/movie.mkv",
			expectedContent: "http://server/movie.mkv",
		},
		{
			name:            "creates subdirectory and file",
			targetPath:      filepath.Join(tmpDir, "movies", "action", "movie.strm"),
			streamURL:       "http://server/movies/action/movie.mkv",
			expectedContent: "http://server/movies/action/movie.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := &ports.SyncPlanItem{
				Op:             ports.SyncOpCreate,
				TargetStrmPath: tt.targetPath,
				StreamURL:      tt.streamURL,
				ModTime:        time.Now(),
			}

			if err := g.createOrUpdateStrm(context.Background(), item); err != nil {
				t.Fatalf("createOrUpdateStrm: %v", err)
			}

			// 验证文件存在且内容正确
			content, err := os.ReadFile(tt.targetPath)
			if err != nil {
				t.Fatalf("read strm file: %v", err)
			}
			if string(content) != tt.expectedContent {
				t.Errorf("strm content = %q, want %q", string(content), tt.expectedContent)
			}
		})
	}
}

func TestGenerator_DeleteStrm(t *testing.T) {
	t.Run("deletes existing strm file", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := &Generator{targetRoot: tmpDir, logger: zap.NewNop()}

		strmPath := filepath.Join(tmpDir, "movie.strm")
		if err := os.WriteFile(strmPath, []byte("http://test"), 0o644); err != nil {
			t.Fatalf("create test strm: %v", err)
		}

		item := &ports.SyncPlanItem{
			Op:             ports.SyncOpDelete,
			TargetStrmPath: strmPath,
		}

		if err := g.deleteStrm(context.Background(), item); err != nil {
			t.Fatalf("deleteStrm: %v", err)
		}

		if _, err := os.Stat(strmPath); !os.IsNotExist(err) {
			t.Errorf("strm file should be deleted but still exists")
		}
	})

	t.Run("idempotent: deleting non-existent file returns nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := &Generator{targetRoot: tmpDir, logger: zap.NewNop()}

		item := &ports.SyncPlanItem{
			Op:             ports.SyncOpDelete,
			TargetStrmPath: filepath.Join(tmpDir, "nonexistent.strm"),
		}

		if err := g.deleteStrm(context.Background(), item); err != nil {
			t.Errorf("deleteStrm on non-existent file: %v", err)
		}
	})

	t.Run("removes empty parent directories after deletion", func(t *testing.T) {
		tmpDir := t.TempDir()
		g := &Generator{targetRoot: tmpDir, logger: zap.NewNop()}

		// 创建嵌套目录和文件
		subDir := filepath.Join(tmpDir, "shows", "season1")
		if err := os.MkdirAll(subDir, 0o755); err != nil {
			t.Fatalf("create dir: %v", err)
		}

		strmPath := filepath.Join(subDir, "ep1.strm")
		if err := os.WriteFile(strmPath, []byte("http://test"), 0o644); err != nil {
			t.Fatalf("create test strm: %v", err)
		}

		item := &ports.SyncPlanItem{
			Op:             ports.SyncOpDelete,
			TargetStrmPath: strmPath,
		}

		if err := g.deleteStrm(context.Background(), item); err != nil {
			t.Fatalf("deleteStrm: %v", err)
		}

		// 验证文件已删除
		if _, err := os.Stat(strmPath); !os.IsNotExist(err) {
			t.Errorf("strm file should be deleted but still exists")
		}

		// 验证空目录也被清理（子目录应该已删除）
		if _, err := os.Stat(subDir); !os.IsNotExist(err) {
			t.Errorf("empty subdirectory should be removed but still exists: %s", subDir)
		}
	})
}
