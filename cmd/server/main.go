package main

import (
	"dgou/pkg/app"
	"dgou/pkg/config"
	"log"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig("./config/config.yaml")

	// 创建应用
	application := app.NewApp(cfg)

	// 初始化应用
	if err := application.Initialize(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// 注册业务路由（示例）
	// application.AddRouter(&api.UserRouter{})
	// application.AddRouter(&api.ProductRouter{})

	// 运行应用
	if err := application.Run(); err != nil {
		log.Fatalf("Failed to run application: %v", err)
	}
}
