package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"github.com/strmsync/strmsync/internal/pkg/qos"
	"go.uber.org/zap"
)

const defaultQoSTimeout = 5 * time.Second

func resolveRateSettings(ctx context.Context, repo SettingsRepository, log *zap.Logger) RateSettings {
	if repo == nil {
		return defaultRateSettings()
	}
	settings, err := repo.GetRateSettings(ctx)
	if err != nil && log != nil {
		log.Warn("读取全局速率设置失败，使用默认值", zap.Error(err))
		return defaultRateSettings()
	}
	return settings
}

func buildServerLimiters(ctx context.Context, server model.DataServer, repo SettingsRepository, manager *qos.Manager, log *zap.Logger) (*qos.Limiter, *qos.Limiter) {
	serverType := strings.ToLower(strings.TrimSpace(server.Type))
	if serverType == filesystem.TypeLocal.String() || manager == nil {
		return nil, nil
	}

	settings := resolveRateSettings(ctx, repo, log)
	apiRate := server.APIRate
	if apiRate <= 0 {
		apiRate = settings.APIRate
	}
	apiConcurrency := server.APIConcurrency
	if apiConcurrency <= 0 {
		apiConcurrency = settings.APIConcurrency
	}

	downloadRate := server.DownloadRatePerSec
	if downloadRate <= 0 {
		downloadRate = settings.DownloadRatePerSec
	}

	apiKey := fmt.Sprintf("api:%s:%d", serverType, server.ID)
	downloadKey := fmt.Sprintf("download:%s:%d", serverType, server.ID)

	apiLimiter := manager.Get(apiKey, qos.Config{
		Rate:        apiRate,
		Concurrency: apiConcurrency,
		Timeout:     defaultQoSTimeout,
	})
	downloadLimiter := manager.Get(downloadKey, qos.Config{
		Rate:        downloadRate,
		Concurrency: apiConcurrency,
		Timeout:     defaultQoSTimeout,
	})

	return apiLimiter, downloadLimiter
}
