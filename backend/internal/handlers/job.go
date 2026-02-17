// Package handlers 提供HTTP API处理器
package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/database"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JobWatchMode 任务监控模式类型
type JobWatchMode string

const (
	JobWatchModeLocal JobWatchMode = "local"
	JobWatchModeAPI   JobWatchMode = "api"
)

// JobStatus 任务状态类型
type JobStatus string

const (
	JobStatusIdle    JobStatus = "idle"
	JobStatusRunning JobStatus = "running"
	JobStatusError   JobStatus = "error"
)

// TaskRunStatus 执行记录状态类型
type TaskRunStatus string

const (
	TaskRunStatusRunning   TaskRunStatus = "running"
	TaskRunStatusCompleted TaskRunStatus = "completed"
	TaskRunStatusFailed    TaskRunStatus = "failed"
	TaskRunStatusCancelled TaskRunStatus = "cancelled"
)

var (
	allowedJobWatchModes = []string{string(JobWatchModeLocal), string(JobWatchModeAPI)}
	allowedJobStatuses   = []string{string(JobStatusIdle), string(JobStatusRunning), string(JobStatusError)}

	// 内部sentinel错误，用于在事务中传递业务逻辑错误
	errJobDisabled       = errors.New("job_disabled")
	errJobAlreadyRunning = errors.New("job_already_running")
	errNoRunningTask     = errors.New("no_running_task")
)

// isAllowedValue 检查值是否在允许的枚举列表中
func isAllowedValue(value string, allowed []string) bool {
	for _, v := range allowed {
		if value == v {
			return true
		}
	}
	return false
}

// JobHandler 任务处理器
type JobHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewJobHandler 创建任务处理器
func NewJobHandler(db *gorm.DB, logger *zap.Logger) *JobHandler {
	return &JobHandler{
		db:     db,
		logger: logger,
	}
}

// createJobRequest 创建任务请求体（复用创建和更新）
type jobRequest struct {
	Name          string `json:"name"`
	Enabled       *bool  `json:"enabled"`
	WatchMode     string `json:"watch_mode"`
	SourcePath    string `json:"source_path"`
	TargetPath    string `json:"target_path"`
	STRMPath      string `json:"strm_path"`
	DataServerID  *uint  `json:"data_server_id"`
	MediaServerID *uint  `json:"media_server_id"`
	Options       string `json:"options"`
}

// validateJobRequest 验证任务请求参数，返回字段错误列表
func validateJobRequest(req *jobRequest) []FieldError {
	var fieldErrors []FieldError

	validateRequiredString("name", req.Name, &fieldErrors)
	validateRequiredString("watch_mode", req.WatchMode, &fieldErrors)
	validateRequiredString("source_path", req.SourcePath, &fieldErrors)
	validateRequiredString("target_path", req.TargetPath, &fieldErrors)
	validateRequiredString("strm_path", req.STRMPath, &fieldErrors)
	validateEnum("watch_mode", req.WatchMode, allowedJobWatchModes, &fieldErrors)
	validateJSONString("options", req.Options, &fieldErrors)

	watchMode := JobWatchMode(strings.TrimSpace(req.WatchMode))
	if watchMode == JobWatchModeAPI {
		if req.DataServerID == nil || *req.DataServerID == 0 {
			fieldErrors = append(fieldErrors, FieldError{
				Field:   "data_server_id",
				Message: "watch_mode为api时必须指定数据服务器",
			})
		}
	}

	// watch_mode=local时，强制清空data_server_id（清理不一致配置）
	if watchMode == JobWatchModeLocal {
		req.DataServerID = nil
	}

	if req.DataServerID != nil && *req.DataServerID == 0 {
		fieldErrors = append(fieldErrors, FieldError{Field: "data_server_id", Message: "无效的ID"})
	}
	if req.MediaServerID != nil && *req.MediaServerID == 0 {
		fieldErrors = append(fieldErrors, FieldError{Field: "media_server_id", Message: "无效的ID"})
	}

	return fieldErrors
}

