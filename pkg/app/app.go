package app

import (
	"context"
	"dgou/pkg/cache"
	"dgou/pkg/config"
	"dgou/pkg/database"
	"dgou/pkg/logger"
	"dgou/pkg/middleware"
	"dgou/pkg/monitor"
	"dgou/pkg/response"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// App 应用结构体
type App struct {
	config     *config.Config   // 配置
	engine     *gin.Engine      // Gin引擎
	httpServer *http.Server     // HTTP服务器
	shutdown   *ShutdownHandler // 优雅关闭处理器
	routers    []Router         // 路由注册器
	health     *HealthChecker   // 健康检查器
	monitors   []Monitor        // 监控器
}

// Router 路由接口
type Router interface {
	Register(router *gin.RouterGroup) // 注册路由
	Priority() int                    // 路由优先级（数字越小优先级越高）
}

// Monitor 监控接口
type Monitor interface {
	Start() error // 启动监控
	Stop() error  // 停止监控
	Name() string // 监控器名称
}

// NewApp 创建新应用
func NewApp(cfg *config.Config) *App {
	// 设置Gin模式
	setGinMode(cfg)

	// 创建Gin引擎
	engine := gin.New()

	// 创建应用实例
	app := &App{
		config:   cfg,
		engine:   engine,
		shutdown: NewShutdownHandler(time.Duration(cfg.Server.ShutdownTimeout) * time.Second),
		routers:  make([]Router, 0),
		monitors: make([]Monitor, 0),
	}

	// 初始化健康检查器
	app.health = NewHealthChecker()

	return app
}

// setGinMode 设置Gin运行模式
func setGinMode(cfg *config.Config) {
	switch cfg.Server.Mode {
	case "debug":
		gin.SetMode(gin.DebugMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}
}

// Initialize 初始化应用
func (app *App) Initialize() error {
	// 初始化日志
	logger.InitLogger(&app.config.Log)

	// 初始化数据库
	if err := database.InitDB(&app.config.MySQL); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	app.shutdown.Register(func() {
		database.CloseDB()
	})

	// 初始化缓存
	if err := cache.InitCache(&app.config.Redis); err != nil {
		logger.Error("Failed to initialize cache", logger.ErrorField(err))
		// 缓存初始化失败不影响应用启动
	} else {
		app.shutdown.Register(func() {
			cache.Close()
		})
	}

	// 设置中间件
	app.setupMiddleware()

	// 设置监控路由
	app.setupMonitorRoutes()

	// 设置业务路由
	app.setupRoutes()

	// 设置404处理
	app.setupNotFoundHandler()

	logger.Info("Application initialized successfully")
	return nil
}

// setupMiddleware 设置中间件
func (app *App) setupMiddleware() {
	// 恢复中间件（必须第一个）
	app.engine.Use(gin.Recovery())

	// 请求ID中间件
	app.engine.Use(middleware.RequestID())

	// CORS中间件
	if app.config.Security.EnableCORS {
		app.engine.Use(middleware.CORS())
	}

	// 日志中间件
	app.engine.Use(middleware.Logger())

	// 限流中间件
	if app.config.Security.EnableRateLimit {
		app.engine.Use(middleware.RateLimiter(app.config.Security.RateLimit))
	}

	// Gzip压缩中间件
	if app.config.Server.EnableGzip {
		app.engine.Use(gzip.Gzip(gzip.DefaultCompression))
	}

	// 安全中间件
	app.engine.Use(middleware.Security())

	// 错误处理中间件
	app.engine.Use(middleware.ErrorHandler())
}

// setupMonitorRoutes 设置监控路由
func (app *App) setupMonitorRoutes() {
	cfg := app.config.Monitor

	// 健康检查路由
	if cfg.EnableHealth {
		app.engine.GET(cfg.HealthPath, app.health.Handler())
		app.engine.GET("/ready", app.health.ReadyHandler())
		app.engine.GET("/live", app.health.LiveHandler())
	}

	// Metrics路由
	if cfg.EnableMetrics {
		metrics := monitor.NewMetrics()
		app.engine.GET(cfg.MetricsPath, metrics.Handler())
		app.engine.Use(metrics.Middleware())
		app.monitors = append(app.monitors, metrics)
	}

	// Profiling路由（仅开发环境）
	if cfg.EnableProfiling && gin.Mode() == gin.DebugMode {
		app.engine.GET(cfg.ProfilePath+"/", gin.WrapH(http.DefaultServeMux))
		app.engine.GET(cfg.ProfilePath+"/:cmd", gin.WrapH(http.DefaultServeMux))
	}
}

// setupRoutes 设置业务路由
func (app *App) setupRoutes() {
	// 按照优先级排序路由
	sortRoutersByPriority(app.routers)

	// 注册所有路由
	for _, router := range app.routers {
		router.Register(app.engine.Group(""))
	}
}

// sortRoutersByPriority 按优先级排序路由
func sortRoutersByPriority(routers []Router) {
	// 实现排序逻辑
	// 这里简化为冒泡排序
	n := len(routers)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if routers[j].Priority() > routers[j+1].Priority() {
				routers[j], routers[j+1] = routers[j+1], routers[j]
			}
		}
	}
}

