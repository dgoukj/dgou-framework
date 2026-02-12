package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dgou/pkg/async"
	"dgou/pkg/auth"
	"dgou/pkg/cache"
	"dgou/pkg/config"
	"dgou/pkg/database"
	"dgou/pkg/logger"
	"dgou/pkg/middleware"
	"dgou/pkg/monitor"
	"dgou/pkg/queue"
	"dgou/pkg/upload"
)

// App 应用框架核心结构
type App struct {
	cfg         *config.Config
	logger      *logger.Logger
	db          *database.DB
	cache       cache.Cache
	auth        *auth.Manager
	asyncPool   *async.Pool
	uploadMgr   *upload.Manager
	monitor     *monitor.Monitor
	engine      *gin.Engine
	httpServer  *http.Server
	queue       queue.Queue
	shutdownCtx context.Context
	cancel      context.CancelFunc
}

// New 创建应用实例
func New(cfg *config.Config, log *logger.Logger) *App {
	gin.SetMode(cfg.Server.Mode)
	engine := gin.New()
	return &App{
		cfg:    cfg,
		logger: log,
		engine: engine,
	}
}

// ---------- 依赖注入 ----------
func (a *App) SetDatabase(db *database.DB)         { a.db = db }
func (a *App) SetCache(c cache.Cache)              { a.cache = c }
func (a *App) SetAuth(am *auth.Manager)            { a.auth = am }
func (a *App) SetAsyncPool(p *async.Pool)          { a.asyncPool = p }
func (a *App) SetUploadManager(um *upload.Manager) { a.uploadMgr = um }
func (a *App) SetMonitor(m *monitor.Monitor)       { a.monitor = m }
func (a *App) SetQueue(q queue.Queue)              { a.queue = q }

// ---------- 获取组件实例 ----------
func (a *App) Logger() *logger.Logger    { return a.logger }
func (a *App) DB() *database.DB          { return a.db }
func (a *App) Cache() cache.Cache        { return a.cache }
func (a *App) Auth() *auth.Manager       { return a.auth }
func (a *App) AsyncPool() *async.Pool    { return a.asyncPool }
func (a *App) Upload() *upload.Manager   { return a.uploadMgr }
func (a *App) Monitor() *monitor.Monitor { return a.monitor }
func (a *App) Queue() queue.Queue        { return a.queue }

// GetEngine 返回 Gin 引擎，供业务层注册路由
func (a *App) GetEngine() *gin.Engine {
	return a.engine
}

// ---------- 快捷注册方法（可选）----------
func (a *App) RegisterDefaultMiddleware() {
	a.engine.Use(middleware.Recovery(a.logger))
	a.engine.Use(middleware.RequestID())
	if a.cfg.Security.EnableCORS {
		a.engine.Use(middleware.CORS(a.cfg.Security.AllowedOrigins))
	}
	if a.monitor != nil {
		a.engine.Use(a.monitor.GinMiddleware())
	}
	a.engine.Use(middleware.Logger(a.logger))
}

func (a *App) RegisterHealthCheck() {
	if a.cfg.Monitor.EnableHealth {
		a.engine.GET(a.cfg.Monitor.HealthPath, func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":    "healthy",
				"timestamp": time.Now().Unix(),
			})
		})
	}
}

func (a *App) RegisterMetrics() {
	if a.monitor != nil && a.cfg.Monitor.EnableMetrics {
		a.engine.GET(a.monitor.MetricsPath(), gin.WrapH(a.monitor.Handler()))
	}
}

func (a *App) RegisterPprof() {
	if a.cfg.Monitor.EnableProfiling {
		// 注册 pprof 路由（略）
	}
}

// ---------- 生命周期管理 ----------
func (a *App) Start() error {
	addr := fmt.Sprintf(":%d", a.cfg.Server.Port)
	a.httpServer = &http.Server{
		Addr:         addr,
		Handler:      a.engine,
		ReadTimeout:  time.Duration(a.cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(a.cfg.Server.WriteTimeout) * time.Second,
	}
	a.logger.Info("HTTP server starting", zap.String("addr", addr))
	var err error
	if a.cfg.Server.EnableHTTPS {
		err = a.httpServer.ListenAndServeTLS(a.cfg.Server.CertFile, a.cfg.Server.KeyFile)
	} else {
		err = a.httpServer.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (a *App) Shutdown() error {
	a.logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(a.cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := a.httpServer.Shutdown(ctx); err != nil {
		a.logger.Error("server shutdown error", zap.Error(err))
	}
	if a.db != nil {
		_ = a.db.Close()
	}
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.asyncPool != nil {
		a.asyncPool.Stop()
	}
	if a.queue != nil {
		_ = a.queue.Close()
	}
	_ = a.logger.Sync()
	return nil
}

func (a *App) Run() error {
	a.shutdownCtx, a.cancel = context.WithCancel(context.Background())
	go func() {
		if err := a.Start(); err != nil {
			a.logger.Fatal("start server", zap.Error(err))
		}
	}()
	<-a.shutdownCtx.Done()
	return a.Shutdown()
}
