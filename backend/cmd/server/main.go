// STRMSync - 自动化STRM媒体文件管理系统
// 主程序入口 (重构后最小可用版本)
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/strmsync/strmsync/internal/domain/model"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"github.com/strmsync/strmsync/internal/pkg/requestid"
	dbpkg "github.com/strmsync/strmsync/internal/infra/db"
	"github.com/strmsync/strmsync/internal/infra/db/repository"
	httphandlers "github.com/strmsync/strmsync/internal/transport"
	"github.com/strmsync/strmsync/internal/scheduler"
	"github.com/strmsync/strmsync/internal/queue"
	"github.com/strmsync/strmsync/internal/worker"
	"go.uber.org/zap"
	"gorm.io/gorm"

	// 导入filesystem provider实现以触发注册
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/clouddrive2"
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/local"
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/openlist"
)

func main() {
	// 从环境变量加载配置
	cfg, err := dbpkg.LoadFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志系统
	if err := logger.InitLogger(cfg.Log.Level, cfg.Log.Path); err != nil {
		fmt.Fprintf(os.Stderr, "日志初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer logger.SyncLogger()

	logger.LogInfo("STRMSync 启动中...",
		zap.String("version", "2.0.0-alpha"),
		zap.Int("port", cfg.Server.Port))

	// 初始化数据库
	if err := dbpkg.Init(cfg.Database.Path); err != nil {
		logger.LogError("数据库初始化失败", zap.Error(err))
		os.Exit(1)
	}
	defer dbpkg.Close()

	logger.LogInfo("数据库初始化成功", zap.String("path", cfg.Database.Path))

	// 获取数据库连接
	db, err := dbpkg.GetDB()
	if err != nil {
		logger.LogError("获取数据库连接失败", zap.Error(err))
		os.Exit(1)
	}

	// 回填服务器 UID（历史数据迁移）
	backfillLogger := logger.With(zap.String("component", "backfill"))
	if stats, err := dbpkg.BackfillServerUIDs(context.Background(), db, backfillLogger); err != nil {
		logger.LogError("回填服务器 UID 失败", zap.Error(err))
		// 参数校验失败时中断启动
		os.Exit(1)
	} else if stats != nil {
		// 检查是否有冲突或失败
		totalFailures := stats.DataServersGenFailed + stats.DataServersUpdateFailed + stats.DataServersConflict +
			stats.MediaServersGenFailed + stats.MediaServersUpdateFailed + stats.MediaServersConflict
		if totalFailures > 0 || stats.DataServersQueryFailed > 0 || stats.MediaServersQueryFailed > 0 {
			logger.LogWarn("回填服务器 UID 部分失败",
				zap.Int("total_failures", totalFailures),
				zap.Int("data_servers_conflict", stats.DataServersConflict),
				zap.Int("media_servers_conflict", stats.MediaServersConflict))
		}
	}

	// 配置日志写入数据库（如果启用）
	if cfg.Log.ToDB {
		logger.SetLogToDBEnabled(true, 1024)
		logger.LogInfo("日志数据库写入已启用")
	}

	// 设置Gin模式
	if cfg.Log.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化 SyncQueue
	queue, err := syncqueue.NewSyncQueue(db)
	if err != nil {
		logger.LogError("SyncQueue 初始化失败", zap.Error(err))
		os.Exit(1)
	}

	// 初始化共享的 Repository（scheduler 和 worker 共享）
	jobRepo, err := repository.NewGormJobRepository(db)
	if err != nil {
		logger.LogError("JobRepository 初始化失败", zap.Error(err))
		os.Exit(1)
	}
	dataServerRepo, err := repository.NewGormDataServerRepository(db)
	if err != nil {
		logger.LogError("DataServerRepository 初始化失败", zap.Error(err))
		os.Exit(1)
	}
	taskRunRepo, err := worker.NewGormTaskRunRepository(db)
	if err != nil {
		logger.LogError("TaskRunRepository 初始化失败", zap.Error(err))
		os.Exit(1)
	}

	// 初始化 Scheduler
	cronScheduler, err := scheduler.NewScheduler(scheduler.SchedulerConfig{
		Queue:  queue,
		Jobs:   jobRepo, // 使用共享的 Repository
		Logger: logger.With(zap.String("component", "scheduler")),
	})
	if err != nil {
		logger.LogError("Scheduler 初始化失败", zap.Error(err))
		os.Exit(1)
	}

	// 初始化 Worker
	workerPool, err := worker.NewWorker(worker.WorkerConfig{
		Queue:       queue,
		Jobs:        jobRepo,        // 使用共享的 Repository
		DataServers: dataServerRepo, // 使用共享的 Repository
		TaskRuns:    taskRunRepo,
		Logger:      logger.With(zap.String("component", "worker")),
	})
	if err != nil {
		logger.LogError("Worker 初始化失败", zap.Error(err))
		os.Exit(1)
	}

	// 启动 Scheduler/Worker
	startCtx := context.Background()
	if err := cronScheduler.Start(startCtx); err != nil {
		logger.LogError("Scheduler 启动失败", zap.Error(err))
		os.Exit(1)
	}
	if err := workerPool.Start(startCtx); err != nil {
		logger.LogError("Worker 启动失败", zap.Error(err))
		os.Exit(1)
	}

	// 创建HTTP服务器
	router := setupRouter(db, cronScheduler, queue)
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
		logger.LogInfo("HTTP服务器启动中", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogError("HTTP服务器错误", zap.Error(err))
			os.Exit(1)
		}
	}()

	logger.LogInfo("STRMSync 启动成功")

	// 等待中断信号（优雅关闭）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.LogInfo("服务器关闭中...")

	// 关闭日志数据库写入worker
	logger.ShutdownLogDBWriter()

	// 优雅关闭：各组件独立超时，顺序为 Scheduler -> HTTP -> Worker
	// 先停调度器，不再产生新的定时任务
	schedCtx, schedCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer schedCancel()
	if err := cronScheduler.Stop(schedCtx); err != nil {
		logger.LogError("Scheduler 关闭失败", zap.Error(err))
	}

	// 停止HTTP，不再接受新请求（包括手动 RunJob）
	httpCtx, httpCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer httpCancel()
	if err := srv.Shutdown(httpCtx); err != nil {
		logger.LogError("服务器强制关闭", zap.Error(err))
	}

	// 最后停止Worker，让已入队的任务尽可能处理完毕
	workerCtx, workerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer workerCancel()
	if err := workerPool.Stop(workerCtx); err != nil {
		logger.LogError("Worker 关闭失败", zap.Error(err))
	}

	logger.LogInfo("服务器已退出")
}

// setupRouter 配置路由 (最小可用版本)
func setupRouter(db *gorm.DB, scheduler httphandlers.JobScheduler, queue httphandlers.TaskQueue) *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(requestIDMiddleware())
	router.Use(ginLogger(db))

	// 获取logger
	logger := logger.With(zap.String("module", "api"))

	// 创建处理器
	logHandler := httphandlers.NewLogHandler(db, logger)
	settingHandler := httphandlers.NewSettingHandler(db, logger)
	fileHandler := httphandlers.NewFileHandler(db, logger)
	dataServerHandler := httphandlers.NewDataServerHandler(db, logger)
	mediaServerHandler := httphandlers.NewMediaServerHandler(db, logger)
	jobHandler := httphandlers.NewJobHandler(db, logger, scheduler, queue)
	taskRunHandler := httphandlers.NewTaskRunHandler(db, logger)

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
			files.POST("/list", fileHandler.ListFiles)
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

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"
)