// setupNotFoundHandler 设置404处理器
func (app *App) setupNotFoundHandler() {
	app.engine.NoRoute(func(c *gin.Context) {
		response.NotFound(c, "Route not found")
	})
}

// AddRouter 添加路由
func (app *App) AddRouter(router Router) {
	app.routers = append(app.routers, router)
}

// AddMonitor 添加监控器
func (app *App) AddMonitor(monitor Monitor) {
	app.monitors = append(app.monitors, monitor)
}

// Run 运行应用
func (app *App) Run() error {
	// 启动监控器
	for _, monitor := range app.monitors {
		if err := monitor.Start(); err != nil {
			logger.Error("Failed to start monitor",
				logger.String("monitor", monitor.Name()),
				logger.ErrorField(err),
			)
		}
	}

	// 创建HTTP服务器
	app.httpServer = &http.Server{
		Addr:           fmt.Sprintf(":%d", app.config.Server.Port),
		Handler:        app.engine,
		ReadTimeout:    time.Duration(app.config.Server.ReadTimeout) * time.Second,
		WriteTimeout:   time.Duration(app.config.Server.WriteTimeout) * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// 启动服务器
	go func() {
		logger.Info("Starting server",
			logger.Int("port", app.config.Server.Port),
			logger.String("mode", app.config.Server.Mode),
		)

		var err error
		if app.config.Server.EnableHTTPS {
			err = app.httpServer.ListenAndServeTLS(
				app.config.Server.CertFile,
				app.config.Server.KeyFile,
			)
		} else {
			err = app.httpServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", logger.ErrorField(err))
		}
	}()

	// 等待关闭信号
	app.waitForShutdown()

	return nil
}

// waitForShutdown 等待关闭信号
func (app *App) waitForShutdown() {
	// 监听系统信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-quit
	logger.Info("Received shutdown signal", logger.String("signal", sig.String()))

	// 执行优雅关闭
	app.gracefulShutdown()
}

// gracefulShutdown 优雅关闭
func (app *App) gracefulShutdown() {
	// 设置关闭超时
	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(app.config.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	// 停止接收新请求
	if err := app.httpServer.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", logger.ErrorField(err))
	}

	// 停止监控器
	for _, monitor := range app.monitors {
		if err := monitor.Stop(); err != nil {
			logger.Error("Failed to stop monitor",
				logger.String("monitor", monitor.Name()),
				logger.ErrorField(err),
			)
		}
	}

	// 执行关闭处理器
	app.shutdown.Execute()

	logger.Info("Server exited gracefully")
}

// GetEngine 获取Gin引擎（用于高级配置）
func (app *App) GetEngine() *gin.Engine {
	return app.engine
}

// GetHealthChecker 获取健康检查器
func (app *App) GetHealthChecker() *HealthChecker {
	return app.health
}

// RegisterStatic 注册静态文件路由
func (app *App) RegisterStatic(relativePath string, fs embed.FS) {
	app.engine.StaticFS(relativePath, http.FS(fs))
}
