// Package db 提供数据库迁移和回填工具
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BackfillConflict 记录 UID 冲突详情
type BackfillConflict struct {
	Table string // 表名
	ID    uint   // 记录ID
	Name  string // 服务器名称
	UID   string // 冲突的UID
}

// BackfillStats 回填统计信息
type BackfillStats struct {
	DataServersTotal         int                // DataServer 总数
	DataServersUpdated       int                // 成功更新数
	DataServersGenFailed     int                // UID生成失败数
	DataServersUpdateFailed  int                // 更新失败数
	DataServersConflict      int                // UID冲突数
	DataServersQueryFailed   int                // 查询失败标志（0或1）
	DataServersConflicts     []BackfillConflict // 冲突详情列表

	MediaServersTotal        int                // MediaServer 总数
	MediaServersUpdated      int                // 成功更新数
	MediaServersGenFailed    int                // UID生成失败数
	MediaServersUpdateFailed int                // 更新失败数
	MediaServersConflict     int                // UID冲突数
	MediaServersQueryFailed  int                // 查询失败标志（0或1）
	MediaServersConflicts    []BackfillConflict // 冲突详情列表

	StartedAt time.Time     // 开始时间
	Elapsed   time.Duration // 总耗时
}

// BackfillServerUIDs 回填所有服务器的 UID 字段
//
// 此函数应在应用启动时调用，确保历史数据也有 UID。
// 安全性：
// - 只更新 UID 为空的记录
// - 错误不中断整体流程（记录日志并统计）
// - 支持 context 取消
// - 冲突检测：更新前检查UID是否已存在于其他记录
//
// 返回值：
// - stats: 详细的回填统计信息
// - error: 仅在参数校验失败时返回错误，其他错误记录在stats中
func BackfillServerUIDs(ctx context.Context, db *gorm.DB, logger *zap.Logger) (*BackfillStats, error) {
	if db == nil {
		return nil, fmt.Errorf("backfill server uids: db is nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Info("开始回填服务器 UID")
	stats := &BackfillStats{
		StartedAt: time.Now(),
	}

	// 回填 DataServers
	backfillDataServerUIDs(ctx, db, logger, stats)

	// 回填 MediaServers
	backfillMediaServerUIDs(ctx, db, logger, stats)

	stats.Elapsed = time.Since(stats.StartedAt)
	logger.Info("服务器 UID 回填完成",
		zap.Int("data_servers_total", stats.DataServersTotal),
		zap.Int("data_servers_updated", stats.DataServersUpdated),
		zap.Int("data_servers_gen_failed", stats.DataServersGenFailed),
		zap.Int("data_servers_update_failed", stats.DataServersUpdateFailed),
		zap.Int("data_servers_conflict", stats.DataServersConflict),
		zap.Int("data_servers_query_failed", stats.DataServersQueryFailed),
		zap.Int("media_servers_total", stats.MediaServersTotal),
		zap.Int("media_servers_updated", stats.MediaServersUpdated),
		zap.Int("media_servers_gen_failed", stats.MediaServersGenFailed),
		zap.Int("media_servers_update_failed", stats.MediaServersUpdateFailed),
		zap.Int("media_servers_conflict", stats.MediaServersConflict),
		zap.Int("media_servers_query_failed", stats.MediaServersQueryFailed),
		zap.Duration("elapsed", stats.Elapsed))

	return stats, nil
}

// backfillDataServerUIDs 回填 DataServer UID
//
// 采用最佳努力策略：错误不中断，记录到统计信息中
func backfillDataServerUIDs(ctx context.Context, db *gorm.DB, logger *zap.Logger, stats *BackfillStats) {
	var servers []model.DataServer

	// 查询 UID 为空的记录
	if err := db.WithContext(ctx).
		Where("uid = ? OR uid IS NULL", "").
		Find(&servers).Error; err != nil {
		stats.DataServersQueryFailed++
		logger.Error("查询 DataServer 失败（继续回填其他表）", zap.Error(err))
		return
	}

	if len(servers) == 0 {
		logger.Debug("没有需要回填 UID 的 DataServer")
		return
	}

	stats.DataServersTotal = len(servers)
	logger.Info("开始回填 DataServer UID", zap.Int("count", stats.DataServersTotal))

	for _, server := range servers {
		// 检查 context 取消
		if ctx.Err() != nil {
			logger.Warn("回填 DataServer UID 中断（context 取消）")
			return
		}

		// 生成 UID
		uid, err := model.GenerateDataServerUID(
			server.Type,
			server.Host,
			server.Port,
			server.Options,
			server.APIKey)
		if err != nil {
			stats.DataServersGenFailed++
			logger.Warn("生成 DataServer UID 失败",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.Error(err))
			continue
		}

		// 冲突检测：查询是否已有其他记录使用该 UID
		var conflictCount int64
		if err := db.WithContext(ctx).
			Model(&model.DataServer{}).
			Where("uid = ? AND id <> ?", uid, server.ID).
			Count(&conflictCount).Error; err != nil {
			stats.DataServersUpdateFailed++
			logger.Warn("检测 DataServer UID 冲突失败",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.String("uid", uid),
				zap.Error(err))
			continue
		}
		if conflictCount > 0 {
			stats.DataServersConflict++
			stats.DataServersConflicts = append(stats.DataServersConflicts, BackfillConflict{
				Table: "data_servers",
				ID:    server.ID,
				Name:  server.Name,
				UID:   uid,
			})
			logger.Warn("检测到 DataServer UID 冲突，已跳过更新",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.String("uid", uid))
			continue
		}

		// 更新 UID
		if err := db.WithContext(ctx).
			Model(&model.DataServer{}).
			Where("id = ?", server.ID).
			Update("uid", uid).Error; err != nil {
			stats.DataServersUpdateFailed++
			logger.Warn("更新 DataServer UID 失败",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.Error(err))
			continue
		}

		stats.DataServersUpdated++
		logger.Debug("回填 DataServer UID 成功",
			zap.Uint("id", server.ID),
			zap.String("name", server.Name),
			zap.String("uid", uid))
	}
}

// backfillMediaServerUIDs 回填 MediaServer UID
//
// 采用最佳努力策略：错误不中断，记录到统计信息中
func backfillMediaServerUIDs(ctx context.Context, db *gorm.DB, logger *zap.Logger, stats *BackfillStats) {
	var servers []model.MediaServer

	// 查询 UID 为空的记录
	if err := db.WithContext(ctx).
		Where("uid = ? OR uid IS NULL", "").
		Find(&servers).Error; err != nil {
		stats.MediaServersQueryFailed++
		logger.Error("查询 MediaServer 失败（继续回填其他表）", zap.Error(err))
		return
	}

	if len(servers) == 0 {
		logger.Debug("没有需要回填 UID 的 MediaServer")
		return
	}

	stats.MediaServersTotal = len(servers)
	logger.Info("开始回填 MediaServer UID", zap.Int("count", stats.MediaServersTotal))

	for _, server := range servers {
		// 检查 context 取消
		if ctx.Err() != nil {
			logger.Warn("回填 MediaServer UID 中断（context 取消）")
			return
		}

		// 生成 UID
		uid, err := model.GenerateMediaServerUID(
			server.Type,
			server.Host,
			server.Port,
			server.Options,
			server.APIKey)
		if err != nil {
			stats.MediaServersGenFailed++
			logger.Warn("生成 MediaServer UID 失败",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.Error(err))
			continue
		}

		// 冲突检测：查询是否已有其他记录使用该 UID
		var conflictCount int64
		if err := db.WithContext(ctx).
			Model(&model.MediaServer{}).
			Where("uid = ? AND id <> ?", uid, server.ID).
			Count(&conflictCount).Error; err != nil {
			stats.MediaServersUpdateFailed++
			logger.Warn("检测 MediaServer UID 冲突失败",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.String("uid", uid),
				zap.Error(err))
			continue
		}
		if conflictCount > 0 {
			stats.MediaServersConflict++
			stats.MediaServersConflicts = append(stats.MediaServersConflicts, BackfillConflict{
				Table: "media_servers",
				ID:    server.ID,
				Name:  server.Name,
				UID:   uid,
			})
			logger.Warn("检测到 MediaServer UID 冲突，已跳过更新",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.String("uid", uid))
			continue
		}

		// 更新 UID
		if err := db.WithContext(ctx).
			Model(&model.MediaServer{}).
			Where("id = ?", server.ID).
			Update("uid", uid).Error; err != nil {
			stats.MediaServersUpdateFailed++
			logger.Warn("更新 MediaServer UID 失败",
				zap.Uint("id", server.ID),
				zap.String("name", server.Name),
				zap.Error(err))
			continue
		}

		stats.MediaServersUpdated++
		logger.Debug("回填 MediaServer UID 成功",
			zap.Uint("id", server.ID),
			zap.String("name", server.Name),
			zap.String("uid", uid))
	}
}
