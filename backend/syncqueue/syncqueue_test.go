package syncqueue

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/strmsync/strmsync/core"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB 创建内存 SQLite 数据库并自动迁移
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&core.TaskRun{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 每次测试前清空数据
	db.Exec("DELETE FROM task_runs")
	return db
}

// =============================================================
// 状态机测试
// =============================================================

func TestTaskStatusTransitions(t *testing.T) {
	cases := []struct {
		from TaskStatus
		to   TaskStatus
		ok   bool
	}{
		// Pending → Running / Cancelled
		{TaskPending, TaskRunning, true},
		{TaskPending, TaskCancelled, true},
		{TaskPending, TaskCompleted, false},
		{TaskPending, TaskFailed, false},

		// Running → Completed / Failed / Cancelled / Pending(retry)
		{TaskRunning, TaskCompleted, true},
		{TaskRunning, TaskFailed, true},
		{TaskRunning, TaskCancelled, true},
		{TaskRunning, TaskPending, true},

		// Failed → Running / Pending(manual retry)
		{TaskFailed, TaskRunning, true},
		{TaskFailed, TaskPending, true},
		{TaskFailed, TaskCompleted, false},

		// Completed → 不可转移(终态)
		{TaskCompleted, TaskRunning, false},
		{TaskCompleted, TaskPending, false},
		{TaskCompleted, TaskFailed, false},

		// Cancelled → 不可转移(终态)
		{TaskCancelled, TaskRunning, false},
		{TaskCancelled, TaskPending, false},
	}

	for _, c := range cases {
		got := c.from.CanTransitionTo(c.to)
		if got != c.ok {
			t.Errorf("%s -> %s: expected %v, got %v", c.from, c.to, c.ok, got)
		}
	}
}

// =============================================================
// 错误分类测试
// =============================================================

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected FailureKind
	}{
		{
			name:     "TaskError with retryable kind",
			err:      &TaskError{Kind: FailureRetryable, Err: errors.New("timeout")},
			expected: FailureRetryable,
		},
		{
			name:     "TaskError with permanent kind",
			err:      &TaskError{Kind: FailurePermanent, Err: errors.New("bad config")},
			expected: FailurePermanent,
		},
		{
			name:     "context.Canceled",
			err:      context.Canceled,
			expected: FailureCancelled,
		},
		{
			name:     "context.DeadlineExceeded",
			err:      context.DeadlineExceeded,
			expected: FailureCancelled,
		},
		{
			name:     "net.DNSError with timeout",
			err:      &net.DNSError{IsTimeout: true},
			expected: FailureRetryable,
		},
		{
			name:     "syscall.EAGAIN",
			err:      syscall.EAGAIN,
			expected: FailureRetryable,
		},
		{
			name:     "syscall.ECONNRESET",
			err:      syscall.ECONNRESET,
			expected: FailureRetryable,
		},
		{
			name:     "unknown error",
			err:      errors.New("something went wrong"),
			expected: FailurePermanent,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: FailurePermanent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyError(tt.err)
			if got != tt.expected {
				t.Errorf("classifyError(%v): expected %s, got %s", tt.err, tt.expected, got)
			}
		})
	}
}

// =============================================================
// retryDelay 测试
// =============================================================

func TestRetryDelay(t *testing.T) {
	tests := []struct {
		attempts int
		expected time.Duration
	}{
		{0, 10 * time.Second},
		{1, 10 * time.Second},
		{2, 20 * time.Second},
		{3, 30 * time.Second},
		{30, 5 * time.Minute},  // 300s = 5min, 命中上限
		{100, 5 * time.Minute}, // 远超上限
	}

	for _, tt := range tests {
		got := retryDelay(tt.attempts)
		if got != tt.expected {
			t.Errorf("retryDelay(%d): expected %v, got %v", tt.attempts, tt.expected, got)
		}
	}
}

// =============================================================
// Enqueue 测试
// =============================================================

