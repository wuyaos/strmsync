package syncqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 队列相关错误定义
var (
	// ErrDuplicateTask 任务已存在错误
	// 当尝试入队一个 DedupKey 已存在的任务时返回
	ErrDuplicateTask = errors.New("task already enqueued")

	// ErrInvalidTransition 无效的状态转移错误
	// 当尝试进行不合法的状态转移时返回
	ErrInvalidTransition = errors.New("invalid task status transition")

	// ErrMissingWorkerID Worker ID 缺失错误
	// 当 ClaimNext 没有提供 WorkerID 时返回
	ErrMissingWorkerID = errors.New("worker id is required")
)

// TaskFilter 任务查询过滤器
//
// 用于过滤和分页查询任务列表。
// 所有字段都是可选的，nil 表示不过滤该字段。
type TaskFilter struct {
	Status   *TaskStatus   // 按状态过滤
	JobID    *uint         // 按任务ID过滤
	WorkerID *string       // 按Worker ID过滤
	Priority *TaskPriority // 按优先级过滤
	Limit    int           // 返回结果数量限制（默认100）
	Offset   int           // 跳过的结果数量
}

// SyncQueue 同步任务队列
//
// 基于数据库实现的任务队列，支持：
// - 优先级调度
// - 状态管理
// - 原子的任务领取
// - 重试机制
// - 并发安全
//
// 使用示例：
//
//	queue, err := NewSyncQueue(db)
//	if err != nil {
//	    return err
//	}
//
//	// 入队
//	task := &model.TaskRun{
//	    JobID: 1,
//	    DedupKey: "job:1:manual:123",
//	}
//	if err := queue.Enqueue(ctx, task); err != nil {
//	    return err
//	}
//
//	// 领取任务
//	task, err := queue.ClaimNext(ctx, "worker-1")
//	if err != nil {
//	    return err
//	}
//
//	// 标记完成
//	if err := queue.Complete(ctx, task.ID); err != nil {
//	    return err
//	}
type SyncQueue struct {
	db  *gorm.DB
	log *zap.Logger
}

// NewSyncQueue 创建任务队列
//
// 参数：
//   - db: GORM 数据库连接（不能为 nil）
//
// 返回：
//   - *SyncQueue: 队列实例
//   - error: 创建失败时返回错误
func NewSyncQueue(db *gorm.DB) (*SyncQueue, error) {
	if db == nil {
		return nil, fmt.Errorf("syncqueue: db is nil")
	}

	return &SyncQueue{
		db:  db,
		log: logger.With(zap.String("component", "syncqueue")),
	}, nil
}

// ClaimNext 原子领取下一个待执行任务
//
// 这是队列的核心方法，实现了原子的任务领取逻辑：
// 1. 在事务中查询一个待执行任务（按优先级和可执行时间排序）
// 2. 检查状态转移的合法性
// 3. 更新任务状态为 Running，记录 WorkerID 和开始时间
// 4. 提交事务
//
// 如果没有可用任务，返回 (nil, nil)。
//
// 并发安全性：
// - 使用数据库事务保证原子性
// - WHERE 条件中包含 status 检查，防止并发领取
//
// 参数：
//   - ctx: 上下文，用于取消
//   - workerID: Worker 标识（不能为空）
//
// 返回：
//   - *model.TaskRun: 领取到的任务，如果没有可用任务则返回 nil
//   - error: 领取失败时返回错误
func (q *SyncQueue) ClaimNext(ctx context.Context, workerID string) (*model.TaskRun, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("syncqueue: db not initialized")
	}
	if workerID == "" {
		return nil, ErrMissingWorkerID
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()

	// 开始事务
	tx := q.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("claim next begin transaction: %w", tx.Error)
	}

	// 查询一个待执行任务
	var task model.TaskRun
	if err := tx.Where("status = ? AND available_at <= ?", string(TaskPending), now).
		Order("priority asc, available_at asc, id asc").
		Limit(1).
		Take(&task).Error; err != nil {

		// 没有找到任务（正常情况）
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if commitErr := tx.Commit().Error; commitErr != nil {
				return nil, fmt.Errorf("claim next commit empty: %w", commitErr)
			}
			return nil, nil
		}

		// 查询失败
		_ = tx.Rollback().Error
		q.log.Error("claim next select failed", zap.Error(err))
		return nil, fmt.Errorf("claim next select: %w", err)
	}

	// 检查状态转移的合法性
	if !TaskStatus(task.Status).CanTransitionTo(TaskRunning) {
		_ = tx.Rollback().Error
		return nil, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, task.Status, TaskRunning)
	}

	// 更新任务状态
	updates := map[string]any{
		"status":     string(TaskRunning),
		"worker_id":  workerID,
		"started_at": now,
	}

	res := tx.Model(&model.TaskRun{}).
		Where("id = ? AND status = ?", task.ID, string(TaskPending)).
		Updates(updates)

	if res.Error != nil {
		_ = tx.Rollback().Error
		q.log.Error("claim next update failed", zap.Error(res.Error))
		return nil, fmt.Errorf("claim next update: %w", res.Error)
	}

	// 如果没有更新任何行（可能被其他 Worker 抢先了），返回 nil
	if res.RowsAffected == 0 {
		_ = tx.Rollback().Error
		return nil, nil
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("claim next commit: %w", err)
	}

	// 更新返回的任务对象
	task.Status = string(TaskRunning)
	task.WorkerID = workerID
	task.StartedAt = now

	jobName := extractJobName(task.Payload)
	q.log.Debug("claimed task",
		zap.Uint("task_id", task.ID),
		zap.Uint("job_id", task.JobID),
		zap.String("job_name", jobName),
		zap.String("worker_id", workerID),
		zap.Int("priority", task.Priority))

	return &task, nil
}

