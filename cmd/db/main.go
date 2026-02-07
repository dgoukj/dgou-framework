package main

import (
	"dgou/pkg/config"
	"dgou/pkg/database"
	"dgou/pkg/logger"
	"flag"
	"fmt"
	"os"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "./config/config.yaml", "配置文件路径")
	command := flag.String("command", "migrate", "命令: migrate, rollback, status, create")
	steps := flag.Int("steps", 1, "回滚步数")
	name := flag.String("name", "", "迁移名称")
	flag.Parse()

	// 加载配置
	cfg := config.LoadConfig(*configPath)

	// 初始化日志
	logger.InitLogger(&cfg.Log)

	// 初始化数据库
	db, err := database.InitDB(cfg)
	if err != nil {
		logger.Error("Failed to initialize database", logger.ErrorField(err))
		os.Exit(1)
	}
	defer db.Close()

	// 获取迁移管理器
	migrator, err := database.GetMigrator("./migrations")
	if err != nil {
		logger.Error("Failed to get migrator", logger.ErrorField(err))
		os.Exit(1)
	}

	// 执行命令
	switch *command {
	case "migrate":
		if err := migrator.Migrate(); err != nil {
			logger.Error("Migration failed", logger.ErrorField(err))
			os.Exit(1)
		}

	case "rollback":
		if err := migrator.Rollback(*steps); err != nil {
			logger.Error("Rollback failed", logger.ErrorField(err))
			os.Exit(1)
		}

	case "status":
		stats, err := migrator.Status()
		if err != nil {
			logger.Error("Failed to get migration status", logger.ErrorField(err))
			os.Exit(1)
		}

		fmt.Printf("Migration Status:\n")
		fmt.Printf("  Total Files: %d\n", stats["total_files"])
		fmt.Printf("  Applied: %d\n", stats["total_applied"])
		fmt.Printf("  Pending: %d\n", stats["total_pending"])

		migrations := stats["migrations"].([]map[string]interface{})
		for _, m := range migrations {
			status := "❌"
			if m["status"] == "applied" {
				status = "✅"
			}
			fmt.Printf("  %s %s - %s (Batch: %d)\n",
				status, m["id"], m["name"], m["batch"])
		}

	case "create":
		if *name == "" {
			fmt.Println("Error: migration name is required")
			os.Exit(1)
		}

		filepath, err := migrator.CreateMigration(*name)
		if err != nil {
			logger.Error("Failed to create migration", logger.ErrorField(err))
			os.Exit(1)
		}

		fmt.Printf("Migration file created: %s\n", filepath)

	case "stats":
		stats := db.GetStats()
		fmt.Printf("Database Stats:\n")
		for name, stat := range stats {
			s := stat.(map[string]interface{})
			fmt.Printf("  %s (%s):\n", name, s["role"])
			fmt.Printf("    Is Connected: %v\n", s["is_connected"])
			fmt.Printf("    Open Connections: %d\n", s["open_connections"])
			fmt.Printf("    In Use: %d\n", s["in_use"])
			fmt.Printf("    Idle: %d\n", s["idle"])
			fmt.Printf("    Wait Count: %d\n", s["wait_count"])
			fmt.Printf("    Max Open: %d\n", s["max_open_conns"])
		}

	default:
		fmt.Printf("Unknown command: %s\n", *command)
		fmt.Println("Available commands: migrate, rollback, status, create, stats")
		os.Exit(1)
	}

	logger.Info("Database operation completed")
}
