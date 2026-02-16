package services

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"

	"github.com/strmsync/strmsync/internal/adapters"
	"github.com/strmsync/strmsync/internal/database"
	"github.com/strmsync/strmsync/internal/utils"
)

// ScannerConfig 扫描器配置
// Author: STRMSync Team
type ScannerConfig struct {
	Concurrency int   // 并发数
	BatchSize   int   // 批量写入大小
	SampleSize  int64 // 哈希采样大小
}

// ScanProgress 扫描进度
// Author: STRMSync Team
type ScanProgress struct {
	TotalFiles     int64     // 总文件数
	ProcessedFiles int64     // 已处理文件数
	VideoFiles     int64     // 视频文件数
	MetadataFiles  int64     // 元数据文件数
	TotalSize      int64     // 总大小
	StartTime      time.Time // 开始时间
	Status         string    // 状态：running, completed, failed
	Error          string    // 错误信息
}

// Scanner 文件扫描服务
// Author: STRMSync Team
type Scanner struct {
	config      *ScannerConfig
	pool        *ants.Pool
	fileRepo    *database.FileRepository
	taskRepo    *database.TaskRepository
	mu          sync.Mutex
	progress    *ScanProgress
	fileBuffer  []*database.File
	cancelFuncs map[uint]context.CancelFunc // 任务取消函数
}

// NewScanner 创建扫描器
// Author: STRMSync Team
func NewScanner(config *ScannerConfig) (*Scanner, error) {
	if config.Concurrency <= 0 {
		config.Concurrency = 50
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 1000
	}
	if config.SampleSize <= 0 {
		config.SampleSize = 1024 * 1024 // 1MB
	}

	pool, err := ants.NewPool(config.Concurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to create goroutine pool: %w", err)
	}

	scanner := &Scanner{
		config:      config,
		pool:        pool,
		fileRepo:    database.NewFileRepository(),
		taskRepo:    database.NewTaskRepository(),
		fileBuffer:  make([]*database.File, 0, config.BatchSize),
		cancelFuncs: make(map[uint]context.CancelFunc),
	}

	return scanner, nil
}

// ScanSource 扫描数据源
// Author: STRMSync Team
func (s *Scanner) ScanSource(ctx context.Context, sourceID uint, adapter adapters.Adapter, options *adapters.ScanOptions) error {
	// 初始化进度
	s.mu.Lock()
	s.progress = &ScanProgress{
		StartTime: time.Now(),
		Status:    "running",
	}
	s.mu.Unlock()

	// 创建任务记录
	task := &database.Task{
		Type:     "scan",
		SourceID: sourceID,
		Status:   "running",
		Progress: 0,
	}
	if err := s.taskRepo.Create(task); err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	taskID := task.ID
	defer func() {
		s.finalizeScan(taskID)
	}()

	// 保存取消函数
	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancelFuncs[sourceID] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.cancelFuncs, sourceID)
		s.mu.Unlock()
	}()

	utils.Info("Starting scan",
		zap.Uint("source_id", sourceID),
		zap.Int("concurrency", s.config.Concurrency),
	)

	// 列出所有文件
	files, err := adapter.ListFiles("", options)
	if err != nil {
		s.updateProgress("failed", fmt.Sprintf("Failed to list files: %v", err))
		return fmt.Errorf("failed to list files: %w", err)
	}

	atomic.StoreInt64(&s.progress.TotalFiles, int64(len(files)))

	// 更新任务总数
	s.taskRepo.Update(&database.Task{
		ID:         taskID,
		TotalItems: len(files),
	})

	utils.Info("Files listed",
		zap.Int("total_files", len(files)),
		zap.Uint("source_id", sourceID),
	)

	// 并发处理文件
	var wg sync.WaitGroup
	for _, fileInfo := range files {
		// 检查取消
		select {
		case <-ctx.Done():
			s.updateProgress("cancelled", "Scan cancelled")
			return ctx.Err()
		default:
		}

		fileInfo := fileInfo // 避免闭包问题
		wg.Add(1)

		err := s.pool.Submit(func() {
			defer wg.Done()
			s.processFile(ctx, sourceID, fileInfo)
		})

		if err != nil {
			wg.Done()
			utils.Error("Failed to submit task", zap.Error(err))
		}
	}

	// 等待所有任务完成
	wg.Wait()

	// 刷新剩余缓冲
	if err := s.flushBuffer(); err != nil {
		utils.Error("Failed to flush buffer", zap.Error(err))
	}

	s.updateProgress("completed", "")

	utils.Info("Scan completed",
		zap.Uint("source_id", sourceID),
		zap.Int64("processed_files", atomic.LoadInt64(&s.progress.ProcessedFiles)),
		zap.Duration("duration", time.Since(s.progress.StartTime)),
	)

	return nil
}