// Complete 标记任务完成
//
// 将任务状态从 Running 转换为 Completed，并记录结束时间和执行时长。
//
// 参数：
//   - ctx: 上下文
//   - taskID: 任务ID
//
// 返回：
//   - error: 操作失败时返回错误
func (q *SyncQueue) Complete(ctx context.Context, taskID uint) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("syncqueue: db not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx := q.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("complete begin transaction: %w", tx.Error)
	}

	// 加载任务
	var task model.TaskRun
	if err := tx.First(&task, taskID).Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("complete load task: %w", err)
	}

	// 检查状态转移
	if !TaskStatus(task.Status).CanTransitionTo(TaskCompleted) {
		_ = tx.Rollback().Error
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, task.Status, TaskCompleted)
	}

	// 计算执行时长
	now := time.Now()
	duration := int64(0)
	if !task.StartedAt.IsZero() {
		duration = int64(now.Sub(task.StartedAt).Seconds())
	}

	// 更新状态
	updates := map[string]any{
		"status":       string(TaskCompleted),
		"ended_at":     now,
		"duration":     duration,
		"failure_kind": "", // 清除失败类型
	}

	res := tx.Model(&model.TaskRun{}).
		Where("id = ? AND status = ?", taskID, string(TaskRunning)).
		Updates(updates)

	if res.Error != nil {
		_ = tx.Rollback().Error
		q.log.Error("complete update failed", zap.Error(res.Error))
		return fmt.Errorf("complete update: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		_ = tx.Rollback().Error
		return fmt.Errorf("complete update: no rows affected")
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("complete commit: %w", err)
	}

	jobName := extractJobName(task.Payload)
	q.log.Info("task completed",
		zap.Uint("task_id", taskID),
		zap.Uint("job_id", task.JobID),
		zap.String("job_name", jobName),
		zap.Int64("duration", duration))

	return nil
}

