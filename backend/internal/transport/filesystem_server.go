// Package http 提供HTTP API处理器
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	cd2sdk "github.com/strmsync/strmsync/internal/pkg/sdk/clouddrive2"
	openlistsdk "github.com/strmsync/strmsync/internal/pkg/sdk/openlist"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DataServerHandler 数据服务器处理器
type DataServerHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// dataServerRequest 数据服务器请求结构
type dataServerRequest struct {
	Name    string                  `json:"name"`
	Type    string                  `json:"type"` // local/clouddrive2/openlist
	Host    string                  `json:"host"`
	Port    int                     `json:"port"`
	APIKey  string                  `json:"api_key"`
	Enabled *bool                   `json:"enabled"`
	Options model.DataServerOptions `json:"options"`
	// 高级配置（独立列，可覆盖全局默认值，0 表示使用全局）
	DownloadRatePerSec  *int `json:"download_rate_per_sec,omitempty"`
	APIRate             *int `json:"api_rate,omitempty"`
	APIConcurrency      *int `json:"api_concurrency,omitempty"`
	APIRetryMax         *int `json:"api_retry_max,omitempty"`
	APIRetryIntervalSec *int `json:"api_retry_interval_sec,omitempty"`
}

// NewDataServerHandler 创建数据服务器处理器
func NewDataServerHandler(db *gorm.DB, logger *zap.Logger) *DataServerHandler {
	return &DataServerHandler{
		db:     db,
		logger: logger,
	}
}

// CreateDataServer 创建数据服务器
// POST /api/servers/data
func (h *DataServerHandler) CreateDataServer(c *gin.Context) {
	var req dataServerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证（使用类型特定验证器）
	fieldErrors := validateDataServerRequest(req.Name, req.Type, req.Host, req.Port, req.APIKey, req.Options)
	if len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 唯一性检查
	var count int64
	if err := h.db.Model(&model.DataServer{}).
		Where("name = ?", strings.TrimSpace(req.Name)).
		Count(&count).Error; err != nil {
		h.logger.Error("检查数据服务器名称唯一性失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		return
	}
	if count > 0 {
		respondError(c, http.StatusConflict, "duplicate_name", "服务器名称已存在", nil)
		return
	}

	// 设置默认值
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	// 高级配置默认值（允许前端不传，使用数据库默认值）
	downloadRatePerSec := 0
	if req.DownloadRatePerSec != nil {
		downloadRatePerSec = *req.DownloadRatePerSec
	}
	apiRate := 0
	if req.APIRate != nil {
		apiRate = *req.APIRate
	}
	apiConcurrency := 0
	if req.APIConcurrency != nil {
		apiConcurrency = *req.APIConcurrency
	}
	apiRetryMax := 0
	if req.APIRetryMax != nil {
		apiRetryMax = *req.APIRetryMax
	}
	apiRetryIntervalSec := 0
	if req.APIRetryIntervalSec != nil {
		apiRetryIntervalSec = *req.APIRetryIntervalSec
	}

	// 创建数据服务器
	server := model.DataServer{
		Name:                strings.TrimSpace(req.Name),
		Type:                strings.TrimSpace(req.Type),
		Host:                strings.TrimSpace(req.Host),
		Port:                req.Port,
		APIKey:              strings.TrimSpace(req.APIKey),
		Enabled:             enabled,
		Options:             req.Options,
		DownloadRatePerSec:  downloadRatePerSec,
		APIRate:             apiRate,
		APIConcurrency:      apiConcurrency,
		APIRetryMax:         apiRetryMax,
		APIRetryIntervalSec: apiRetryIntervalSec,
	}

	// Local 类型特殊处理：强制设置 host 和 port
	if strings.EqualFold(server.Type, "local") {
		server.Host = "localhost"
		server.Port = 0
	}

	h.logger.Debug(fmt.Sprintf("创建数据服务器请求：%s", server.Name),
		zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
			"name":                   server.Name,
			"type":                   server.Type,
			"host":                   server.Host,
			"port":                   server.Port,
			"enabled":                server.Enabled,
			"options":                buildOptionsLog(server.Options),
			"download_rate_per_sec":  server.DownloadRatePerSec,
			"api_rate":               server.APIRate,
			"api_concurrency":        server.APIConcurrency,
			"api_retry_max":          server.APIRetryMax,
			"api_retry_interval_sec": server.APIRetryIntervalSec,
		})))

	if err := h.db.Create(&server).Error; err != nil {
		// 检查是否为唯一约束错误
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			// 区分是名称重复还是配置重复（UID冲突）
			if strings.Contains(err.Error(), "uid") || strings.Contains(err.Error(), "UID") {
				respondError(c, http.StatusConflict, "duplicate_config", "相同配置的服务器已存在，请修改服务器配置", nil)
				return
			}
			respondError(c, http.StatusConflict, "duplicate_name", "服务器名称已存在", nil)
			return
		}
		h.logger.Error(fmt.Sprintf("创建数据服务器「%s」失败", server.Name),
			zap.Error(err),
			zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
				"name":    server.Name,
				"type":    server.Type,
				"host":    server.Host,
				"port":    server.Port,
				"api_key": server.APIKey,
			})))
		respondError(c, http.StatusInternalServerError, "db_error", "创建失败", nil)
		return
	}

	h.logger.Info(fmt.Sprintf("创建数据服务器「%s」成功", server.Name),
		zap.Uint("id", server.ID),
		zap.String("name", server.Name),
		zap.String("type", server.Type),
		zap.String("host", server.Host),
		zap.Int("port", server.Port),
		zap.Bool("enabled", server.Enabled),
		zap.Any("options", buildOptionsLog(server.Options)))

	c.JSON(http.StatusCreated, gin.H{"server": server})
}

