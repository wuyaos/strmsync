package config

import (
	"os"
	"testing"
)

// TestLoadConfig 测试配置加载
func TestLoadConfig(t *testing.T) {
	// 设置测试环境变量
	os.Setenv("PORT", "8080")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("ENCRYPTION_KEY", "12345678901234567890123456789012")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("ENCRYPTION_KEY")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Server.Port != "8080" {
		t.Errorf("Expected port 8080, got %s", cfg.Server.Port)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("Expected log level debug, got %s", cfg.Log.Level)
	}
}

// TestValidate 测试配置验证
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Server:   ServerConfig{Port: "3000", Mode: "release"},
				Database: DatabaseConfig{Type: "sqlite", Path: "/app/data/test.db"},
				Log:      LogConfig{Level: "info", OutputPath: "/app/logs/test.log"},
				Security: SecurityConfig{EncryptionKey: "12345678901234567890123456789012"},
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: &Config{
				Server:   ServerConfig{Port: "3000", Mode: "release"},
				Database: DatabaseConfig{Type: "sqlite", Path: "/app/data/test.db"},
				Log:      LogConfig{Level: "invalid", OutputPath: "/app/logs/test.log"},
				Security: SecurityConfig{EncryptionKey: "12345678901234567890123456789012"},
			},
			wantErr: true,
		},
		{
			name: "empty port",
			config: &Config{
				Server:   ServerConfig{Port: "", Mode: "release"},
				Database: DatabaseConfig{Type: "sqlite", Path: "/app/data/test.db"},
				Log:      LogConfig{Level: "info", OutputPath: "/app/logs/test.log"},
				Security: SecurityConfig{EncryptionKey: "12345678901234567890123456789012"},
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