// Fail 标记任务失败
//
// 根据失败类型和重试次数决定是否重试：
// - 如果是可重试错误且未超过最大重试次数，将任务转为 Pending 状态并延迟执行
// - 否则，将任务转为 Failed 状态
//
// 参数：
//   - ctx: 上下文
//   - taskID: 任务ID
//   - err: 失败错误
//
// 返回：
//   - error: 操作失败时返回错误
func (q *SyncQueue) Fail(ctx context.Context, taskID uint, err error) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("syncqueue: db not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx := q.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("fail begin transaction: %w", tx.Error)
	}

	// 加载任务
	var task model.TaskRun
	if loadErr := tx.First(&task, taskID).Error; loadErr != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("fail load task: %w", loadErr)
	}

	jobName := extractJobName(task.Payload)

	// 分类错误
	kind := classifyError(err)
	if kind == "" {
		kind = FailurePermanent
	}

	// 计算重试
	attempts := task.Attempts + 1
	maxAttempts := task.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = 3
	}

	retry := kind == FailureRetryable && attempts < maxAttempts
	now := time.Now()

	// 准备更新字段
	updates := map[string]any{
		"attempts":     attempts,
		"failure_kind": string(kind),
		"error_message": func() string {
			if err == nil {
				return ""
			}
			return err.Error()
		}(),
	}

	if retry {
		// 可重试：转为 Pending 状态，延迟执行
		if !TaskStatus(task.Status).CanTransitionTo(TaskPending) {
			_ = tx.Rollback().Error
			return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, task.Status, TaskPending)
		}

		updates["status"] = string(TaskPending)
		updates["available_at"] = now.Add(retryDelay(attempts))
		updates["started_at"] = time.Time{}
		updates["ended_at"] = nil
		updates["duration"] = int64(0)
		updates["worker_id"] = "" // 清除 WorkerID

		q.log.Warn("task will retry",
			zap.Uint("task_id", taskID),
			zap.Uint("job_id", task.JobID),
			zap.String("job_name", jobName),
			zap.Int("attempts", attempts),
			zap.Int("max_attempts", maxAttempts),
			zap.String("error", err.Error()))
	} else {
		// 不可重试或超过最大重试次数：转为 Failed 状态
		if !TaskStatus(task.Status).CanTransitionTo(TaskFailed) {
			_ = tx.Rollback().Error
			return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, task.Status, TaskFailed)
		}

		duration := int64(0)
		if !task.StartedAt.IsZero() {
			duration = int64(now.Sub(task.StartedAt).Seconds())
		}

		updates["status"] = string(TaskFailed)
		updates["ended_at"] = now
		updates["duration"] = duration

		q.log.Error("task failed",
			zap.Uint("task_id", taskID),
			zap.Uint("job_id", task.JobID),
			zap.String("job_name", jobName),
			zap.Int("attempts", attempts),
			zap.String("failure_kind", string(kind)),
			zap.String("error", err.Error()))
	}

	// 更新任务
	res := tx.Model(&model.TaskRun{}).
		Where("id = ? AND status = ?", taskID, string(TaskRunning)).
		Updates(updates)

	if res.Error != nil {
		_ = tx.Rollback().Error
		q.log.Error("fail update failed", zap.Error(res.Error))
		return fmt.Errorf("fail update: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		_ = tx.Rollback().Error
		return fmt.Errorf("fail update: no rows affected")
	}

	if commitErr := tx.Commit().Error; commitErr != nil {
		return fmt.Errorf("fail commit: %w", commitErr)
	}

	return nil
}

// Cancel 取消任务
//
// 将任务状态转为 Cancelled。
// 可以取消 Pending 或 Running 状态的任务。
//
// 参数：
//   - ctx: 上下文
//   - taskID: 任务ID
//
// 返回：
//   - error: 操作失败时返回错误
func (q *SyncQueue) Cancel(ctx context.Context, taskID uint) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("syncqueue: db not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx := q.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("cancel begin transaction: %w", tx.Error)
	}

	// 加载任务
	var task model.TaskRun
	if err := tx.First(&task, taskID).Error; err != nil {
		_ = tx.Rollback().Error
		return fmt.Errorf("cancel load task: %w", err)
	}

	// 检查状态转移
	if !TaskStatus(task.Status).CanTransitionTo(TaskCancelled) {
		_ = tx.Rollback().Error
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, task.Status, TaskCancelled)
	}

	// 计算执行时长
	now := time.Now()
	duration := int64(0)
	if !task.StartedAt.IsZero() {
		duration = int64(now.Sub(task.StartedAt).Seconds())
	}

	// 更新状态
	updates := map[string]any{
		"status":       string(TaskCancelled),
		"ended_at":     now,
		"duration":     duration,
		"failure_kind": string(FailureCancelled),
	}

	res := tx.Model(&model.TaskRun{}).
		Where("id = ? AND status IN (?)", taskID, []string{string(TaskPending), string(TaskRunning)}).
		Updates(updates)

	if res.Error != nil {
		_ = tx.Rollback().Error
		q.log.Error("cancel update failed", zap.Error(res.Error))
		return fmt.Errorf("cancel update: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		_ = tx.Rollback().Error
		return fmt.Errorf("cancel update: no rows affected")
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("cancel commit: %w", err)
	}

	jobName := extractJobName(task.Payload)
	q.log.Info("task cancelled",
		zap.Uint("task_id", taskID),
		zap.Uint("job_id", task.JobID),
		zap.String("job_name", jobName),
		zap.String("previous_status", task.Status))

	return nil
}

