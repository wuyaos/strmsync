package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/strmsync/strmsync/internal/config"
	"github.com/strmsync/strmsync/internal/database"
	"github.com/strmsync/strmsync/internal/utils"
)

// HealthResponse 健康检查响应
// Author: STRMSync Team
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Version   string            `json:"version"`
	Uptime    int64             `json:"uptime"`
	Checks    map[string]string `json:"checks"`
}

var (
	startTime = time.Now()
	version   = "1.0.0"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化日志系统
	if err := utils.InitLogger(cfg.Log.Level, cfg.Log.OutputPath); err != nil {
		fmt.Printf("Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer utils.Sync()

	utils.Info("STRMSync starting...",
		zap.String("version", version),
		zap.String("port", cfg.Server.Port),
		zap.String("log_level", cfg.Log.Level),
	)

	// 3. 初始化数据库
	if err := database.InitDB(cfg.Database.Path, cfg.Log.Level); err != nil {
		utils.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer database.CloseDB()

	// 4. 执行数据库迁移
	if err := database.AutoMigrate(); err != nil {
		utils.Fatal("Failed to migrate database", zap.Error(err))
	}

	utils.Info("Database initialized successfully",
		zap.String("path", cfg.Database.Path),
	)

	// 5. 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 6. 创建 Gin 路由器
	r := gin.New()

	// 自定义中间件：日志
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		utils.Info("HTTP Request",
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("client_ip", param.ClientIP),
		)
		return ""
	}))

	// 恢复中间件
	r.Use(gin.Recovery())

	// 7. 注册路由
	registerRoutes(r)

	utils.Info("Routes registered successfully")

	// 8. 启动服务器
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	utils.Info("STRMSync server starting", zap.String("address", addr))

	// 启动服务器（在 goroutine 中）
	go func() {
		if err := r.Run(addr); err != nil {
			utils.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// 9. 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.Info("Shutting down server...")

	// 清理资源
	database.CloseDB()
	utils.Sync()

	utils.Info("Server exited")
}

// registerRoutes 注册所有路由
// Author: STRMSync Team
func registerRoutes(r *gin.Engine) {
	// API 路由组
	api := r.Group("/api")
	{
		// 健康检查
		api.GET("/health", healthCheckHandler)
		api.GET("/health/detail", healthCheckDetailHandler)

		// 版本信息
		api.GET("/version", versionHandler)
	}
}

// healthCheckHandler 简单健康检查
// Author: STRMSync Team
func healthCheckHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "STRMSync is running",
	})
}

// healthCheckDetailHandler 详细健康检查
// Author: STRMSync Team
func healthCheckDetailHandler(c *gin.Context) {
	checks := make(map[string]string)

	// 检查数据库
	if err := database.HealthCheck(); err != nil {
		checks["database"] = fmt.Sprintf("unhealthy: %v", err)
	} else {
		checks["database"] = "healthy"
	}

	// 检查日志系统
	if utils.Logger != nil {
		checks["logger"] = "healthy"
	} else {
		checks["logger"] = "unhealthy"
	}

	// 计算运行时间
	uptime := time.Since(startTime).Seconds()

	// 判断整体状态
	status := "ok"
	for _, check := range checks {
		if check != "healthy" {
			status = "degraded"
			break
		}
	}

	response := HealthResponse{
		Status:    status,
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   version,
		Uptime:    int64(uptime),
		Checks:    checks,
	}

	if status == "ok" {
		c.JSON(200, response)
	} else {
		c.JSON(503, response)
	}
}

// versionHandler 版本信息
// Author: STRMSync Team
func versionHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"version":    version,
		"go_version": "1.19",
		"build_time": "2024-02-16",
	})
}
