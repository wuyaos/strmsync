package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/strmsync/strmsync/core"
	"github.com/strmsync/strmsync/syncqueue"
	"go.uber.org/zap"
)

// =============================================================
// Mock 实现
// =============================================================

// mockQueue 模拟任务队列
type mockQueue struct {
	mu       sync.Mutex
	tasks    []*core.TaskRun
	enqueued int32
}

func (q *mockQueue) Enqueue(ctx context.Context, task *core.TaskRun) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, t := range q.tasks {
		if t.DedupKey == task.DedupKey {
			return syncqueue.ErrDuplicateTask
		}
	}
	task.ID = uint(len(q.tasks) + 1)
	q.tasks = append(q.tasks, task)
	atomic.AddInt32(&q.enqueued, 1)
	return nil
}

func (q *mockQueue) count() int {
	return int(atomic.LoadInt32(&q.enqueued))
}

// mockJobRepo 模拟 Job 仓储
type mockJobRepo struct {
	mu   sync.RWMutex
	jobs map[uint]core.Job
}

func newMockJobRepo(jobs ...core.Job) *mockJobRepo {
	repo := &mockJobRepo{jobs: make(map[uint]core.Job)}
	for _, j := range jobs {
		repo.jobs[j.ID] = j
	}
	return repo
}

func (r *mockJobRepo) ListEnabledJobs(ctx context.Context) ([]core.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []core.Job
	for _, j := range r.jobs {
		if j.Enabled {
			result = append(result, j)
		}
	}
	return result, nil
}

func (r *mockJobRepo) GetByID(ctx context.Context, id uint) (core.Job, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	j, ok := r.jobs[id]
	if !ok {
		return core.Job{}, errors.New("job not found")
	}
	return j, nil
}

// =============================================================
// 测试辅助函数
// =============================================================

// waitForCount 轮询等待队列入队数量达到目标值
//
// 使用 10ms 间隔轮询，超时后 Fatal。
// 比固定 time.Sleep 更可靠，不受 cron 秒级对齐影响。
func waitForCount(t *testing.T, queue *mockQueue, want int, timeout time.Duration) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(timeout)
	for {
		if queue.count() >= want {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			t.Fatalf("timeout waiting for enqueued count>=%d, got=%d", want, queue.count())
		}
	}
}

// =============================================================
// 构造器测试
// =============================================================

func TestNewScheduler_NilQueue(t *testing.T) {
	_, err := NewScheduler(SchedulerConfig{
		Queue: nil,
		Jobs:  newMockJobRepo(),
	})
	if err == nil {
		t.Fatal("expected error for nil queue")
	}
}

func TestNewScheduler_NilJobs(t *testing.T) {
	_, err := NewScheduler(SchedulerConfig{
		Queue: &mockQueue{},
		Jobs:  nil,
	})
	if err == nil {
		t.Fatal("expected error for nil jobs")
	}
}

// =============================================================
// 生命周期测试
// =============================================================

func TestScheduler_StartStop(t *testing.T) {
	queue := &mockQueue{}
	jobs := newMockJobRepo()

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	// 不能重复启动
	if err := s.Start(ctx); err == nil {
		t.Fatal("expected error for double start")
	}

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// 停止后 Stop 应幂等
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("stop again: %v", err)
	}
}

// TestScheduler_Restart 验证 Stop 后重新 Start 能继续触发 cron
//
// 注意：robfig/cron v3 的 @every 最小间隔为 1s（内部 floor），
// 因此使用 @every 1s 并通过 waitForCount 轮询等待。
func TestScheduler_Restart(t *testing.T) {
	queue := &mockQueue{}
	job := core.Job{ID: 1, Name: "test", Enabled: true, Cron: "@every 1s"}
	jobs := newMockJobRepo(job)

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
		Now:    time.Now,
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	ctx := context.Background()

	// 第一次启动：等待至少 1 个任务入队
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	waitForCount(t, queue, 1, 3*time.Second)
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("stop 1: %v", err)
	}
	count1 := queue.count()

	// 第二次启动：验证重启后能继续触发（Critical fix 验证）
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start 2: %v", err)
	}
	waitForCount(t, queue, count1+1, 3*time.Second)
	if err := s.Stop(ctx); err != nil {
		t.Fatalf("stop 2: %v", err)
	}
	count2 := queue.count()
	if count2 <= count1 {
		t.Fatalf("restart did not produce new tasks: before=%d, after=%d", count1, count2)
	}
}

