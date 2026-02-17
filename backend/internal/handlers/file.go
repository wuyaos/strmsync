package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type FileHandler struct{}

func NewFileHandler() *FileHandler {
	return &FileHandler{}
}

// ListDirectories 列出指定路径下的目录
func (h *FileHandler) ListDirectories(c *gin.Context) {
	path := c.Query("path")
	mode := c.Query("mode")       // local/api
	apiType := c.Query("type")    // clouddrive2/openlist
	host := c.Query("host")
	port := c.Query("port")
	apiKey := c.Query("apiKey")

	if path == "" {
		path = "/"
	}

	// 根据监控方式选择不同的获取方法
	if mode == "api" {
		if apiType == "clouddrive2" {
			h.listCloudDrive2Directories(c, path, host, port, apiKey)
			return
		} else if apiType == "openlist" {
			h.listOpenListDirectories(c, path, host, port)
			return
		}
	}

	// 默认使用本地文件系统
	h.listLocalDirectories(c, path)
}

// listLocalDirectories 列出本地文件系统的目录
func (h *FileHandler) listLocalDirectories(c *gin.Context, path string) {
	// 读取目录内容
	entries, err := os.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取目录: " + err.Error()})
		return
	}

	// 筛选出目录
	var directories []string
	for _, entry := range entries {
		if entry.IsDir() {
			// 跳过隐藏目录
			if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
				continue
			}
			directories = append(directories, entry.Name())
		}
	}

	// 排序
	sort.Strings(directories)

	c.JSON(http.StatusOK, gin.H{
		"path":        path,
		"directories": directories,
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

// listOpenListDirectories 通过OpenList API列出目录
func (h *FileHandler) listOpenListDirectories(c *gin.Context, path, host, port string) {
	// 获取可选的apiKey参数
	apiKey := c.Query("apiKey")

	// 构建OpenList API URL
	baseURL := fmt.Sprintf("http://%s:%s", host, port)
	apiURL := fmt.Sprintf("%s/api/fs/list", baseURL)

	// 创建请求体
	reqBody := map[string]interface{}{
		"path": path,
	}
	reqJSON, _ := json.Marshal(reqBody)

	// 创建请求
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(reqJSON)))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建请求失败: " + err.Error()})
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// 添加认证头(如果提供了apiKey)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	// 发送请求 (设置10秒超时)
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenList API请求失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取响应失败: " + err.Error()})
		return
	}

	// 解析响应
	var apiResp struct {
		Code int `json:"code"`
		Data struct {
			Files []struct {
				Name  string `json:"name"`
				IsDir bool   `json:"is_dir"`
			} `json:"files"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解析响应失败: " + err.Error()})
		return
	}

	// 检查业务错误码
	if apiResp.Code != 0 && apiResp.Code != 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("OpenList API返回错误码: %d", apiResp.Code)})
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
