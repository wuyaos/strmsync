// Package http 提供HTTP API处理器
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SettingHandler 系统设置处理器
type SettingHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewSettingHandler 创建设置处理器
func NewSettingHandler(db *gorm.DB, logger *zap.Logger) *SettingHandler {
	return &SettingHandler{
		db:     db,
		logger: logger,
	}
}

const appSettingsKey = "app_settings"

type scannerSettings struct {
	Concurrency int `json:"concurrency"`
	BatchSize   int `json:"batchSize"`
}

type logSettings struct {
	Level string `json:"level"`
	ToDB  bool   `json:"toDB"`
	Path  string `json:"path"`
}

type themeSettings struct {
	Mode         string `json:"mode"`
	PrimaryColor string `json:"primaryColor"`
	Compact      bool   `json:"compact"`
}

type notificationSettings struct {
	Position string `json:"position"`
	Duration int    `json:"duration"`
	ShowIcon bool   `json:"showIcon"`
	Sound    bool   `json:"sound"`
}

type qosSettings struct {
	DownloadRatePerSec  int `json:"download_rate_per_sec"`
	APIRate             int `json:"api_rate"`
	APIRetryMax         int `json:"api_retry_max"`
	APIRetryIntervalSec int `json:"api_retry_interval_sec"`
}

type uiSettings struct {
	AutoRefreshIntervalMs int `json:"auto_refresh_interval_ms"`
}

type appSettings struct {
	Scanner      scannerSettings      `json:"scanner"`
	Log          logSettings          `json:"log"`
	Theme        themeSettings        `json:"theme"`
	Notification notificationSettings `json:"notification"`
	Rate         qosSettings          `json:"rate"`
	UI           uiSettings           `json:"ui"`
}

// GetSettings 获取系统设置
// GET /api/settings
func (h *SettingHandler) GetSettings(c *gin.Context) {
	settings, err := h.loadAppSettings()
	if err != nil {
		h.logger.Error("获取系统设置失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "获取设置失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"settings": settings,
	})
}

// UpdateSettings 更新系统设置
// PUT /api/settings
func (h *SettingHandler) UpdateSettings(c *gin.Context) {
	var payload map[string]any
	if err := c.ShouldBindJSON(&payload); err != nil {
		respondError(c, http.StatusBadRequest, "invalid_request", "请求体格式错误", nil)
		return
	}

	settings := normalizeAppSettings(payload)
	if err := h.saveAppSettings(settings); err != nil {
		h.logger.Error("保存系统设置失败", zap.Error(err))
		respondError(c, http.StatusInternalServerError, "db_error", "保存设置失败", nil)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "设置已更新",
		"settings": settings,
	})
}

// ServerHandler 服务器配置处理器
type ServerHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewServerHandler 创建服务器处理器
func NewServerHandler(db *gorm.DB, logger *zap.Logger) *ServerHandler {
	return &ServerHandler{
		db:     db,
		logger: logger,
	}
}

// GetDataServer 获取数据服务器配置
// GET /api/servers/data
func (h *ServerHandler) GetDataServer(c *gin.Context) {
	// TODO: 实现数据服务器配置查询
	c.JSON(http.StatusOK, gin.H{
		"servers": []interface{}{},
	})
}

// UpdateDataServer 更新数据服务器配置
// PUT /api/servers/data
func (h *ServerHandler) UpdateDataServer(c *gin.Context) {
	// TODO: 实现数据服务器配置更新
	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
	})
}

// GetMediaServer 获取媒体服务器配置
// GET /api/servers/media
func (h *ServerHandler) GetMediaServer(c *gin.Context) {
	// TODO: 实现媒体服务器配置查询
	c.JSON(http.StatusOK, gin.H{
		"servers": []interface{}{},
	})
}

// UpdateMediaServer 更新媒体服务器配置
// PUT /api/servers/media
func (h *ServerHandler) UpdateMediaServer(c *gin.Context) {
	// TODO: 实现媒体服务器配置更新
	c.JSON(http.StatusOK, gin.H{
		"message": "配置已更新",
	})
}

func defaultAppSettings() appSettings {
	return appSettings{
		Scanner: scannerSettings{
			Concurrency: 20,
			BatchSize:   500,
		},
		Log: logSettings{
			Level: "info",
			ToDB:  false,
			Path:  "logs",
		},
		Theme: themeSettings{
			Mode:         "light",
			PrimaryColor: "#409EFF",
			Compact:      false,
		},
		Notification: notificationSettings{
			Position: "top-right",
			Duration: 3,
			ShowIcon: true,
			Sound:    false,
		},
		Rate: qosSettings{
			DownloadRatePerSec:  10,
			APIRate:             10,
			APIRetryMax:         3,
			APIRetryIntervalSec: 60,
		},
		UI: uiSettings{
			AutoRefreshIntervalMs: 2000,
		},
	}
}

