// Package handlers 提供HTTP API处理器
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TaskRunHandler 任务执行记录处理器
type TaskRunHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewTaskRunHandler 创建任务执行记录处理器
func NewTaskRunHandler(db *gorm.DB, logger *zap.Logger) *TaskRunHandler {
	return &TaskRunHandler{
		db:     db,
		logger: logger,
	}
}

// ListTaskRuns 获取执行记录列表
// GET /api/runs
func (h *TaskRunHandler) ListTaskRuns(c *gin.Context) {
	pagination := parsePagination(c, 1, 50, 200)

	query := h.db.Model(&model.TaskRun{}).
		Preload("Job")

	// job_id过滤
	if jobIDStr := strings.TrimSpace(c.Query("job_id")); jobIDStr != "" {
		id, err := strconv.ParseUint(jobIDStr, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的job_id参数", nil)
			return
		}
		query = query.Where("job_id = ?", id)
	}

	// status过滤（支持枚举值：running/completed/failed/cancelled）
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		validStatuses := []string{"running", "completed", "failed", "cancelled"}
		isValid := false
		for _, s := range validStatuses {
			if status == s {
				isValid = true
				break
			}
		}
		if !isValid {
			respondError(c, http.StatusBadRequest, "invalid_request", "status必须是running/completed/failed/cancelled之一", nil)
			return
		}
		query = query.Where("status = ?", status)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("统计执行记录失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 查询列表（按开始时间降序）
	var runs []model.TaskRun
	if err := query.Order("started_at DESC").
		Offset(pagination.Offset).
		Limit(pagination.PageSize).
		Find(&runs).Error; err != nil {
		h.logger.Error("查询执行记录列表失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"runs":      runs,
		"total":     total,
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
	})
}

// GetTaskRun 获取单个执行记录
// GET /api/runs/:id
func (h *TaskRunHandler) GetTaskRun(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var run model.TaskRun
	if err := h.db.
		Preload("Job").
		Preload("Job.DataServer").
		Preload("Job.MediaServer").
		First(&run, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "执行记录不存在", nil)
			return
		}
		h.logger.Error("查询执行记录失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"run": run})
}
