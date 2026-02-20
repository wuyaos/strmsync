package worker

import (
	"context"
	"errors"
	"github.com/strmsync/strmsync/internal/domain/model"
	"testing"
	"time"

	"github.com/strmsync/strmsync/internal/engine"
	"github.com/strmsync/strmsync/internal/queue"
)

// =============================================================
// buildEngineOptions 测试
// =============================================================

func TestBuildEngineOptions_Basic(t *testing.T) {
	job := model.Job{
		ID:         1,
		TargetPath: "/output/strm",
	}

	extra, err := parseJobOptions(job.Options)
	if err != nil {
		t.Fatalf("parse job options: %v", err)
	}
	opts, err := buildEngineOptions(job, extra)
	if err != nil {
		t.Fatalf("build engine options: %v", err)
	}
	if opts.OutputRoot != "/output/strm" {
		t.Errorf("OutputRoot: expected /output/strm, got %s", opts.OutputRoot)
	}
}

func TestBuildEngineOptions_EmptyTargetPath(t *testing.T) {
	job := model.Job{
		ID:         1,
		TargetPath: "",
	}

	extra, err := parseJobOptions(job.Options)
	if err != nil {
		t.Fatalf("parse job options: %v", err)
	}
	_, err = buildEngineOptions(job, extra)
	if err == nil {
		t.Fatal("expected error for empty target_path")
	}
}

func TestBuildEngineOptions_WithOptions(t *testing.T) {
	job := model.Job{
		ID:         1,
		TargetPath: "/output",
		Options:    `{"max_concurrency":8,"media_exts":[".mkv",".mp4"],"min_file_size":10,"dry_run":true,"force_update":false,"mod_time_epsilon_seconds":5}`,
	}

	extra, err := parseJobOptions(job.Options)
	if err != nil {
		t.Fatalf("parse job options: %v", err)
	}
	opts, err := buildEngineOptions(job, extra)
	if err != nil {
		t.Fatalf("build engine options: %v", err)
	}
	if opts.MaxConcurrency != 8 {
		t.Errorf("MaxConcurrency: expected 8, got %d", opts.MaxConcurrency)
	}
	if len(opts.FileExtensions) != 2 {
		t.Errorf("FileExtensions: expected 2, got %d", len(opts.FileExtensions))
	}
	if !opts.DryRun {
		t.Error("DryRun should be true")
	}
	if opts.ForceUpdate {
		t.Error("ForceUpdate should be false")
	}
	if opts.ModTimeEpsilon != 5*time.Second {
		t.Errorf("ModTimeEpsilon: expected 5s, got %v", opts.ModTimeEpsilon)
	}
	if opts.MinFileSize != 10*1024*1024 {
		t.Errorf("MinFileSize: expected 10MB, got %d", opts.MinFileSize)
	}
}

