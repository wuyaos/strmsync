// Package repository 定义领域层的Repository接口
package repository

import (
	"context"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
)

// JobRepository Job仓储接口
type JobRepository interface {
	GetByID(ctx context.Context, id uint) (model.Job, error)
	ListEnabledJobs(ctx context.Context) ([]model.Job, error)
	UpdateStatus(ctx context.Context, id uint, status string) error
	UpdateLastRunAt(ctx context.Context, id uint, lastRunAt time.Time) error
}
