package job

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"github.com/strmsync/strmsync/internal/domain/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mockTaskRunService 模拟 TaskRunService
type mockTaskRunService struct {
	startFn    func(ctx context.Context, jobID ports.JobID) (ports.TaskRunID, error)
	completeFn func(ctx context.Context, taskRunID ports.TaskRunID, summary *ports.TaskRunSummary) error
	failFn     func(ctx context.Context, taskRunID ports.TaskRunID, err error) error
	cancelFn   func(ctx context.Context, taskRunID ports.TaskRunID) error
}

func (m *mockTaskRunService) Start(ctx context.Context, jobID ports.JobID) (ports.TaskRunID, error) {
	if m.startFn == nil {
		return 1, nil // 默认返回 taskRunID=1
	}
	return m.startFn(ctx, jobID)
}

func (m *mockTaskRunService) UpdateProgress(ctx context.Context, taskRunID ports.TaskRunID, processed int, total int) error {
	return nil
}

func (m *mockTaskRunService) Complete(ctx context.Context, taskRunID ports.TaskRunID, summary *ports.TaskRunSummary) error {
	if m.completeFn == nil {
		return nil
	}
	return m.completeFn(ctx, taskRunID, summary)
}

func (m *mockTaskRunService) Fail(ctx context.Context, taskRunID ports.TaskRunID, err error) error {
	if m.failFn == nil {
		return nil
	}
	return m.failFn(ctx, taskRunID, err)
}

func (m *mockTaskRunService) Cancel(ctx context.Context, taskRunID ports.TaskRunID) error {
	if m.cancelFn == nil {
		return nil
	}
	return m.cancelFn(ctx, taskRunID)
}

// blockingExecutor 可控的 TaskExecutor，用于测试异步执行
type blockingExecutor struct {
	started   chan struct{}
	done      chan struct{}
	once      sync.Once
	resultErr error
	resultSum *ports.TaskRunSummary
}

