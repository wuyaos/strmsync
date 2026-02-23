// Package file 实现文件服务
package file

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/strmsync/strmsync/internal/app/ports"
	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/infra/filesystem"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrDataServerNotFound = errors.New("data server not found")
	ErrDataServerDisabled = errors.New("data server disabled")
)

const (
	defaultListMaxDepth = 5  // 默认最大递归深度
	maxListMaxDepth     = 50 // 最大允许的递归深度
)

type fileService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewFileService 创建FileService实例
func NewFileService(db *gorm.DB, logger *zap.Logger) ports.FileService {
	return &fileService{
		db:     db,
		logger: logger,
	}
}

func (s *fileService) List(ctx context.Context, req ports.FileListRequest) ([]ports.RemoteFile, error) {
	if req.ServerID == 0 {
		return nil, fmt.Errorf("server_id 不能为空")
	}

	// 加载DataServer
	var server model.DataServer
	if err := s.db.WithContext(ctx).First(&server, req.ServerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrDataServerNotFound, req.ServerID)
		}
		return nil, fmt.Errorf("加载数据服务器失败: %w", err)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("%w: id=%d", ErrDataServerDisabled, req.ServerID)
	}

	options := server.Options

	// 构建filesystem.Config
	cfg := filesystem.Config{
		Type:      filesystem.Type(strings.TrimSpace(server.Type)),
		Username:  strings.TrimSpace(options.Username),
		Password:  strings.TrimSpace(options.Password),
		STRMMode:  filesystem.STRMMode(strings.TrimSpace(options.STRMMode)),
		MountPath: strings.TrimSpace(options.MountPath),
	}
	if cfg.MountPath == "" {
		cfg.MountPath = strings.TrimSpace(options.AccessPath)
	}

	if cfg.STRMMode == "" {
		cfg.STRMMode = filesystem.STRMModeHTTP
	}
	if options.TimeoutSeconds > 0 {
		cfg.Timeout = time.Duration(options.TimeoutSeconds) * time.Second
	}

	// 根据类型设置BaseURL或MountPath
	switch cfg.Type {
	case filesystem.TypeLocal:
		if cfg.MountPath == "" {
			return nil, fmt.Errorf("本地服务器必须配置 mount_path")
		}
	default:
		baseURL := strings.TrimSpace(options.BaseURL)
		if baseURL == "" {
			baseURL = fmt.Sprintf("http://%s:%d", server.Host, server.Port)
		}
		cfg.BaseURL = baseURL
	}

	// CloudDrive2 使用 APIKey 作为 token
	if cfg.Type == filesystem.TypeCloudDrive2 {
		cfg.Password = strings.TrimSpace(server.APIKey)
	}

	// 创建filesystem客户端
	client, err := filesystem.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("创建文件系统客户端失败: %w", err)
	}

	listPath := strings.TrimSpace(req.Path)
	if listPath == "" {
		listPath = "/"
	}

	// 校验 MaxDepth
	if req.MaxDepth != nil {
		if *req.MaxDepth < 0 || *req.MaxDepth > maxListMaxDepth {
			return nil, fmt.Errorf("max_depth 必须在 0 到 %d 之间", maxListMaxDepth)
		}
	}

	// 计算有效的 MaxDepth
	effectiveMaxDepth := 0
	if req.Recursive {
		if req.MaxDepth != nil {
			effectiveMaxDepth = *req.MaxDepth
		} else {
			effectiveMaxDepth = defaultListMaxDepth
		}
	}

	// 如果递归模式但深度为0，则改为非递归
	effectiveRecursive := req.Recursive
	if effectiveRecursive && effectiveMaxDepth == 0 {
		effectiveRecursive = false
	}

	// 调用Scan，传递深度参数
	fileCh, errCh := client.Scan(ctx, listPath, effectiveRecursive, effectiveMaxDepth)

	result := make([]ports.RemoteFile, 0)
	for fileCh != nil || errCh != nil {
		select {
		case f, ok := <-fileCh:
			if !ok {
				fileCh = nil
				continue
			}
			result = append(result, ports.RemoteFile{
				Path:    f.Path,
				Name:    f.Name,
				Size:    f.Size,
				ModTime: f.ModTime,
				IsDir:   f.IsDir,
			})
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("扫描文件失败: %w", err)
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return result, nil
}