// validateRelatedServers 校验关联服务器是否存在
func (h *JobHandler) validateRelatedServers(req *jobRequest) []FieldError {
	var fieldErrors []FieldError

	if req.DataServerID != nil && *req.DataServerID > 0 {
		var count int64
		if err := h.db.Model(&database.DataServer{}).
			Where("id = ?", *req.DataServerID).
			Count(&count).Error; err != nil {
			h.logger.Error("检查数据服务器存在性失败", zap.Error(err))
			return []FieldError{{Field: "data_server_id", Message: "服务器验证失败"}}
		}
		if count == 0 {
			fieldErrors = append(fieldErrors, FieldError{Field: "data_server_id", Message: "数据服务器不存在"})
		}
	}

	if req.MediaServerID != nil && *req.MediaServerID > 0 {
		var count int64
		if err := h.db.Model(&database.MediaServer{}).
			Where("id = ?", *req.MediaServerID).
			Count(&count).Error; err != nil {
			h.logger.Error("检查媒体服务器存在性失败", zap.Error(err))
			return []FieldError{{Field: "media_server_id", Message: "服务器验证失败"}}
		}
		if count == 0 {
			fieldErrors = append(fieldErrors, FieldError{Field: "media_server_id", Message: "媒体服务器不存在"})
		}
	}

	return fieldErrors
}

// CreateJob 创建任务
// POST /api/jobs
func (h *JobHandler) CreateJob(c *gin.Context) {
	var req jobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证
	if fieldErrors := validateJobRequest(&req); len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 关联服务器存在性验证
	if fieldErrors := h.validateRelatedServers(&req); len(fieldErrors) > 0 {
		if len(fieldErrors) == 1 && fieldErrors[0].Message == "服务器验证失败" {
			respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		} else {
			respondValidationError(c, fieldErrors)
		}
		return
	}

	// 唯一性检查
	var count int64
	if err := h.db.Model(&database.Job{}).
		Where("name = ?", strings.TrimSpace(req.Name)).
		Count(&count).Error; err != nil {
		h.logger.Error("检查任务名称唯一性失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		return
	}
	if count > 0 {
		respondError(c, http.StatusConflict, "duplicate_name", "任务名称已存在", nil)
		return
	}

	// 设置默认值
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	job := database.Job{
		Name:          strings.TrimSpace(req.Name),
		Enabled:       enabled,
		WatchMode:     string(JobWatchMode(strings.TrimSpace(req.WatchMode))),
		SourcePath:    strings.TrimSpace(req.SourcePath),
		TargetPath:    strings.TrimSpace(req.TargetPath),
		STRMPath:      strings.TrimSpace(req.STRMPath),
		DataServerID:  req.DataServerID,
		MediaServerID: req.MediaServerID,
		Options:       strings.TrimSpace(req.Options),
		Status:        string(JobStatusIdle),
	}

	if err := h.db.Create(&job).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			respondError(c, http.StatusConflict, "duplicate_name", "任务名称已存在", nil)
			return
		}
		h.logger.Error("创建任务失败",
			zap.Error(err),
			zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
				"name":       job.Name,
				"watch_mode": job.WatchMode,
				"source":     job.SourcePath,
				"target":     job.TargetPath,
			})))
		respondError(c, http.StatusInternalServerError, "db_error", "创建失败", nil)
		return
	}

	h.logger.Info("创建任务成功",
		zap.Uint("id", job.ID),
		zap.String("name", job.Name))

	c.JSON(http.StatusCreated, gin.H{"job": job})
}

// ListJobs 获取任务列表
// GET /api/jobs
func (h *JobHandler) ListJobs(c *gin.Context) {
	pagination := parsePagination(c, 1, 50, 200)

	query := h.db.Model(&database.Job{}).
		Preload("DataServer").
		Preload("MediaServer")

	// 名称模糊搜索
	if name := strings.TrimSpace(c.Query("name")); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// 启用状态过滤（严格校验，非法值返回400）
	if enabled := strings.TrimSpace(c.Query("enabled")); enabled != "" {
		switch enabled {
		case "true":
			query = query.Where("enabled = ?", true)
		case "false":
			query = query.Where("enabled = ?", false)
		default:
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的enabled参数，仅接受 true 或 false", nil)
			return
		}
	}

	// watch_mode过滤（验证枚举值）
	if watchMode := strings.TrimSpace(c.Query("watch_mode")); watchMode != "" {
		if !isAllowedValue(watchMode, allowedJobWatchModes) {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的watch_mode参数", nil)
			return
		}
		query = query.Where("watch_mode = ?", watchMode)
	}

	// status过滤（验证枚举值）
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		if !isAllowedValue(status, allowedJobStatuses) {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的status参数", nil)
			return
		}
		query = query.Where("status = ?", status)
	}

	// data_server_id过滤
	if dataServerID := strings.TrimSpace(c.Query("data_server_id")); dataServerID != "" {
		id, err := strconv.ParseUint(dataServerID, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的data_server_id参数", nil)
			return
		}
		query = query.Where("data_server_id = ?", id)
	}

	// media_server_id过滤
	if mediaServerID := strings.TrimSpace(c.Query("media_server_id")); mediaServerID != "" {
		id, err := strconv.ParseUint(mediaServerID, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的media_server_id参数", nil)
			return
		}
		query = query.Where("media_server_id = ?", id)
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("统计任务失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 查询列表
	var jobs []database.Job
	if err := query.Order("created_at DESC").
		Offset(pagination.Offset).
		Limit(pagination.PageSize).
		Find(&jobs).Error; err != nil {
		h.logger.Error("查询任务列表失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"jobs":      jobs,
		"total":     total,
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
	})
}

// GetJob 获取单个任务
// GET /api/jobs/:id
func (h *JobHandler) GetJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job database.Job
	if err := h.db.
		Preload("DataServer").
		Preload("MediaServer").
		First(&job, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"job": job})
}