func normalizeAppSettings(payload map[string]any) appSettings {
	settings := defaultAppSettings()
	if payload == nil {
		return settings
	}

	if raw, ok := payload["scanner"].(map[string]any); ok {
		settings.Scanner.Concurrency = parseIntWithDefault(raw, "concurrency", settings.Scanner.Concurrency)
		settings.Scanner.BatchSize = parseIntWithDefault(raw, "batchSize", settings.Scanner.BatchSize)
	}
	if raw, ok := payload["log"].(map[string]any); ok {
		settings.Log.Level = parseStringWithDefault(raw, "level", settings.Log.Level)
		if val, exists := raw["toDB"]; exists {
			settings.Log.ToDB = parseBool(val, settings.Log.ToDB)
		}
		settings.Log.Path = parseStringWithDefault(raw, "path", settings.Log.Path)
	}
	if raw, ok := payload["theme"].(map[string]any); ok {
		settings.Theme.Mode = parseStringWithDefault(raw, "mode", settings.Theme.Mode)
		settings.Theme.PrimaryColor = parseStringWithDefault(raw, "primaryColor", settings.Theme.PrimaryColor)
		if val, exists := raw["compact"]; exists {
			settings.Theme.Compact = parseBool(val, settings.Theme.Compact)
		}
	}
	if raw, ok := payload["notification"].(map[string]any); ok {
		settings.Notification.Position = parseStringWithDefault(raw, "position", settings.Notification.Position)
		settings.Notification.Duration = parseIntWithDefault(raw, "duration", settings.Notification.Duration)
		if val, exists := raw["showIcon"]; exists {
			settings.Notification.ShowIcon = parseBool(val, settings.Notification.ShowIcon)
		}
		if val, exists := raw["sound"]; exists {
			settings.Notification.Sound = parseBool(val, settings.Notification.Sound)
		}
	}
	if raw, ok := payload["rate"].(map[string]any); ok {
		settings.Rate.DownloadRatePerSec = parseIntWithDefault(raw, "download_rate_per_sec", settings.Rate.DownloadRatePerSec)
		settings.Rate.APIRate = parseIntWithDefault(raw, "api_rate", settings.Rate.APIRate)
		settings.Rate.APIRetryMax = parseIntWithDefault(raw, "api_retry_max", settings.Rate.APIRetryMax)
		settings.Rate.APIRetryIntervalSec = parseIntWithDefault(raw, "api_retry_interval_sec", settings.Rate.APIRetryIntervalSec)
	} else if raw, ok := payload["qos"].(map[string]any); ok {
		settings.Rate.DownloadRatePerSec = parseIntWithDefault(raw, "download_rate_per_sec", settings.Rate.DownloadRatePerSec)
		settings.Rate.APIRate = parseIntWithDefault(raw, "api_rate", settings.Rate.APIRate)
		settings.Rate.APIRetryMax = parseIntWithDefault(raw, "api_retry_max", settings.Rate.APIRetryMax)
		settings.Rate.APIRetryIntervalSec = parseIntWithDefault(raw, "api_retry_interval_sec", settings.Rate.APIRetryIntervalSec)
	}
	if raw, ok := payload["ui"].(map[string]any); ok {
		settings.UI.AutoRefreshIntervalMs = parseIntWithDefault(raw, "auto_refresh_interval_ms", settings.UI.AutoRefreshIntervalMs)
	}

	return settings
}

func (h *SettingHandler) loadAppSettings() (appSettings, error) {
	if h == nil || h.db == nil {
		return defaultAppSettings(), errors.New("db not initialized")
	}

	var setting model.Setting
	if err := h.db.First(&setting, "key = ?", appSettingsKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultAppSettings(), nil
		}
		return defaultAppSettings(), err
	}

	raw := strings.TrimSpace(setting.Value)
	if raw == "" {
		return defaultAppSettings(), nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		h.logger.Warn("系统设置JSON解析失败，使用默认值", zap.Error(err))
		return defaultAppSettings(), nil
	}

	return normalizeAppSettings(payload), nil
}

func (h *SettingHandler) saveAppSettings(settings appSettings) error {
	if h == nil || h.db == nil {
		return errors.New("db not initialized")
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	var setting model.Setting
	if err := h.db.First(&setting, "key = ?", appSettingsKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			setting = model.Setting{
				Key:       appSettingsKey,
				Value:     string(raw),
				UpdatedAt: time.Now(),
			}
			return h.db.Create(&setting).Error
		}
		return err
	}

	setting.Value = string(raw)
	setting.UpdatedAt = time.Now()
	return h.db.Save(&setting).Error
}

func parseIntWithDefault(raw map[string]any, key string, def int) int {
	val, ok := raw[key]
	if !ok || val == nil {
		return def
	}
	switch v := val.(type) {
	case float64:
		return int(v)
	case float32:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		if num, err := v.Int64(); err == nil {
			return int(num)
		}
	case string:
		if parsed := strings.TrimSpace(v); parsed != "" {
			if num, err := strconv.Atoi(parsed); err == nil {
				return num
			}
		}
	}
	return def
}

func parseStringWithDefault(raw map[string]any, key string, def string) string {
	val, ok := raw[key]
	if !ok || val == nil {
		return def
	}
	if str, ok := val.(string); ok {
		if strings.TrimSpace(str) == "" {
			return def
		}
		return str
	}
	return def
}

func parseBool(val any, def bool) bool {
	switch v := val.(type) {
	case bool:
		return v
	case string:
		parsed := strings.TrimSpace(strings.ToLower(v))
		if parsed == "true" || parsed == "1" || parsed == "yes" {
			return true
		}
		if parsed == "false" || parsed == "0" || parsed == "no" {
			return false
		}
	}
	return def
}
