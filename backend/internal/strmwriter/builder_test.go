// Package strmwriter_test 测试STRM内容生成
package strmwriter_test

import (
	"context"
	"strings"
	"testing"

	"github.com/strmsync/strmsync/internal/strmwriter"
)

// TestLocalBuilder 测试LocalBuilder基本功能
func TestLocalBuilder(t *testing.T) {
	tests := []struct {
		name       string
		localRoot  string
		remotePath string
		want       string
		wantErr    bool
	}{
		{
			name:       "basic path",
			localRoot:  "/mnt/media",
			remotePath: "/Movies/Avatar.mp4",
			want:       "/mnt/media/Movies/Avatar.mp4",
		},
		{
			name:       "path with trailing slash",
			localRoot:  "/mnt/media",
			remotePath: "/Movies/Avatar.mp4/",
			want:       "/mnt/media/Movies/Avatar.mp4",
		},
		{
			name:       "path with double slashes",
			localRoot:  "/mnt/media",
			remotePath: "/Movies//Avatar.mp4",
			want:       "/mnt/media/Movies/Avatar.mp4",
		},
		{
			name:       "path with dot segments",
			localRoot:  "/mnt/media",
			remotePath: "/Movies/./Avatar.mp4",
			want:       "/mnt/media/Movies/Avatar.mp4",
		},
		{
			name:       "path with parent segments",
			localRoot:  "/mnt/media",
			remotePath: "/Movies/../Videos/Avatar.mp4",
			want:       "/mnt/media/Videos/Avatar.mp4",
		},
		{
			name:       "empty path",
			localRoot:  "/mnt/media",
			remotePath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := strmwriter.BuildConfig{
				Format:    strmwriter.StrmFormatLocal,
				LocalRoot: tt.localRoot,
			}
			builder, err := strmwriter.NewLocalBuilder(cfg)
			if err != nil {
				t.Fatalf("NewLocalBuilder() error = %v", err)
			}

			req := strmwriter.BuildRequest{
				RemotePath: tt.remotePath,
				ServerID:   1,
			}

			got, err := builder.Build(context.Background(), req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Build() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Build() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestURLBuilder 测试URLBuilder基本功能
func TestURLBuilder(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		prefix     string
		remotePath string
		want       string
		wantErr    bool
	}{
		{
			name:       "basic URL",
			baseURL:    "http://192.168.1.100:19798",
			prefix:     "/d",
			remotePath: "/Movies/Avatar.mp4",
			want:       "http://192.168.1.100:19798/d/Movies/Avatar.mp4",
		},
		{
			name:       "URL with base path",
			baseURL:    "http://192.168.1.100:19798/api",
			prefix:     "/d",
			remotePath: "/Movies/Avatar.mp4",
			want:       "http://192.168.1.100:19798/api/d/Movies/Avatar.mp4",
		},
		{
			name:       "path with double slashes",
			baseURL:    "http://192.168.1.100:19798",
			prefix:     "/d",
			remotePath: "/Movies//Avatar.mp4",
			want:       "http://192.168.1.100:19798/d/Movies/Avatar.mp4",
		},
		{
			name:       "path with parent segments",
			baseURL:    "http://192.168.1.100:19798",
			prefix:     "/d",
			remotePath: "/Movies/../Videos/Avatar.mp4",
			want:       "http://192.168.1.100:19798/d/Videos/Avatar.mp4",
		},
		{
			name:       "empty base URL",
			baseURL:    "",
			prefix:     "/d",
			remotePath: "/Movies/Avatar.mp4",
			wantErr:    true,
		},
		{
			name:       "invalid base URL",
			baseURL:    "not-a-url",
			prefix:     "/d",
			remotePath: "/Movies/Avatar.mp4",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := strmwriter.BuildConfig{
				Format:        strmwriter.StrmFormatHTTP,
				BaseURL:       tt.baseURL,
				URLPathPrefix: tt.prefix,
			}
			builder, err := strmwriter.NewURLBuilder(cfg)
			if tt.wantErr {
				if err == nil {
					t.Error("NewURLBuilder() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("NewURLBuilder() error = %v", err)
			}

			req := strmwriter.BuildRequest{
				RemotePath: tt.remotePath,
				ServerID:   1,
			}

			got, err := builder.Build(context.Background(), req)
			if err != nil {
				t.Errorf("Build() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Build() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNilContext 测试nil context的处理
func TestNilContext(t *testing.T) {
	t.Run("LocalBuilder", func(t *testing.T) {
		cfg := strmwriter.BuildConfig{
			Format:    strmwriter.StrmFormatLocal,
			LocalRoot: "/mnt/media",
		}
		builder, _ := strmwriter.NewLocalBuilder(cfg)
		req := strmwriter.BuildRequest{
			RemotePath: "/Movies/Avatar.mp4",
			ServerID:   1,
		}

		// nil context should not panic
		_, err := builder.Build(nil, req)
		if err != nil {
			t.Errorf("Build(nil) error = %v", err)
		}
	})

	t.Run("URLBuilder", func(t *testing.T) {
		cfg := strmwriter.BuildConfig{
			Format:  strmwriter.StrmFormatHTTP,
			BaseURL: "http://192.168.1.100:19798",
		}
		builder, _ := strmwriter.NewURLBuilder(cfg)
		req := strmwriter.BuildRequest{
			RemotePath: "/Movies/Avatar.mp4",
			ServerID:   1,
		}

		// nil context should not panic
		_, err := builder.Build(nil, req)
		if err != nil {
			t.Errorf("Build(nil) error = %v", err)
		}
	})
}

// TestContentBuilderFactory 测试工厂方法
func TestContentBuilderFactory(t *testing.T) {
	tests := []struct {
		name    string
		format  strmwriter.StrmFormat
		config  strmwriter.BuildConfig
		wantErr bool
	}{
		{
			name:   "create local builder",
			format: strmwriter.StrmFormatLocal,
			config: strmwriter.BuildConfig{
				LocalRoot: "/mnt/media",
			},
			wantErr: false,
		},
		{
			name:   "create URL builder",
			format: strmwriter.StrmFormatHTTP,
			config: strmwriter.BuildConfig{
				BaseURL: "http://192.168.1.100:19798",
			},
			wantErr: false,
		},
		{
			name:   "invalid format",
			format: "invalid",
			config: strmwriter.BuildConfig{},
			wantErr: true,
		},
		{
			name:   "missing local root",
			format: strmwriter.StrmFormatLocal,
			config: strmwriter.BuildConfig{},
			wantErr: true,
		},
		{
			name:   "missing base URL",
			format: strmwriter.StrmFormatHTTP,
			config: strmwriter.BuildConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := strmwriter.NewContentBuilder(tt.format, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewContentBuilder() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStrmFormat 测试格式验证
func TestStrmFormat(t *testing.T) {
	tests := []struct {
		format strmwriter.StrmFormat
		valid  bool
	}{
		{strmwriter.StrmFormatHTTP, true},
		{strmwriter.StrmFormatLocal, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			if got := tt.format.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

// TestBuildConfigValidate 测试配置验证
func TestBuildConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  strmwriter.BuildConfig
		wantErr bool
	}{
		{
			name: "valid local config",
			config: strmwriter.BuildConfig{
				Format:    strmwriter.StrmFormatLocal,
				LocalRoot: "/mnt/media",
			},
			wantErr: false,
		},
		{
			name: "valid HTTP config",
			config: strmwriter.BuildConfig{
				Format:  strmwriter.StrmFormatHTTP,
				BaseURL: "http://192.168.1.100:19798",
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			config: strmwriter.BuildConfig{
				Format: "invalid",
			},
			wantErr: true,
		},
		{
			name: "local without root",
			config: strmwriter.BuildConfig{
				Format: strmwriter.StrmFormatLocal,
			},
			wantErr: true,
		},
		{
			name: "HTTP without base URL",
			config: strmwriter.BuildConfig{
				Format: strmwriter.StrmFormatHTTP,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestChinese 测试中文路径处理
func TestChinese(t *testing.T) {
	t.Run("LocalBuilder with Chinese", func(t *testing.T) {
		cfg := strmwriter.BuildConfig{
			Format:    strmwriter.StrmFormatLocal,
			LocalRoot: "/mnt/媒体",
		}
		builder, _ := strmwriter.NewLocalBuilder(cfg)
		req := strmwriter.BuildRequest{
			RemotePath: "/电影/阿凡达.mp4",
			ServerID:   1,
		}

		got, err := builder.Build(context.Background(), req)
		if err != nil {
			t.Errorf("Build() error = %v", err)
			return
		}
		if !strings.Contains(got, "电影") || !strings.Contains(got, "阿凡达") {
			t.Errorf("Build() = %v, want path with Chinese characters", got)
		}
	})

	t.Run("URLBuilder with Chinese", func(t *testing.T) {
		cfg := strmwriter.BuildConfig{
			Format:  strmwriter.StrmFormatHTTP,
			BaseURL: "http://192.168.1.100:19798",
		}
		builder, _ := strmwriter.NewURLBuilder(cfg)
		req := strmwriter.BuildRequest{
			RemotePath: "/电影/阿凡达.mp4",
			ServerID:   1,
		}

		got, err := builder.Build(context.Background(), req)
		if err != nil {
			t.Errorf("Build() error = %v", err)
			return
		}
		// URL会对中文进行编码，检查URL是否有效
		// 注意：url.URL.String() 会自动对非ASCII字符进行编码
		// 所以生成的URL包含 %E7%94%B5... 等编码是正确的
		if !strings.HasPrefix(got, "http://192.168.1.100:19798/d/") {
			t.Errorf("Build() = %v, want URL starting with http://192.168.1.100:19798/d/", got)
		}
		if !strings.HasSuffix(got, ".mp4") {
			t.Errorf("Build() = %v, want URL ending with .mp4", got)
		}
	})
}