func TestEnqueue_DefaultValues(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := &core.TaskRun{JobID: 1}
	if err := q.Enqueue(context.Background(), task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}

	if stored.Status != string(TaskPending) {
		t.Errorf("status: expected %s, got %s", TaskPending, stored.Status)
	}
	if stored.Priority != int(TaskPriorityNormal) {
		t.Errorf("priority: expected %d, got %d", TaskPriorityNormal, stored.Priority)
	}
	if stored.MaxAttempts != 3 {
		t.Errorf("max_attempts: expected 3, got %d", stored.MaxAttempts)
	}
	if stored.Attempts != 0 {
		t.Errorf("attempts: expected 0, got %d", stored.Attempts)
	}
	if stored.AvailableAt.IsZero() {
		t.Error("available_at should be set")
	}
	if stored.DedupKey == "" {
		t.Error("dedup_key should be set")
	}
}

func TestEnqueue_DuplicateKey(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := &core.TaskRun{JobID: 1, DedupKey: "unique-key-1"}
	if err := q.Enqueue(context.Background(), task); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}

	dup := &core.TaskRun{JobID: 1, DedupKey: "unique-key-1"}
	err = q.Enqueue(context.Background(), dup)
	if !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("expected ErrDuplicateTask, got %v", err)
	}
}

func TestEnqueue_NilTask(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	if err := q.Enqueue(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil task")
	}
}

// =============================================================
// ClaimNext 测试
// =============================================================

