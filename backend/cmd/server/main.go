// STRMSync - 自动化STRM媒体文件管理系统
// 主程序入口 (重构后最小可用版本)
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	dbpkg "github.com/strmsync/strmsync/internal/infra/db"
	"github.com/strmsync/strmsync/internal/infra/db/repository"
	"github.com/strmsync/strmsync/internal/pkg/logger"
	"github.com/strmsync/strmsync/internal/pkg/requestid"
	"github.com/strmsync/strmsync/internal/queue"
	"github.com/strmsync/strmsync/internal/scheduler"
	httphandlers "github.com/strmsync/strmsync/internal/transport"
	"github.com/strmsync/strmsync/internal/worker"
	"go.uber.org/zap"
	"gorm.io/gorm"

	// 导入filesystem provider实现以触发注册
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/clouddrive2"
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/local"
	_ "github.com/strmsync/strmsync/internal/infra/filesystem/openlist"
)

var appVersion = "unknown"
var frontendVersion = "unknown"

func main() {
	// 尝试加载 .env 文件（生产环境支持）
	loadDotEnv()

	version := loadVersionFromFile()
	if version != "" {
		appVersion = version
		frontendVersion = version
	} else if value := strings.TrimSpace(os.Getenv("FRONTEND_VERSION")); value != "" {
		appVersion = value
		frontendVersion = value
	}

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

	envSnapshot := buildEnvSnapshot()
	sysLogger := logger.With(zap.String("module", "system"))
	if len(envSnapshot) > 0 {
		message := formatEnvSnapshot(envSnapshot)
		sysLogger.Info(message, zap.Any("env", envSnapshot))
	} else {
		sysLogger.Info("未检测到环境变量覆盖")
	}
	logSystemInfo(cfg)

	logger.LogInfo("STRMSync 启动中...",
		zap.String("app", "STRMSync"),
		zap.String("version", appVersion),
		zap.String("frontend_version", frontendVersion),
		zap.String("host", cfg.Server.Host),
		zap.Int("port", cfg.Server.Port),
		zap.String("db_path", cfg.Database.Path),
		zap.String("log_level", cfg.Log.Level),
		zap.String("log_path", cfg.Log.Path),
		zap.Bool("log_to_db", cfg.Log.ToDB),
		zap.Bool("log_sql", cfg.Log.SQL),
		zap.Int("log_sql_slow_ms", cfg.Log.SQLSlowMs))

	// 初始化数据库
	if err := dbpkg.InitWithConfig(cfg.Database.Path, &cfg.Log); err != nil {
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
	router := setupRouter(db, cfg.Log.Path, cronScheduler, queue)
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

	logger.LogInfo("STRMSync 启动成功",
		zap.String("app", "STRMSync"),
		zap.String("version", appVersion),
		zap.String("frontend_version", frontendVersion),
		zap.String("addr", addr))

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

func loadVersionFromFile() string {
	candidates := []string{"VERSION", "../VERSION"}
	if execPath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), "VERSION"))
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		value := strings.TrimSpace(string(data))
		if value != "" {
			return value
		}
	}

	return ""
}

func buildEnvSnapshot() map[string]string {
	keys := []string{
		"PORT",
		"HOST",
		"DB_PATH",
		"LOG_LEVEL",
		"LOG_PATH",
		"LOG_TO_DB",
		"LOG_SQL",
		"LOG_SQL_SLOW_MS",
		"ENCRYPTION_KEY",
		"SCANNER_CONCURRENCY",
		"SCANNER_BATCH_SIZE",
		"NOTIFIER_ENABLED",
		"NOTIFIER_PROVIDER",
		"NOTIFIER_BASE_URL",
		"NOTIFIER_TOKEN",
		"NOTIFIER_TIMEOUT",
		"NOTIFIER_RETRY_MAX",
		"NOTIFIER_RETRY_BASE_MS",
		"NOTIFIER_DEBOUNCE",
		"NOTIFIER_SCOPE",
	}

	snapshot := make(map[string]string)
	for _, key := range keys {
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		snapshot[key] = maskEnvValue(key, value)
	}

	return snapshot
}