// requestIDMiddleware Request ID中间件
func requestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if id == "" {
			id = requestid.NewRequestID()
		}
		if id != "" {
			c.Set(requestIDKey, id)
			c.Writer.Header().Set(requestIDHeader, id)
		}
		c.Next()
	}
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

		requestID := strings.TrimSpace(c.GetString(requestIDKey))
		userAction := strings.TrimSpace(c.GetHeader("X-User-Action"))

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		}
		if requestID != "" {
			fields = append(fields, zap.String("request_id", requestID))
		}
		if userAction != "" {
			fields = append(fields, zap.String("user_action", userAction))
		}

		// 记录到zap日志
		logger.LogInfo("HTTP请求", fields...)

		// 同时写入数据库日志
		module := "api"
		message := fmt.Sprintf("%s %s - %d (%s)", c.Request.Method, path, status, latency)
		level := "info"
		if status >= 500 {
			level = "error"
		} else if status >= 400 {
			level = "warn"
		}

		var reqID *string
		if requestID != "" {
			reqID = &requestID
		}
		var action *string
		if userAction != "" {
			action = &userAction
		}

		logger.WriteLogToDB(db, &model.LogEntry{
			Level:      level,
			Module:     &module,
			Message:    message,
			RequestID:  reqID,
			UserAction: action,
		})
	}
}

// healthCheckHandler 健康检查处理器
func healthCheckHandler(c *gin.Context) {
	// 检查数据库连接
	db, err := dbpkg.GetDB()
	dbStatus := "ok"
	if err != nil {
		dbStatus = "error"
		logger.LogError("健康检查: 数据库错误", zap.Error(err))
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