// ListDataServers 获取数据服务器列表
// GET /api/servers/data
func (h *DataServerHandler) ListDataServers(c *gin.Context) {
	pagination := parsePagination(c, 1, 50, 200)

	query := h.db.Model(&model.DataServer{})

	// 类型过滤
	if serverType := strings.TrimSpace(c.Query("type")); serverType != "" {
		query = query.Where("type = ?", serverType)
	}

	// 启用状态过滤
	if enabled := strings.TrimSpace(c.Query("enabled")); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	// 统计总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		h.logger.Error("统计数据服务器失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 查询列表
	var servers []model.DataServer
	if err := query.Order("created_at DESC").
		Offset(pagination.Offset).
		Limit(pagination.PageSize).
		Find(&servers).Error; err != nil {
		h.logger.Error("查询数据服务器列表失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"servers":   servers,
		"total":     total,
		"page":      pagination.Page,
		"page_size": pagination.PageSize,
	})
}

// GetDataServer 获取单个数据服务器
// GET /api/servers/data/:id
func (h *DataServerHandler) GetDataServer(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var server model.DataServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "数据服务器不存在", nil)
			return
		}
		h.logger.Error("查询数据服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"server": server})
}

// UpdateDataServer 更新数据服务器
// PUT /api/servers/data/:id
func (h *DataServerHandler) UpdateDataServer(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var req dataServerRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证（使用类型特定验证器）
	fieldErrors := validateDataServerRequest(req.Name, req.Type, req.Host, req.Port, req.APIKey, req.Options)
	if len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 查询现有记录
	var server model.DataServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "数据服务器不存在", nil)
			return
		}
		h.logger.Error("查询数据服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 唯一性检查（名称变更时）
	newName := strings.TrimSpace(req.Name)
	if newName != server.Name {
		var count int64
		if err := h.db.Model(&model.DataServer{}).
			Where("name = ? AND id <> ?", newName, server.ID).
			Count(&count).Error; err != nil {
			h.logger.Error("检查数据服务器名称唯一性失败", zap.Error(err))
			respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
			return
		}
		if count > 0 {
			respondError(c, http.StatusConflict, "duplicate_name", "服务器名称已存在", nil)
			return
		}
	}

	// 更新字段
	previousEnabled := server.Enabled
	server.Name = newName
	server.Type = strings.TrimSpace(req.Type)
	server.Host = strings.TrimSpace(req.Host)
	server.Port = req.Port
	server.APIKey = strings.TrimSpace(req.APIKey)
	server.Options = req.Options
	if req.Enabled != nil {
		server.Enabled = *req.Enabled
	}

	// 更新高级配置（如果前端提供）
	if req.DownloadRatePerSec != nil {
		server.DownloadRatePerSec = *req.DownloadRatePerSec
	}
	if req.APIRate != nil {
		server.APIRate = *req.APIRate
	}
	if req.APIConcurrency != nil {
		server.APIConcurrency = *req.APIConcurrency
	}
	if req.APIRetryMax != nil {
		server.APIRetryMax = *req.APIRetryMax
	}
	if req.APIRetryIntervalSec != nil {
		server.APIRetryIntervalSec = *req.APIRetryIntervalSec
	}

	// Local 类型特殊处理：强制设置 host 和 port
	if strings.EqualFold(server.Type, "local") {
		server.Host = "localhost"
		server.Port = 0
	}

	h.logger.Debug(fmt.Sprintf("更新数据服务器请求：%s", server.Name),
		zap.Uint("id", server.ID),
		zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
			"name":                   server.Name,
			"type":                   server.Type,
			"host":                   server.Host,
			"port":                   server.Port,
			"enabled":                server.Enabled,
			"options":                buildOptionsLog(server.Options),
			"download_rate_per_sec":  server.DownloadRatePerSec,
			"api_rate":               server.APIRate,
			"api_concurrency":        server.APIConcurrency,
			"api_retry_max":          server.APIRetryMax,
			"api_retry_interval_sec": server.APIRetryIntervalSec,
		})))

	if err := h.db.Save(&server).Error; err != nil {
		// 检查是否为唯一约束错误
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			// 区分是名称重复还是配置重复（UID冲突）
			if strings.Contains(err.Error(), "uid") || strings.Contains(err.Error(), "UID") {
				respondError(c, http.StatusConflict, "duplicate_config", "相同配置的服务器已存在，请修改服务器配置", nil)
				return
			}
			respondError(c, http.StatusConflict, "duplicate_name", "服务器名称已存在", nil)
			return
		}
		h.logger.Error(fmt.Sprintf("更新数据服务器「%s」失败", server.Name),
			zap.Error(err),
			zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
				"id":      server.ID,
				"name":    server.Name,
				"type":    server.Type,
				"host":    server.Host,
				"port":    server.Port,
				"api_key": server.APIKey,
			})))
		respondError(c, http.StatusInternalServerError, "db_error", "更新失败", nil)
		return
	}

	h.logger.Info(fmt.Sprintf("更新数据服务器「%s」成功", server.Name),
		zap.Uint("id", server.ID),
		zap.String("name", server.Name),
		zap.String("type", server.Type),
		zap.String("host", server.Host),
		zap.Int("port", server.Port),
		zap.Bool("enabled", server.Enabled))

	if previousEnabled != server.Enabled {
		h.logger.Info(fmt.Sprintf("数据服务器状态变更：%s", server.Name),
			zap.Uint("id", server.ID),
			zap.String("name", server.Name),
			zap.Bool("enabled", server.Enabled))
	}

	c.JSON(http.StatusOK, gin.H{"server": server})
}

