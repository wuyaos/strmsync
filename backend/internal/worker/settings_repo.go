package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/strmsync/strmsync/internal/domain/model"
	"gorm.io/gorm"
)

const appSettingsKey = "app_settings"

// DefaultSettingsRepository 返回默认速率配置。
type DefaultSettingsRepository struct{}

func (DefaultSettingsRepository) GetRateSettings(ctx context.Context) (RateSettings, error) {
	return defaultRateSettings(), nil
}

// GormSettingsRepository 基于 settings 表读取全局设置。
type GormSettingsRepository struct {
	db *gorm.DB
}

func NewGormSettingsRepository(db *gorm.DB) (*GormSettingsRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("worker: settings db is nil")
	}
	return &GormSettingsRepository{db: db}, nil
}

func (r *GormSettingsRepository) GetRateSettings(ctx context.Context) (RateSettings, error) {
	if r == nil || r.db == nil {
		return defaultRateSettings(), errors.New("settings repo is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var setting model.Setting
	if err := r.db.WithContext(ctx).First(&setting, "key = ?", appSettingsKey).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return defaultRateSettings(), nil
		}
		return defaultRateSettings(), err
	}

	raw := strings.TrimSpace(setting.Value)
	if raw == "" {
		return defaultRateSettings(), nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return defaultRateSettings(), nil
	}

	settings := defaultRateSettings()
	if rawRate, ok := payload["rate"].(map[string]any); ok {
		applyRateSettings(&settings, rawRate)
	} else if rawRate, ok := payload["qos"].(map[string]any); ok {
		applyRateSettings(&settings, rawRate)
	}
	if rawGlobal, ok := payload["global"].(map[string]any); ok {
		applyRateSettings(&settings, rawGlobal)
	}

	return settings, nil
}

func defaultRateSettings() RateSettings {
	return RateSettings{
		DownloadRatePerSec:  10,
		APIRate:             5,
		APIConcurrency:      3,
		APIRetryMax:         3,
		APIRetryIntervalSec: 60,
	}
}

func applyRateSettings(settings *RateSettings, raw map[string]any) {
	if settings == nil || raw == nil {
		return
	}
	settings.DownloadRatePerSec = parseIntWithDefault(raw, "download_rate_per_sec", settings.DownloadRatePerSec)
	settings.APIRate = parseIntWithDefault(raw, "api_rate", settings.APIRate)
	settings.APIConcurrency = parseIntWithDefault(raw, "api_concurrency", settings.APIConcurrency)
	settings.APIRetryMax = parseIntWithDefault(raw, "api_retry_max", settings.APIRetryMax)
	settings.APIRetryIntervalSec = parseIntWithDefault(raw, "api_retry_interval_sec", settings.APIRetryIntervalSec)
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
