package utils

import (
	"os"
	"testing"

	"go.uber.org/zap"
)

// TestInitLogger 测试日志初始化
func TestInitLogger(t *testing.T) {
	// 使用临时日志文件
	logPath := "/tmp/test_strmsync.log"
	defer os.Remove(logPath)

	err := InitLogger("debug", logPath)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	if Logger == nil {
		t.Fatal("Logger instance is nil")
	}

	// 测试各级别日志
	Debug("This is a debug message")
	Info("This is an info message", zap.String("key", "value"))
	Warn("This is a warning message", zap.Int("count", 10))
	Error("This is an error message", zap.Error(err))

	// 同步日志
	err = Sync()
	if err != nil {
		// Sync 在某些系统上可能返回错误（如 stdout），这是正常的
		t.Logf("Sync returned error (this may be normal): %v", err)
	}

	// 验证日志文件是否创建
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}
}

// TestWith 测试子 logger
func TestWith(t *testing.T) {
	// 使用临时日志文件
	logPath := "/tmp/test_with.log"
	defer os.Remove(logPath)

	err := InitLogger("info", logPath)
	if err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}

	// 创建带有字段的子 logger
	subLogger := With(
		zap.String("component", "test"),
		zap.Int("id", 123),
	)

	if subLogger == nil {
		t.Fatal("Sub logger is nil")
	}

	// 使用子 logger
	subLogger.Info("This is a message from sub logger")

	Sync()
}

// TestLoggerLevels 测试不同日志级别
func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		level    string
		expected bool
	}{
		{"debug", true},
		{"info", true},
		{"warn", true},
		{"error", true},
		{"invalid", true}, // 应该回退到 info 级别
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			logPath := "/tmp/test_level_" + tt.level + ".log"
			defer os.Remove(logPath)

			err := InitLogger(tt.level, logPath)
			if (err == nil) != tt.expected {
				t.Errorf("InitLogger(%s) error = %v, expected success = %v", tt.level, err, tt.expected)
			}

			if err == nil {
				Sync()
			}
		})
	}
}