// DeleteDataServer 删除数据服务器
// DELETE /api/servers/data/:id
func (h *DataServerHandler) DeleteDataServer(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	// 检查是否被Job引用
	var count int64
	if err := h.db.Model(&model.Job{}).
		Where("data_server_id = ?", id).
		Count(&count).Error; err != nil {
		h.logger.Error("检查数据服务器引用失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		return
	}
	if count > 0 {
		respondError(c, http.StatusConflict, "in_use",
			fmt.Sprintf("存在 %d 个任务正在使用该数据服务器，无法删除", count), nil)
		return
	}

	var server model.DataServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "数据服务器不存在", nil)
			return
		}
		h.logger.Error("查询数据服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 执行删除
	h.logger.Debug(fmt.Sprintf("删除数据服务器请求：%s", server.Name), zap.Uint64("id", id))
	result := h.db.Delete(&server)
	if result.Error != nil {
		h.logger.Error(fmt.Sprintf("删除数据服务器「%s」失败", server.Name), zap.Error(result.Error), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}

	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "数据服务器不存在", nil)
		return
	}

	h.logger.Info(fmt.Sprintf("删除数据服务器「%s」成功", server.Name), zap.Uint64("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// TestDataServerConnection 测试数据服务器连接
// POST /api/servers/data/:id/test
func (h *DataServerHandler) TestDataServerConnection(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var server model.DataServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "数据服务器不存在", nil)
			return
		}
		h.logger.Error("查询数据服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 检查是否已启用
	if !server.Enabled {
		h.logger.Debug(fmt.Sprintf("尝试测试已禁用的数据服务器：%s", server.Name),
			zap.Uint("id", server.ID),
			zap.String("name", server.Name))
		c.JSON(http.StatusOK, ConnectionTestResult{
			Success: false,
			Message: "数据服务器已禁用，建议启用后再测试",
		})
		return
	}

	// SSRF防护
	allowed, isPrivate, message := validateHostForSSRF(server.Host)
	if !allowed {
		h.logger.Warn("拒绝测试危险地址",
			zap.Uint("id", server.ID),
			zap.String("host", server.Host),
			zap.String("reason", message))
		respondError(c, http.StatusBadRequest, "invalid_host", message, nil)
		return
	}

	// 如果是内网地址，记录警告但允许访问
	if isPrivate {
		h.logger.Debug("测试内网地址（SSRF风险）",
			zap.Uint("id", server.ID),
			zap.String("name", server.Name),
			zap.String("host", server.Host),
			zap.Int("port", server.Port))
	}

	h.logger.Debug(fmt.Sprintf("开始测试数据服务器连接：%s", server.Name),
		zap.Uint("id", server.ID),
		zap.String("name", server.Name),
		zap.String("type", server.Type),
		zap.String("host", server.Host),
		zap.Int("port", server.Port))

	var result ConnectionTestResult
	switch strings.TrimSpace(server.Type) {
	case "clouddrive2":
		result = testCloudDrive2Connection(server, h.logger)
	case "openlist":
		result = testOpenListConnection(server, h.logger)
	default:
		respondError(c, http.StatusBadRequest, "invalid_type", "不支持的服务器类型", nil)
		return
	}

	h.logger.Debug(fmt.Sprintf("测试数据服务器连接完成：%s", server.Name),
		zap.Uint("id", server.ID),
		zap.String("type", server.Type),
		zap.Bool("success", result.Success),
		zap.Int64("latency_ms", result.LatencyMs))

	c.JSON(http.StatusOK, result)
}

// TestDataServerTemp 临时测试数据服务器连接（未保存）
// POST /api/servers/data/test
func (h *DataServerHandler) TestDataServerTemp(c *gin.Context) {
	var req dataServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证（使用类型特定验证器）
	fieldErrors := validateDataServerRequest(req.Name, req.Type, req.Host, req.Port, req.APIKey, req.Options)
	if len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 组装临时数据服务器（不保存）
	server := model.DataServer{
		Name:    strings.TrimSpace(req.Name),
		Type:    strings.TrimSpace(req.Type),
		Host:    strings.TrimSpace(req.Host),
		Port:    req.Port,
		APIKey:  strings.TrimSpace(req.APIKey),
		Enabled: true,
		Options: req.Options,
	}

	// Local 类型特殊处理：强制设置 host 和 port
	if strings.EqualFold(server.Type, "local") {
		server.Host = "localhost"
		server.Port = 0
	}

	h.logger.Debug(fmt.Sprintf("临时测试数据服务器连接请求：%s", server.Name),
		zap.Any("payload", sanitizeMapForLog(map[string]interface{}{
			"name":    server.Name,
			"type":    server.Type,
			"host":    server.Host,
			"port":    server.Port,
			"enabled": server.Enabled,
			"options": buildOptionsLog(server.Options),
		})))

	var result ConnectionTestResult
	switch strings.TrimSpace(server.Type) {
	case "clouddrive2":
		result = testCloudDrive2Connection(server, h.logger)
	case "openlist":
		result = testOpenListConnection(server, h.logger)
	case "local":
		// local 类型无需远程连接，直接返回成功
		result = ConnectionTestResult{
			Success: true,
			Message: "本地数据源无需测试连接",
		}
	default:
		respondError(c, http.StatusBadRequest, "invalid_type", "不支持的服务器类型", nil)
		return
	}

	h.logger.Debug(fmt.Sprintf("临时测试数据服务器连接完成：%s", server.Name),
		zap.String("type", server.Type),
		zap.Bool("success", result.Success),
		zap.Int64("latency_ms", result.LatencyMs))

	c.JSON(http.StatusOK, result)
}

// testCloudDrive2Connection 测试CloudDrive2连接（使用gRPC）
//
// CloudDrive2 使用 gRPC/HTTP2 协议，通过调用 GetSystemInfo（公开接口）
// 来验证服务器连接和认证信息
func testCloudDrive2Connection(server model.DataServer, logger *zap.Logger) ConnectionTestResult {
	start := time.Now()
	target := fmt.Sprintf("%s:%d", server.Host, server.Port)

	// 创建 gRPC 客户端
	client := cd2sdk.NewCloudDrive2Client(
		target,
		server.APIKey,
		cd2sdk.WithTimeout(10*time.Second),
	)
	defer func() {
		if err := client.Close(); err != nil {
			logger.Warn("关闭CloudDrive2客户端连接失败", zap.Error(err))
		}
	}()

	// 调用 GetSystemInfo（公开接口，可验证服务器是否可达）
	ctx := context.Background()
	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		logger.Warn("CloudDrive2连接失败",
			zap.Error(err),
			zap.String("target", target))
		return ConnectionTestResult{
			Success: false,
			Message: "连接失败，请检查服务器地址、端口和网络连接",
		}
	}

	// 检查系统状态
	if info.GetHasError() {
		logger.Warn("CloudDrive2系统有错误",
			zap.String("target", target),
			zap.String("system_message", info.GetSystemMessage()))
		return ConnectionTestResult{
			Success: false,
			Message: "CloudDrive2系统有错误：" + info.GetSystemMessage(),
		}
	}

	if !info.GetSystemReady() {
		logger.Warn("CloudDrive2系统未就绪",
			zap.String("target", target))
		return ConnectionTestResult{
			Success: false,
			Message: "CloudDrive2系统未就绪，请稍后再试",
		}
	}

	logger.Debug("CloudDrive2系统信息",
		zap.Bool("is_login", info.GetIsLogin()),
		zap.String("user_name", info.GetUserName()),
		zap.Bool("system_ready", info.GetSystemReady()))

	return ConnectionTestResult{
		Success:   true,
		Message:   "CloudDrive2连接测试成功",
		LatencyMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"rpc":          "GetSystemInfo",
			"is_login":     info.GetIsLogin(),
			"user_name":    info.GetUserName(),
			"system_ready": info.GetSystemReady(),
		},
	}
}

