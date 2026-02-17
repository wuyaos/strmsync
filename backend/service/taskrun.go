// Package taskrun 实现TaskRun记录管理服务
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/strmsync/strmsync/core"
	"gorm.io/gorm"
)

// Service TaskRun服务实现
type taskRunService struct {
	db *gorm.DB
}

// NewService 创建TaskRun服务
func NewTaskRunService(db *gorm.DB) TaskRunService {
	return &taskRunService{
		db: db,
	}
}

// Start 创建并开始TaskRun
func (s *taskRunService) Start(ctx context.Context, jobID JobID) (TaskRunID, error) {
	now := time.Now()
	taskRun := &core.TaskRun{
		JobID:     jobID,
		Status:    "running",
		StartedAt: now,
	}

	if err := s.db.WithContext(ctx).Create(taskRun).Error; err != nil {
		return 0, fmt.Errorf("taskrun: start: create task_run failed: %w", err)
	}

	return taskRun.ID, nil
}

// UpdateProgress 更新进度
func (s *taskRunService) UpdateProgress(ctx context.Context, taskRunID TaskRunID, processed int, total int) error {
	updates := map[string]interface{}{
		"processed_count": processed,
		"total_count":     total,
	}

	if err := s.db.WithContext(ctx).
		Model(&core.TaskRun{}).
		Where("id = ?", taskRunID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("taskrun: update_progress: %w", err)
	}

	return nil
}

// Complete 标记TaskRun完成
func (s *taskRunService) Complete(ctx context.Context, taskRunID TaskRunID, summary *TaskRunSummary) error {
	if summary == nil {
		return fmt.Errorf("taskrun: complete: summary is nil")
	}

	// 确保EndedAt已设置
	if summary.EndedAt.IsZero() {
		summary.EndedAt = time.Now()
	}

	// 计算duration（如果未设置）
	if summary.Duration == 0 && !summary.StartedAt.IsZero() {
		summary.Duration = int64(summary.EndedAt.Sub(summary.StartedAt).Seconds())
		if summary.Duration < 0 {
			summary.Duration = 0
		}
	}

	updates := map[string]interface{}{
		"status":      "completed",
		"ended_at":    summary.EndedAt,
		"duration":    summary.Duration,
		"files_added": summary.CreatedCount,
		"files_updated": summary.UpdatedCount,
		"files_deleted": summary.DeletedCount,
	}

	var taskRun core.TaskRun
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新记录
		if err := tx.Model(&core.TaskRun{}).
			Where("id = ?", taskRunID).
			Updates(updates).Error; err != nil {
			return err
		}

		// 读取更新后的记录以获取关联的Job ID
		if err := tx.First(&taskRun, taskRunID).Error; err != nil {
			return err
		}

		// 更新Job的last_run_at
		now := time.Now()
		if err := tx.Model(&core.Job{}).
			Where("id = ?", taskRun.JobID).
			Updates(map[string]interface{}{
				"last_run_at": &now,
				"status":      "idle",
			}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("taskrun: complete: %w", err)
	}

	return nil
}

// Fail 标记TaskRun失败
func (s *taskRunService) Fail(ctx context.Context, taskRunID TaskRunID, taskErr error) error {
	now := time.Now()

	// 读取TaskRun以获取started_at
	var taskRun core.TaskRun
	if err := s.db.WithContext(ctx).First(&taskRun, taskRunID).Error; err != nil {
		return fmt.Errorf("taskrun: fail: get task_run: %w", err)
	}

	// 计算duration
	duration := int64(now.Sub(taskRun.StartedAt).Seconds())
	if duration < 0 {
		duration = 0
	}

	errorMsg := ""
	if taskErr != nil {
		errorMsg = taskErr.Error()
	}

	updates := map[string]interface{}{
		"status":        "failed",
		"ended_at":      &now,
		"duration":      duration,
		"error_message": errorMsg,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新TaskRun
		if err := tx.Model(&core.TaskRun{}).
			Where("id = ?", taskRunID).
			Updates(updates).Error; err != nil {
			return err
		}

		// 更新Job状态为error
		if err := tx.Model(&core.Job{}).
			Where("id = ?", taskRun.JobID).
			Updates(map[string]interface{}{
				"status": "error",
			}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("taskrun: fail: %w", err)
	}

	return nil
}

// Cancel 标记TaskRun被取消（幂等操作）
func (s *taskRunService) Cancel(ctx context.Context, taskRunID TaskRunID) error {
	now := time.Now()

	// 读取TaskRun以获取started_at和当前状态
	var taskRun core.TaskRun
	if err := s.db.WithContext(ctx).First(&taskRun, taskRunID).Error; err != nil {
		return fmt.Errorf("taskrun: cancel: get task_run: %w", err)
	}

	// 幂等性检查：如果已经是cancelled或其他终态，直接返回成功
	if taskRun.Status == "cancelled" || taskRun.Status == "completed" || taskRun.Status == "failed" {
		return nil // 已经是终态，无需重复操作
	}

	// 计算duration
	duration := int64(now.Sub(taskRun.StartedAt).Seconds())
	if duration < 0 {
		duration = 0
	}

	updates := map[string]interface{}{
		"status":   "cancelled",
		"ended_at": &now,
		"duration": duration,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 原子更新：仅当status为running时才更新（防止并发冲突）
		result := tx.Model(&core.TaskRun{}).
			Where("id = ? AND status = ?", taskRunID, "running").
			Updates(updates)

		if result.Error != nil {
			return result.Error
		}

		// 如果RowsAffected=0，说明状态已被其他goroutine更新，属于正常情况
		if result.RowsAffected == 0 {
			return nil // 幂等返回成功
		}

		// 更新Job状态为idle
		if err := tx.Model(&core.Job{}).
			Where("id = ?", taskRun.JobID).
			Updates(map[string]interface{}{
				"status": "idle",
			}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("taskrun: cancel: %w", err)
	}

	return nil
}
