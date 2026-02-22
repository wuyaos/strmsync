// Package http 提供HTTP API处理器
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/strmsync/strmsync/internal/queue"
	"go.uber.org/zap"
	"gorm.io/gorm"
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

// JobSTRMMode STRM生成模式类型
type JobSTRMMode string

const (
	JobSTRMModeLocal JobSTRMMode = "local"
	JobSTRMModeURL   JobSTRMMode = "url"
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
	allowedJobSTRMModes  = []string{string(JobSTRMModeLocal), string(JobSTRMModeURL)}

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

	scheduler JobScheduler
	queue     TaskQueue
}

// JobScheduler 任务调度器接口（用于注入）
type JobScheduler interface {
	UpsertJob(ctx context.Context, job model.Job) error
	RemoveJob(ctx context.Context, jobID uint) error
}

// TaskQueue 任务队列接口（用于注入）
type TaskQueue interface {
	Enqueue(ctx context.Context, task *model.TaskRun) error
	Cancel(ctx context.Context, taskID uint) error
}

// NewJobHandler 创建任务处理器
func NewJobHandler(db *gorm.DB, logger *zap.Logger, scheduler JobScheduler, queue TaskQueue) *JobHandler {
	return &JobHandler{
		db:        db,
		logger:    logger,
		scheduler: scheduler,
		queue:     queue,
	}
}

// createJobRequest 创建任务请求体（复用创建和更新）
type jobRequest struct {
	Name          string           `json:"name"`
	Enabled       *bool            `json:"enabled"`
	Cron          string           `json:"cron"`
	WatchMode     string           `json:"watch_mode"`
	SourcePath    string           `json:"source_path"`
	RemoteRoot    string           `json:"remote_root"`
	TargetPath    string           `json:"target_path"`
	STRMPath      string           `json:"strm_path"`
	DataServerID  *uint            `json:"data_server_id"`
	MediaServerID *uint            `json:"media_server_id"`
	Options       model.JobOptions `json:"options"`
}

func buildJobLogPayload(req jobRequest) map[string]interface{} {
	payload := map[string]interface{}{
		"name":            strings.TrimSpace(req.Name),
		"enabled":         req.Enabled,
		"cron":            strings.TrimSpace(req.Cron),
		"watch_mode":      strings.TrimSpace(req.WatchMode),
		"source_path":     strings.TrimSpace(req.SourcePath),
		"remote_root":     strings.TrimSpace(req.RemoteRoot),
		"target_path":     strings.TrimSpace(req.TargetPath),
		"strm_path":       strings.TrimSpace(req.STRMPath),
		"data_server_id":  req.DataServerID,
		"media_server_id": req.MediaServerID,
	}

	payload["options"] = sanitizeOptionsForLog(req.Options)
	return payload
}

func buildOptionsLog(options any) interface{} {
	return sanitizeOptionsForLog(options)
}

func validateCronSpec(spec string, errs *[]FieldError) {
	cronSpec := strings.TrimSpace(spec)
	if cronSpec == "" {
		return
	}

	parser := cron.NewParser(
		cron.SecondOptional |
			cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)
	if _, err := parser.Parse(cronSpec); err != nil {
		*errs = append(*errs, FieldError{
			Field:   "cron",
			Message: "无效的Cron表达式",
		})
	}
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
	validateJobOptions(req.Options, &fieldErrors)
	validateCronSpec(req.Cron, &fieldErrors)

	watchMode := JobWatchMode(strings.TrimSpace(req.WatchMode))
	if watchMode == JobWatchModeAPI {
		if req.DataServerID == nil || *req.DataServerID == 0 {
			fieldErrors = append(fieldErrors, FieldError{
				Field:   "data_server_id",
				Message: "watch_mode为api时必须指定数据服务器",
			})
		}
	}

	if req.DataServerID != nil && *req.DataServerID == 0 {
		fieldErrors = append(fieldErrors, FieldError{Field: "data_server_id", Message: "无效的ID"})
	}
	if req.MediaServerID != nil && *req.MediaServerID == 0 {
		fieldErrors = append(fieldErrors, FieldError{Field: "media_server_id", Message: "无效的ID"})
	}

	return fieldErrors
}