// UpdateJob 更新任务
// PUT /api/jobs/:id
func (h *JobHandler) UpdateJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var req jobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证
	if fieldErrors := validateJobRequest(&req); len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 查询现有记录
	var job database.Job
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 关联服务器存在性验证
	if fieldErrors := h.validateRelatedServers(&req); len(fieldErrors) > 0 {
		if len(fieldErrors) == 1 && fieldErrors[0].Message == "服务器验证失败" {
			respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		} else {
			respondValidationError(c, fieldErrors)
		}
		return
	}

	// 唯一性检查（名称变更时）
	newName := strings.TrimSpace(req.Name)
	if newName != job.Name {
		var count int64
		if err := h.db.Model(&database.Job{}).
			Where("name = ? AND id <> ?", newName, job.ID).
			Count(&count).Error; err != nil {
			h.logger.Error("检查任务名称唯一性失败", zap.Error(err))
			respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
			return
		}
		if count > 0 {
			respondError(c, http.StatusConflict, "duplicate_name", "任务名称已存在", nil)
			return
		}
	}

	// 更新字段
	job.Name = newName
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
	}
	job.WatchMode = string(JobWatchMode(strings.TrimSpace(req.WatchMode)))
	job.SourcePath = strings.TrimSpace(req.SourcePath)
	job.TargetPath = strings.TrimSpace(req.TargetPath)
	job.STRMPath = strings.TrimSpace(req.STRMPath)
	job.DataServerID = req.DataServerID
	job.MediaServerID = req.MediaServerID
	job.Options = strings.TrimSpace(req.Options)

	if err := h.db.Save(&job).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			respondError(c, http.StatusConflict, "duplicate_name", "任务名称已存在", nil)
			return
		}
		h.logger.Error("更新任务失败",
			zap.Error(err),
			zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
				"id":         job.ID,
				"name":       job.Name,
				"watch_mode": job.WatchMode,
			})))
		respondError(c, http.StatusInternalServerError, "db_error", "更新失败", nil)
		return
	}

	h.logger.Info("更新任务成功",
		zap.Uint("id", job.ID),
		zap.String("name", job.Name))

	c.JSON(http.StatusOK, gin.H{"job": job})
}

