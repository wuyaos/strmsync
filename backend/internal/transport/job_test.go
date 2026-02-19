package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	syncqueue "github.com/strmsync/strmsync/internal/queue"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------
// 测试模式初始化
// ---------------------

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------
// Mock 实现
// ---------------------

// testScheduler 记录调度器调用，用于断言调度器是否被正确通知
type testScheduler struct {
	upsertCalls []model.Job
	removeCalls []uint
	upsertErr   error
	removeErr   error
}

func (s *testScheduler) UpsertJob(_ context.Context, job model.Job) error {
	s.upsertCalls = append(s.upsertCalls, job)
	return s.upsertErr
}

func (s *testScheduler) RemoveJob(_ context.Context, jobID uint) error {
	s.removeCalls = append(s.removeCalls, jobID)
	return s.removeErr
}

// testQueue 记录队列调用，可选地将取消操作同步写回 DB
type testQueue struct {
	db           *gorm.DB // 非 nil 时 Cancel 会更新 DB 中任务状态
	enqueueCalls []*model.TaskRun
	cancelCalls  []uint
	enqueueErr   error
	cancelErr    error
}

func (q *testQueue) Enqueue(_ context.Context, task *model.TaskRun) error {
	if q.enqueueErr != nil {
		return q.enqueueErr
	}
	q.enqueueCalls = append(q.enqueueCalls, task)
	// 模拟入队后分配 ID（防止 handler 使用 task.ID == 0 做判断）
	if task.ID == 0 {
		task.ID = uint(100 + len(q.enqueueCalls))
	}
	return nil
}

func (q *testQueue) Cancel(_ context.Context, taskID uint) error {
	if q.cancelErr != nil {
		return q.cancelErr
	}
	q.cancelCalls = append(q.cancelCalls, taskID)
	// 将 DB 中对应任务状态同步为 cancelled，以便 StopJob 重新查询后能返回正确结果
	if q.db != nil {
		_ = q.db.Model(&model.TaskRun{}).
			Where("id = ?", taskID).
			Update("status", string(syncqueue.TaskCancelled))
	}
	return nil
}

// ---------------------
// 测试辅助函数
// ---------------------

