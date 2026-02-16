// STRMSync - 自动化STRM媒体文件管理系统
// 主程序入口
//
// Author: STRMSync Team
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/config"
	"github.com/strmsync/strmsync/internal/database"
	"github.com/strmsync/strmsync/internal/utils"
	"go.uber.org/zap"
)

func main() {
	// 从环境变量加载配置
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	if err := utils.InitLogger(cfg.Log.Level, cfg.Log.Path); err != nil {
		fmt.Fprintf(os.Stderr, "日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer utils.SyncLogger()

	utils.LogInfo("STRMSync 启动中...",
		zap.String("version", "1.0.0"),
		zap.Int("port", cfg.Server.Port))

	// 初始化数据库
	if err := database.Init(cfg.Database.Path); err != nil {
		utils.LogError("数据库初始化失败", zap.Error(err))
		os.Exit(1)
	}
	defer database.Close()

	utils.LogInfo("数据库初始化成功", zap.String("path", cfg.Database.Path))

	// 设置Gin模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建HTTP服务器
	router := setupRouter()
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	srv := &http.Server{
		Addr:           addr,
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// 启动服务器（goroutine）
	go func() {
		utils.LogInfo("HTTP服务器启动中", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.LogError("HTTP服务器错误", zap.Error(err))
			os.Exit(1)
		}
	}()

	utils.LogInfo("STRMSync 启动成功")

	// 等待中断信号（优雅关闭）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	utils.LogInfo("服务器关闭中...")

	// 优雅关闭（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		utils.LogError("服务器强制关闭", zap.Error(err))
	}

	utils.LogInfo("服务器已退出")
}

// setupRouter 配置路由
func setupRouter() *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(ginLogger())

	// API路由组
	api := router.Group("/api")
	{
		// 健康检查
		api.GET("/health", healthCheckHandler)

		// TODO: 其他API端点将在后续阶段添加
		// api.GET("/dashboard/stats", ...)
		// api.GET("/sources", ...)
	}

	return router
}

// ginLogger Gin日志中间件（集成zap）
func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		utils.LogInfo("HTTP请求",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// healthCheckHandler 健康检查处理器
func healthCheckHandler(c *gin.Context) {
	// 检查数据库连接
	db, err := database.GetDB()
	dbStatus := "ok"
	if err != nil {
		dbStatus = "error"
		utils.LogError("健康检查: 数据库错误", zap.Error(err))
	} else {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.Ping() != nil {
			dbStatus = "error"
		}
	}

	// 整体状态
	status := "healthy"
	httpStatus := http.StatusOK
	if dbStatus != "ok" {
		status = "unhealthy"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, gin.H{
		"status":    status,
		"timestamp": time.Now().Unix(),
		"database":  dbStatus,
		"version":   "1.0.0",
	})
}
