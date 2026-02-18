// Package repository 提供 Job 相关的 GORM Repository 实现
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"gorm.io/gorm"
)

// GormJobRepository 是基于 GORM 的 model.Job 数据访问实现
//
// 此实现提供了 scheduler 和 worker 包所需的所有 model.Job 查询和更新方法。
// 各包可以通过接口类型声明使用此实现，确保接口隔离。
type GormJobRepository struct {
	db *gorm.DB
}

// NewGormJobRepository 创建 GormJobRepository 实例
//
// 参数：
//   - db: GORM 数据库连接（不能为 nil）
//
// 返回：
//   - *GormJobRepository: Repository 实例
//   - error: db 为 nil 时返回错误
func NewGormJobRepository(db *gorm.DB) (*GormJobRepository, error) {
	if db == nil {
		return nil, fmt.Errorf("core: gorm db is nil")
	}
	return &GormJobRepository{db: db}, nil
}

// GetByID 获取指定 model.Job
//
// 参数：
//   - ctx: 上下文（为 nil 时自动使用 Background）
//   - id: model.Job ID
//
// 返回：
//   - model.Job: model.Job 对象
//   - error: 查询失败或不存在时返回 gorm.ErrRecordNotFound
func (r *GormJobRepository) GetByID(ctx context.Context, id uint) (model.Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var job model.Job
	if err := r.db.WithContext(ctx).First(&job, id).Error; err != nil {
		return model.Job{}, err
	}
	return job, nil
}

// ListEnabledJobs 返回所有启用的 model.Job 列表
//
// 仅返回 Enabled=true 的 model.Job，按 ID 升序排序。
//
// 参数：
//   - ctx: 上下文（为 nil 时自动使用 Background）
//
// 返回：
//   - []model.Job: model.Job 列表（可能为空）
//   - error: 查询失败时返回错误
func (r *GormJobRepository) ListEnabledJobs(ctx context.Context) ([]model.Job, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var jobs []model.Job
	if err := r.db.WithContext(ctx).
		Where("enabled = ?", true).
		Order("id ASC").
		Find(&jobs).Error; err != nil {
		return nil, fmt.Errorf("core: list enabled jobs: %w", err)
	}
	return jobs, nil
}

// UpdateStatus 更新 model.Job 的运行状态
//
// 用于 Worker 在任务完成/失败后将 model.Job.status 回写为 idle 或 error。
//
// 参数：
//   - ctx: 上下文（为 nil 时自动使用 Background）
//   - id: model.Job ID
//   - status: 目标状态（"idle"/"running"/"error"）
//
// 返回：
//   - error: 更新失败时返回错误（包括 model.Job 不存在）
func (r *GormJobRepository) UpdateStatus(ctx context.Context, id uint, status string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx).
		Model(&model.Job{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// UpdateLastRunAt 更新 model.Job 的 last_run_at 字段
//
// 用于记录任务的最后成功执行时间。
//
// 参数：
//   - ctx: 上下文（为 nil 时自动使用 Background）
//   - id: model.Job ID
//   - lastRunAt: 最后运行时间
//
// 返回：
//   - error: 更新失败时返回错误（包括 model.Job 不存在）
func (r *GormJobRepository) UpdateLastRunAt(ctx context.Context, id uint, lastRunAt time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return r.db.WithContext(ctx).
		Model(&model.Job{}).
		Where("id = ?", id).
		Update("last_run_at", &lastRunAt).Error
}
