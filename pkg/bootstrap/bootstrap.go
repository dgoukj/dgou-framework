package bootstrap

import (
	"time"

	"go.uber.org/zap"

	"github.com/dgoukj/dgou-framework/pkg/app"
	"github.com/dgoukj/dgou-framework/pkg/async"
	"github.com/dgoukj/dgou-framework/pkg/auth"
	"github.com/dgoukj/dgou-framework/pkg/cache"
	"github.com/dgoukj/dgou-framework/pkg/config"
	"github.com/dgoukj/dgou-framework/pkg/database"
	"github.com/dgoukj/dgou-framework/pkg/logger"
	"github.com/dgoukj/dgou-framework/pkg/monitor"
	"github.com/dgoukj/dgou-framework/pkg/queue"
	"github.com/dgoukj/dgou-framework/pkg/upload"
)

// AppResult 封装应用实例及其清理函数
type AppResult struct {
	App    *app.App
	Closer func() // 优雅关闭时调用的清理函数
}

// Init 初始化所有组件，返回可运行的 App 实例
func Init(configPath ...string) (*AppResult, error) {
	// ------------------------------------------------------------
	// 1. 加载配置
	// ------------------------------------------------------------
	cfg, err := config.Load(configPath...)
	if err != nil {
		return nil, err
	}

	// ------------------------------------------------------------
	// 2. 初始化日志
	// ------------------------------------------------------------
	logInstance, err := logger.New(logger.Config{
		Level:      cfg.Log.Level,
		File:       cfg.Log.File,
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
		Compress:   cfg.Log.Compress,
		AddCaller:  cfg.Log.AddCaller,
		JSONFormat: cfg.Log.JSONFormat,
	})
	if err != nil {
		return nil, err
	}

	// ------------------------------------------------------------
	// 3. 数据库（如果配置）
	// ------------------------------------------------------------
	var db *database.DB
	if cfg.MySQL.Master.Host != "" {
		dbCfg := database.Config{
			Master: database.DSNConfig{
				Host:         cfg.MySQL.Master.Host,
				Port:         cfg.MySQL.Master.Port,
				User:         cfg.MySQL.Master.User,
				Password:     cfg.MySQL.Master.Password,
				DBName:       cfg.MySQL.Master.DBName,
				Charset:      cfg.MySQL.Master.Charset,
				ParseTime:    cfg.MySQL.Master.ParseTime,
				Loc:          cfg.MySQL.Master.Loc,
				MaxOpenConns: cfg.MySQL.Master.MaxOpenConns,
				MaxIdleConns: cfg.MySQL.Master.MaxIdleConns,
			},
			Pool: database.PoolConfig{
				MaxOpenConns:    cfg.MySQL.Pool.MaxOpenConns,
				MaxIdleConns:    cfg.MySQL.Pool.MaxIdleConns,
				ConnMaxLifetime: cfg.MySQL.Pool.ConnMaxLifetime,
				ConnMaxIdleTime: cfg.MySQL.Pool.ConnMaxIdleTime,
			},
			Log: database.LogConfig{
				SlowThreshold: cfg.MySQL.Log.SlowThreshold,
				EnableLogging: cfg.MySQL.Log.EnableLogging,
				LogLevel:      cfg.MySQL.Log.LogLevel,
			},
			Performance: database.PerformanceConfig{
				PrepareStmt:       cfg.MySQL.Performance.PrepareStmt,
				DisableForeignKey: cfg.MySQL.Performance.DisableForeignKey,
			},
		}
		for _, s := range cfg.MySQL.Slaves {
			dbCfg.Slaves = append(dbCfg.Slaves, database.DSNConfig{
				Host:         s.Host,
				Port:         s.Port,
				User:         s.User,
				Password:     s.Password,
				DBName:       s.DBName,
				Charset:      s.Charset,
				ParseTime:    s.ParseTime,
				Loc:          s.Loc,
				MaxOpenConns: s.MaxOpenConns,
				MaxIdleConns: s.MaxIdleConns,
			})
		}
		db, err = database.New(dbCfg)
		if err != nil {
			logInstance.Warn("database init failed, continue without db", zap.Error(err))
		}
	}

	// ------------------------------------------------------------
	// 4. 缓存（健壮降级）
	// ------------------------------------------------------------
	var cacheInstance cache.Cache
	if cfg.Redis.Addr != "" {
		redisCfg := cache.RedisConfig{
			Addr:         cfg.Redis.Addr,
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			PoolSize:     cfg.Redis.PoolSize,
			MinIdleConns: cfg.Redis.MinIdleConns,
			MaxRetries:   cfg.Redis.MaxRetries,
			DialTimeout:  time.Duration(cfg.Redis.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.Redis.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.Redis.WriteTimeout) * time.Second,
		}
		redisCache, err := cache.NewRedis(redisCfg, "app")
		if err != nil {
			logInstance.Warn("redis cache failed, fallback to memory", zap.Error(err))
			cacheInstance = cache.NewMemory(cache.MemoryConfig{
				MaxMemoryMB: 100,
				DefaultTTL:  time.Hour,
			})
		} else {
			cacheInstance = redisCache
		}
	} else {
		cacheInstance = cache.NewMemory(cache.MemoryConfig{
			MaxMemoryMB: 100,
			DefaultTTL:  time.Hour,
		})
	}
	// 绝对防御：如果仍为 nil（理论上不会），创建内存缓存
	if cacheInstance == nil {
		logInstance.Error("cache instance is nil, using fallback memory cache")
		cacheInstance = cache.NewMemory(cache.MemoryConfig{
			MaxMemoryMB: 100,
			DefaultTTL:  time.Hour,
		})
	}

	// ------------------------------------------------------------
	// 5. 认证
	// ------------------------------------------------------------
	authCfg := auth.Config{
		Secret:     cfg.JWT.Secret,
		Issuer:     cfg.JWT.Issuer,
		AccessTTL:  time.Duration(cfg.JWT.Expire) * time.Hour,
		RefreshTTL: time.Duration(cfg.JWT.Refresh) * time.Hour,
	}
	authManager := auth.NewManager(cacheInstance, authCfg)

	// ------------------------------------------------------------
	// 6. 异步任务池
	// ------------------------------------------------------------
	asyncPool := async.NewPool("default", cfg.Async.MaxWorkers, cfg.Async.MaxQueueSize)
	if err := asyncPool.Start(); err != nil {
		logInstance.Fatal("start async pool failed", zap.Error(err))
	}

	// ------------------------------------------------------------
	// 7. 文件上传
	// ------------------------------------------------------------
	var uploadStorage upload.Storage
	switch cfg.Upload.StorageType {
	case "local":
		uploadStorage, err = upload.NewLocalStorage(upload.LocalConfig{
			BasePath: cfg.Upload.BasePath,
			BaseURL:  cfg.Upload.BaseURL,
			CDNURL:   cfg.Upload.CDNURL,
		})
	case "oss":
		uploadStorage, err = upload.NewOSSStorage(upload.OSSConfig{
			Endpoint:        cfg.Upload.Endpoint,
			AccessKeyID:     cfg.Upload.AccessKeyID,
			AccessKeySecret: cfg.Upload.AccessKeySecret,
			Bucket:          cfg.Upload.Bucket,
			Region:          cfg.Upload.Region,
			UseHTTPS:        cfg.Upload.UseHTTPS,
			CDNURL:          cfg.Upload.CDNURL,
		})
	default:
		logInstance.Fatal("unsupported storage type", zap.String("type", cfg.Upload.StorageType))
	}
	if err != nil {
		logInstance.Fatal("init upload storage failed", zap.Error(err))
	}
	uploadCfg := upload.Config{
		MaxFileSize:  cfg.Upload.MaxFileSize,
		AllowedExts:  cfg.Upload.AllowedExtensions,
		AllowedMimes: cfg.Upload.AllowedMimeTypes,
	}
	uploadManager := upload.NewManager(uploadStorage, uploadCfg, logInstance)

	// ------------------------------------------------------------
	// 8. 监控
	// ------------------------------------------------------------
	monitorCfg := monitor.Config{
		ServiceName:     cfg.Monitor.ServiceName,
		ServiceVersion:  cfg.Monitor.ServiceVersion,
		Environment:     cfg.Monitor.Environment,
		EnableMetrics:   cfg.Monitor.EnableMetrics,
		MetricsPath:     cfg.Monitor.MetricsPath,
		EnableHealth:    cfg.Monitor.EnableHealth,
		HealthPath:      cfg.Monitor.HealthPath,
		EnableProfiling: cfg.Monitor.EnableProfiling,
		ProfilePath:     cfg.Monitor.ProfilePath,
	}
	monitorInstance := monitor.New(monitorCfg)

	// ------------------------------------------------------------
	// 9. 创建 App 实例并注入基础依赖
	// ------------------------------------------------------------
	application := app.New(cfg, logInstance)
	application.SetDatabase(db)
	application.SetCache(cacheInstance)
	application.SetAuth(authManager)
	application.SetAsyncPool(asyncPool)
	application.SetUploadManager(uploadManager)
	application.SetMonitor(monitorInstance)

	// ------------------------------------------------------------
	// 10. 队列（RabbitMQ）
	// ------------------------------------------------------------
	var queueManager queue.Queue
	if cfg.Queue.Driver == "rabbitmq" {
		rabbitCfg := &queue.RabbitMQConfig{
			URL:            cfg.Queue.RabbitMQ.URL,
			Host:           cfg.Queue.RabbitMQ.Host,
			Port:           cfg.Queue.RabbitMQ.Port,
			Username:       cfg.Queue.RabbitMQ.Username,
			Password:       cfg.Queue.RabbitMQ.Password,
			Vhost:          cfg.Queue.RabbitMQ.Vhost,
			Heartbeat:      cfg.Queue.RabbitMQ.Heartbeat,
			DialTimeout:    time.Duration(cfg.Queue.RabbitMQ.ConnectionTimeout) * time.Second,
			MaxRetries:     5,
			RetryDelay:     5 * time.Second,
			PrefetchCount:  cfg.Queue.RabbitMQ.PrefetchCount,
			PrefetchSize:   cfg.Queue.RabbitMQ.PrefetchSize,
			GlobalPrefetch: cfg.Queue.RabbitMQ.GlobalPrefetch,
		}
		queueManager, err = queue.NewRabbitMQ(rabbitCfg, logInstance)
		if err != nil {
			logInstance.Warn("rabbitmq init failed, queue disabled", zap.Error(err))
		} else {
			// 注入队列管理器到 App
			application.SetQueue(queueManager)

			// 声明默认交换机和队列（如果配置了）
			if cfg.Queue.RabbitMQ.ExchangeName != "" {
				if err := queueManager.DeclareExchange(
					cfg.Queue.RabbitMQ.ExchangeName,
					cfg.Queue.RabbitMQ.ExchangeType,
					cfg.Queue.RabbitMQ.Durable,
					false, // autoDelete
					false, // internal
					cfg.Queue.RabbitMQ.NoWait,
					nil,
				); err != nil {
					logInstance.Warn("declare exchange failed", zap.Error(err))
				}
			}
			if cfg.Queue.RabbitMQ.QueueName != "" {
				if err := queueManager.DeclareQueue(
					cfg.Queue.RabbitMQ.QueueName,
					cfg.Queue.RabbitMQ.Durable,
					cfg.Queue.RabbitMQ.AutoDelete,
					cfg.Queue.RabbitMQ.Exclusive,
					cfg.Queue.RabbitMQ.NoWait,
					nil,
				); err != nil {
					logInstance.Warn("declare queue failed", zap.Error(err))
				}
			}
			if cfg.Queue.RabbitMQ.QueueName != "" && cfg.Queue.RabbitMQ.ExchangeName != "" && cfg.Queue.RabbitMQ.RoutingKey != "" {
				if err := queueManager.BindQueue(
					cfg.Queue.RabbitMQ.QueueName,
					cfg.Queue.RabbitMQ.RoutingKey,
					cfg.Queue.RabbitMQ.ExchangeName,
					cfg.Queue.RabbitMQ.NoWait,
					nil,
				); err != nil {
					logInstance.Warn("bind queue failed", zap.Error(err))
				}
			}
		}
	}

	// ------------------------------------------------------------
	// 11. 注册框架内置中间件/路由
	// ------------------------------------------------------------
	application.RegisterDefaultMiddleware()
	application.RegisterHealthCheck()
	application.RegisterMetrics()
	application.RegisterPprof()

	// ------------------------------------------------------------
	// 12. 返回结果及清理函数
	// ------------------------------------------------------------
	closer := func() {
		if db != nil {
			_ = db.Close()
		}
		cacheInstance.Close()
		asyncPool.Stop()
		if queueManager != nil {
			_ = queueManager.Close()
		}
		_ = logInstance.Sync()
	}

	return &AppResult{
		App:    application,
		Closer: closer,
	}, nil
}