// formatEnvSnapshot 将环境变量快照格式化为可读消息（仅用于日志展示）
func formatEnvSnapshot(snapshot map[string]string) string {
	if len(snapshot) == 0 {
		return "未检测到环境变量覆盖"
	}

	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, snapshot[key]))
	}

	return "已加载环境变量：" + strings.Join(parts, ", ")
}

func maskEnvValue(key, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	isSensitive := strings.Contains(lowerKey, "key") ||
		strings.Contains(lowerKey, "token") ||
		strings.Contains(lowerKey, "secret") ||
		strings.Contains(lowerKey, "password")
	if !isSensitive {
		return trimmed
	}

	if len(trimmed) <= 4 {
		return "****"
	}
	return "****" + trimmed[len(trimmed)-4:]
}

// logSystemInfo 输出系统信息（用于日志页展示）
func logSystemInfo(cfg *dbpkg.Config) {
	if cfg == nil {
		return
	}

	workDir, err := os.Getwd()
	if err != nil {
		workDir = ""
	}
	logDir, logFile := logger.ResolveLogFilePath(cfg.Log.Path)

	sysLogger := logger.With(zap.String("module", "system"))
	message := fmt.Sprintf("系统信息：后端版本=%s，前端版本=%s，Go=%s，系统=%s/%s，PID=%d，工作目录=%s，日志=%s，数据库=%s",
		appVersion,
		frontendVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
		os.Getpid(),
		workDir,
		logFile,
		cfg.Database.Path)

	sysLogger.Info(message,
		zap.String("version", appVersion),
		zap.String("frontend_version", frontendVersion),
		zap.String("go_version", runtime.Version()),
		zap.String("os", runtime.GOOS),
		zap.String("arch", runtime.GOARCH),
		zap.Int("pid", os.Getpid()),
		zap.String("work_dir", workDir),
		zap.String("log_dir", logDir),
		zap.String("log_file", logFile),
		zap.String("db_path", cfg.Database.Path))
}

// setupRouter 配置路由 (最小可用版本)
func setupRouter(db *gorm.DB, logDir string, scheduler httphandlers.JobScheduler, queue httphandlers.TaskQueue) *gin.Engine {
	router := gin.New()

	// 中间件
	router.Use(gin.Recovery())
	router.Use(requestIDMiddleware())
	router.Use(ginLogger())

	// 获取logger
	logger := logger.With(zap.String("module", "api"))

	// 创建处理器
	logHandler := httphandlers.NewLogHandler(logDir, logger)
	settingHandler := httphandlers.NewSettingHandler(db, logger)
	fileHandler := httphandlers.NewFileHandler(db, logger)
	dataServerHandler := httphandlers.NewDataServerHandler(db, logger)
	mediaServerHandler := httphandlers.NewMediaServerHandler(db, logger)
	serverTypeHandler := httphandlers.NewServerTypeHandler()
	jobHandler := httphandlers.NewJobHandler(db, logger, scheduler, queue)
	taskRunHandler := httphandlers.NewTaskRunHandler(db, logger, queue)

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
			dataServers.POST("/test", dataServerHandler.TestDataServerTemp)
		}

		// 服务器类型定义
		serverTypes := api.Group("/servers/types")
		{
			serverTypes.GET("", serverTypeHandler.ListServerTypes)
			serverTypes.GET("/:type", serverTypeHandler.GetServerType)
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
			mediaServers.POST("/test", mediaServerHandler.TestMediaServerTemp)
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
			jobs.PUT("/:id/enable", jobHandler.EnableJob)
			jobs.PUT("/:id/disable", jobHandler.DisableJob)
		}

		// 执行记录查询
		runs := api.Group("/runs")
		{
			runs.GET("", taskRunHandler.ListTaskRuns)
			runs.GET("/:id", taskRunHandler.GetTaskRun)
			runs.POST("/:id/cancel", taskRunHandler.CancelRun)
			runs.GET("/stats", taskRunHandler.GetRunStats)
		}
	}

	// 前端静态文件服务（使用 StaticFS）
	setupStaticFiles(router)

	// SPA 路由回退（所有非 API 路由返回 index.html）
	router.NoRoute(func(c *gin.Context) {
		// API 路由返回 404
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "API not found",
				"message": "This is a minimal version during refactoring",
			})
			return
		}

		// 其他路由返回前端 index.html（SPA 路由）
		serveIndexHTML(c)
	})

	return router
}

