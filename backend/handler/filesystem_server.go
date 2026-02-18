// Package handlers 提供HTTP API处理器
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	cd2 "github.com/strmsync/strmsync/internal/infra/filesystem/clouddrive2"
	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DataServerHandler 数据服务器处理器
type DataServerHandler struct {
	db     *gorm.DB
	logger *zap.Logger
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
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"` // clouddrive2/openlist
		Host    string `json:"host"`
		Port    int    `json:"port"`
		APIKey  string `json:"api_key"`
		Enabled *bool  `json:"enabled"`
		Options string `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证（使用组合验证器）
	fieldErrors := validateServerRequest(req.Name, req.Type, req.Host, req.Port, req.Options, allowedDataServerTypes())
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

	// 创建数据服务器
	server := model.DataServer{
		Name:    strings.TrimSpace(req.Name),
		Type:    strings.TrimSpace(req.Type),
		Host:    strings.TrimSpace(req.Host),
		Port:    req.Port,
		APIKey:  strings.TrimSpace(req.APIKey),
		Enabled: enabled,
		Options: strings.TrimSpace(req.Options),
	}

	if err := h.db.Create(&server).Error; err != nil {
		// 检查是否为唯一约束错误
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			respondError(c, http.StatusConflict, "duplicate_name", "服务器名称已存在", nil)
			return
		}
		h.logger.Error("创建数据服务器失败",
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

	h.logger.Info("创建数据服务器成功",
		zap.Uint("id", server.ID),
		zap.String("name", server.Name),
		zap.String("type", server.Type))

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

	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"` // clouddrive2/openlist
		Host    string `json:"host"`
		Port    int    `json:"port"`
		APIKey  string `json:"api_key"`
		Enabled *bool  `json:"enabled"`
		Options string `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	// 参数验证（使用组合验证器）
	fieldErrors := validateServerRequest(req.Name, req.Type, req.Host, req.Port, req.Options, allowedDataServerTypes())
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
	server.Name = newName
	server.Type = strings.TrimSpace(req.Type)
	server.Host = strings.TrimSpace(req.Host)
	server.Port = req.Port
	server.APIKey = strings.TrimSpace(req.APIKey)
	server.Options = strings.TrimSpace(req.Options)
	if req.Enabled != nil {
		server.Enabled = *req.Enabled
	}

	if err := h.db.Save(&server).Error; err != nil {
		// 检查是否为唯一约束错误
		if strings.Contains(err.Error(), "UNIQUE constraint failed") ||
			strings.Contains(err.Error(), "duplicate key") {
			respondError(c, http.StatusConflict, "duplicate_name", "服务器名称已存在", nil)
			return
		}
		h.logger.Error("更新数据服务器失败",
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

	h.logger.Info("更新数据服务器成功",
		zap.Uint("id", server.ID),
		zap.String("name", server.Name))

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

	// 执行删除
	result := h.db.Delete(&model.DataServer{}, uint(id))
	if result.Error != nil {
		h.logger.Error("删除数据服务器失败", zap.Error(result.Error), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}

	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "数据服务器不存在", nil)
		return
	}

	h.logger.Info("删除数据服务器成功", zap.Uint64("id", id))
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
		h.logger.Warn("尝试测试已禁用的数据服务器",
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
		h.logger.Warn("测试内网地址（SSRF风险）",
			zap.Uint("id", server.ID),
			zap.String("name", server.Name),
			zap.String("host", server.Host),
			zap.Int("port", server.Port))
	}

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

	h.logger.Info("测试数据服务器连接",
		zap.Uint("id", server.ID),
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
	client := cd2.NewCloudDrive2Client(
		target,
		server.APIKey,
		cd2.WithTimeout(10*time.Second),
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
	apiURL := fmt.Sprintf("http://%s:%d/api/fs/list", server.Host, server.Port)

	reqBody := map[string]interface{}{"path": "/"}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		logger.Error("序列化OpenList请求失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "创建测试请求失败",
		}
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logger.Error("创建OpenList测试请求失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "创建测试请求失败",
		}
	}

	req.Header.Set("Content-Type", "application/json")
	if server.APIKey != "" {
		req.Header.Set("Authorization", server.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("OpenList连接失败", zap.Error(err), zap.String("url", apiURL))
		return ConnectionTestResult{
			Success: false,
			Message: "连接失败，请检查服务器地址和网络连接",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取OpenList响应失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "读取服务器响应失败",
		}
	}

	var apiResp struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(body, &apiResp); err != nil {
		logger.Error("解析OpenList响应失败",
			zap.Error(err),
			zap.Int("body_size", len(body)),
			zap.String("body_preview", truncateString(string(body), 100)))
		return ConnectionTestResult{
			Success: false,
			Message: "服务器响应格式错误",
		}
	}

	if apiResp.Code != 0 && apiResp.Code != 200 {
		logger.Warn("OpenList返回错误码",
			zap.Int("code", apiResp.Code),
			zap.Int("http_status", resp.StatusCode))
		return ConnectionTestResult{
			Success: false,
			Message: fmt.Sprintf("服务器返回错误码: %d", apiResp.Code),
		}
	}

	return ConnectionTestResult{
		Success:   true,
		Message:   "OpenList连接测试成功",
		LatencyMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"http_status": resp.StatusCode,
			"api_code":    apiResp.Code,
		},
	}
}

// allowedDataServerTypes 返回允许的数据服务器类型列表
//
// 生产环境仅允许 clouddrive2 和 openlist。
// 测试环境（ALLOW_LOCAL_DATASERVER=true）额外允许 local 类型。
func allowedDataServerTypes() []string {
	types := []string{"clouddrive2", "openlist"}
	if strings.EqualFold(os.Getenv("ALLOW_LOCAL_DATASERVER"), "true") {
		types = append(types, "local")
	}
	return types
}