// newJobTestDB 创建测试用的内存数据库并完成迁移
func newJobTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}

	if err := db.AutoMigrate(
		&model.Job{},
		&model.TaskRun{},
		&model.DataServer{},
		&model.MediaServer{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

// setupJobRouter 创建挂载了 JobHandler 所有路由的 gin 引擎
func setupJobRouter(h *JobHandler) *gin.Engine {
	r := gin.New()
	r.POST("/api/jobs", h.CreateJob)
	r.GET("/api/jobs", h.ListJobs)
	r.GET("/api/jobs/:id", h.GetJob)
	r.PUT("/api/jobs/:id", h.UpdateJob)
	r.DELETE("/api/jobs/:id", h.DeleteJob)
	r.POST("/api/jobs/:id/run", h.RunJob)
	r.POST("/api/jobs/:id/stop", h.StopJob)
	return r
}

// doReq 构造并执行一次 HTTP 请求，body 为 nil 时发送空体
func doReq(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			panic(fmt.Sprintf("doReq: marshal body: %v", err))
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// insertJobRaw 使用原始 SQL 插入任务，绕过 GORM 对 bool 零值的跳过行为
// 当 Enabled=false 时必须使用此函数，否则 GORM 会写入数据库默认值 true
func insertJobRaw(t *testing.T, db *gorm.DB, name string, enabled bool) model.Job {
	t.Helper()

	if err := db.Exec(
		`INSERT INTO jobs (name, enabled, watch_mode, source_path, target_path, strm_path, options, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		name, enabled, string(JobWatchModeLocal), "/src", "/dst", "/strm", "{}", string(JobStatusIdle),
	).Error; err != nil {
		t.Fatalf("insertJobRaw: %v", err)
	}

	var job model.Job
	if err := db.Where("name = ?", name).Last(&job).Error; err != nil {
		t.Fatalf("insertJobRaw: reload: %v", err)
	}
	return job
}

// insertTaskRun 直接向 DB 插入一条 TaskRun 记录
func insertTaskRun(t *testing.T, db *gorm.DB, jobID uint, status, dedupKey string) model.TaskRun {
	t.Helper()

	task := model.TaskRun{
		JobID:       jobID,
		Status:      status,
		Priority:    int(syncqueue.TaskPriorityNormal),
		AvailableAt: time.Now(),
		DedupKey:    dedupKey,
	}
	if err := db.Create(&task).Error; err != nil {
		t.Fatalf("insertTaskRun: %v", err)
	}
	return task
}

// insertDataServer 直接向 DB 插入一条 DataServer 记录（绕开 SSRF 检查）
func insertDataServer(t *testing.T, db *gorm.DB, name, dsType, host string, port int) model.DataServer {
	t.Helper()

	ds := model.DataServer{
		Name:    name,
		Type:    dsType,
		Host:    host,
		Port:    port,
		Enabled: true,
		Options: "{}",
	}
	if err := db.Create(&ds).Error; err != nil {
		t.Fatalf("insertDataServer: %v", err)
	}
	return ds
}

// ---------------------
// CreateJob 测试
// ---------------------

func TestJobHandler_CreateJob_Success(t *testing.T) {
	db := newJobTestDB(t)
	scheduler := &testScheduler{}
	handler := NewJobHandler(db, zap.NewNop(), scheduler, nil)
	router := setupJobRouter(handler)

	// 创建关联的数据服务器（直接写 DB，绕过 SSRF 限制）
	ds := insertDataServer(t, db, "ds-create", "openlist", "127.0.0.1", 8080)

	dsID := ds.ID
	payload := map[string]interface{}{
		"name":           "job-create",
		"enabled":        true,
		"watch_mode":     "api",
		"source_path":    "/src",
		"target_path":    "/dst",
		"strm_path":      "/strm",
		"data_server_id": dsID,
		"options":        "{}",
	}

	resp := doReq(router, http.MethodPost, "/api/jobs", payload)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.Code, resp.Body)
	}
	if len(scheduler.upsertCalls) != 1 {
		t.Errorf("scheduler.UpsertJob call count: want 1, got %d", len(scheduler.upsertCalls))
	}

	// 验证 DB 中确实存在该记录
	var count int64
	db.Model(&model.Job{}).Where("name = ?", "job-create").Count(&count)
	if count != 1 {
		t.Errorf("job not found in DB after creation")
	}
}

func TestJobHandler_CreateJob_ValidationError(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	// 缺少 name、watch_mode、source_path 等必填字段
	payload := map[string]interface{}{"options": "{}"}

	resp := doReq(router, http.MethodPost, "/api/jobs", payload)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}

	var body ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "invalid_request" {
		t.Errorf("error code: want %q, got %q", "invalid_request", body.Code)
	}
	if len(body.FieldErrors) == 0 {
		t.Errorf("expected field_errors to be non-empty")
	}
}

func TestJobHandler_CreateJob_APIWatchModeRequiresDataServerID(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	payload := map[string]interface{}{
		"name":        "api-job",
		"watch_mode":  "api",
		"source_path": "/src",
		"target_path": "/dst",
		"strm_path":   "/strm",
		// data_server_id 缺失或为0
		"options": "{}",
	}

	resp := doReq(router, http.MethodPost, "/api/jobs", payload)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}

	var body ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// 验证 field_errors 中包含 data_server_id 相关错误
	found := false
	for _, fe := range body.FieldErrors {
		if fe.Field == "data_server_id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected field_errors to contain data_server_id, got: %+v", body.FieldErrors)
	}
}

func TestJobHandler_CreateJob_InvalidOptionsJSON(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	payload := map[string]interface{}{
		"name":        "bad-options-job",
		"watch_mode":  "local",
		"source_path": "/src",
		"target_path": "/dst",
		"strm_path":   "/strm",
		"options":     "{invalid-json}",
	}

	resp := doReq(router, http.MethodPost, "/api/jobs", payload)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}
}

func TestJobHandler_CreateJob_DuplicateName(t *testing.T) {
	db := newJobTestDB(t)
	handler := NewJobHandler(db, zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	insertJobRaw(t, db, "dup-job", true)

	payload := map[string]interface{}{
		"name":        "dup-job",
		"watch_mode":  "local",
		"source_path": "/src",
		"target_path": "/dst",
		"strm_path":   "/strm",
		"options":     "{}",
	}

	resp := doReq(router, http.MethodPost, "/api/jobs", payload)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, resp.Code, resp.Body)
	}
}

// ---------------------
// ListJobs 测试
// ---------------------

func TestJobHandler_ListJobs_Empty(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodGet, "/api/jobs", nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	total, _ := body["total"].(float64)
	if total != 0 {
		t.Errorf("total: want 0, got %v", total)
	}
}

func TestJobHandler_ListJobs_FilterByEnabled(t *testing.T) {
	db := newJobTestDB(t)
	handler := NewJobHandler(db, zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	insertJobRaw(t, db, "enabled-job", true)
	insertJobRaw(t, db, "disabled-job", false)

	resp := doReq(router, http.MethodGet, "/api/jobs?enabled=true", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	total, _ := body["total"].(float64)
	if total != 1 {
		t.Errorf("total with enabled=true filter: want 1, got %v", total)
	}
}

func TestJobHandler_ListJobs_InvalidEnabledParam(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodGet, "/api/jobs?enabled=maybe", nil)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}
}

func TestJobHandler_ListJobs_InvalidWatchModeParam(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodGet, "/api/jobs?watch_mode=invalid", nil)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}
}

// ---------------------
// GetJob 测试
// ---------------------

func TestJobHandler_GetJob_Success(t *testing.T) {
	db := newJobTestDB(t)
	handler := NewJobHandler(db, zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "get-job", true)

	resp := doReq(router, http.MethodGet, fmt.Sprintf("/api/jobs/%d", job.ID), nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	jobData, ok := body["job"].(map[string]interface{})
	if !ok {
		t.Fatalf("response missing 'job' key")
	}
	if name, _ := jobData["name"].(string); name != "get-job" {
		t.Errorf("job name: want %q, got %q", "get-job", name)
	}
}

func TestJobHandler_GetJob_NotFound(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodGet, "/api/jobs/9999", nil)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, resp.Code, resp.Body)
	}
}

func TestJobHandler_GetJob_InvalidID(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodGet, "/api/jobs/abc", nil)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}
}

// ---------------------
// UpdateJob 测试
// ---------------------

func TestJobHandler_UpdateJob_Success(t *testing.T) {
	db := newJobTestDB(t)
	scheduler := &testScheduler{}
	handler := NewJobHandler(db, zap.NewNop(), scheduler, nil)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "old-name", true)

	payload := map[string]interface{}{
		"name":        "new-name",
		"enabled":     false,
		"watch_mode":  "local",
		"source_path": "/src-new",
		"target_path": "/dst-new",
		"strm_path":   "/strm-new",
		"options":     "{}",
	}

	resp := doReq(router, http.MethodPut, fmt.Sprintf("/api/jobs/%d", job.ID), payload)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body)
	}
	if len(scheduler.upsertCalls) != 1 {
		t.Errorf("scheduler.UpsertJob call count: want 1, got %d", len(scheduler.upsertCalls))
	}

	// 验证 DB 中的字段已更新
	var updated model.Job
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatalf("reload updated job: %v", err)
	}
	if updated.Name != "new-name" {
		t.Errorf("name: want %q, got %q", "new-name", updated.Name)
	}
	if updated.Enabled {
		t.Errorf("enabled: want false, got true")
	}
}

func TestJobHandler_UpdateJob_NotFound(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	payload := map[string]interface{}{
		"name":        "any-name",
		"watch_mode":  "local",
		"source_path": "/src",
		"target_path": "/dst",
		"strm_path":   "/strm",
		"options":     "{}",
	}

	resp := doReq(router, http.MethodPut, "/api/jobs/9999", payload)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, resp.Code, resp.Body)
	}
}

func TestJobHandler_UpdateJob_DuplicateName(t *testing.T) {
	db := newJobTestDB(t)
	handler := NewJobHandler(db, zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	insertJobRaw(t, db, "job-a", true)
	jobB := insertJobRaw(t, db, "job-b", true)

	// 把 job-b 改名为 job-a → 冲突
	payload := map[string]interface{}{
		"name":        "job-a",
		"watch_mode":  "local",
		"source_path": "/src",
		"target_path": "/dst",
		"strm_path":   "/strm",
		"options":     "{}",
	}

	resp := doReq(router, http.MethodPut, fmt.Sprintf("/api/jobs/%d", jobB.ID), payload)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, resp.Code, resp.Body)
	}
}

// ---------------------
// DeleteJob 测试
// ---------------------

func TestJobHandler_DeleteJob_Success(t *testing.T) {
	db := newJobTestDB(t)
	scheduler := &testScheduler{}
	handler := NewJobHandler(db, zap.NewNop(), scheduler, nil)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "delete-ok", true)

	resp := doReq(router, http.MethodDelete, fmt.Sprintf("/api/jobs/%d", job.ID), nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body)
	}
	if len(scheduler.removeCalls) != 1 {
		t.Errorf("scheduler.RemoveJob call count: want 1, got %d", len(scheduler.removeCalls))
	}

	// 验证 DB 中记录已被删除
	var count int64
	db.Model(&model.Job{}).Where("id = ?", job.ID).Count(&count)
	if count != 0 {
		t.Errorf("job should be deleted from DB, but still exists")
	}
}

func TestJobHandler_DeleteJob_NotFound(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodDelete, "/api/jobs/9999", nil)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, resp.Code, resp.Body)
	}
}

func TestJobHandler_DeleteJob_RunningConflict(t *testing.T) {
	db := newJobTestDB(t)
	handler := NewJobHandler(db, zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "delete-running", true)
	insertTaskRun(t, db, job.ID, string(TaskRunStatusRunning), "dedup-del-running")

	resp := doReq(router, http.MethodDelete, fmt.Sprintf("/api/jobs/%d", job.ID), nil)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, resp.Code, resp.Body)
	}
}

// ---------------------
// RunJob 测试
// ---------------------

func TestJobHandler_RunJob_Success(t *testing.T) {
	db := newJobTestDB(t)
	queue := &testQueue{}
	handler := NewJobHandler(db, zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "run-ok", true)

	resp := doReq(router, http.MethodPost, fmt.Sprintf("/api/jobs/%d/run", job.ID), nil)

	if resp.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, resp.Code, resp.Body)
	}
	if len(queue.enqueueCalls) != 1 {
		t.Errorf("enqueue call count: want 1, got %d", len(queue.enqueueCalls))
	}

	// 验证 Job 状态已更新为 running
	var updated model.Job
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if updated.Status != string(JobStatusRunning) {
		t.Errorf("job status: want %q, got %q", JobStatusRunning, updated.Status)
	}
}

func TestJobHandler_RunJob_Disabled(t *testing.T) {
	db := newJobTestDB(t)
	queue := &testQueue{}
	handler := NewJobHandler(db, zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	// 必须使用 insertJobRaw，否则 GORM 会跳过 Enabled=false
	job := insertJobRaw(t, db, "run-disabled", false)

	resp := doReq(router, http.MethodPost, fmt.Sprintf("/api/jobs/%d/run", job.ID), nil)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, resp.Code, resp.Body)
	}
	if len(queue.enqueueCalls) != 0 {
		t.Errorf("enqueue should not be called for disabled job, got %d calls", len(queue.enqueueCalls))
	}
}

func TestJobHandler_RunJob_AlreadyRunning(t *testing.T) {
	db := newJobTestDB(t)
	queue := &testQueue{}
	handler := NewJobHandler(db, zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "run-conflict", true)
	// pending 状态的任务也视为"已在运行"
	insertTaskRun(t, db, job.ID, string(syncqueue.TaskPending), "dedup-run-conflict")

	resp := doReq(router, http.MethodPost, fmt.Sprintf("/api/jobs/%d/run", job.ID), nil)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, resp.Code, resp.Body)
	}
}

func TestJobHandler_RunJob_NotFound(t *testing.T) {
	queue := &testQueue{}
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodPost, "/api/jobs/9999/run", nil)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, resp.Code, resp.Body)
	}
}

func TestJobHandler_RunJob_QueueNotInitialized(t *testing.T) {
	// queue=nil 时应返回 500
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodPost, "/api/jobs/1/run", nil)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, resp.Code, resp.Body)
	}
}

// ---------------------
// StopJob 测试
// ---------------------

func TestJobHandler_StopJob_Success(t *testing.T) {
	db := newJobTestDB(t)
	// 将 db 传入 queue，使 Cancel 能把 DB 中的任务状态更新为 cancelled
	queue := &testQueue{db: db}
	handler := NewJobHandler(db, zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "stop-ok", true)
	insertTaskRun(t, db, job.ID, string(syncqueue.TaskPending), "dedup-stop-1")
	insertTaskRun(t, db, job.ID, string(syncqueue.TaskRunning), "dedup-stop-2")

	resp := doReq(router, http.MethodPost, fmt.Sprintf("/api/jobs/%d/stop", job.ID), nil)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, resp.Code, resp.Body)
	}
	if len(queue.cancelCalls) != 2 {
		t.Errorf("cancel call count: want 2, got %d", len(queue.cancelCalls))
	}

	// 验证 Job 状态已重置为 idle
	var updated model.Job
	if err := db.First(&updated, job.ID).Error; err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if updated.Status != string(JobStatusIdle) {
		t.Errorf("job status: want %q, got %q", JobStatusIdle, updated.Status)
	}
}

func TestJobHandler_StopJob_NoActiveTasks(t *testing.T) {
	db := newJobTestDB(t)
	queue := &testQueue{db: db}
	handler := NewJobHandler(db, zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	job := insertJobRaw(t, db, "stop-none", true)

	resp := doReq(router, http.MethodPost, fmt.Sprintf("/api/jobs/%d/stop", job.ID), nil)

	if resp.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, resp.Code, resp.Body)
	}
}

func TestJobHandler_StopJob_NotFound(t *testing.T) {
	queue := &testQueue{}
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, queue)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodPost, "/api/jobs/9999/stop", nil)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, resp.Code, resp.Body)
	}
}

func TestJobHandler_StopJob_QueueNotInitialized(t *testing.T) {
	handler := NewJobHandler(newJobTestDB(t), zap.NewNop(), nil, nil)
	router := setupJobRouter(handler)

	resp := doReq(router, http.MethodPost, "/api/jobs/1/stop", nil)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d: %s", http.StatusInternalServerError, resp.Code, resp.Body)
	}
}