// loadDotEnv 尝试加载 .env 文件（按优先级查找）
func loadDotEnv() {
	// 查找候选路径（优先级从高到低）
	candidates := []string{
		".env",                    // 当前工作目录
		"../.env",                 // 父目录（开发环境）
		"../../.env",              // 祖父目录
		"/app/.env",               // Docker 容器标准路径
		"/etc/strmsync/.env",      // 系统级配置路径
	}

	// 如果可执行文件路径可获取，添加可执行文件同级目录
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append([]string{filepath.Join(execDir, ".env")}, candidates...)
	}

	// 尝试加载第一个存在的 .env 文件
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err == nil {
				// 成功加载，静默返回（避免日志污染）
				return
			}
		}
	}

	// 未找到 .env 文件，静默继续（使用系统环境变量）
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
func ginLogger() gin.HandlerFunc {
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
			zap.String("component", "api"),
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

		// 记录到zap日志（数据库写入由日志核心统一处理）
		message := buildAPILogMessage(c.Request.Method, path, status, latency)
		switch {
		case status >= http.StatusInternalServerError:
			logger.LogError(message, fields...)
		case status >= http.StatusBadRequest:
			logger.LogWarn(message, fields...)
		default:
			logger.Debug(message, fields...)
		}
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
		"status":           status,
		"timestamp":        time.Now().Unix(),
		"database":         dbStatus,
		"version":          appVersion,
		"frontend_version": frontendVersion,
		"note":             "Minimal version during refactoring",
	})
}

// buildAPILogMessage 将API请求转换为更友好的中文提示
func buildAPILogMessage(method string, path string, status int, latency time.Duration) string {
	action := describeAPIAction(method, path)
	if action != "" {
		return fmt.Sprintf("%s（%d）", action, status)
	}
	return fmt.Sprintf("%s %s - %d (%s)", method, path, status, latency)
}