// DeleteJob 删除任务
// DELETE /api/jobs/:id
func (h *JobHandler) DeleteJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	// 检查是否存在running的TaskRun
	var runningCount int64
	if err := h.db.Model(&database.TaskRun{}).
		Where("job_id = ? AND status = ?", id, string(TaskRunStatusRunning)).
		Count(&runningCount).Error; err != nil {
		h.logger.Error("检查任务运行状态失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		return
	}
	if runningCount > 0 {
		respondError(c, http.StatusConflict, "job_running", "任务正在运行，无法删除", nil)
		return
	}

	result := h.db.Delete(&database.Job{}, uint(id))
	if result.Error != nil {
		h.logger.Error("删除任务失败", zap.Error(result.Error), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
		return
	}

	h.logger.Info("删除任务成功", zap.Uint64("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// RunJob 手动触发任务执行
// POST /api/jobs/:id/run
func (h *JobHandler) RunJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job database.Job
	var taskRun database.TaskRun

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		// SELECT FOR UPDATE：锁定行，防止并发RunJob产生多个running记录
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&job, uint(id)).Error; err != nil {
			return err
		}

		if !job.Enabled {
			return errJobDisabled
		}

		var runningCount int64
		if err := tx.Model(&database.TaskRun{}).
			Where("job_id = ? AND status = ?", job.ID, string(TaskRunStatusRunning)).
			Count(&runningCount).Error; err != nil {
			return err
		}
		if runningCount > 0 {
			return errJobAlreadyRunning
		}

		taskRun = database.TaskRun{
			JobID:     job.ID,
			Status:    string(TaskRunStatusRunning),
			StartedAt: time.Now(),
		}
		if err := tx.Create(&taskRun).Error; err != nil {
			return err
		}

		// 同步更新Job的last_run_at和status
		now := time.Now()
		if err := tx.Model(&job).Updates(map[string]interface{}{
			"last_run_at": &now,
			"status":      string(JobStatusRunning),
		}).Error; err != nil {
			return err
		}

		// 手动同步struct字段（防止未来误用）
		job.LastRunAt = &now
		job.Status = string(JobStatusRunning)

		return nil
	})

	if txErr != nil {
		switch {
		case txErr == gorm.ErrRecordNotFound:
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
		case errors.Is(txErr, errJobDisabled):
			respondError(c, http.StatusBadRequest, "job_disabled", "任务已禁用，无法运行", nil)
		case errors.Is(txErr, errJobAlreadyRunning):
			respondError(c, http.StatusConflict, "job_running", "任务已在运行中", nil)
		default:
			h.logger.Error("触发任务运行失败", zap.Error(txErr), zap.Uint64("id", id))
			respondError(c, http.StatusInternalServerError, "db_error", "运行失败", nil)
		}
		return
	}

	h.logger.Info("触发任务运行成功",
		zap.Uint("job_id", job.ID),
		zap.Uint("task_run_id", taskRun.ID))

	c.JSON(http.StatusCreated, gin.H{"task_run": taskRun})
}

// StopJob 停止正在运行的任务
// POST /api/jobs/:id/stop
func (h *JobHandler) StopJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job database.Job
	var taskRun database.TaskRun

	txErr := h.db.Transaction(func(tx *gorm.DB) error {
		// 锁定Job记录（在SQLite中会获取写锁）
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&job, uint(id)).Error; err != nil {
			return err
		}

		now := time.Now()

		// 使用单条SQL批量更新所有running记录为cancelled
		// SQLite中，使用CASE保护负duration: duration = MAX(0, seconds)
		result := tx.Exec(`
			UPDATE task_runs
			SET status = ?,
				ended_at = ?,
				duration = CASE
					WHEN CAST((julianday(?) - julianday(started_at)) * 86400 AS INTEGER) < 0 THEN 0
					ELSE CAST((julianday(?) - julianday(started_at)) * 86400 AS INTEGER)
				END
			WHERE job_id = ? AND status = ?`,
			string(TaskRunStatusCancelled),
			now,
			now,
			now,
			job.ID,
			string(TaskRunStatusRunning))

		if result.Error != nil {
			return result.Error
		}

		// 检查是否有记录被更新（用RowsAffected消除竞态）
		if result.RowsAffected == 0 {
			return errNoRunningTask
		}

		// 如果取消了多条running记录，记录警告
		if result.RowsAffected > 1 {
			h.logger.Warn("检测到多条running记录，统一取消",
				zap.Uint("job_id", job.ID),
				zap.Int64("count", result.RowsAffected))
		}

		// 查询最新的已取消记录（用于返回）
		if err := tx.Where("job_id = ? AND status = ?", job.ID, string(TaskRunStatusCancelled)).
			Order("started_at DESC").
			First(&taskRun).Error; err != nil {
			return err
		}

		// 更新Job状态为idle
		if err := tx.Model(&job).Update("status", string(JobStatusIdle)).Error; err != nil {
			return err
		}

		return nil
	})

	if txErr != nil {
		switch {
		case txErr == gorm.ErrRecordNotFound:
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
		case errors.Is(txErr, errNoRunningTask):
			respondError(c, http.StatusConflict, "no_running_task", "没有正在运行的任务", nil)
		default:
			h.logger.Error("停止任务失败", zap.Error(txErr), zap.Uint64("id", id))
			respondError(c, http.StatusInternalServerError, "db_error", "停止失败", nil)
		}
		return
	}

	h.logger.Info("停止任务成功",
		zap.Uint("job_id", job.ID),
		zap.Uint("task_run_id", taskRun.ID))

	c.JSON(http.StatusOK, gin.H{
		"task_run": taskRun,
		"message":  fmt.Sprintf("任务已停止，执行时长: %d秒", taskRun.Duration),
	})
}
