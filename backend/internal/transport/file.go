package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/app/file"
	"github.com/strmsync/strmsync/internal/app/ports"
	"github.com/strmsync/strmsync/internal/domain/model"
	openlistsdk "github.com/strmsync/strmsync/internal/pkg/sdk/openlist"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type FileHandler struct {
	db      *gorm.DB
	logger  *zap.Logger
	fileSvc ports.FileService
}

func NewFileHandler(db *gorm.DB, logger *zap.Logger) *FileHandler {
	return &FileHandler{
		db:      db,
		logger:  logger,
		fileSvc: file.NewFileService(db, logger),
	}
}

// ListDirectories 列出指定路径下的目录
func (h *FileHandler) ListDirectories(c *gin.Context) {
	// 目录列表必须实时，避免浏览器/代理缓存
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	path := c.Query("path")
	mode := c.Query("mode")    // local/api
	apiType := c.Query("type") // clouddrive2/openlist
	host := c.Query("host")
	port := c.Query("port")
	apiKey := c.Query("apiKey")
	serverIDStr := c.Query("serverId")
	username := c.Query("username")
	password := c.Query("password")
	limitStr := c.Query("limit")
	offsetStr := c.Query("offset")
	limit := 0
	offset := 0
	if strings.TrimSpace(limitStr) != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if strings.TrimSpace(offsetStr) != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed > 0 {
			offset = parsed
		}
	}

	if path == "" {
		path = "/"
	}

	// 根据监控方式选择不同的获取方法
	if mode == "api" {
		if apiType == "clouddrive2" {
			h.listCloudDrive2Directories(c, path, host, port, apiKey)
			return
		} else if apiType == "openlist" {
			if strings.TrimSpace(serverIDStr) != "" {
				serverID, err := strconv.ParseUint(serverIDStr, 10, 64)
				if err != nil || serverID == 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList serverId 无效"})
					return
				}
				var server model.DataServer
				if err := h.db.First(&server, serverID).Error; err != nil {
					c.JSON(http.StatusNotFound, gin.H{"error": "OpenList 服务器不存在"})
					return
				}
				if !server.Enabled {
					c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList 服务器已禁用"})
					return
				}
				h.listOpenListDirectories(c, path, server)
				return
			}

			if strings.TrimSpace(host) == "" || strings.TrimSpace(port) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList 需要提供 host/port 或 serverId"})
				return
			}
			if strings.TrimSpace(username) == "" || strings.TrimSpace(password) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList 需要提供 username/password"})
				return
			}
			h.listOpenListDirectoriesWithAuth(c, path, host, port, username, password)
			return
		}
	}

	// 默认使用本地文件系统
	h.listLocalDirectories(c, path, limit, offset)
}

// listLocalDirectories 列出本地文件系统的目录
func (h *FileHandler) listLocalDirectories(c *gin.Context, path string, limit int, offset int) {
	dir, err := os.Open(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取目录: " + err.Error()})
		return
	}
	defer dir.Close()

	var directories []string
	batch := 256

	for {
		entries, readErr := dir.ReadDir(batch)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取目录: " + readErr.Error()})
			return
		}

		for _, entry := range entries {
			isDir := entry.Type().IsDir()
			if !isDir && entry.Type() == 0 {
				isDir = entry.IsDir()
			}
			if !isDir {
				continue
			}
			if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
				continue
			}
			directories = append(directories, entry.Name())
		}

		if errors.Is(readErr, io.EOF) || len(entries) == 0 {
			break
		}
	}

	// 始终对目录列表排序，确保分页与新建文件夹显示一致
	sort.Strings(directories)

	total := len(directories)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := total
	if limit > 0 && offset+limit < total {
		end = offset + limit
	}
	truncated := limit > 0 && end < total
	directories = directories[offset:end]

	c.JSON(http.StatusOK, gin.H{
		"path":        path,
		"directories": directories,
		"truncated":   truncated,
		"offset":      offset,
		"limit":       limit,
	})
}