func validateJobOptions(options model.JobOptions, errs *[]FieldError) {
	strmMode := strings.TrimSpace(options.STRMMode)
	if strmMode != "" {
		validateEnum("options.strm_mode", strmMode, allowedJobSTRMModes, errs)
	}
	metadataMode := strings.TrimSpace(options.MetadataMode)
	if metadataMode != "" {
		validateEnum("options.metadata_mode", metadataMode, []string{"copy", "download", "none"}, errs)
	}
	if options.MaxConcurrency < 0 {
		*errs = append(*errs, FieldError{Field: "options.max_concurrency", Message: "max_concurrency 不能为负数"})
	}
	if options.MinFileSize < 0 {
		*errs = append(*errs, FieldError{Field: "options.min_file_size", Message: "min_file_size 不能为负数"})
	}
	if options.Interval != nil && *options.Interval <= 0 {
		*errs = append(*errs, FieldError{Field: "options.interval", Message: "interval 必须为正数"})
	}
	if options.ModTimeEpsilonSeconds < 0 {
		*errs = append(*errs, FieldError{Field: "options.mod_time_epsilon_seconds", Message: "mod_time_epsilon_seconds 不能为负数"})
	}
}

// validateRelatedServers 校验关联服务器是否存在
func (h *JobHandler) validateRelatedServers(req *jobRequest) []FieldError {
	var fieldErrors []FieldError

	if req.DataServerID != nil && *req.DataServerID > 0 {
		var count int64
		if err := h.db.Model(&model.DataServer{}).
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
		if err := h.db.Model(&model.MediaServer{}).
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

func requiresRemoteRoot(serverType string) bool {
	switch strings.ToLower(strings.TrimSpace(serverType)) {
	case filesystem.TypeCloudDrive2.String(), filesystem.TypeOpenList.String():
		return true
	default:
		return false
	}
}

func (h *JobHandler) validateRemoteRootForServer(req *jobRequest) []FieldError {
	if req == nil {
		return nil
	}
	if strings.TrimSpace(req.WatchMode) != string(JobWatchModeAPI) {
		return nil
	}
	if req.DataServerID == nil || *req.DataServerID == 0 {
		return nil
	}
	var server model.DataServer
	if err := h.db.First(&server, *req.DataServerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []FieldError{{Field: "data_server_id", Message: "数据服务器不存在"}}
		}
		h.logger.Error("查询数据服务器失败", zap.Error(err), zap.Uint("data_server_id", *req.DataServerID))
		return []FieldError{{Field: "data_server_id", Message: "服务器验证失败"}}
	}
	if requiresRemoteRoot(server.Type) && strings.TrimSpace(req.RemoteRoot) == "" {
		return []FieldError{{Field: "remote_root", Message: "远程根目录不能为空"}}
	}
	return nil
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
	if fieldErrors := h.validateRemoteRootForServer(&req); len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 唯一性检查
	var count int64
	if err := h.db.Model(&model.Job{}).
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

	logPayload := buildJobLogPayload(req)
	h.logger.Info("创建任务请求",
		zap.String("name", strings.TrimSpace(req.Name)),
		zap.Any("payload", logPayload))

	job := model.Job{
		Name:          strings.TrimSpace(req.Name),
		Enabled:       enabled,
		Cron:          strings.TrimSpace(req.Cron),
		WatchMode:     string(JobWatchMode(strings.TrimSpace(req.WatchMode))),
		SourcePath:    strings.TrimSpace(req.SourcePath),
		RemoteRoot:    strings.TrimSpace(req.RemoteRoot),
		TargetPath:    strings.TrimSpace(req.TargetPath),
		STRMPath:      strings.TrimSpace(req.STRMPath),
		DataServerID:  req.DataServerID,
		MediaServerID: req.MediaServerID,
		Options:       req.Options,
		Status:        string(JobStatusIdle),
	}

	if err := h.db.Create(&job).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			respondError(c, http.StatusConflict, "duplicate_name", "任务名称已存在", nil)
			return
		}
		h.logger.Error("创建任务失败",
			zap.String("name", job.Name),
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
		zap.String("name", job.Name),
		zap.String("watch_mode", job.WatchMode),
		zap.String("source_path", job.SourcePath),
		zap.String("remote_root", job.RemoteRoot),
		zap.String("target_path", job.TargetPath),
		zap.String("strm_path", job.STRMPath),
		zap.Any("options", logPayload["options"]))

	// 通知调度器
	if h.scheduler != nil {
		if err := h.scheduler.UpsertJob(c.Request.Context(), job); err != nil {
			h.logger.Error("调度器更新失败", zap.Error(err), zap.Uint("job_id", job.ID))
			respondError(c, http.StatusInternalServerError, "scheduler_error", "调度器更新失败", nil)
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{"job": job})
}

// ListJobs 获取任务列表
// GET /api/jobs
func (h *JobHandler) ListJobs(c *gin.Context) {
	pagination := parsePagination(c, 1, 50, 200)

	query := h.db.Model(&model.Job{}).
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

	// strm_mode过滤
	if strmMode := strings.TrimSpace(c.Query("strm_mode")); strmMode != "" {
		if !isAllowedValue(strmMode, allowedJobSTRMModes) {
			respondError(c, http.StatusBadRequest, "invalid_request", "无效的strm_mode参数", nil)
			return
		}
		query = query.Where("options LIKE ?", fmt.Sprintf("%%\"strm_mode\":\"%s\"%%", strmMode))
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("统计任务失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 查询列表
	var jobs []model.Job
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

	var job model.Job
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
	var job model.Job
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
	if fieldErrors := h.validateRemoteRootForServer(&req); len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 唯一性检查（名称变更时）
	newName := strings.TrimSpace(req.Name)
	if newName != job.Name {
		var count int64
		if err := h.db.Model(&model.Job{}).
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

	logPayload := buildJobLogPayload(req)
	h.logger.Info("更新任务请求",
		zap.Uint("id", job.ID),
		zap.String("name", newName),
		zap.Any("payload", logPayload))

	// 更新字段
	job.Name = newName
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
	}
	job.Cron = strings.TrimSpace(req.Cron)
	job.WatchMode = string(JobWatchMode(strings.TrimSpace(req.WatchMode)))
	job.SourcePath = strings.TrimSpace(req.SourcePath)
	job.RemoteRoot = strings.TrimSpace(req.RemoteRoot)
	job.TargetPath = strings.TrimSpace(req.TargetPath)
	job.STRMPath = strings.TrimSpace(req.STRMPath)
	job.DataServerID = req.DataServerID
	job.MediaServerID = req.MediaServerID
	job.Options = req.Options

	if err := h.db.Save(&job).Error; err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			respondError(c, http.StatusConflict, "duplicate_name", "任务名称已存在", nil)
			return
		}
		h.logger.Error("更新任务失败",
			zap.String("name", job.Name),
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
		zap.String("name", job.Name),
		zap.String("watch_mode", job.WatchMode),
		zap.String("source_path", job.SourcePath),
		zap.String("remote_root", job.RemoteRoot),
		zap.String("target_path", job.TargetPath),
		zap.String("strm_path", job.STRMPath),
		zap.Any("options", logPayload["options"]))

	// 通知调度器
	if h.scheduler != nil {
		if err := h.scheduler.UpsertJob(c.Request.Context(), job); err != nil {
			h.logger.Error("调度器更新失败", zap.Error(err), zap.Uint("job_id", job.ID))
			respondError(c, http.StatusInternalServerError, "scheduler_error", "调度器更新失败", nil)
			return
		}
	}

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
	if err := h.db.Model(&model.TaskRun{}).
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

	var job model.Job
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	h.logger.Info("删除任务请求",
		zap.String("name", job.Name),
		zap.Uint64("id", id))

	result := h.db.Delete(&job)
	if result.Error != nil {
		h.logger.Error("删除任务失败",
			zap.String("name", job.Name),
			zap.Error(result.Error),
			zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}
	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
		return
	}

	h.logger.Info("删除任务成功",
		zap.String("name", job.Name),
		zap.Uint64("id", id))

	// 通知调度器
	if h.scheduler != nil {
		if err := h.scheduler.RemoveJob(c.Request.Context(), uint(id)); err != nil {
			h.logger.Error("调度器移除失败", zap.Error(err), zap.Uint64("job_id", id))
			respondError(c, http.StatusInternalServerError, "scheduler_error", "调度器更新失败", nil)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// RunJob 手动触发任务执行
// POST /api/jobs/:id/run
func (h *JobHandler) RunJob(c *gin.Context) {
	if h.queue == nil {
		respondError(c, http.StatusInternalServerError, "queue_not_ready", "任务队列未初始化", nil)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job model.Job
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}
	if !job.Enabled {
		respondError(c, http.StatusBadRequest, "job_disabled", "任务已禁用，无法运行", nil)
		return
	}

	optionsLog := buildOptionsLog(job.Options)
	h.logger.Info("手动触发任务执行请求",
		zap.Uint("job_id", job.ID),
		zap.String("name", job.Name),
		zap.String("watch_mode", job.WatchMode),
		zap.String("source_path", job.SourcePath),
		zap.String("target_path", job.TargetPath),
		zap.String("strm_path", job.STRMPath),
		zap.Any("options", optionsLog))

	// 防止重复运行：pending/running 均视为已在运行
	var runningCount int64
	if err := h.db.Model(&model.TaskRun{}).
		Where("job_id = ? AND status IN ?", job.ID, []string{string(syncqueue.TaskPending), string(syncqueue.TaskRunning)}).
		Count(&runningCount).Error; err != nil {
		h.logger.Error("检查任务运行状态失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		return
	}
	if runningCount > 0 {
		respondError(c, http.StatusConflict, "job_running", "任务已在运行中", nil)
		return
	}

	// 入队任务（手动触发：立即可执行）
	taskRun := &model.TaskRun{
		JobID:       job.ID,
		Priority:    int(syncqueue.TaskPriorityNormal),
		AvailableAt: time.Now(),
		Payload:     buildManualRunPayload(job),
	}
	if err := h.queue.Enqueue(c.Request.Context(), taskRun); err != nil {
		h.logger.Error("任务入队失败", zap.Error(err), zap.Uint("job_id", job.ID))
		respondError(c, http.StatusInternalServerError, "queue_error", "任务入队失败", nil)
		return
	}

	// 更新Job运行状态
	now := time.Now()
	if err := h.db.Model(&job).Updates(map[string]interface{}{
		"last_run_at": &now,
		"status":      string(JobStatusRunning),
	}).Error; err != nil {
		h.logger.Warn("更新任务状态失败",
			zap.String("name", job.Name),
			zap.Error(err),
			zap.Uint("job_id", job.ID))
	}

	h.logger.Info("触发任务运行成功",
		zap.Uint("job_id", job.ID),
		zap.Uint("task_run_id", taskRun.ID),
		zap.String("name", job.Name),
		zap.Any("options", optionsLog))

	c.JSON(http.StatusCreated, gin.H{"task_run": taskRun})
}

// StopJob 停止正在运行的任务
// POST /api/jobs/:id/stop
func (h *JobHandler) StopJob(c *gin.Context) {
	if h.queue == nil {
		respondError(c, http.StatusInternalServerError, "queue_not_ready", "任务队列未初始化", nil)
		return
	}

	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job model.Job
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 查找所有 pending 和 running 状态的任务（均需取消）
	var activeTasks []model.TaskRun
	if err := h.db.
		Where("job_id = ? AND status IN ?", job.ID,
			[]string{string(syncqueue.TaskPending), string(syncqueue.TaskRunning)}).
		Order("id DESC").
		Find(&activeTasks).Error; err != nil {
		h.logger.Error("查询活跃任务失败", zap.Error(err), zap.Uint("job_id", job.ID))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}
	if len(activeTasks) == 0 {
		respondError(c, http.StatusConflict, "no_running_task", "没有正在运行的任务", nil)
		return
	}

	// 取消所有活跃任务，记录失败但继续处理其余任务
	var failedIDs []uint
	for _, task := range activeTasks {
		if err := h.queue.Cancel(c.Request.Context(), task.ID); err != nil {
			h.logger.Error("取消任务失败", zap.Error(err), zap.Uint("task_id", task.ID))
			failedIDs = append(failedIDs, task.ID)
		}
	}
	if len(failedIDs) > 0 {
		respondError(c, http.StatusInternalServerError, "queue_error",
			fmt.Sprintf("部分任务取消失败: %v", failedIDs), nil)
		return
	}

	// 重新查询最新状态（Cancel 可能已更新 DB）
	taskIDs := make([]uint, 0, len(activeTasks))
	for _, t := range activeTasks {
		taskIDs = append(taskIDs, t.ID)
	}
	var cancelledTasks []model.TaskRun
	if err := h.db.Where("id IN ?", taskIDs).Order("id DESC").Find(&cancelledTasks).Error; err != nil {
		h.logger.Error("查询取消结果失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 更新Job状态为idle
	if err := h.db.Model(&job).Update("status", string(JobStatusIdle)).Error; err != nil {
		h.logger.Warn("更新任务状态失败",
			zap.String("name", job.Name),
			zap.Error(err),
			zap.Uint("job_id", job.ID))
	}

	h.logger.Info("停止任务成功",
		zap.Uint("job_id", job.ID),
		zap.String("name", job.Name),
		zap.Int("cancelled_count", len(cancelledTasks)))

	c.JSON(http.StatusOK, gin.H{
		"task_runs": cancelledTasks,
		"cancelled": len(cancelledTasks),
		"message":   fmt.Sprintf("已取消 %d 个任务", len(cancelledTasks)),
	})
}

// EnableJob 启用任务
// PUT /api/jobs/:id/enable
func (h *JobHandler) EnableJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job model.Job
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	if job.Enabled {
		c.JSON(http.StatusOK, gin.H{"job": job})
		return
	}

	if err := h.db.Model(&job).Update("enabled", true).Error; err != nil {
		h.logger.Error("启用任务失败",
			zap.String("name", job.Name),
			zap.Error(err),
			zap.Uint("job_id", job.ID))
		respondError(c, http.StatusInternalServerError, "db_error", "更新失败", nil)
		return
	}
	job.Enabled = true

	h.logger.Info("启用任务成功",
		zap.Uint("job_id", job.ID),
		zap.String("name", job.Name))

	// 通知调度器
	if h.scheduler != nil {
		if err := h.scheduler.UpsertJob(c.Request.Context(), job); err != nil {
			h.logger.Error("调度器更新失败", zap.Error(err), zap.Uint("job_id", job.ID))
			respondError(c, http.StatusInternalServerError, "scheduler_error", "调度器更新失败", nil)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"job": job})
}

// DisableJob 禁用任务
// PUT /api/jobs/:id/disable
func (h *JobHandler) DisableJob(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var job model.Job
	if err := h.db.First(&job, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "任务不存在", nil)
			return
		}
		h.logger.Error("查询任务失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	if !job.Enabled {
		c.JSON(http.StatusOK, gin.H{"job": job})
		return
	}

	if err := h.db.Model(&job).Update("enabled", false).Error; err != nil {
		h.logger.Error("禁用任务失败",
			zap.String("name", job.Name),
			zap.Error(err),
			zap.Uint("job_id", job.ID))
		respondError(c, http.StatusInternalServerError, "db_error", "更新失败", nil)
		return
	}
	job.Enabled = false

	h.logger.Info("禁用任务成功",
		zap.Uint("job_id", job.ID),
		zap.String("name", job.Name))

	// 通知调度器
	if h.scheduler != nil {
		if err := h.scheduler.RemoveJob(c.Request.Context(), job.ID); err != nil {
			h.logger.Error("调度器移除失败", zap.Error(err), zap.Uint("job_id", job.ID))
			respondError(c, http.StatusInternalServerError, "scheduler_error", "调度器更新失败", nil)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"job": job})
}

func buildManualRunPayload(job model.Job) string {
	payload := map[string]any{
		"job_id":       job.ID,
		"job_name":     job.Name,
		"trigger":      "manual",
		"triggered_at": time.Now().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
