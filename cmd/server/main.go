package main

import (
	"context"
	"dgou/pkg/app"
	"dgou/pkg/bootstrap"
	"dgou/pkg/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. 初始化所有组件
	result, err := bootstrap.Init()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer result.Closer()

	// 2. 注册业务路由（由业务层实现）
	registerBusinessRoutes(result.App)

	// 3. 运行应用（非阻塞）
	go func() {
		if err := result.App.Run(); err != nil {
			result.App.Logger().Fatal("app run failed", zap.Error(err))
		}
	}()

	// 4. 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// 5. 优雅关闭（超时控制）
	_, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := result.App.Shutdown(); err != nil {
		result.App.Logger().Error("shutdown error", zap.Error(err))
	}
}

// registerBusinessRoutes 业务路由定义（应由业务开发人员在此编写）
func registerBusinessRoutes(app *app.App) {
	engine := app.GetEngine()
	authManager := app.Auth() // 需要为 App 添加 Auth() 等方法

	// 示例：登录接口
	engine.POST("/api/v1/login", func(c *gin.Context) {
		// ... 业务逻辑
	})

	// 示例：需要认证的接口
	authGroup := engine.Group("/api/v1")
	authGroup.Use(middleware.Auth(authManager))
	{
		authGroup.GET("/profile", func(c *gin.Context) {
			// ...
		})
	}
}