// Enqueue 入队新任务
//
// 创建一个新任务并加入队列。
// 如果 DedupKey 已存在，返回 ErrDuplicateTask。
//
// 参数：
//   - ctx: 上下文
//   - task: 任务对象（必须设置 JobID 和 DedupKey）
//
// 返回：
//   - error: 操作失败时返回错误
func (q *SyncQueue) Enqueue(ctx context.Context, task *model.TaskRun) error {
	if q == nil || q.db == nil {
		return fmt.Errorf("syncqueue: db not initialized")
	}
	if task == nil {
		return fmt.Errorf("enqueue: task is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()

	// 设置默认值
	if task.Priority == 0 {
		task.Priority = int(TaskPriorityNormal)
	}
	if task.MaxAttempts == 0 {
		task.MaxAttempts = 3
	}
	if task.AvailableAt.IsZero() {
		task.AvailableAt = now
	}
	if task.DedupKey == "" {
		task.DedupKey = fmt.Sprintf("job:%d:manual:%d", task.JobID, now.UnixNano())
	}

	task.Status = string(TaskPending)
	task.Attempts = 0

	// 创建任务
	if err := q.db.WithContext(ctx).Create(task).Error; err != nil {
		// 检查是否是重复键错误
		// GORM 的不同版本可能使用不同的错误类型
		// 这里检查错误消息中是否包含"UNIQUE"或"duplicate"
		errMsg := err.Error()
		if contains(errMsg, "UNIQUE") || contains(errMsg, "duplicate") {
			return ErrDuplicateTask
		}
		q.log.Error("enqueue failed", zap.Error(err))
		return fmt.Errorf("enqueue task: %w", err)
	}

	jobName := extractJobName(task.Payload)
	q.log.Info("task enqueued",
		zap.Uint("task_id", task.ID),
		zap.Uint("job_id", task.JobID),
		zap.String("job_name", jobName),
		zap.Int("priority", task.Priority),
		zap.String("dedup_key", task.DedupKey))

	return nil
}

// List 查询任务列表
//
// 根据过滤条件查询任务列表，支持分页。
//
// 参数：
//   - ctx: 上下文
//   - filter: 过滤条件
//
// 返回：
//   - []model.TaskRun: 任务列表
//   - error: 查询失败时返回错误
func (q *SyncQueue) List(ctx context.Context, filter TaskFilter) ([]model.TaskRun, error) {
	if q == nil || q.db == nil {
		return nil, fmt.Errorf("syncqueue: db not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	db := q.db.WithContext(ctx).Model(&model.TaskRun{})

	// 应用过滤条件
	if filter.Status != nil {
		db = db.Where("status = ?", string(*filter.Status))
	}
	if filter.JobID != nil {
		db = db.Where("job_id = ?", *filter.JobID)
	}
	if filter.WorkerID != nil {
		db = db.Where("worker_id = ?", *filter.WorkerID)
	}
	if filter.Priority != nil {
		db = db.Where("priority = ?", int(*filter.Priority))
	}

	// 分页
	if filter.Offset > 0 {
		db = db.Offset(filter.Offset)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	// 查询
	var tasks []model.TaskRun
	if err := db.Order("id desc").Limit(limit).Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	return tasks, nil
}

// retryDelay 计算重试延迟时间
//
// 使用指数退避策略，最长延迟不超过 5 分钟。
//
// 参数：
//   - attempts: 已重试次数
//
// 返回：
//   - time.Duration: 延迟时间
func retryDelay(attempts int) time.Duration {
	if attempts <= 0 {
		return 10 * time.Second
	}

	// 指数退避：10s, 20s, 30s, 40s, 50s, ...
	delay := time.Duration(attempts) * 10 * time.Second

	// 最长延迟 5 分钟
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}

	return delay
}

// contains 检查字符串是否包含子串（大小写不敏感）
//
// 参数：
//   - s: 源字符串
//   - substr: 子串
//
// 返回：
//   - bool: 是否包含
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func extractJobName(payload string) string {
	if strings.TrimSpace(payload) == "" {
		return ""
	}
	var data struct {
		JobName string `json:"job_name"`
	}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return ""
	}
	return strings.TrimSpace(data.JobName)
}