func TestClaimNext_PriorityOrdering(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	now := time.Now().Add(-time.Second)
	tasks := []*core.TaskRun{
		{JobID: 1, Priority: int(TaskPriorityNormal), AvailableAt: now, DedupKey: "a"},
		{JobID: 2, Priority: int(TaskPriorityHigh), AvailableAt: now, DedupKey: "b"},
		{JobID: 3, Priority: int(TaskPriorityLow), AvailableAt: now, DedupKey: "c"},
	}
	for _, task := range tasks {
		if err := q.Enqueue(context.Background(), task); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	// 应该先领取高优先级任务 (JobID=2, Priority=1)
	claimed, err := q.ClaimNext(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if claimed == nil {
		t.Fatal("expected a task, got nil")
	}
	if claimed.JobID != 2 {
		t.Errorf("expected job 2 (high priority), got job %d", claimed.JobID)
	}
	if claimed.Status != string(TaskRunning) {
		t.Errorf("expected running status, got %s", claimed.Status)
	}
	if claimed.WorkerID != "worker-1" {
		t.Errorf("expected worker-1, got %s", claimed.WorkerID)
	}
}

func TestClaimNext_EmptyQueue(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	claimed, err := q.ClaimNext(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if claimed != nil {
		t.Fatalf("expected nil for empty queue, got %+v", claimed)
	}
}

func TestClaimNext_RespectsAvailableAt(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	// 创建一个未来才可执行的任务
	future := &core.TaskRun{
		JobID:       1,
		Priority:    int(TaskPriorityHigh),
		AvailableAt: time.Now().Add(1 * time.Hour),
		DedupKey:    "future-1",
	}
	if err := q.Enqueue(context.Background(), future); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// 不应该领取到未来的任务
	claimed, err := q.ClaimNext(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("claim next: %v", err)
	}
	if claimed != nil {
		t.Fatalf("should not claim future task, got %+v", claimed)
	}
}

func TestClaimNext_MissingWorkerID(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	_, err = q.ClaimNext(context.Background(), "")
	if !errors.Is(err, ErrMissingWorkerID) {
		t.Fatalf("expected ErrMissingWorkerID, got %v", err)
	}
}

// =============================================================
// Complete 测试
// =============================================================

func TestComplete(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	// 直接创建一个 Running 状态的任务
	task := core.TaskRun{
		JobID:     1,
		Status:    string(TaskRunning),
		Priority:  int(TaskPriorityNormal),
		DedupKey:  "complete-test",
		StartedAt: time.Now().Add(-2 * time.Second),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := q.Complete(context.Background(), task.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != string(TaskCompleted) {
		t.Errorf("expected completed, got %s", stored.Status)
	}
	if stored.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestComplete_InvalidTransition(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	// 创建一个 Pending 状态的任务，不能直接 Complete
	task := core.TaskRun{
		JobID:    1,
		Status:   string(TaskPending),
		Priority: int(TaskPriorityNormal),
		DedupKey: "invalid-transition",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	err = q.Complete(context.Background(), task.ID)
	if err == nil {
		t.Fatal("expected error for invalid transition")
	}
}

// =============================================================
// Fail 测试
// =============================================================

func TestFail_Retryable(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := core.TaskRun{
		JobID:       1,
		Status:      string(TaskRunning),
		Priority:    int(TaskPriorityNormal),
		DedupKey:    "retry-test",
		Attempts:    0,
		MaxAttempts: 3,
		StartedAt:   time.Now().Add(-1 * time.Second),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	retryErr := &TaskError{Kind: FailureRetryable, Err: errors.New("network timeout")}
	if err := q.Fail(context.Background(), task.ID, retryErr); err != nil {
		t.Fatalf("fail: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	// 可重试且未超过最大次数，应转为 Pending
	if stored.Status != string(TaskPending) {
		t.Errorf("expected pending (retry), got %s", stored.Status)
	}
	if stored.Attempts != 1 {
		t.Errorf("attempts: expected 1, got %d", stored.Attempts)
	}
	if !stored.AvailableAt.After(time.Now()) {
		t.Error("available_at should be in the future for retry")
	}
}

func TestFail_Permanent(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := core.TaskRun{
		JobID:       1,
		Status:      string(TaskRunning),
		Priority:    int(TaskPriorityNormal),
		DedupKey:    "permanent-test",
		Attempts:    0,
		MaxAttempts: 3,
		StartedAt:   time.Now().Add(-1 * time.Second),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	permErr := &TaskError{Kind: FailurePermanent, Err: errors.New("bad configuration")}
	if err := q.Fail(context.Background(), task.ID, permErr); err != nil {
		t.Fatalf("fail: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	// 永久失败，应转为 Failed
	if stored.Status != string(TaskFailed) {
		t.Errorf("expected failed, got %s", stored.Status)
	}
	if stored.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestFail_MaxAttemptsExceeded(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := core.TaskRun{
		JobID:       1,
		Status:      string(TaskRunning),
		Priority:    int(TaskPriorityNormal),
		DedupKey:    "max-attempts-test",
		Attempts:    2, // 已重试 2 次
		MaxAttempts: 3,
		StartedAt:   time.Now().Add(-1 * time.Second),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	retryErr := &TaskError{Kind: FailureRetryable, Err: errors.New("still failing")}
	if err := q.Fail(context.Background(), task.ID, retryErr); err != nil {
		t.Fatalf("fail: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	// 达到最大重试次数，应转为 Failed
	if stored.Status != string(TaskFailed) {
		t.Errorf("expected failed (max attempts), got %s", stored.Status)
	}
}

// =============================================================
// Cancel 测试
// =============================================================

func TestCancel_Pending(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := core.TaskRun{
		JobID:    1,
		Status:   string(TaskPending),
		Priority: int(TaskPriorityNormal),
		DedupKey: "cancel-pending",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := q.Cancel(context.Background(), task.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != string(TaskCancelled) {
		t.Errorf("expected cancelled, got %s", stored.Status)
	}
}

func TestCancel_Running(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := core.TaskRun{
		JobID:     1,
		Status:    string(TaskRunning),
		Priority:  int(TaskPriorityNormal),
		DedupKey:  "cancel-running",
		StartedAt: time.Now().Add(-1 * time.Second),
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	if err := q.Cancel(context.Background(), task.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != string(TaskCancelled) {
		t.Errorf("expected cancelled, got %s", stored.Status)
	}
}

func TestCancel_InvalidTransition(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	task := core.TaskRun{
		JobID:    1,
		Status:   string(TaskCompleted),
		Priority: int(TaskPriorityNormal),
		DedupKey: "cancel-completed",
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	err = q.Cancel(context.Background(), task.ID)
	if err == nil {
		t.Fatal("expected error for cancelling completed task")
	}
}

// =============================================================
// List 测试
// =============================================================

func TestList_Filters(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	tasks := []core.TaskRun{
		{JobID: 1, Status: string(TaskPending), Priority: int(TaskPriorityNormal), DedupKey: "list-a"},
		{JobID: 2, Status: string(TaskRunning), Priority: int(TaskPriorityHigh), DedupKey: "list-b", WorkerID: "w1"},
		{JobID: 3, Status: string(TaskCompleted), Priority: int(TaskPriorityLow), DedupKey: "list-c"},
	}
	for i := range tasks {
		if err := db.Create(&tasks[i]).Error; err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	// 按状态过滤
	status := TaskRunning
	got, err := q.List(context.Background(), TaskFilter{Status: &status})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].JobID != 2 {
		t.Errorf("filter by status: expected 1 running task (job 2), got %d", len(got))
	}

	// 按 JobID 过滤
	jobID := uint(1)
	got, err = q.List(context.Background(), TaskFilter{JobID: &jobID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 1 || got[0].JobID != 1 {
		t.Errorf("filter by job_id: expected 1 task (job 1), got %d", len(got))
	}

	// 无过滤
	got, err = q.List(context.Background(), TaskFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("no filter: expected 3 tasks, got %d", len(got))
	}
}

// =============================================================
// 完整工作流测试
// =============================================================

func TestFullWorkflow_EnqueueClaimComplete(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	// 1. 入队
	task := &core.TaskRun{JobID: 1, DedupKey: "workflow-1"}
	if err := q.Enqueue(context.Background(), task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// 2. 领取
	claimed, err := q.ClaimNext(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed == nil || claimed.ID != task.ID {
		t.Fatal("claim returned wrong task")
	}

	// 3. 完成
	if err := q.Complete(context.Background(), claimed.ID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// 4. 验证最终状态
	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != string(TaskCompleted) {
		t.Errorf("final status: expected completed, got %s", stored.Status)
	}

	// 5. 队列应为空
	next, err := q.ClaimNext(context.Background(), "worker-2")
	if err != nil {
		t.Fatalf("claim after complete: %v", err)
	}
	if next != nil {
		t.Error("queue should be empty after completion")
	}
}

func TestFullWorkflow_EnqueueClaimFailRetry(t *testing.T) {
	db := newTestDB(t)
	q, err := NewSyncQueue(db)
	if err != nil {
		t.Fatalf("new queue: %v", err)
	}

	// 1. 入队
	task := &core.TaskRun{JobID: 1, DedupKey: "workflow-retry"}
	if err := q.Enqueue(context.Background(), task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// 2. 领取
	claimed, err := q.ClaimNext(context.Background(), "worker-1")
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	// 3. 失败（可重试）
	retryErr := &TaskError{Kind: FailureRetryable, Err: errors.New("temporary failure")}
	if err := q.Fail(context.Background(), claimed.ID, retryErr); err != nil {
		t.Fatalf("fail: %v", err)
	}

	// 4. 验证已转为 Pending 且 available_at 在未来
	var stored core.TaskRun
	if err := db.First(&stored, task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if stored.Status != string(TaskPending) {
		t.Errorf("after retry: expected pending, got %s", stored.Status)
	}
	if stored.Attempts != 1 {
		t.Errorf("attempts: expected 1, got %d", stored.Attempts)
	}
}
