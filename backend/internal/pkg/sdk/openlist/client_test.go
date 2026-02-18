// Package openlist_test tests the OpenList SDK client
package openlist_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/pkg/sdk/openlist"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     openlist.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: openlist.Config{
				BaseURL: "http://localhost:5244",
				Timeout: 5 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "invalid base URL",
			cfg: openlist.Config{
				BaseURL: "not-a-url",
			},
			wantErr: true,
		},
		{
			name: "empty base URL",
			cfg: openlist.Config{
				BaseURL: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := openlist.NewClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_List(t *testing.T) {
	// 模拟 OpenList 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/fs/list" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"code": 200,
				"message": "success",
				"data": {
					"content": [
						{
							"name": "test.mp4",
							"size": 1024,
							"is_dir": false,
							"modified": "2024-01-01T00:00:00Z"
						},
						{
							"name": "folder",
							"size": 0,
							"is_dir": true,
							"modified": "2024-01-01T00:00:00Z"
						}
					],
					"total": 2
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := openlist.NewClient(openlist.Config{
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	items, err := client.List(context.Background(), "/")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(items) != 2 {
		t.Errorf("List() returned %d items, want 2", len(items))
	}

	// 检查第一个文件
	if items[0].Name != "test.mp4" {
		t.Errorf("First item name = %v, want test.mp4", items[0].Name)
	}
	if items[0].Size != 1024 {
		t.Errorf("First item size = %v, want 1024", items[0].Size)
	}
	if items[0].IsDir {
		t.Errorf("First item should not be a directory")
	}

	// 检查第二个目录
	if items[1].Name != "folder" {
		t.Errorf("Second item name = %v, want folder", items[1].Name)
	}
	if !items[1].IsDir {
		t.Errorf("Second item should be a directory")
	}
}
