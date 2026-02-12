# DGou Framework

**生产级 Go 微服务脚手架**

基于 Gin、GORM、Redis、JWT、RabbitMQ、Prometheus 等成熟技术栈，提供开箱即用、业务解耦、高可扩展的微服务开发体验。

![Go版本](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![Gin版本](https://img.shields.io/badge/Gin-1.9.1-00ADD8?style=flat&logo=go)
![Gorm版本](https://img.shields.io/badge/GORM-1.25.2-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
## 📦 项目概述
### 设计哲学
- ✅ **无全局变量：** 所有组件通过依赖注入传递，彻底告别全局单例。
- ✅ **接口驱动：** 缓存、存储、队列等均抽象为接口，易于替换和测试。
- ✅ **业务与框架分离：** 框架不包含任何业务逻辑，业务代码放在 internal/ 中。
- ✅ **开箱即用：** bootstrap 包一键初始化所有组件，main 函数仅需几行代码。

### 核心特性

| 模块 | 功能 | 生产级特性 |
| :--- | :--- | :--- |
| 配置 | YAML + 环境变量 + 热加载 | Viper，默认值全覆盖 |
| 日志 | Zap + Lumberjack | 结构化、轮转、多输出 |
| 数据库 | GORM + MySQL | 读写分离、连接池、自动重连 |
| 缓存 | Redis / 内存 | 自动降级、分布式锁、集合/哈希 |
| 认证 | JWT | Access/Refresh Token，基于 Redis 吊销 |
| 异步 | 协程池 | 优先级队列、重试、超时、取消 |
| 上传 | 本地 / 阿里云 OSS | 统一接口、MD5 校验、URL 生成 |
| 监控 | Prometheus + pprof | HTTP 指标、健康检查、性能分析 |
| 队列 | RabbitMQ | 自动重连、QoS、消息确认、交换机/队列管理 |
| 中间件 | Gin | 恢复、请求 ID、CORS、日志、限流、认证 |
| 应用 | 生命周期管理 | 优雅启停、信号监听、依赖注入 |

## 📁 目录结构
```text

your-project/
├── cmd/
│   └── server/
│       └── main.go           # 业务入口（仅需几行代码）
├── internal/                 # 私有业务代码（不对外导出）
│   ├── models/              # GORM 数据模型
│   ├── services/            # 业务逻辑
│   ├── handlers/            # HTTP 处理器
│   └── ...                  
├── pkg/                     # 可复用的框架组件
│   ├── app/                # 应用生命周期管理
│   ├── bootstrap/          # 一键初始化
│   ├── config/             # 配置加载
│   ├── logger/             # 日志封装
│   ├── errors/             # 自定义错误
│   ├── database/           # 读写分离数据库
│   ├── cache/              # 缓存接口及实现（Redis/内存）
│   ├── auth/               # JWT 认证
│   ├── async/              # 异步协程池
│   ├── upload/             # 文件上传（本地/OSS）
│   ├── monitor/            # 监控（Prometheus/pprof）
│   ├── queue/              # 队列（RabbitMQ）
│   └── middleware/         # Gin 中间件
├── config/                  # 配置文件目录
│   └── config.yaml         # 示例配置
├── go.mod
└── go.sum
```
## 🚀 快速开始
### 1. 创建新项目
```bash

# 克隆脚手架模板（替换为您的仓库地址）
git clone https://github.com/your/dgou-framework.git my-service
cd my-service
go mod edit -module your-project-name
```
### 2. 编写配置文件

创建 `config/config.yaml`（至少配置 `JWT` 密钥）：
```yaml

server:
    port: 8080
    mode: debug

jwt:
    secret: your-32-character-secret-key-here!!  # 必须 ≥32 字符

mysql:
    master:
        host: localhost
        port: 3306
        user: root
        password: 123456
        dbname: test

redis:
    addr: localhost:6379
```
### 3. 编写第一个业务接口

编辑 `cmd/server/main.go`：
```go

package main

import (
    "context"
    "log"

	"your-project/internal/handlers"
	"your-project/pkg/bootstrap"
	"your-project/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func main() {
    // 1. 初始化框架组件
    result, err := bootstrap.Init()
    if err != nil {
    log.Fatal(err)
    }
    defer result.Closer()

	// 2. 注册业务路由
	app := result.App
	engine := app.GetEngine()

	// 健康检查已在框架中自动注册，无需重复
	engine.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"message": "pong"})
	})

	// 登录接口（需自行实现 handlers.Login）
	engine.POST("/login", handlers.Login(app.Auth(), app.Logger()))

	// 需要认证的接口组
	api := engine.Group("/api")
	api.Use(middleware.Auth(app.Auth()))
	{
		api.GET("/user/:id", handlers.GetUser(app.DB()))
	}

	// 3. 启动服务（阻塞）
	app.Run()
}
```
### 4. 运行服务
```bash

go run cmd/server/main.go
```
访问 http://localhost:8080/health 验证服务健康状态。
## 🧩 组件使用详解
### 1. 配置 (`config`)
```go

import "your-project/pkg/config"

// 加载配置（默认查找 ./config/config.yaml）
cfg, _ := config.Load()

// 加载指定路径的配置文件
cfg, _ := config.Load("/path/to/config.yaml")

// 支持热加载（需调用 WatchConfig）
config.WatchConfig(cfg, "/path/to/config.yaml")
```
**配置优先级：** 默认值 < 配置文件 < 环境变量（前缀 APP_，点号转下划线）。
### 2. 日志 (`logger`)
```go

import "your-project/pkg/logger"

// 初始化（bootstrap 已自动完成）
log, _ := logger.New(logger.Config{Level: "info"})

// 使用结构化日志
log.Info("hello", zap.String("key", "value"))

// 使用 SugaredLogger（适用于带格式的日志）
log.Sugar().Infof("format %s", "world")

// 创建带固定字段的子 Logger
userLog := log.With(zap.Uint64("user_id", 123))
userLog.Info("user action")
```
### 3. 数据库 (`database`)
```go

import "your-project/pkg/database"

// 获取实例（由 bootstrap 注入）
db := app.DB()

// 主库（写）
master := db.Master()

// 从库（读，自动轮询负载均衡）
slave := db.Slave()

// 事务
db.Master().Transaction(func(tx *gorm.DB) error {
// ...
return nil
})
```
**模型定义：** 内嵌 `database.BaseModel` 自动获得 ID、CreatedAt、UpdatedAt、DeletedAt（软删除）。
### 4. 缓存 (`cache`)

缓存接口 `cache.Cache` 提供 `Redis` 和内存两种实现。优先使用 `Redis`，失败自动降级内存。
```go

// 获取缓存实例
cache := app.Cache()

// 基础操作
cache.Set(ctx, "key", "value", time.Hour)
val, _ := cache.Get(ctx, "key")
cache.Delete(ctx, "key")
cache.Exists(ctx, "key")

// 自增/自减
cache.Incr(ctx, "counter", 1)
cache.Decr(ctx, "counter", 1)

// 分布式锁
token, _ := cache.Lock(ctx, "resource", 10*time.Second)
defer cache.Unlock(ctx, "resource", token)

// 集合操作（仅 Redis 支持完整功能，内存缓存返回“未实现”）
cache.SAdd(ctx, "set", "member1", "member2")
members, _ := cache.SMembers(ctx, "set")

// 哈希操作
cache.HSet(ctx, "hash", "field", "value")
val, _ := cache.HGet(ctx, "hash", "field")
```
### 5. 认证 (`auth`)

JWT 认证管理器，依赖 `cache.Cache` 存储刷新令牌。
```go

import "your-project/pkg/auth"

// 获取实例（bootstrap 注入）
authManager := app.Auth()

// 生成令牌对
accessToken, refreshToken, _ := authManager.Generate(1, "alice", []string{"admin"})

// 解析访问令牌
claims, _ := authManager.Parse(accessToken)
fmt.Println(claims.UserID, claims.Username)

// 刷新令牌
newAccess, newRefresh, _ := authManager.Refresh(refreshToken)

// 吊销刷新令牌
authManager.Revoke(tokenID)
```
**中间件：** `middleware.Auth(authManager)` 用于 Gin 路由认证。
### 6. 异步任务 (`async`)

基于优先级队列的协程池，支持重试、超时、取消。
```go

import "your-project/pkg/async"

// 获取协程池（bootstrap 注入）
pool := app.AsyncPool()

// 定义任务处理函数
handler := func(ctx context.Context, payload interface{}) (interface{}, error) {
    // 业务逻辑...
    return result, nil
}

// 创建任务
task := async.NewTask("email", handler, map[string]string{"to": "user@example.com"})
task.WithPriority(10)               // 数字越小优先级越高
task.WithRetries(3, time.Second)    // 最大重试3次，延迟1秒
task.WithTimeout(10 * time.Second)  // 超时10秒

// 提交任务
pool.Submit(task)

// 等待完成（生产环境建议异步获取结果）
task.Wait(5 * time.Second)
result, err := task.Result()
```
### 7. 文件上传 (`upload`)

统一存储接口，支持本地文件系统和阿里云 `OSS`。
```go

import "your-project/pkg/upload"

// 获取上传管理器（bootstrap 注入）
uploadManager := app.Upload()

// 上传单个文件
file, _ := c.FormFile("file")
info, _ := uploadManager.Upload(ctx, file,
    upload.WithCategory("avatar"),
    upload.WithPublic(true),
)

// 获取文件访问 URL
url, _ := uploadManager.GetURL(ctx, info.Path, true)

// 删除文件
uploadManager.Delete(ctx, info.Path)
```
配置文件示例：
```yaml

upload:
    storage_type: local          # local 或 oss
    base_path: ./uploads
    base_url: http://localhost:8080/uploads
    max_file_size: 10485760      # 10MB
    allowed_extensions: [".jpg", ".png", ".pdf"]
```
### 8. 监控 (`monitor`)

集成 `Prometheus` 指标、健康检查、`pprof` 性能分析。
```go

import "your-project/pkg/monitor"

// 获取监控实例（bootstrap 注入）
monitor := app.Monitor()

// Gin 中间件（已在 RegisterDefaultMiddleware 中注册）
engine.Use(monitor.GinMiddleware())

// 暴露指标端点（已在 RegisterMetrics 中注册）
engine.GET("/metrics", gin.WrapH(monitor.Handler()))
```
默认指标：
- http_requests_total：HTTP 请求总数（标签：method, path, status）
- http_request_duration_seconds：HTTP 请求耗时分布
- Go 运行时指标（goroutine、GC、内存等）

### 9. 队列 (`queue`)

支持 RabbitMQ 驱动，提供生产者-消费者模式，自动重连、消息确认、QoS 预取。
#### 配置示例
```yaml

queue:
    driver: rabbitmq
    rabbitmq:
    url: amqp://guest:guest@localhost:5672/
    exchange_name: order.exchange
    exchange_type: topic
    queue_name: order.queue
    routing_key: order.created
    durable: true
    prefetch_count: 10
    heartbeat: 30
```
#### 发布消息
```go

func PublishOrderHandler(app *app.App) gin.HandlerFunc {
    return func(c *gin.Context) {
        q := app.Queue()
        if q == nil {
            app.Logger().Error("queue not available")
            c.JSON(500, gin.H{"error": "queue unavailable"})
            return
        }
        body := []byte(`{"order_id": "12345"}`)
        err := q.Publish(c.Request.Context(), body,
        queue.WithExchange("order.exchange"),
        queue.WithRoutingKey("order.created"),
        queue.WithContentType("application/json"),
        queue.WithDeliveryMode(amqp.Persistent), // 持久化消息
        )
        if err != nil {
            app.Logger().Error("publish failed", zap.Error(err))
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
        c.JSON(200, gin.H{"status": "published"})
    }
}
```
#### 消费消息
```go

func startOrderConsumer(app *app.App) {
    q := app.Queue()
    if q == nil {
        return
    }
    ctx := context.Background()
    err := q.Consume(ctx, func(ctx context.Context, msg *queue.Message) error {
    app.Logger().Info("received order", zap.ByteString("body", msg.Body))
        // 业务处理...
        return nil // 返回 nil 自动 Ack，返回 error 自动 Nack 并重新入队
    }, queue.WithQueue("order.queue"), queue.WithAutoAck(false))
    if err != nil {
        app.Logger().Error("consume failed", zap.Error(err))
    }
}
```
#### 高级操作
```go

// 声明交换机（幂等）
q.DeclareExchange("my.exchange", "direct", true, false, false, false, nil)

// 声明队列（幂等）
q.DeclareQueue("my.queue", true, false, false, false, nil)

// 绑定队列到交换机
q.BindQueue("my.queue", "my.routing", "my.exchange", false, nil)

// 查看队列信息（返回名称、消息数、消费者数）
info, _ := q.QueueInspect("my.queue")
fmt.Println(info["messages"])

// 清空队列
q.QueuePurge("my.queue")
```
注意：`QueueInspect` 只能获取队列名称、消息数和消费者数，无法获取 durable、auto_delete、exclusive 等属性（AMQP 协议限制）。
### 10. 中间件 (`middleware`)

已内置以下 `Gin` 中间件，可通过 `app.RegisterDefaultMiddleware()` 批量注册：

| 中间件 | 功能 | 依赖 |
| :--- | :--- | :--- |
| Recovery | panic 恢复 | logger.Logger |
| RequestID | 生成/传递请求 ID | 无 |
| CORS | 跨域 | 配置中的 allowed_origins |
| Logger | 访问日志 | logger.Logger |
| Auth | JWT 认证 | auth.Manager |
| RateLimiter | IP 限流（内存） | 无 |
| Security | 安全头 | 无 |

单独使用示例：
```go

engine.Use(middleware.Recovery(log))
engine.Use(middleware.RequestID())
engine.Use(middleware.CORS([]string{"*"}))
engine.Use(middleware.Logger(log))
engine.Use(middleware.RateLimiter(100, time.Minute))
engine.Use(middleware.Security())
```
## 🔧 扩展与自定义
### 添加新组件
1. 在 `pkg/` 下新建包，定义接口和实现。
2. 在 `bootstrap.Init()` 中初始化该组件，并注入到 `app.App`。
3. 为 `app.App` 添加对应的 `Getter/Setter` 方法。

### 替换默认实现

例如将缓存从 `Redis` 替换为 `Memcached`，只需实现 `cache.Cache` 接口，并在 `bootstrap.go` 中条件选择即可，无需修改任何业务代码。
### 自定义错误码

在 `pkg/errors/errors.go` 中添加新的错误码：
```go

const CodeMyBizError errors.ErrorCode = 3001

err := errors.New(CodeMyBizError, "something wrong")
```
在 `app.Response` 中会根据错误码自动映射 HTTP 状态码。
## ❓ 常见问题 (FAQ)
#### Q1: Redis 连接失败会导致服务崩溃吗？

A: 不会。框架内置降级策略，自动切换为内存缓存，服务可继续运行，日志会记录警告。
#### Q2: 如何实现读写分离？

A: 在配置文件中填写 `mysql.slaves` 列表，业务代码中使用 `db.Slave()` 获取从库连接，框架自动轮询。
#### Q3: JWT 密钥长度要求？

A: 至少 32 字符，否则启动时会报错。建议使用 `openssl rand -base64 32` 生成。
#### Q4: 文件上传支持断点续传吗？

A: 当前版本支持普通上传和分片上传，但断点续传功能暂未集成，可基于 chunk_upload.go 自行扩展。
#### Q5: 如何在业务代码中获取配置？

A: 通过依赖注入：`app.Config()` 或直接将配置作为参数传递给业务层（推荐后者）。例如：
```go

func NewUserService(db *gorm.DB, cfg *config.Config) *UserService {
// ...
}
```
#### Q6: 队列组件的 `QueueInspect` 为什么没有返回 `durable` 等属性？

A: `AMQP` 协议本身不提供查询队列持久化属性的方法，这是协议限制，非框架缺陷。您可以通过 RabbitMQ Management HTTP API 获取完整信息。
#### Q7: 如何优雅关闭？

A: `bootstrap.Init()` 返回的 Closer 函数会关闭所有组件（数据库、缓存、协程池、队列连接）。在收到 SIGTERM/SIGINT 后调用 app.Shutdown() 即可。
## 📄 附录：完整配置示例
```yaml

server:
    port: 8080
    mode: release
    read_timeout: 30
    write_timeout: 30
    shutdown_timeout: 10
    enable_gzip: true

mysql:
    master:
        host: localhost
        port: 3306
        user: root
        password: 123456
        dbname: mydb
        max_open_conns: 100
        max_idle_conns: 10
    slaves:
      - host: slave1
        port: 3306
        user: root
        password: 123456
        dbname: mydb
    pool:
        max_open_conns: 100
        max_idle_conns: 10
        conn_max_lifetime: 3600
        conn_max_idle_time: 1800
    log:
        slow_threshold: 200
        enable_logging: true
        log_level: warn

redis:
    addr: localhost:6379
    password: ""
    db: 0
    pool_size: 100
    min_idle_conns: 10
    max_retries: 3
    dial_timeout: 5
    read_timeout: 3
    write_timeout: 3

queue:
    driver: rabbitmq
    rabbitmq:
        url: amqp://guest:guest@localhost:5672/
        exchange_name: order.exchange
        exchange_type: topic
        queue_name: order.queue
        routing_key: order.created
        durable: true
        prefetch_count: 10
        heartbeat: 30
        connection_timeout: 10

jwt:
    secret: your-32-character-secret-key-here
    issuer: myapp
    expire_hours: 24
    refresh_hours: 168

upload:
    storage_type: local
    base_path: ./uploads
    base_url: http://localhost:8080/uploads
    max_file_size: 10485760
    allowed_extensions:
    - .jpg
    - .png
    - .pdf
    chunk_enabled: false

async:
    max_workers: 100
    max_queue_size: 10000
    worker_idle_time: 30s
    task_retries: 3
    task_timeout: 30s

monitor:
    enable_metrics: true
    metrics_path: /metrics
    enable_health: true
    health_path: /health
    enable_profiling: false
    service_name: myapp
    service_version: 1.0.0
    environment: development

security:
    enable_cors: true
    allowed_origins:
    - "*"
    enable_rate_limit: true
    rate_limit: 100
```
## 📄 许可证

本项目基于 MIT 许可证开源，详见 LICENSE 文件。

DGou Framework — 让 Go 微服务开发更简单。✨


**文档版本：** v1.0.0

**最后更新：** 2026年2月12日