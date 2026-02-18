// Package http 提供HTTP API处理器
package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// LogHandler 日志查询处理器
type LogHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewLogHandler 创建日志处理器
func NewLogHandler(db *gorm.DB, logger *zap.Logger) *LogHandler {
	return &LogHandler{
		db:     db,
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

	query := h.db.Model(&model.LogEntry{})

	// 级别过滤
	if level := strings.TrimSpace(c.Query("level")); level != "" {
		query = query.Where("level = ?", level)
	}

	// 模块过滤
	if module := strings.TrimSpace(c.Query("module")); module != "" {
		query = query.Where("module = ?", module)
	}

	// 任务ID过滤
	if jobIDStr := strings.TrimSpace(c.Query("job_id")); jobIDStr != "" {
		if id, err := strconv.ParseUint(jobIDStr, 10, 64); err == nil {
			query = query.Where("job_id = ?", id)
		}
	}

	// 消息搜索
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		query = query.Where("message LIKE ?", "%"+search+"%")
	}

	// 时间范围
	if startAtStr := strings.TrimSpace(c.Query("start_at")); startAtStr != "" {
		if t, err := parseTime(startAtStr); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endAtStr := strings.TrimSpace(c.Query("end_at")); endAtStr != "" {
		if t, err := parseTime(endAtStr); err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.LogError("统计日志总数失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	// 查询日志列表
	var logs []model.LogEntry
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		logger.LogError("查询日志列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      logs,
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

	result := h.db.Where("created_at < ?", cutoff).Delete(&model.LogEntry{})
	if result.Error != nil {
		logger.LogError("清理日志失败", zap.Error(result.Error))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "清理失败"})
		return
	}

	logger.LogInfo("清理日志成功", zap.Int("days", req.Days), zap.Int64("deleted", result.RowsAffected))
	c.JSON(http.StatusOK, gin.H{
		"message": "清理成功",
		"deleted": result.RowsAffected,
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
