// STRMSync - 自动化STRM媒体文件管理系统
// 主程序入口 (重构后最小可用版本)
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
	"github.com/strmsync/strmsync/core"
	"github.com/strmsync/strmsync/handler"
	"github.com/strmsync/strmsync/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	// 从环境变量加载配置
	cfg, err := core.LoadFromEnv()
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
		zap.String("version", "2.0.0-alpha"),
		zap.Int("port", cfg.Server.Port))

	// 初始化数据库
	if err := core.Init(cfg.Database.Path); err != nil {
		utils.LogError("数据库初始化失败", zap.Error(err))
		os.Exit(1)
	}
	defer core.Close()

	utils.LogInfo("数据库初始化成功", zap.String("path", cfg.Database.Path))

	// 获取数据库连接
	db, err := core.GetDB()
	if err != nil {
		utils.LogError("获取数据库连接失败", zap.Error(err))
		os.Exit(1)
	}

	// 配置日志写入数据库（如果启用）
	if cfg.Log.ToDB {
		utils.SetLogToDBEnabled(true, 1024)
		utils.LogInfo("日志数据库写入已启用")
	}

	// 设置Gin模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建HTTP服务器
	router := setupRouter(db)
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

	// 关闭日志数据库写入worker
	utils.ShutdownLogDBWriter()

	// 优雅关闭（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		utils.LogError("服务器强制关闭", zap.Error(err))
	}

	utils.LogInfo("服务器已退出")
}

// setupRouter 配置路由 (最小可用版本)
func setupRouter(db *gorm.DB) *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(ginLogger(db))

	// 获取logger
	logger := utils.With(zap.String("module", "api"))

	// 创建处理器
	logHandler := handlers.NewLogHandler(db, logger)
	settingHandler := handlers.NewSettingHandler(db, logger)
	fileHandler := handlers.NewFileHandler()
	dataServerHandler := handlers.NewDataServerHandler(db, logger)
	mediaServerHandler := handlers.NewMediaServerHandler(db, logger)
	jobHandler := handlers.NewJobHandler(db, logger)
	taskRunHandler := handlers.NewTaskRunHandler(db, logger)

	// API路由组
	api := router.Group("/api")
	{
		// 健康检查
		api.GET("/health", healthCheckHandler)

		// 日志查询
		logs := api.Group("/logs")
		{
			logs.GET("", logHandler.ListLogs)
			logs.POST("/cleanup", logHandler.CleanupLogs)
		}

		// 系统设置
		settings := api.Group("/settings")
		{
			settings.GET("", settingHandler.GetSettings)
			settings.PUT("", settingHandler.UpdateSettings)
		}

		// 文件系统浏览
		files := api.Group("/files")
		{
			files.GET("/directories", fileHandler.ListDirectories)
		}

		// 数据服务器管理
		dataServers := api.Group("/servers/data")
		{
			dataServers.POST("", dataServerHandler.CreateDataServer)
			dataServers.GET("", dataServerHandler.ListDataServers)
			dataServers.GET("/:id", dataServerHandler.GetDataServer)
			dataServers.PUT("/:id", dataServerHandler.UpdateDataServer)
			dataServers.DELETE("/:id", dataServerHandler.DeleteDataServer)
			dataServers.POST("/:id/test", dataServerHandler.TestDataServerConnection)
		}

		// 媒体服务器管理
		mediaServers := api.Group("/servers/media")
		{
			mediaServers.POST("", mediaServerHandler.CreateMediaServer)
			mediaServers.GET("", mediaServerHandler.ListMediaServers)
			mediaServers.GET("/:id", mediaServerHandler.GetMediaServer)
			mediaServers.PUT("/:id", mediaServerHandler.UpdateMediaServer)
			mediaServers.DELETE("/:id", mediaServerHandler.DeleteMediaServer)
			mediaServers.POST("/:id/test", mediaServerHandler.TestMediaServerConnection)
		}

		// 任务管理
		jobs := api.Group("/jobs")
		{
			jobs.POST("", jobHandler.CreateJob)
			jobs.GET("", jobHandler.ListJobs)
			jobs.GET("/:id", jobHandler.GetJob)
			jobs.PUT("/:id", jobHandler.UpdateJob)
			jobs.DELETE("/:id", jobHandler.DeleteJob)
			jobs.POST("/:id/run", jobHandler.RunJob)
			jobs.POST("/:id/stop", jobHandler.StopJob)
		}

		// 执行记录查询
		runs := api.Group("/runs")
		{
			runs.GET("", taskRunHandler.ListTaskRuns)
			runs.GET("/:id", taskRunHandler.GetTaskRun)
		}
	}

	// API未找到处理
	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "API not found",
			"message": "This is a minimal version during refactoring",
		})
	})

	return router
}

// ginLogger Gin日志中间件
func ginLogger(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		// 记录到zap日志
		utils.LogInfo("HTTP请求",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)

		// 同时写入数据库日志
		module := "api"
		message := fmt.Sprintf("%s %s - %d (%s)", c.Request.Method, path, status, latency)
		level := "info"
		if status >= 500 {
			level = "error"
		} else if status >= 400 {
			level = "warn"
		}

		utils.WriteLogToDB(db, &core.LogEntry{
			Level:   level,
			Module:  &module,
			Message: message,
		})
	}
}

// healthCheckHandler 健康检查处理器
func healthCheckHandler(c *gin.Context) {
	// 检查数据库连接
	db, err := core.GetDB()
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
		"version":   "2.0.0-alpha",
		"note":      "Minimal version during refactoring",
	})
}