func newBlockingExecutor() *blockingExecutor {
	return &blockingExecutor{
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (e *blockingExecutor) Execute(ctx context.Context, execCtx *ports.ExecutionContext) (*ports.TaskRunSummary, error) {
	e.once.Do(func() { close(e.started) })
	<-e.done
	if e.resultErr != nil {
		return nil, e.resultErr
	}
	if e.resultSum != nil {
		return e.resultSum, nil
	}
	return &ports.TaskRunSummary{
		StartedAt: time.Now(),
		EndedAt:   time.Now(),
	}, nil
}

// Release 释放阻塞的 Execute
func (e *blockingExecutor) Release() {
	close(e.done)
}

// WaitStarted 等待 Execute 被调用
func (e *blockingExecutor) WaitStarted(timeout time.Duration) bool {
	select {
	case <-e.started:
		return true
	case <-time.After(timeout):
		return false
	}
}

// newTestDB 创建测试用的内存数据库
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&model.Job{}, &model.TaskRun{}, &model.DataServer{}, &model.MediaServer{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

// createJob 创建测试用的 Job，支持自定义字段
//
// 使用 raw SQL 直接插入，完全绕开 GORM 的零值处理机制
// （避免 GORM 在 Create 时跳过 Enabled=false 等零值字段使用数据库默认值）
func createJob(t *testing.T, db *gorm.DB, mutate func(job *model.Job)) model.Job {
	t.Helper()

	job := model.Job{
		Name:       "test-job",
		Status:     "idle",
		Enabled:    true,
		WatchMode:  string(ports.WatchModeLocal),
		SourcePath: "/source",
		TargetPath: "/target",
		STRMPath:   "/strm",
		Options:    "{}",
	}

	if mutate != nil {
		mutate(&job)
	}

	// 使用 raw SQL 确保所有字段（包括 bool/string 零值）都被正确插入
	res := db.Exec(
		`INSERT INTO jobs (name, enabled, watch_mode, source_path, target_path, strm_path, options, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.Name, job.Enabled, job.WatchMode, job.SourcePath, job.TargetPath, job.STRMPath, job.Options, job.Status,
	)
	if res.Error != nil {
		t.Fatalf("create job: %v", res.Error)
	}

	// 如果有 data_server_id，单独更新
	if job.DataServerID != nil {
		db.Exec("UPDATE jobs SET data_server_id = ? WHERE name = ?", *job.DataServerID, job.Name)
	}

	// 读取回插入的记录（获取自增 ID）
	if err := db.Where("name = ?", job.Name).Last(&job).Error; err != nil {
		t.Fatalf("load created job: %v", err)
	}

	// 如果有 data_server_id，改用主键更新（确保更新到正确行）
	if job.DataServerID != nil {
		if err := db.Exec("UPDATE jobs SET data_server_id = ? WHERE id = ?", *job.DataServerID, job.ID).Error; err != nil {
			t.Fatalf("update data_server_id: %v", err)
		}
	}

	return job
}

// createDataServer 创建测试用的 DataServer
func createDataServer(t *testing.T, db *gorm.DB) model.DataServer {
	t.Helper()

	server := model.DataServer{
		Name:    "ds-1",
		Type:    "clouddrive2",
		Host:    "localhost",
		Port:    8080,
		Enabled: true,
		Options: "{}",
	}

	if err := db.Create(&server).Error; err != nil {
		t.Fatalf("create data server: %v", err)
	}

	return server
}

func TestJobService_Validate(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, db *gorm.DB) ports.JobID
		wantErrSubstr string
	}{
		{
			name: "job not found",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				return 999
			},
			wantErrSubstr: "job not found",
		},
		{
			name: "job disabled",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				job := createJob(t, db, func(j *model.Job) {
					j.Enabled = false
				})
				return job.ID
			},
			wantErrSubstr: "job is disabled",
		},
		{
			name: "watch_mode api requires data_server_id",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				job := createJob(t, db, func(j *model.Job) {
					j.WatchMode = string(ports.WatchModeAPI)
					j.DataServerID = nil
				})
				return job.ID
			},
			wantErrSubstr: "watch_mode=api requires data_server_id",
		},
		{
			name: "watch_mode api with data server not found",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				nonExistentID := uint(999)
				job := createJob(t, db, func(j *model.Job) {
					j.WatchMode = string(ports.WatchModeAPI)
					j.DataServerID = &nonExistentID
				})
				return job.ID
			},
			wantErrSubstr: "data_server not found",
		},
		{
			name: "watch_mode api with data server exists - success",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				server := createDataServer(t, db)
				job := createJob(t, db, func(j *model.Job) {
					j.WatchMode = string(ports.WatchModeAPI)
					j.DataServerID = &server.ID
				})
				return job.ID
			},
			wantErrSubstr: "",
		},
		{
			name: "watch_mode local - success",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				job := createJob(t, db, func(j *model.Job) {
					j.WatchMode = string(ports.WatchModeLocal)
				})
				return job.ID
			},
			wantErrSubstr: "",
		},
		{
			name: "source_path required",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				job := createJob(t, db, func(j *model.Job) {
					j.SourcePath = ""
				})
				return job.ID
			},
			wantErrSubstr: "source_path is required",
		},
		{
			name: "target_path required",
			setup: func(t *testing.T, db *gorm.DB) ports.JobID {
				job := createJob(t, db, func(j *model.Job) {
					j.TargetPath = ""
				})
				return job.ID
			},
			wantErrSubstr: "target_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			jobID := tt.setup(t, db)

			svc := NewJobService(db, &mockTaskRunService{}, newBlockingExecutor())
			err := svc.Validate(context.Background(), jobID)

			if tt.wantErrSubstr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected error containing %q, got nil", tt.wantErrSubstr)
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstr) {
				t.Errorf("error mismatch: want contains %q, got %q", tt.wantErrSubstr, err.Error())
			}
		})
	}
}

func TestJobService_Run_AlreadyRunning(t *testing.T) {
	db := newTestDB(t)
	job := createJob(t, db, nil)

	taskRunSvc := &mockTaskRunService{}
	executor := newBlockingExecutor()
	t.Cleanup(executor.Release)

	svc := NewJobService(db, taskRunSvc, executor)

	taskRunID, err := svc.Run(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	if taskRunID == 0 {
		t.Fatalf("first run returned zero taskRunID")
	}

	// 第二次运行应该返回 already running 错误
	_, err = svc.Run(context.Background(), job.ID)
	if err == nil {
		t.Fatalf("expected already running error, got nil")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestJobService_Stop_NotRunning(t *testing.T) {
	db := newTestDB(t)
	svc := NewJobService(db, &mockTaskRunService{}, newBlockingExecutor())

	err := svc.Stop(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected not running error, got nil")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestJobService_Run_JobDisabled(t *testing.T) {
	db := newTestDB(t)
	job := createJob(t, db, func(j *model.Job) {
		j.Enabled = false
	})

	svc := NewJobService(db, &mockTaskRunService{}, newBlockingExecutor())

	_, err := svc.Run(context.Background(), job.ID)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "job is disabled") {
		t.Errorf("unexpected error: %v", err)
	}
}