// describeAPIAction 返回API动作的中文描述
func describeAPIAction(method string, path string) string {
	switch {
	case method == http.MethodGet && path == "/api/health":
		return "连接线测试：健康"
	case method == http.MethodGet && path == "/api/logs":
		return "系统日志：查询"
	case method == http.MethodPost && path == "/api/logs/cleanup":
		return "系统日志：清理"
	case method == http.MethodGet && path == "/api/settings":
		return "系统设置：查询"
	case method == http.MethodPut && path == "/api/settings":
		return "系统设置：更新"
	case method == http.MethodGet && path == "/api/files/directories":
		return "目录浏览：列出目录"
	case method == http.MethodPost && path == "/api/files/list":
		return "目录浏览：列出文件"
	case method == http.MethodPost && path == "/api/servers/data":
		return "数据服务器：创建"
	case method == http.MethodGet && path == "/api/servers/data":
		return "数据服务器：列表"
	case method == http.MethodPost && path == "/api/servers/data/test":
		return "数据服务器：临时测试"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/servers/data/") && strings.HasSuffix(path, "/test"):
		return "数据服务器：连接测试"
	case method == http.MethodGet && strings.HasPrefix(path, "/api/servers/data/"):
		return "数据服务器：详情"
	case method == http.MethodPut && strings.HasPrefix(path, "/api/servers/data/"):
		return "数据服务器：更新"
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/servers/data/"):
		return "数据服务器：删除"
	case method == http.MethodGet && path == "/api/servers/types":
		return "服务器类型：列表"
	case method == http.MethodGet && strings.HasPrefix(path, "/api/servers/types/"):
		return "服务器类型：详情"
	case method == http.MethodPost && path == "/api/servers/media":
		return "媒体服务器：创建"
	case method == http.MethodGet && path == "/api/servers/media":
		return "媒体服务器：列表"
	case method == http.MethodPost && path == "/api/servers/media/test":
		return "媒体服务器：临时测试"
	case method == http.MethodPost && strings.HasPrefix(path, "/api/servers/media/") && strings.HasSuffix(path, "/test"):
		return "媒体服务器：连接测试"
	case method == http.MethodGet && strings.HasPrefix(path, "/api/servers/media/"):
		return "媒体服务器：详情"
	case method == http.MethodPut && strings.HasPrefix(path, "/api/servers/media/"):
		return "媒体服务器：更新"
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/servers/media/"):
		return "媒体服务器：删除"
	case method == http.MethodPost && path == "/api/jobs":
		return "任务：创建"
	case method == http.MethodGet && path == "/api/jobs":
		return "任务：列表"
	case method == http.MethodPost && strings.HasSuffix(path, "/run") && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：执行"
	case method == http.MethodPost && strings.HasSuffix(path, "/stop") && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：停止"
	case method == http.MethodPut && strings.HasSuffix(path, "/enable") && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：启用"
	case method == http.MethodPut && strings.HasSuffix(path, "/disable") && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：禁用"
	case method == http.MethodGet && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：详情"
	case method == http.MethodPut && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：更新"
	case method == http.MethodDelete && strings.HasPrefix(path, "/api/jobs/"):
		return "任务：删除"
	case method == http.MethodGet && path == "/api/runs":
		return "执行历史：列表"
	case method == http.MethodGet && path == "/api/runs/stats":
		return "执行历史：统计"
	case method == http.MethodPost && strings.HasSuffix(path, "/cancel") && strings.HasPrefix(path, "/api/runs/"):
		return "执行历史：取消"
	case method == http.MethodGet && strings.HasPrefix(path, "/api/runs/"):
		return "执行历史：详情"
	default:
		return ""
	}
}

// setupStaticFiles 配置前端静态文件服务（使用 StaticFS）
func setupStaticFiles(router *gin.Engine) {
	// 查找 web_statics 目录（按优先级）
	webStaticsPath := findWebStaticsDir()
	if webStaticsPath == "" {
		logger.LogWarn("前端静态文件目录未找到，前端功能将不可用")
		return
	}

	logger.LogInfo("前端静态文件目录", zap.String("path", webStaticsPath))

	// 托管静态资源（JS/CSS/图片等）
	router.Static("/assets", filepath.Join(webStaticsPath, "assets"))
	router.StaticFile("/favicon.ico", filepath.Join(webStaticsPath, "favicon.ico"))

	// 根路径返回 index.html
	router.GET("/", func(c *gin.Context) {
		c.File(filepath.Join(webStaticsPath, "index.html"))
	})
}

// findWebStaticsDir 查找 web_statics 目录（按优先级）
func findWebStaticsDir() string {
	candidates := []string{
		"web_statics",           // 当前工作目录
		"../web_statics",        // 父目录（开发环境）
		"../../web_statics",     // 祖父目录
	}

	// 如果可执行文件路径可获取，添加可执行文件同级目录
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidates = append([]string{filepath.Join(execDir, "web_statics")}, candidates...)
	}

	// 查找第一个存在的目录
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			// 验证是否包含 index.html
			indexPath := filepath.Join(path, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				return path
			}
		}
	}

	return ""
}

// serveIndexHTML 返回前端 index.html（用于 SPA 路由回退）
func serveIndexHTML(c *gin.Context) {
	webStaticsPath := findWebStaticsDir()
	if webStaticsPath == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Frontend not available",
			"message": "web_statics directory not found",
		})
		return
	}

	c.File(filepath.Join(webStaticsPath, "index.html"))
}