func TestBuildEngineOptions_InvalidJSON(t *testing.T) {
	job := model.Job{
		ID:         1,
		TargetPath: "/output",
		Options:    `{invalid json`,
	}

	_, err := parseJobOptions(job.Options)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// =============================================================
// progressFromStats 测试
// =============================================================

func TestProgressFromStats_Normal(t *testing.T) {
	stats := syncengine.SyncStats{
		TotalFiles:     100,
		ProcessedFiles: 75,
		FailedFiles:    5,
	}

	meta := metadataStats{
		Total:     10,
		Processed: 8,
		Failed:    2,
	}
	p := progressFromStats(stats, meta)
	if p.TotalFiles != 100 {
		t.Errorf("TotalFiles: expected 100, got %d", p.TotalFiles)
	}
	if p.ProcessedFiles != 75 {
		t.Errorf("ProcessedFiles: expected 75, got %d", p.ProcessedFiles)
	}
	if p.FailedFiles != 5 {
		t.Errorf("FailedFiles: expected 5, got %d", p.FailedFiles)
	}
	if p.Progress != 75 {
		t.Errorf("Progress: expected 75, got %d", p.Progress)
	}
	if p.MetaTotalFiles != 10 {
		t.Errorf("MetaTotalFiles: expected 10, got %d", p.MetaTotalFiles)
	}
	if p.MetaProcessedFiles != 8 {
		t.Errorf("MetaProcessedFiles: expected 8, got %d", p.MetaProcessedFiles)
	}
	if p.MetaFailedFiles != 2 {
		t.Errorf("MetaFailedFiles: expected 2, got %d", p.MetaFailedFiles)
	}
}

func TestProgressFromStats_ZeroTotal(t *testing.T) {
	stats := syncengine.SyncStats{
		TotalFiles:     0,
		ProcessedFiles: 0,
		EndTime:        time.Now(),
	}

	p := progressFromStats(stats, metadataStats{})
	if p.Progress != 100 {
		t.Errorf("Progress: expected 100 for completed with no files, got %d", p.Progress)
	}
}

func TestProgressFromStats_OverflowProtection(t *testing.T) {
	stats := syncengine.SyncStats{
		TotalFiles:     100,
		ProcessedFiles: 150, // 超过总数
	}

	p := progressFromStats(stats, metadataStats{})
	if p.Progress > 100 {
		t.Errorf("Progress should not exceed 100, got %d", p.Progress)
	}
}

func TestProgressFromStats_NegativeValues(t *testing.T) {
	stats := syncengine.SyncStats{
		TotalFiles:     -1,
		ProcessedFiles: -5,
	}

	p := progressFromStats(stats, metadataStats{})
	if p.TotalFiles < 0 {
		t.Errorf("TotalFiles should not be negative, got %d", p.TotalFiles)
	}
	if p.ProcessedFiles < 0 {
		t.Errorf("ProcessedFiles should not be negative, got %d", p.ProcessedFiles)
	}
}

// =============================================================
// wrapTaskError 测试
// =============================================================

func TestWrapTaskError_Nil(t *testing.T) {
	err := wrapTaskError(nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWrapTaskError_AlreadyTaskError(t *testing.T) {
	original := &syncqueue.TaskError{Kind: syncqueue.FailureRetryable, Err: errors.New("test")}
	err := wrapTaskError(original)

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailureRetryable {
		t.Errorf("expected retryable, got %s", taskErr.Kind)
	}
}

func TestWrapTaskError_ContextCanceled(t *testing.T) {
	err := wrapTaskError(context.Canceled)

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailureCancelled {
		t.Errorf("expected cancelled, got %s", taskErr.Kind)
	}
}

func TestWrapTaskError_ContextDeadlineExceeded(t *testing.T) {
	err := wrapTaskError(context.DeadlineExceeded)

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailureCancelled {
		t.Errorf("expected cancelled, got %s", taskErr.Kind)
	}
}

func TestWrapTaskError_InvalidInput(t *testing.T) {
	err := wrapTaskError(syncengine.ErrInvalidInput)

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailurePermanent {
		t.Errorf("expected permanent, got %s", taskErr.Kind)
	}
}

func TestWrapTaskError_NotSupported(t *testing.T) {
	err := wrapTaskError(syncengine.ErrNotSupported)

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailurePermanent {
		t.Errorf("expected permanent, got %s", taskErr.Kind)
	}
}

func TestWrapTaskError_UnknownError(t *testing.T) {
	err := wrapTaskError(errors.New("some unknown error"))

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailureRetryable {
		t.Errorf("expected retryable for unknown, got %s", taskErr.Kind)
	}
}

// =============================================================
// permanentTaskError 测试
// =============================================================

func TestPermanentTaskError_Nil(t *testing.T) {
	err := permanentTaskError(nil)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestPermanentTaskError_NonNil(t *testing.T) {
	err := permanentTaskError(errors.New("bad config"))

	var taskErr *syncqueue.TaskError
	if !errors.As(err, &taskErr) {
		t.Fatal("expected TaskError")
	}
	if taskErr.Kind != syncqueue.FailurePermanent {
		t.Errorf("expected permanent, got %s", taskErr.Kind)
	}
}

// =============================================================
// clampInt64 测试
// =============================================================

func TestClampInt64(t *testing.T) {
	tests := []struct {
		input    int64
		expected int
	}{
		{0, 0},
		{100, 100},
		{-1, 0},
		{-100, 0},
	}

	for _, tt := range tests {
		got := clampInt64(tt.input)
		if got != tt.expected {
			t.Errorf("clampInt64(%d): expected %d, got %d", tt.input, tt.expected, got)
		}
	}
}

// =============================================================
// buildFilesystemConfig 测试
// =============================================================

func TestBuildFilesystemConfig_CloudDrive2(t *testing.T) {
	server := model.DataServer{
		ID:     1,
		Name:   "test-cd2",
		Type:   "clouddrive2",
		Host:   "127.0.0.1",
		Port:   19798,
		APIKey: "test-api-key",
	}

	cfg, err := buildFilesystemConfig(server)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	if cfg.Type.String() != "clouddrive2" {
		t.Errorf("Type: expected clouddrive2, got %s", cfg.Type)
	}
	if cfg.BaseURL != "http://127.0.0.1:19798" {
		t.Errorf("BaseURL: expected http://127.0.0.1:19798, got %s", cfg.BaseURL)
	}
	if cfg.Password != "test-api-key" {
		t.Errorf("Password: expected test-api-key, got %s", cfg.Password)
	}
}

func TestBuildFilesystemConfig_WithOptions(t *testing.T) {
	server := model.DataServer{
		ID:      1,
		Name:    "test-with-opts",
		Type:    "openlist",
		Host:    "localhost",
		Port:    5244,
		Options: `{"base_url":"http://custom:8080","username":"admin","password":"secret","strm_mode":"mount","mount_path":"/mnt/data","timeout_seconds":30}`,
	}

	cfg, err := buildFilesystemConfig(server)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}

	if cfg.BaseURL != "http://custom:8080" {
		t.Errorf("BaseURL: expected http://custom:8080, got %s", cfg.BaseURL)
	}
	if cfg.Username != "admin" {
		t.Errorf("Username: expected admin, got %s", cfg.Username)
	}
	if cfg.Password != "secret" {
		t.Errorf("Password: expected secret, got %s", cfg.Password)
	}
	if cfg.MountPath != "/mnt/data" {
		t.Errorf("MountPath: expected /mnt/data, got %s", cfg.MountPath)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout: expected 30s, got %v", cfg.Timeout)
	}
}

func TestBuildFilesystemConfig_InvalidType(t *testing.T) {
	server := model.DataServer{
		ID:   1,
		Name: "bad-type",
		Type: "unknown_type",
	}

	_, err := buildFilesystemConfig(server)
	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestBuildFilesystemConfig_EmptyType(t *testing.T) {
	server := model.DataServer{
		ID:   1,
		Name: "empty-type",
		Type: "",
	}

	_, err := buildFilesystemConfig(server)
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestBuildFilesystemConfig_InvalidSTRMMode(t *testing.T) {
	server := model.DataServer{
		ID:      1,
		Name:    "bad-strm-mode",
		Type:    "clouddrive2",
		Host:    "localhost",
		Port:    19798,
		Options: `{"strm_mode":"invalid_mode"}`,
	}

	_, err := buildFilesystemConfig(server)
	if err == nil {
		t.Fatal("expected error for invalid strm_mode")
	}
}

// =============================================================
// NewWorker 构造测试
// =============================================================

func TestNewWorker_NilQueue(t *testing.T) {
	_, err := NewWorker(WorkerConfig{
		Queue:       nil,
		Jobs:        &mockJobRepo{},
		DataServers: &mockDataServerRepo{},
		TaskRuns:    &mockTaskRunRepo{},
	})
	if err == nil {
		t.Fatal("expected error for nil queue")
	}
}

func TestNewWorker_NilJobs(t *testing.T) {
	_, err := NewWorker(WorkerConfig{
		Queue:       &mockTaskQueue{},
		Jobs:        nil,
		DataServers: &mockDataServerRepo{},
		TaskRuns:    &mockTaskRunRepo{},
	})
	if err == nil {
		t.Fatal("expected error for nil jobs")
	}
}

// =============================================================
// Mock 实现（仅用于构造测试）
// =============================================================

type mockJobRepo struct{}

func (r *mockJobRepo) GetByID(ctx context.Context, id uint) (model.Job, error) {
	return model.Job{}, errors.New("not implemented")
}

func (r *mockJobRepo) UpdateStatus(ctx context.Context, id uint, status string) error {
	return nil
}

func (r *mockJobRepo) UpdateLastRunAt(ctx context.Context, id uint, lastRunAt time.Time) error {
	return nil
}

func (r *mockJobRepo) ListEnabledJobs(ctx context.Context) ([]model.Job, error) {
	return nil, errors.New("not implemented")
}

type mockDataServerRepo struct{}

func (r *mockDataServerRepo) GetByID(ctx context.Context, id uint) (model.DataServer, error) {
	return model.DataServer{}, errors.New("not implemented")
}

type mockTaskRunRepo struct{}

func (r *mockTaskRunRepo) UpdateProgress(ctx context.Context, taskID uint, progress TaskRunProgress) error {
	return nil
}

type mockTaskQueue struct{}

func (q *mockTaskQueue) ClaimNext(ctx context.Context, workerID string) (*model.TaskRun, error) {
	return nil, nil
}

func (q *mockTaskQueue) Complete(ctx context.Context, taskID uint) error {
	return nil
}

func (q *mockTaskQueue) Fail(ctx context.Context, taskID uint, err error) error {
	return nil
}