// testOpenListConnection 测试OpenList连接
func testOpenListConnection(server model.DataServer, logger *zap.Logger) ConnectionTestResult {
	start := time.Now()
	apiURL := fmt.Sprintf("http://%s:%d", server.Host, server.Port)
	client, err := openlistsdk.NewClient(openlistsdk.Config{
		BaseURL:  apiURL,
		Username: server.Options.Username,
		Password: server.Options.Password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		logger.Error("创建OpenList客户端失败", zap.Error(err), zap.String("url", apiURL))
		return ConnectionTestResult{
			Success: false,
			Message: "创建测试请求失败",
		}
	}
	if _, err := client.List(context.Background(), "/"); err != nil {
		logger.Warn("OpenList连接失败", zap.Error(err), zap.String("url", apiURL))
		return ConnectionTestResult{
			Success: false,
			Message: "连接失败，请检查服务器地址和网络连接",
		}
	}

	return ConnectionTestResult{
		Success:   true,
		Message:   "OpenList连接测试成功",
		LatencyMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"rpc": "list",
		},
	}
}

// allowedDataServerTypes 返回允许的数据服务器类型列表
//
// 支持三种类型：local（本地文件系统）、clouddrive2（CloudDrive2服务）、openlist（OpenList服务）
func allowedDataServerTypes() []string {
	return []string{"local", "clouddrive2", "openlist"}
}
