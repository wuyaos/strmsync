// Package http 提供HTTP API处理器
package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type batchDeleteRunsRequest struct {
	IDs []uint `json:"ids"`
}

func isTaskRunDeletable(status string) bool {
	return status != "running" && status != "pending"
}

// TaskRunHandler 任务执行记录处理器
type TaskRunHandler struct {
	db     *gorm.DB
	logger *zap.Logger
	queue  TaskQueue
}

// NewTaskRunHandler 创建任务执行记录处理器
func NewTaskRunHandler(db *gorm.DB, logger *zap.Logger, queue TaskQueue) *TaskRunHandler {
	return &TaskRunHandler{
		db:     db,
		logger: logger,
		queue:  queue,
	}
}

// ListTaskRuns 获取执行记录列表
// GET /api/runs
func (h *TaskRunHandler) ListTaskRuns(c *gin.Context) {
	pagination := parsePagination(c, 1, 50, 200)

	query := h.db.Model(&model.TaskRun{}).
		Preload("Job").
		Preload("Job.DataServer").
		Preload("Job.MediaServer")

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

// DeleteTaskRun 删除单个执行记录
// DELETE /api/runs/:id
func (h *TaskRunHandler) DeleteTaskRun(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var run model.TaskRun
	if err := h.db.First(&run, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "执行记录不存在", nil)
			return
		}
		h.logger.Error("查询执行记录失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	if !isTaskRunDeletable(run.Status) {
		respondError(c, http.StatusConflict, "run_running", "运行中的执行记录无法删除", nil)
		return
	}

	tx := h.db.Begin()
	if err := tx.Where("task_run_id = ?", run.ID).Delete(&model.TaskRunEvent{}).Error; err != nil {
		tx.Rollback()
		h.logger.Error("删除执行记录事件失败", zap.Error(err), zap.Uint("run_id", run.ID))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}
	if err := tx.Where("id = ?", run.ID).Delete(&model.TaskRun{}).Error; err != nil {
		tx.Rollback()
		h.logger.Error("删除执行记录失败", zap.Error(err), zap.Uint("run_id", run.ID))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}
	if err := tx.Commit().Error; err != nil {
		h.logger.Error("提交删除执行记录失败", zap.Error(err), zap.Uint("run_id", run.ID))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": 1})
}

// BatchDeleteTaskRuns 批量删除执行记录
// POST /api/runs/batch-delete
func (h *TaskRunHandler) BatchDeleteTaskRuns(c *gin.Context) {
	var req batchDeleteRunsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}
	if len(req.IDs) == 0 {
		respondError(c, http.StatusBadRequest, "invalid_request", "ids不能为空", nil)
		return
	}

	ids := uniqueUintIDs(req.IDs)
	var runs []model.TaskRun
	if err := h.db.Where("id IN ?", ids).Find(&runs).Error; err != nil {
		h.logger.Error("查询执行记录失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}
	if len(runs) == 0 {
		respondError(c, http.StatusNotFound, "not_found", "执行记录不存在", nil)
		return
	}
	for _, run := range runs {
		if !isTaskRunDeletable(run.Status) {
			respondError(c, http.StatusConflict, "run_running", "存在运行中的执行记录，无法删除", nil)
			return
		}
	}

	tx := h.db.Begin()
	if err := tx.Where("task_run_id IN ?", ids).Delete(&model.TaskRunEvent{}).Error; err != nil {
		tx.Rollback()
		h.logger.Error("批量删除执行记录事件失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}
	if err := tx.Where("id IN ?", ids).Delete(&model.TaskRun{}).Error; err != nil {
		tx.Rollback()
		h.logger.Error("批量删除执行记录失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}
	if err := tx.Commit().Error; err != nil {
		h.logger.Error("提交批量删除执行记录失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": len(runs)})
}

func uniqueUintIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// ListRunEvents 获取执行事件明细
// GET /api/runs/:id/events
func (h *TaskRunHandler) ListRunEvents(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	pagination := parsePagination(c, 1, 100, 500)

	query := h.db.Model(&model.TaskRunEvent{}).
		Where("task_run_id = ?", id)

	if kind := strings.TrimSpace(c.Query("kind")); kind != "" {
		query = query.Where("kind = ?", kind)
	}
	if op := strings.TrimSpace(c.Query("op")); op != "" {
		query = query.Where("op = ?", op)
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("统计执行事件失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	var items []model.TaskRunEvent
	if err := query.Order("created_at DESC").
		Offset(pagination.Offset).
		Limit(pagination.PageSize).
		Find(&items).Error; err != nil {
		h.logger.Error("查询执行事件失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
	})
}

// CancelRun 取消正在运行的任务
// POST /api/runs/:id/cancel
func (h *TaskRunHandler) CancelRun(c *gin.Context) {
	if h.queue == nil {
		respondError(c, http.StatusInternalServerError, "queue_not_ready", "任务队列未初始化", nil)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var run model.TaskRun
	if err := h.db.First(&run, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "执行记录不存在", nil)
			return
		}
		h.logger.Error("查询执行记录失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 检查状态是否可取消
	if run.Status != "running" && run.Status != "pending" {
		respondError(c, http.StatusConflict, "invalid_status", "只能取消运行中或待执行的任务", nil)
		return
	}

	// 调用队列取消方法
	if err := h.queue.Cancel(c.Request.Context(), run.ID); err != nil {
		h.logger.Error("取消任务失败", zap.Error(err), zap.Uint("run_id", run.ID))
		respondError(c, http.StatusInternalServerError, "queue_error", "取消失败", nil)
		return
	}

	h.logger.Info("取消任务成功", zap.Uint("run_id", run.ID))

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// GetRunStats 获取运行统计信息
// GET /api/runs/stats
func (h *TaskRunHandler) GetRunStats(c *gin.Context) {
	query := h.db.Model(&model.TaskRun{})

	// job_id过滤
	if jobIDStr := strings.TrimSpace(c.Query("job_id")); jobIDStr != "" {
		id, err := strconv.ParseUint(jobIDStr, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的job_id参数", nil)
			return
		}
		query = query.Where("job_id = ?", id)
	}

	// 时间范围过滤
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		query = query.Where("started_at >= ?", from)
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		query = query.Where("started_at <= ?", to)
	}

	// 统计各状态数量
	type StatusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []StatusCount
	if err := query.Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts).Error; err != nil {
		h.logger.Error("统计状态失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 转换为map
	stats := gin.H{
		"total":     int64(0),
		"completed": int64(0),
		"failed":    int64(0),
		"cancelled": int64(0),
		"running":   int64(0),
		"pending":   int64(0),
	}
	for _, sc := range statusCounts {
		stats[sc.Status] = sc.Count
		stats["total"] = stats["total"].(int64) + sc.Count
	}

	c.JSON(http.StatusOK, stats)
}