// processFile 处理单个文件
func (s *Scanner) processFile(ctx context.Context, sourceID uint, fileInfo *adapters.FileInfo) {
	// 检查取消
	select {
	case <-ctx.Done():
		return
	default:
	}

	// 计算快速哈希
	fastHash, err := utils.CalculateFastHash(fileInfo.Path, s.config.SampleSize)
	if err != nil {
		utils.Warn("Failed to calculate hash",
			zap.String("file", fileInfo.Path),
			zap.Error(err),
		)
		fastHash = "" // 继续处理，但不设置哈希
	}

	// 创建文件记录
	file := &database.File{
		SourceID:     sourceID,
		RelativePath: fileInfo.RelativePath,
		FileName:     fileInfo.Name,
		FileSize:     fileInfo.Size,
		ModTime:      fileInfo.ModTime,
		FastHash:     fastHash,
		LastScanTime: time.Now(),
	}

	// 添加到缓冲区
	s.mu.Lock()
	s.fileBuffer = append(s.fileBuffer, file)
	shouldFlush := len(s.fileBuffer) >= s.config.BatchSize
	s.mu.Unlock()

	// 批量写入
	if shouldFlush {
		s.flushBuffer()
	}

	// 更新进度
	atomic.AddInt64(&s.progress.ProcessedFiles, 1)
	atomic.AddInt64(&s.progress.TotalSize, fileInfo.Size)

	if fileInfo.IsVideo {
		atomic.AddInt64(&s.progress.VideoFiles, 1)
	}
	if fileInfo.IsMetadata {
		atomic.AddInt64(&s.progress.MetadataFiles, 1)
	}
}

// flushBuffer 刷新文件缓冲区到数据库
func (s *Scanner) flushBuffer() error {
	s.mu.Lock()
	if len(s.fileBuffer) == 0 {
		s.mu.Unlock()
		return nil
	}

	// 复制缓冲区
	files := make([]database.File, len(s.fileBuffer))
	for i, f := range s.fileBuffer {
		files[i] = *f
	}
	s.fileBuffer = s.fileBuffer[:0] // 清空缓冲区
	s.mu.Unlock()

	// 批量写入数据库
	if err := s.fileRepo.BatchCreate(files); err != nil {
		utils.Error("Failed to batch create files", zap.Error(err))
		return err
	}

	utils.Debug("Flushed files to database", zap.Int("count", len(files)))
	return nil
}

// updateProgress 更新进度状态
func (s *Scanner) updateProgress(status, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.progress.Status = status
	s.progress.Error = errMsg
}

// finalizeScan 完成扫描，更新任务状态
func (s *Scanner) finalizeScan(taskID uint) {
	s.mu.Lock()
	progress := s.progress
	s.mu.Unlock()

	status := "completed"
	message := fmt.Sprintf("Scanned %d files", progress.ProcessedFiles)

	if progress.Status == "failed" || progress.Error != "" {
		status = "failed"
		message = progress.Error
	}

	s.taskRepo.UpdateStatus(taskID, status, message)
}

// GetProgress 获取当前扫描进度
func (s *Scanner) GetProgress() *ScanProgress {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.progress == nil {
		return nil
	}

	// 返回副本
	progress := *s.progress
	return &progress
}

// CancelScan 取消扫描
func (s *Scanner) CancelScan(sourceID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, ok := s.cancelFuncs[sourceID]; ok {
		cancel()
	}
}

// Close 关闭扫描器
func (s *Scanner) Close() {
	if s.pool != nil {
		s.pool.Release()
	}
}