// =============================================================
// 运行状态检查测试
// =============================================================

func TestScheduler_OperationsRequireRunning(t *testing.T) {
	queue := &mockQueue{}
	jobs := newMockJobRepo()

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	ctx := context.Background()

	// 未启动时应拒绝操作
	if err := s.UpsertJob(ctx, core.Job{ID: 1}); err == nil {
		t.Error("UpsertJob should fail when not running")
	}
	if err := s.RemoveJob(ctx, 1); err == nil {
		t.Error("RemoveJob should fail when not running")
	}
	if err := s.Reload(ctx); err == nil {
		t.Error("Reload should fail when not running")
	}
}

// =============================================================
// UpsertJob 测试
// =============================================================

func TestScheduler_UpsertJob(t *testing.T) {
	queue := &mockQueue{}
	jobs := newMockJobRepo()

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
		Now:    time.Now,
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Stop(ctx)

	// 添加一个新任务
	job := core.Job{ID: 1, Name: "test-upsert", Enabled: true, Cron: "@every 1s"}
	jobs.mu.Lock()
	jobs.jobs[1] = job
	jobs.mu.Unlock()

	if err := s.UpsertJob(ctx, job); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// 等待触发（轮询而非固定 sleep）
	waitForCount(t, queue, 1, 3*time.Second)
}

// =============================================================
// RemoveJob 测试
// =============================================================

func TestScheduler_RemoveJob(t *testing.T) {
	queue := &mockQueue{}
	job := core.Job{ID: 1, Name: "test-remove", Enabled: true, Cron: "@every 1s"}
	jobs := newMockJobRepo(job)

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
		Now:    time.Now,
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Stop(ctx)

	// 等待至少一次触发
	waitForCount(t, queue, 1, 3*time.Second)
	countBefore := queue.count()

	// 移除任务
	if err := s.RemoveJob(ctx, 1); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// 移除后等待 2 个 cron 周期，验证不再触发
	time.Sleep(2500 * time.Millisecond)
	countAfter := queue.count()
	if countAfter != countBefore {
		t.Errorf("tasks after remove: before=%d, after=%d", countBefore, countAfter)
	}
}

// =============================================================
// Cron 表达式错误处理测试
// =============================================================

func TestScheduler_InvalidCronSpec(t *testing.T) {
	queue := &mockQueue{}
	job := core.Job{ID: 1, Name: "bad-cron", Enabled: true, Cron: "not-a-valid-cron"}
	jobs := newMockJobRepo(job)

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	// 无效 cron 表达式不应阻塞 Start
	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start should not fail for invalid cron: %v", err)
	}
	s.Stop(ctx)
}

func TestScheduler_DisabledJob(t *testing.T) {
	queue := &mockQueue{}
	job := core.Job{ID: 1, Name: "disabled", Enabled: false, Cron: "@every 1s"}
	jobs := newMockJobRepo(job)

	s, err := NewScheduler(SchedulerConfig{
		Queue:  queue,
		Jobs:   jobs,
		Logger: zap.NewNop(),
	})
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}

	ctx := context.Background()
	if err := s.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	// 等待 2 个 cron 周期，禁用任务不应触发
	time.Sleep(2500 * time.Millisecond)
	s.Stop(ctx)

	if queue.count() != 0 {
		t.Error("disabled job should not trigger tasks")
	}
}
