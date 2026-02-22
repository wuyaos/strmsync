package taskrun

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"github.com/strmsync/strmsync/internal/domain/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newTestDB 创建测试用的内存数据库
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(&model.Job{}, &model.TaskRun{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

// createJob 创建测试用的 Job 记录
func createJob(t *testing.T, db *gorm.DB) model.Job {
	t.Helper()

	job := model.Job{
		Name:       "test-job",
		Status:     "running",
		Enabled:    true,
		WatchMode:  "local",
		SourcePath: "/source",
		TargetPath: "/target",
		STRMPath:   "/strm",
		Options:    model.JobOptions{},
	}

	if err := db.Create(&job).Error; err != nil {
		t.Fatalf("create job: %v", err)
	}

	return job
}

// loadTaskRun 从数据库加载 TaskRun
func loadTaskRun(t *testing.T, db *gorm.DB, id uint) model.TaskRun {
	t.Helper()

	var taskRun model.TaskRun
	if err := db.First(&taskRun, id).Error; err != nil {
		t.Fatalf("load task_run %d: %v", id, err)
	}

	return taskRun
}

// loadJob 从数据库加载 Job
func loadJob(t *testing.T, db *gorm.DB, id uint) model.Job {
	t.Helper()

	var job model.Job
	if err := db.First(&job, id).Error; err != nil {
		t.Fatalf("load job %d: %v", id, err)
	}

	return job
}

func TestTaskRunService_Start(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "creates running task run"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			job := createJob(t, db)
			svc := NewTaskRunService(db)

			id, err := svc.Start(context.Background(), ports.JobID(job.ID))
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			if id == 0 {
				t.Fatalf("start returned zero id")
			}

			taskRun := loadTaskRun(t, db, uint(id))
			if taskRun.Status != "running" {
				t.Errorf("status mismatch: want %q, got %q", "running", taskRun.Status)
			}
			if taskRun.JobID != job.ID {
				t.Errorf("job_id mismatch: want %d, got %d", job.ID, taskRun.JobID)
			}
			if taskRun.StartedAt.IsZero() {
				t.Errorf("started_at is zero")
			}
		})
	}
}

func TestTaskRunService_UpdateProgress(t *testing.T) {
	tests := []struct {
		name            string
		processed       int
		total           int
		expectedProgres int
	}{
		{
			name:            "updates processed and total counts",
			processed:       3,
			total:           10,
			expectedProgres: 30,
		},
		{
			name:            "handles 100 percent progress",
			processed:       10,
			total:           10,
			expectedProgres: 100,
		},
		{
			name:            "handles zero total",
			processed:       0,
			total:           0,
			expectedProgres: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			job := createJob(t, db)
			svc := NewTaskRunService(db)

			taskRunID, err := svc.Start(context.Background(), ports.JobID(job.ID))
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			if err := svc.UpdateProgress(context.Background(), taskRunID, tt.processed, tt.total); err != nil {
				t.Fatalf("update progress: %v", err)
			}

			taskRun := loadTaskRun(t, db, uint(taskRunID))
			if taskRun.ProcessedFiles != tt.processed {
				t.Errorf("processed_files mismatch: want %d, got %d", tt.processed, taskRun.ProcessedFiles)
			}
			if taskRun.TotalFiles != tt.total {
				t.Errorf("total_files mismatch: want %d, got %d", tt.total, taskRun.TotalFiles)
			}
			if taskRun.Progress != tt.expectedProgres {
				t.Errorf("progress mismatch: want %d, got %d", tt.expectedProgres, taskRun.Progress)
			}
		})
	}
}

