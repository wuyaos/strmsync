// Package http 提供HTTP API处理器
package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MediaServerHandler 媒体服务器处理器
type MediaServerHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewMediaServerHandler 创建媒体服务器处理器
func NewMediaServerHandler(db *gorm.DB, logger *zap.Logger) *MediaServerHandler {
	return &MediaServerHandler{
		db:     db,
		logger: logger,
	}
}

// CreateMediaServer 创建媒体服务器
// POST /api/servers/media
func (h *MediaServerHandler) CreateMediaServer(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"` // emby/jellyfin/plex
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
	fieldErrors := validateServerRequest(req.Name, req.Type, req.Host, req.Port, req.Options, []string{"emby", "jellyfin", "plex"})
	if len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 唯一性检查
	var count int64
	if err := h.db.Model(&model.MediaServer{}).
		Where("name = ?", strings.TrimSpace(req.Name)).
		Count(&count).Error; err != nil {
		h.logger.Error("检查媒体服务器名称唯一性失败", zap.Error(err))
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

	// 创建媒体服务器
	server := model.MediaServer{
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
		h.logger.Error("创建媒体服务器失败",
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

	h.logger.Info("创建媒体服务器成功",
		zap.Uint("id", server.ID),
		zap.String("name", server.Name),
		zap.String("type", server.Type))

	c.JSON(http.StatusCreated, gin.H{"server": server})
}

// ListMediaServers 获取媒体服务器列表
// GET /api/servers/media
func (h *MediaServerHandler) ListMediaServers(c *gin.Context) {
	pagination := parsePagination(c, 1, 50, 200)

	query := h.db.Model(&model.MediaServer{})

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
		h.logger.Error("统计媒体服务器失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 查询列表
	var servers []model.MediaServer
	if err := query.Order("created_at DESC").
		Offset(pagination.Offset).
		Limit(pagination.PageSize).
		Find(&servers).Error; err != nil {
		h.logger.Error("查询媒体服务器列表失败", zap.Error(err))
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

// GetMediaServer 获取单个媒体服务器
// GET /api/servers/media/:id
func (h *MediaServerHandler) GetMediaServer(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var server model.MediaServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "媒体服务器不存在", nil)
			return
		}
		h.logger.Error("查询媒体服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{"server": server})
}

// UpdateMediaServer 更新媒体服务器
// PUT /api/servers/media/:id
func (h *MediaServerHandler) UpdateMediaServer(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var req struct {
		Name    string `json:"name"`
		Type    string `json:"type"` // emby/jellyfin/plex
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
	fieldErrors := validateServerRequest(req.Name, req.Type, req.Host, req.Port, req.Options, []string{"emby", "jellyfin", "plex"})
	if len(fieldErrors) > 0 {
		respondValidationError(c, fieldErrors)
		return
	}

	// 查询现有记录
	var server model.MediaServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "媒体服务器不存在", nil)
			return
		}
		h.logger.Error("查询媒体服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 唯一性检查（名称变更时）
	newName := strings.TrimSpace(req.Name)
	if newName != server.Name {
		var count int64
		if err := h.db.Model(&model.MediaServer{}).
			Where("name = ? AND id <> ?", newName, server.ID).
			Count(&count).Error; err != nil {
			h.logger.Error("检查媒体服务器名称唯一性失败", zap.Error(err))
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
		h.logger.Error("更新媒体服务器失败",
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

	h.logger.Info("更新媒体服务器成功",
		zap.Uint("id", server.ID),
		zap.String("name", server.Name))

	c.JSON(http.StatusOK, gin.H{"server": server})
}

// DeleteMediaServer 删除媒体服务器
// DELETE /api/servers/media/:id
func (h *MediaServerHandler) DeleteMediaServer(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	// 检查是否被Job引用
	var count int64
	if err := h.db.Model(&model.Job{}).
		Where("media_server_id = ?", id).
		Count(&count).Error; err != nil {
		h.logger.Error("检查媒体服务器引用失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "数据库错误", nil)
		return
	}
	if count > 0 {
		respondError(c, http.StatusConflict, "in_use",
			fmt.Sprintf("存在 %d 个任务正在使用该媒体服务器，无法删除", count), nil)
		return
	}

	// 执行删除
	result := h.db.Delete(&model.MediaServer{}, uint(id))
	if result.Error != nil {
		h.logger.Error("删除媒体服务器失败", zap.Error(result.Error), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "删除失败", nil)
		return
	}

	if result.RowsAffected == 0 {
		respondError(c, http.StatusNotFound, "not_found", "媒体服务器不存在", nil)
		return
	}

	h.logger.Info("删除媒体服务器成功", zap.Uint64("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// TestMediaServerConnection 测试媒体服务器连接
// POST /api/servers/media/:id/test
func (h *MediaServerHandler) TestMediaServerConnection(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "无效的ID参数", nil)
		return
	}

	var server model.MediaServer
	if err := h.db.First(&server, uint(id)).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondError(c, http.StatusNotFound, "not_found", "媒体服务器不存在", nil)
			return
		}
		h.logger.Error("查询媒体服务器失败", zap.Error(err), zap.Uint64("id", id))
		respondError(c, http.StatusInternalServerError, "db_error", "查询失败", nil)
		return
	}

	// 检查是否已启用
	if !server.Enabled {
		h.logger.Warn("尝试测试已禁用的媒体服务器",
			zap.Uint("id", server.ID),
			zap.String("name", server.Name))
		c.JSON(http.StatusOK, ConnectionTestResult{
			Success: false,
			Message: "媒体服务器已禁用，建议启用后再测试",
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
	case "emby":
		result = testEmbyConnection(server, h.logger)
	case "jellyfin":
		result = testJellyfinConnection(server, h.logger)
	case "plex":
		result = testPlexConnection(server, h.logger)
	default:
		respondError(c, http.StatusBadRequest, "invalid_type", "不支持的服务器类型", nil)
		return
	}

	h.logger.Info("测试媒体服务器连接",
		zap.Uint("id", server.ID),
		zap.String("type", server.Type),
		zap.Bool("success", result.Success),
		zap.Int64("latency_ms", result.LatencyMs))

	c.JSON(http.StatusOK, result)
}

// testEmbyConnection 测试Emby连接
func testEmbyConnection(server model.MediaServer, logger *zap.Logger) ConnectionTestResult {
	start := time.Now()
	apiURL := fmt.Sprintf("http://%s:%d/System/Info/Public", server.Host, server.Port)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logger.Error("创建Emby测试请求失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "创建测试请求失败",
		}
	}

	if server.APIKey != "" {
		req.Header.Set("X-Emby-Token", server.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Emby连接失败", zap.Error(err), zap.String("url", apiURL))
		return ConnectionTestResult{
			Success: false,
			Message: "连接失败，请检查服务器地址和网络连接",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取Emby响应失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "读取服务器响应失败",
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("Emby返回非成功状态码",
			zap.Int("http_status", resp.StatusCode),
			zap.Int("body_size", len(body)),
			zap.String("body_preview", truncateString(string(body), 100)))
		return ConnectionTestResult{
			Success: false,
			Message: "服务器返回错误状态",
		}
	}

	return ConnectionTestResult{
		Success:   true,
		Message:   "Emby连接测试成功",
		LatencyMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"http_status": resp.StatusCode,
		},
	}
}

// testJellyfinConnection 测试Jellyfin连接
func testJellyfinConnection(server model.MediaServer, logger *zap.Logger) ConnectionTestResult {
	start := time.Now()
	apiURL := fmt.Sprintf("http://%s:%d/System/Info/Public", server.Host, server.Port)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logger.Error("创建Jellyfin测试请求失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "创建测试请求失败",
		}
	}

	if server.APIKey != "" {
		// Jellyfin支持两种认证方式，为了兼容性同时设置
		req.Header.Set("X-MediaBrowser-Token", server.APIKey)
		req.Header.Set("Authorization", fmt.Sprintf(`MediaBrowser Token="%s"`, server.APIKey))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Jellyfin连接失败", zap.Error(err), zap.String("url", apiURL))
		return ConnectionTestResult{
			Success: false,
			Message: "连接失败，请检查服务器地址和网络连接",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取Jellyfin响应失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "读取服务器响应失败",
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("Jellyfin返回非成功状态码",
			zap.Int("http_status", resp.StatusCode),
			zap.Int("body_size", len(body)),
			zap.String("body_preview", truncateString(string(body), 100)))
		return ConnectionTestResult{
			Success: false,
			Message: "服务器返回错误状态",
		}
	}

	return ConnectionTestResult{
		Success:   true,
		Message:   "Jellyfin连接测试成功",
		LatencyMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"http_status": resp.StatusCode,
		},
	}
}

// testPlexConnection 测试Plex连接
func testPlexConnection(server model.MediaServer, logger *zap.Logger) ConnectionTestResult {
	start := time.Now()
	apiURL := fmt.Sprintf("http://%s:%d/identity", server.Host, server.Port)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logger.Error("创建Plex测试请求失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "创建测试请求失败",
		}
	}

	if server.APIKey != "" {
		req.Header.Set("X-Plex-Token", server.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Plex连接失败", zap.Error(err), zap.String("url", apiURL))
		return ConnectionTestResult{
			Success: false,
			Message: "连接失败，请检查服务器地址和网络连接",
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取Plex响应失败", zap.Error(err))
		return ConnectionTestResult{
			Success: false,
			Message: "读取服务器响应失败",
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("Plex返回非成功状态码",
			zap.Int("http_status", resp.StatusCode),
			zap.Int("body_size", len(body)),
			zap.String("body_preview", truncateString(string(body), 100)))
		return ConnectionTestResult{
			Success: false,
			Message: "服务器返回错误状态",
		}
	}

	return ConnectionTestResult{
		Success:   true,
		Message:   "Plex连接测试成功",
		LatencyMs: time.Since(start).Milliseconds(),
		Details: map[string]interface{}{
			"http_status": resp.StatusCode,
		},
	}
}
