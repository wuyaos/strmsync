package adapters

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/strmsync/strmsync/internal/utils"
)

// LocalAdapter 本地文件系统适配器
// Author: STRMSync Team
type LocalAdapter struct {
	config   *AdapterConfig
	basePath string
}

// NewLocalAdapter 创建本地文件系统适配器
// Author: STRMSync Team
func NewLocalAdapter(config *AdapterConfig) (*LocalAdapter, error) {
	if config.BasePath == "" {
		return nil, fmt.Errorf("base_path is required for LocalAdapter")
	}

	// 检查路径是否存在
	if _, err := os.Stat(config.BasePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("base path does not exist: %s", config.BasePath)
	}

	adapter := &LocalAdapter{
		config:   config,
		basePath: utils.NormalizePath(config.BasePath),
	}

	return adapter, nil
}

// GetType 获取适配器类型
func (a *LocalAdapter) GetType() string {
	return "local"
}

// IsAvailable 检查数据源是否可用
func (a *LocalAdapter) IsAvailable() error {
	// 检查路径是否存在且可读
	info, err := os.Stat(a.basePath)
	if err != nil {
		return fmt.Errorf("base path not accessible: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("base path is not a directory: %s", a.basePath)
	}

	// 尝试读取目录
	_, err = os.ReadDir(a.basePath)
	if err != nil {
		return fmt.Errorf("cannot read directory: %w", err)
	}

	return nil
}

// ListFiles 列出指定路径下的文件
func (a *LocalAdapter) ListFiles(path string, options *ScanOptions) ([]*FileInfo, error) {
	if options == nil {
		options = DefaultScanOptions()
	}

	// 构建完整路径
	fullPath := filepath.Join(a.basePath, path)
	fullPath = utils.NormalizePath(fullPath)

	var files []*FileInfo

	// 递归扫描
	if options.Recursive {
		err := a.walkDirectory(fullPath, "", options, 0, &files)
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory: %w", err)
		}
	} else {
		// 非递归，只扫描当前目录
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read directory: %w", err)
		}

		for _, entry := range entries {
			if options.ShouldExclude(entry.Name()) {
				continue
			}

			fileInfo, err := a.getFileInfoFromEntry(fullPath, entry, "")
			if err != nil {
				continue // 跳过错误文件
			}

			if a.shouldIncludeFile(fileInfo, options) {
				files = append(files, fileInfo)
			}
		}
	}

	return files, nil
}

// GetFileInfo 获取单个文件的详细信息
func (a *LocalAdapter) GetFileInfo(path string) (*FileInfo, error) {
	fullPath := filepath.Join(a.basePath, path)
	fullPath = utils.NormalizePath(fullPath)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	fileName := filepath.Base(fullPath)
	relPath, _ := utils.RelativePath(a.basePath, fullPath)

	fileInfo := &FileInfo{
		Path:         fullPath,
		Name:         fileName,
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		IsDir:        info.IsDir(),
		IsVideo:      utils.IsVideoFile(fileName),
		IsMetadata:   utils.IsMetadataFile(fileName),
		RelativePath: relPath,
	}

	return fileInfo, nil
}

// Close 关闭适配器
func (a *LocalAdapter) Close() error {
	// 本地适配器无需释放资源
	return nil
}

// walkDirectory 递归遍历目录
func (a *LocalAdapter) walkDirectory(dirPath string, relativePath string, options *ScanOptions, depth int, files *[]*FileInfo) error {
	// 检查深度限制
	if options.MaxDepth > 0 && depth >= options.MaxDepth {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	for _, entry := range entries {
		// 检查排除规则
		if options.ShouldExclude(entry.Name()) {
			continue
		}

		entryRelPath := filepath.Join(relativePath, entry.Name())
		fileInfo, err := a.getFileInfoFromEntry(dirPath, entry, entryRelPath)
		if err != nil {
			continue // 跳过错误文件
		}

		if fileInfo.IsDir {
			// 递归扫描子目录
			subPath := filepath.Join(dirPath, entry.Name())
			if err := a.walkDirectory(subPath, entryRelPath, options, depth+1, files); err != nil {
				// 记录错误但继续扫描
				continue
			}
		} else {
			// 检查是否应该包含此文件
			if a.shouldIncludeFile(fileInfo, options) {
				*files = append(*files, fileInfo)
			}
		}
	}

	return nil
}

// getFileInfoFromEntry 从目录项创建 FileInfo
func (a *LocalAdapter) getFileInfoFromEntry(dirPath string, entry fs.DirEntry, relativePath string) (*FileInfo, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	fullPath := filepath.Join(dirPath, entry.Name())

	fileInfo := &FileInfo{
		Path:         utils.NormalizePath(fullPath),
		Name:         entry.Name(),
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		IsDir:        entry.IsDir(),
		IsVideo:      utils.IsVideoFile(entry.Name()),
		IsMetadata:   utils.IsMetadataFile(entry.Name()),
		RelativePath: utils.NormalizePath(relativePath),
	}

	return fileInfo, nil
}

// shouldIncludeFile 判断是否应该包含此文件
func (a *LocalAdapter) shouldIncludeFile(fileInfo *FileInfo, options *ScanOptions) bool {
	// 跳过目录
	if fileInfo.IsDir {
		return false
	}

	// 检查是否包含视频文件
	if fileInfo.IsVideo && options.IncludeVideo {
		return true
	}

	// 检查是否包含元数据文件
	if fileInfo.IsMetadata && options.IncludeMeta {
		return true
	}

	return false
}

// ScanDirectory 扫描目录并返回结果统计
// Author: STRMSync Team
func (a *LocalAdapter) ScanDirectory(path string, options *ScanOptions) (*ScanResult, error) {
	files, err := a.ListFiles(path, options)
	if err != nil {
		return nil, err
	}

	result := &ScanResult{
		TotalFiles: len(files),
		VideoFiles: 0,
		Metadata:   0,
		TotalSize:  0,
		Files:      files,
	}

	for _, file := range files {
		result.TotalSize += file.Size
		if file.IsVideo {
			result.VideoFiles++
		}
		if file.IsMetadata {
			result.Metadata++
		}
	}

	return result, nil
}

// GetBasePath 获取基础路径
func (a *LocalAdapter) GetBasePath() string {
	return a.basePath
}
