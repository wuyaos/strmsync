// Package service implements file list service.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/strmsync/strmsync/core"
	"github.com/strmsync/strmsync/filesystem"
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

// DataServerOptions DataServer的Options字段结构
type DataServerOptions struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	MountPath      string `json:"mount_path"`
	STRMMode       string `json:"strm_mode"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	BaseURL        string `json:"base_url"`
}

type fileService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewFileService 创建FileService实例
func NewFileService(db *gorm.DB, logger *zap.Logger) FileService {
	return &fileService{
		db:     db,
		logger: logger,
	}
}

func (s *fileService) List(ctx context.Context, req FileListRequest) ([]filesystem.RemoteFile, error) {
	if req.ServerID == 0 {
		return nil, fmt.Errorf("server_id is required")
	}

	// 加载DataServer
	var server core.DataServer
	if err := s.db.WithContext(ctx).First(&server, req.ServerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: id=%d", ErrDataServerNotFound, req.ServerID)
		}
		return nil, fmt.Errorf("load data server: %w", err)
	}

	if !server.Enabled {
		return nil, fmt.Errorf("%w: id=%d", ErrDataServerDisabled, req.ServerID)
	}

	// 解析Options
	var options DataServerOptions
	if strings.TrimSpace(server.Options) != "" {
		if err := json.Unmarshal([]byte(server.Options), &options); err != nil {
			return nil, fmt.Errorf("parse data server options: %w", err)
		}
	}

	// 构建filesystem.Config
	cfg := filesystem.Config{
		Type:      filesystem.Type(strings.TrimSpace(server.Type)),
		Username:  strings.TrimSpace(options.Username),
		Password:  strings.TrimSpace(options.Password),
		STRMMode:  filesystem.STRMMode(strings.TrimSpace(options.STRMMode)),
		MountPath: strings.TrimSpace(options.MountPath),
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
			return nil, fmt.Errorf("mount_path is required for local server")
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
		return nil, fmt.Errorf("create filesystem client: %w", err)
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

	// 调用List，传递深度参数
	files, err := client.List(ctx, listPath, effectiveRecursive, effectiveMaxDepth)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}

	return files, nil
}
