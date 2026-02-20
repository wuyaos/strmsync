// Package http 提供HTTP API处理器
package http

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"go.uber.org/zap"
)

// LogHandler 日志查询处理器
type LogHandler struct {
	logDir string
	logger *zap.Logger
}

// NewLogHandler 创建日志处理器
func NewLogHandler(logDir string, logger *zap.Logger) *LogHandler {
	return &LogHandler{
		logDir: strings.TrimSpace(logDir),
		logger: logger,
	}
}

// ListLogs 获取日志列表
// GET /api/logs
func (h *LogHandler) ListLogs(c *gin.Context) {
	page := parseIntQuery(c, "page", 1)
	pageSize := parseIntQuery(c, "page_size", 50)
	if pageSize > 200 {
		pageSize = 200
	}

	entries, err := h.readLogEntries()
	if err != nil {
		logger.LogError("读取日志文件失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取日志失败"})
		return
	}

	levelFilter := strings.ToLower(strings.TrimSpace(c.Query("level")))
	moduleFilter := strings.TrimSpace(c.Query("module"))
	search := strings.TrimSpace(c.Query("search"))
	var jobIDFilter *uint
	if jobIDStr := strings.TrimSpace(c.Query("job_id")); jobIDStr != "" {
		if id, err := strconv.ParseUint(jobIDStr, 10, 64); err == nil && id > 0 {
			parsed := uint(id)
			jobIDFilter = &parsed
		}
	}
	startAt, _ := parseTime(strings.TrimSpace(c.Query("start_at")))
	endAt, _ := parseTime(strings.TrimSpace(c.Query("end_at")))

	filtered := make([]logEntry, 0, len(entries))
	for _, entry := range entries {
		if levelFilter != "" && entry.Level != levelFilter {
			continue
		}
		if moduleFilter != "" && entry.Module != moduleFilter {
			continue
		}
		if jobIDFilter != nil {
			if entry.JobID == nil || *entry.JobID != *jobIDFilter {
				continue
			}
		}
		if search != "" && !strings.Contains(entry.Message, search) {
			continue
		}
		if !startAt.IsZero() && entry.CreatedAt.Before(startAt) {
			continue
		}
		if !endAt.IsZero() && entry.CreatedAt.After(endAt) {
			continue
		}
		filtered = append(filtered, entry)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	start := (page - 1) * pageSize
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      filtered[start:end],
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CleanupLogs 清理日志
// POST /api/logs/cleanup
// 请求体: {"days": 30} - 清理30天前的日志
func (h *LogHandler) CleanupLogs(c *gin.Context) {
	var req struct {
		Days int `json:"days" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	cutoff := time.Now().AddDate(0, 0, -req.Days)
	kept, deleted, err := h.cleanupLogFile(cutoff)
	if err != nil {
		logger.LogError("清理日志失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理失败"})
		return
	}

	logger.LogInfo("清理日志成功", zap.Int("days", req.Days), zap.Int("deleted", deleted))
	c.JSON(http.StatusOK, gin.H{
		"message": "清理成功",
		"deleted": deleted,
		"kept":    kept,
	})
}

// parseTime 解析时间字符串
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, nil
}

type logEntry struct {
	ID         int64     `json:"id"`
	Level      string    `json:"level"`
	Module     string    `json:"module,omitempty"`
	Message    string    `json:"message"`
	JobID      *uint     `json:"job_id,omitempty"`
	RequestID  *string   `json:"request_id,omitempty"`
	UserAction *string   `json:"user_action,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type rawLogEntry struct {
	Level      string `json:"level"`
	TS         string `json:"ts"`
	Time       string `json:"time"`
	Msg        string `json:"msg"`
	Message    string `json:"message"`
	Module     string `json:"module"`
	Component  string `json:"component"`
	Logger     string `json:"logger"`
	JobID      *uint  `json:"job_id"`
	RequestID  string `json:"request_id"`
	UserAction string `json:"user_action"`
}

type logLine struct {
	Raw     string
	HasTime bool
	Time    time.Time
}

func (h *LogHandler) logFilePath() string {
	dir, filePath := logger.ResolveLogFilePath(h.logDir)
	if filePath != "" {
		return filePath
	}
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "app.log")
}

func (h *LogHandler) readLogEntries() ([]logEntry, error) {
	lines, err := h.readLogLines()
	if err != nil {
		return nil, err
	}

	entries := make([]logEntry, 0, len(lines))
	var id int64
	for _, line := range lines {
		var raw rawLogEntry
		if err := json.Unmarshal([]byte(line.Raw), &raw); err != nil {
			fallback := buildFallbackEntry(line)
			if fallback == nil {
				continue
			}
			id++
			fallback.ID = id
			entries = append(entries, *fallback)
			continue
		}

		id++
		level := strings.ToLower(strings.TrimSpace(raw.Level))
		if level == "" {
			level = "info"
		}

		message := strings.TrimSpace(raw.Msg)
		if message == "" {
			message = strings.TrimSpace(raw.Message)
		}
		if message == "" {
			message = level
		}

		module := strings.TrimSpace(raw.Module)
		if module == "" {
			module = strings.TrimSpace(raw.Component)
		}
		if module == "" {
			module = strings.TrimSpace(raw.Logger)
		}

		requestID := strings.TrimSpace(raw.RequestID)
		var requestPtr *string
		if requestID != "" {
			requestPtr = &requestID
		}
		userAction := strings.TrimSpace(raw.UserAction)
		var userPtr *string
		if userAction != "" {
			userPtr = &userAction
		}

		createdAt := line.Time
		if !line.HasTime {
			createdAt = time.Time{}
		}

		entries = append(entries, logEntry{
			ID:         id,
			Level:      level,
			Module:     module,
			Message:    message,
			JobID:      raw.JobID,
			RequestID:  requestPtr,
			UserAction: userPtr,
			CreatedAt:  createdAt,
		})
	}

	return entries, nil
}

func (h *LogHandler) readLogLines() ([]logLine, error) {
	filePath := h.logFilePath()
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []logLine{}, nil
		}
		return nil, fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	lines := make([]logLine, 0)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		line := logLine{Raw: text}
		var raw rawLogEntry
		if err := json.Unmarshal([]byte(text), &raw); err == nil {
			if ts, ok := parseLogTime(raw.TS); ok {
				line.HasTime = true
				line.Time = ts
			} else if ts, ok := parseLogTime(raw.Time); ok {
				line.HasTime = true
				line.Time = ts
			}
		} else {
			parts := strings.Fields(text)
			if len(parts) > 0 {
				if ts, ok := parseLogTime(parts[0]); ok {
					line.HasTime = true
					line.Time = ts
				}
			}
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan log file: %w", err)
	}

	return lines, nil
}