// listCloudDrive2Directories 通过CloudDrive2 API列出目录
func (h *FileHandler) listCloudDrive2Directories(c *gin.Context, path, host, port, apiKey string) {
	// 构建CloudDrive2 API URL (对path进行URL编码)
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	apiURL := fmt.Sprintf("%s/api/fs/list?path=%s", baseURL, url.QueryEscape(path))

	// 创建请求
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建请求失败: " + err.Error()})
		return
	}

	// 添加认证头
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// 发送请求 (设置10秒超时)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CloudDrive2 API请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取响应失败: " + err.Error()})
		return
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("CloudDrive2 API返回HTTP %d: %s", resp.StatusCode, preview),
		})
		return
	}

	// 检查响应体是否为空
	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "CloudDrive2 API返回空响应"})
		return
	}

	// 解析响应
	var apiResp struct {
		Code int `json:"code"`
		Data struct {
			Files []struct {
				Name  string `json:"name"`
				IsDir bool   `json:"isDir"`
			} `json:"files"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析响应失败: " + err.Error()})
		return
	}

	// 检查业务错误码
	if apiResp.Code != 0 && apiResp.Code != 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("CloudDrive2 API返回错误码: %d", apiResp.Code)})
		return
	}

	// 筛选出目录
	var directories []string
	for _, file := range apiResp.Data.Files {
		if file.IsDir && !strings.HasPrefix(file.Name, ".") {
			directories = append(directories, file.Name)
		}
	}

	sort.Strings(directories)

	c.JSON(http.StatusOK, gin.H{
		"path":        path,
		"directories": directories,
	})
}

// listOpenListDirectories 通过 OpenList API 列出目录（目录选择用途，非同步扫描）
func (h *FileHandler) listOpenListDirectories(c *gin.Context, path string, server model.DataServer) {
	baseURL := fmt.Sprintf("http://%s:%d", server.Host, server.Port)
	client, err := openlistsdk.NewClient(openlistsdk.Config{
		BaseURL:  baseURL,
		Username: server.Options.Username,
		Password: server.Options.Password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList API客户端初始化失败: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	files, err := client.List(ctx, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList API请求失败: " + err.Error()})
		return
	}

	// 筛选出目录
	var directories []string
	for _, file := range files {
		if file.IsDir && !strings.HasPrefix(file.Name, ".") {
			directories = append(directories, file.Name)
		}
	}

	sort.Strings(directories)

	c.JSON(http.StatusOK, gin.H{
		"path":        path,
		"directories": directories,
	})
}

// listOpenListDirectoriesWithAuth 通过 OpenList API 列出目录（临时用户名密码，目录选择用途）
func (h *FileHandler) listOpenListDirectoriesWithAuth(
	c *gin.Context,
	path string,
	host string,
	port string,
	username string,
	password string,
) {
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	client, err := openlistsdk.NewClient(openlistsdk.Config{
		BaseURL:  baseURL,
		Username: username,
		Password: password,
		Timeout:  10 * time.Second,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList API客户端初始化失败: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	files, err := client.List(ctx, path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList API请求失败: " + err.Error()})
		return
	}

	var directories []string
	for _, file := range files {
		if file.IsDir && !strings.HasPrefix(file.Name, ".") {
			directories = append(directories, file.Name)
		}
	}

	sort.Strings(directories)

	c.JSON(http.StatusOK, gin.H{
		"path":        path,
		"directories": directories,
	})
}

// ListFiles 获取数据服务器文件（新架构API）
// POST /api/files/list
// 请求体: {"server_id": 1, "path": "/", "recursive": false, "max_depth": 5}
// max_depth: 递归最大深度（可选），默认5，上限50，0表示非递归
func (h *FileHandler) ListFiles(c *gin.Context) {
	const (
		defaultListMaxDepth = 5  // 默认最大递归深度
		maxListMaxDepth     = 50 // 最大允许的递归深度
	)

	var req struct {
		ServerID  uint   `json:"server_id" binding:"required"`
		Path      string `json:"path"`
		Recursive bool   `json:"recursive"`
		MaxDepth  *int   `json:"max_depth"` // 使用指针类型以区分"未传"和"传0"
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数: " + err.Error()})
		return
	}

	// 校验 max_depth 参数
	if req.MaxDepth != nil {
		if *req.MaxDepth < 0 || *req.MaxDepth > maxListMaxDepth {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("max_depth 必须在 0 到 %d 之间", maxListMaxDepth),
			})
			return
		}
	}

	// 递归模式下设置默认深度
	if req.Recursive && req.MaxDepth == nil {
		defaultDepth := defaultListMaxDepth
		req.MaxDepth = &defaultDepth
	}

	// 调用 service 层（Scan 语义）
	files, err := h.fileSvc.Scan(c.Request.Context(), ports.FileListRequest{
		ServerID:  req.ServerID,
		Path:      req.Path,
		Recursive: req.Recursive,
		MaxDepth:  req.MaxDepth,
	})

	if err != nil {
		h.logger.Error("列出文件失败",
			zap.Uint("server_id", req.ServerID),
			zap.String("path", req.Path),
			zap.Error(err))

		statusCode := http.StatusInternalServerError
		if errors.Is(err, file.ErrDataServerNotFound) {
			statusCode = http.StatusNotFound
		} else if errors.Is(err, file.ErrDataServerDisabled) {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{"error": err.Error()})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"server_id": req.ServerID,
		"path":      req.Path,
		"recursive": req.Recursive,
		"count":     len(files),
		"files":     files,
	})
}