func TestTaskRunService_Complete(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "marks task run completed and updates job"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			job := createJob(t, db)
			svc := NewTaskRunService(db)

			taskRunID, err := svc.Start(context.Background(), ports.JobID(job.ID))
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			taskRun := loadTaskRun(t, db, uint(taskRunID))
			summary := &ports.TaskRunSummary{
				CreatedCount: 2,
				UpdatedCount: 1,
				DeletedCount: 1,
				FailedCount:  0,
				StartedAt:    taskRun.StartedAt,
			}

			if err := svc.Complete(context.Background(), taskRunID, summary); err != nil {
				t.Fatalf("complete: %v", err)
			}

			updated := loadTaskRun(t, db, uint(taskRunID))
			if updated.Status != "completed" {
				t.Errorf("status mismatch: want %q, got %q", "completed", updated.Status)
			}
			if updated.EndedAt == nil || updated.EndedAt.IsZero() {
				t.Errorf("ended_at is nil or zero")
			}
			// 验证汇总统计：processedFiles = Created + Updated + Deleted = 4
			if updated.ProcessedFiles != 4 {
				t.Errorf("processed_files mismatch: want 4, got %d", updated.ProcessedFiles)
			}
			// 验证 totalFiles = ProcessedFiles + FailedFiles = 4 + 0 = 4
			if updated.TotalFiles != 4 {
				t.Errorf("total_files mismatch: want 4, got %d", updated.TotalFiles)
			}
			if updated.FailedFiles != 0 {
				t.Errorf("failed_files mismatch: want 0, got %d", updated.FailedFiles)
			}
			if updated.Progress != 100 {
				t.Errorf("progress mismatch: want 100, got %d", updated.Progress)
			}

			updatedJob := loadJob(t, db, job.ID)
			if updatedJob.Status != "idle" {
				t.Errorf("job status mismatch: want %q, got %q", "idle", updatedJob.Status)
			}
			if updatedJob.LastRunAt == nil {
				t.Errorf("job last_run_at is nil")
			}
		})
	}
}

func TestTaskRunService_Fail(t *testing.T) {
	tests := []struct {
		name    string
		taskErr error
	}{
		{
			name:    "marks task run failed and updates job",
			taskErr: errors.New("boom"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			job := createJob(t, db)
			svc := NewTaskRunService(db)

			taskRunID, err := svc.Start(context.Background(), ports.JobID(job.ID))
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			if err := svc.Fail(context.Background(), taskRunID, tt.taskErr); err != nil {
				t.Fatalf("fail: %v", err)
			}

			updated := loadTaskRun(t, db, uint(taskRunID))
			if updated.Status != "failed" {
				t.Errorf("status mismatch: want %q, got %q", "failed", updated.Status)
			}
			if updated.ErrorMessage != tt.taskErr.Error() {
				t.Errorf("error_message mismatch: want %q, got %q", tt.taskErr.Error(), updated.ErrorMessage)
			}
			if updated.EndedAt == nil || updated.EndedAt.IsZero() {
				t.Errorf("ended_at is nil or zero")
			}

			updatedJob := loadJob(t, db, job.ID)
			if updatedJob.Status != "error" {
				t.Errorf("job status mismatch: want %q, got %q", "error", updatedJob.Status)
			}
		})
	}
}

func TestTaskRunService_CancelRunning(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "cancels running task run and updates job"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			job := createJob(t, db)
			svc := NewTaskRunService(db)

			taskRunID, err := svc.Start(context.Background(), ports.JobID(job.ID))
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			if err := svc.Cancel(context.Background(), taskRunID); err != nil {
				t.Fatalf("cancel: %v", err)
			}

			updated := loadTaskRun(t, db, uint(taskRunID))
			if updated.Status != "cancelled" {
				t.Errorf("status mismatch: want %q, got %q", "cancelled", updated.Status)
			}
			if updated.EndedAt == nil || updated.EndedAt.IsZero() {
				t.Errorf("ended_at is nil or zero")
			}

			updatedJob := loadJob(t, db, job.ID)
			if updatedJob.Status != "idle" {
				t.Errorf("job status mismatch: want %q, got %q", "idle", updatedJob.Status)
			}
		})
	}
}

func TestTaskRunService_CancelIdempotent(t *testing.T) {
	tests := []struct {
		name          string
		initialStatus string
	}{
		{name: "completed is idempotent", initialStatus: "completed"},
		{name: "failed is idempotent", initialStatus: "failed"},
		{name: "cancelled is idempotent", initialStatus: "cancelled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			job := createJob(t, db)
			svc := NewTaskRunService(db)

			now := time.Now().Add(-time.Minute)
			end := time.Now().Add(-time.Second)
			taskRun := model.TaskRun{
				JobID:     job.ID,
				Status:    tt.initialStatus,
				StartedAt: now,
				EndedAt:   &end,
			}
			if err := db.Create(&taskRun).Error; err != nil {
				t.Fatalf("create task_run: %v", err)
			}

			if err := svc.Cancel(context.Background(), ports.TaskRunID(taskRun.ID)); err != nil {
				t.Fatalf("cancel: %v", err)
			}

			updated := loadTaskRun(t, db, taskRun.ID)
			if updated.Status != tt.initialStatus {
				t.Errorf("status changed: want %q, got %q", tt.initialStatus, updated.Status)
			}
			if updated.EndedAt == nil || updated.EndedAt.IsZero() {
				t.Errorf("ended_at is nil or zero")
			}
		})
	}
}