func (h *LogHandler) cleanupLogFile(cutoff time.Time) (int, int, error) {
	lines, err := h.readLogLines()
	if err != nil {
		return 0, 0, err
	}
	if len(lines) == 0 {
		return 0, 0, nil
	}

	keptLines := make([]string, 0, len(lines))
	deleted := 0
	for _, line := range lines {
		if line.HasTime && line.Time.Before(cutoff) {
			deleted++
			continue
		}
		keptLines = append(keptLines, line.Raw)
	}

	filePath := h.logFilePath()
	file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	if err != nil {
		return 0, 0, fmt.Errorf("open log file for cleanup: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for i, line := range keptLines {
		if i > 0 {
			if _, err := writer.WriteString("\n"); err != nil {
				return 0, 0, fmt.Errorf("write log file: %w", err)
			}
		}
		if _, err := writer.WriteString(line); err != nil {
			return 0, 0, fmt.Errorf("write log file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return 0, 0, fmt.Errorf("flush log file: %w", err)
	}

	return len(keptLines), deleted, nil
}

func parseLogTime(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}

	formats := []string{
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05Z0700",
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, trimmed); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func buildFallbackEntry(line logLine) *logEntry {
	raw := strings.TrimSpace(line.Raw)
	if raw == "" {
		return nil
	}

	parts := strings.Fields(raw)
	start := 0
	if line.HasTime {
		start = 1
	}

	level := ""
	if len(parts) > start {
		level = normalizeLogLevel(parts[start])
	}
	if level == "" {
		level = "info"
	}

	message := raw
	if len(parts) > start+1 {
		message = strings.TrimSpace(strings.Join(parts[start+1:], " "))
	}
	if message == "" {
		message = level
	}

	createdAt := time.Time{}
	if line.HasTime {
		createdAt = line.Time
	}

	return &logEntry{
		Level:     level,
		Message:   message,
		CreatedAt: createdAt,
	}
}

func normalizeLogLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return "debug"
	case "info":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error":
		return "error"
	default:
		return ""
	}
}
