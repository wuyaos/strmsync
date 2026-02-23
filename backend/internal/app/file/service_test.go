package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/strmsync/strmsync/internal/app/ports"
	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	// 导入 local provider，触发 init() 注册 TypeLocal Provider
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/local"
)

// newTestDB 创建测试用的内存数据库
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&model.DataServer{}, &model.MediaServer{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

// createDataServer 创建 DataServer 记录（使用 raw SQL 确保 bool 零值被保存）
func createDataServer(t *testing.T, db *gorm.DB, name, serverType, host string, port int, enabled bool, options string) model.DataServer {
	t.Helper()

	if options == "" {
		options = "{}"
	}

	if err := db.Exec(
		`INSERT INTO data_servers (name, type, host, port, enabled, options) VALUES (?, ?, ?, ?, ?, ?)`,
		name, serverType, host, port, enabled, options,
	).Error; err != nil {
		t.Fatalf("create data server: %v", err)
	}

	var ds model.DataServer
	if err := db.Where("name = ?", name).Last(&ds).Error; err != nil {
		t.Fatalf("load data server: %v", err)
	}

	return ds
}

func newNopLogger() *zap.Logger {
	return zap.NewNop()
}

func TestFileService_List_ServerIDRequired(t *testing.T) {
	db := newTestDB(t)
	svc := NewFileService(db, newNopLogger())

	_, err := svc.Scan(context.Background(), ports.FileListRequest{
		ServerID: 0,
	})

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestFileService_List_DataServerNotFound(t *testing.T) {
	db := newTestDB(t)
	svc := NewFileService(db, newNopLogger())

	_, err := svc.Scan(context.Background(), ports.FileListRequest{
		ServerID: 999,
	})

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrDataServerNotFound) {
		t.Errorf("expected ErrDataServerNotFound, got: %v", err)
	}
}

func TestFileService_List_DataServerDisabled(t *testing.T) {
	db := newTestDB(t)
	ds := createDataServer(t, db, "disabled-server", "local", "localhost", 0, false, `{"mount_path":"/tmp"}`)
	svc := NewFileService(db, newNopLogger())

	_, err := svc.Scan(context.Background(), ports.FileListRequest{
		ServerID: ds.ID,
	})

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrDataServerDisabled) {
		t.Errorf("expected ErrDataServerDisabled, got: %v", err)
	}
}

func TestFileService_List_LocalMountPathRequired(t *testing.T) {
	db := newTestDB(t)
	// local 类型但 mount_path 为空
	ds := createDataServer(t, db, "local-no-path", "local", "localhost", 0, true, `{"mount_path":""}`)
	svc := NewFileService(db, newNopLogger())

	_, err := svc.Scan(context.Background(), ports.FileListRequest{
		ServerID: ds.ID,
	})

	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestFileService_List_MaxDepthOutOfRange(t *testing.T) {
	db := newTestDB(t)
	dir := t.TempDir()
	options := fmt.Sprintf(`{"mount_path":"%s"}`, dir)
	ds := createDataServer(t, db, "local-server", "local", "localhost", 0, true, options)
	svc := NewFileService(db, newNopLogger())

	tests := []struct {
		name     string
		maxDepth int
	}{
		{name: "negative max_depth", maxDepth: -1},
		{name: "too large max_depth", maxDepth: maxListMaxDepth + 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depth := tt.maxDepth
			_, err := svc.Scan(context.Background(), ports.FileListRequest{
				ServerID:  ds.ID,
				Recursive: true,
				MaxDepth:  &depth,
			})

			if err == nil {
				t.Fatalf("expected error for max_depth=%d, got nil", tt.maxDepth)
			}
		})
	}
}

func TestFileService_List_LocalSuccess(t *testing.T) {
	db := newTestDB(t)
	dir := t.TempDir()

	// 创建测试文件
	testFile := filepath.Join(dir, "movie.mkv")
	if err := os.WriteFile(testFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	options := fmt.Sprintf(`{"mount_path":"%s"}`, dir)
	ds := createDataServer(t, db, "local-server", "local", "localhost", 0, true, options)
	svc := NewFileService(db, newNopLogger())

	files, err := svc.Scan(context.Background(), ports.FileListRequest{
		ServerID:  ds.ID,
		Path:      "/",
		Recursive: false,
	})

	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(files) == 0 {
		t.Fatalf("expected at least one file, got 0")
	}
}
