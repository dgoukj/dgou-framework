# 📘 DGou Framework

**生产级Go语言微服务脚手架**

DGou Framework是一个高性能、可扩展的Go语言微服务脚手架，基于Gin、GORM、Redis、JWT等成熟技术栈构建，为您提供开箱即用的微服务开发体验。

## 🚀 特性

### 核心功能

- **配置加载**：支持YAML、环境变量、热加载
- **结构化日志**：Zap + Lumberjack日志系统
- **读写分离数据库**：GORM + MySQL支持
- **缓存抽象**：Redis/内存，自动降级机制
- **JWT认证**：Access/Refresh Token，基于Redis吊销
- **异步任务**：协程池支持优先级队列、重试、超时
- **文件上传**：本地/阿里云OSS，统一接口
- **监控可观测性**：Prometheus、健康检查、pprof
- **统一错误处理**：带堆栈的自定义错误
- **优雅关闭**：信号监听、超时控制

### 设计哲学

✅ **无全局变量**：所有组件通过依赖注入传递

✅ **接口驱动**：缓存、存储、认证等均抽象为接口，易于替换和测试

✅ **业务与框架分离**：框架不包含任何业务逻辑，业务代码放在`internal/`中

✅ **开箱即用**：bootstrap包一键初始化所有组件

## 📁 目录结构

```
your-project/
├── cmd/
│   └── server/
│       └── main.go           # 业务入口，仅需几行代码
├── internal/                 # 私有业务代码（不对外导出）
│   ├── models/              # GORM数据模型
│   ├── services/            # 业务逻辑
│   ├── handlers/            # HTTP处理器
│   └── ...                  
├── pkg/                     # 可复用的框架组件
│   ├── app/                # 应用生命周期管理
│   ├── bootstrap/          # 一键初始化（新增）
│   ├── config/             # 配置加载
│   ├── logger/             # 日志封装
│   ├── errors/             # 自定义错误
│   ├── database/           # 读写分离数据库
│   ├── cache/              # 缓存接口及实现（Redis/内存）
│   ├── auth/               # JWT认证
│   ├── async/              # 异步协程池
│   ├── upload/             # 文件上传（本地/OSS）
│   ├── monitor/            # 监控（Prometheus/pprof）
│   └── middleware/         # Gin中间件
├── config/                  # 配置文件目录
│   └── config.yaml         # 示例配置
├── go.mod
└── go.sum
```

## 🚀 快速开始

### 3.1 创建新项目

```
# 克隆脚手架模板（假设已托管）
git clone https://github.com/your/dgou-framework.git my-service
cd my-service
go mod edit -module your-project-name
```

### 3.2 编写配置文件

在`config/config.yaml`中填写配置（完整示例见附录）：

```
server:
  port: 8080
  mode: debug
mysql:
  master:
    host: localhost
    port: 3306
    user: root
    password: 123456
    dbname: test
redis:
  addr: localhost:6379
jwt:
  secret: your-32-character-secret-key
upload:
  storage_type: local
  base_path: ./uploads
async:
  max_workers: 10
monitor:
  enable_metrics: true
```

### 3.3 编写业务代码

在`internal/`下创建业务模型、服务、处理器。
在`cmd/server/main.go`中注册路由：

```
package main

import (
"context"
"log"

"your-project/internal/handlers"
"your-project/pkg/bootstrap"
"your-project/pkg/middleware"
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
authMiddleware := middleware.Auth(app.Auth())

// 示例：登录
engine.POST("/login", handlers.Login(app.Auth(), app.Logger()))

// 示例：需要认证的接口
api := engine.Group("/api")
api.Use(authMiddleware)
{
api.GET("/user/:id", handlers.GetUser(app.DB()))
}

// 3. 启动服务
app.Run()
}
```

### 3.4 运行服务

```
go run cmd/server/main.go
```

访问`http://localhost:8080/health`验证服务正常。

## ⚙️ 配置详解

所有配置均通过Viper加载，支持默认值、YAML文件、环境变量（前缀APP_）。

### 常用配置项

| 模块 | 字段 | 说明 | 默认值 |
| ------ |------ |------ |------ |
| server | port | HTTP监听端口 | 8080 |
| server | mode | gin模式（debug/release） | release |
| mysql.master | host | 主库地址 | localhost |
| redis | addr | Redis地址 | localhost:6379 |
| redis | password | Redis密码 | "" |
| jwt | secret | JWT签名密钥（≥32字符） | 无默认，必须配置 |
| jwt | expire_hours | Access Token有效期（小时） | 24 |
| jwt | refresh_hours | Refresh Token有效期（小时） | 168 |
| upload | storage_type | 存储类型（local/oss） | local |
| upload | base_path | 本地存储根目录 | ./uploads |
| async | max_workers | 协程池最大工作协程数 | 100 |
| monitor | enable_metrics | 是否启用Prometheus | true |
| monitor | metrics_path | 指标路径 | /metrics |

**环境变量覆盖**：例如`APP_SERVER_PORT=9090`会覆盖`server.port`。

## 🧩 组件使用指南

### 5.1 配置 (config)

```
import "your-project/pkg/config"

cfg, _ := config.Load() // 默认查找 ./config/config.yaml
cfg, _ := config.Load("/path/to/config.yaml")
```

### 5.2 日志 (logger)

```
import "your-project/pkg/logger"

log, _ := logger.New(logger.Config{Level: "info"})
log.Info("hello", zap.String("key", "value"))
log.Sugar().Infof("format %s", "world")
```

在handler中可通过`app.Logger()`获取。

### 5.3 数据库 (database)

```
import "your-project/pkg/database"

dbCfg := database.Config{...}
db, _ := database.New(dbCfg)
defer db.Close()

// 读写分离
master := db.Master()
slave  := db.Slave()
```

模型定义：继承`database.BaseModel`即可自动拥有ID、创建时间、更新时间、软删除。

### 5.4 缓存 (cache)

缓存接口`cache.Cache`支持Redis和内存两种实现，优先使用Redis，失败自动降级内存。

```
import "your-project/pkg/cache"

// 获取实例（由bootstrap注入）
cache := app.Cache()

// 基础操作
cache.Set(ctx, "key", "value", time.Hour)
val, _ := cache.Get(ctx, "key")
cache.Delete(ctx, "key")

// 自增
cache.Incr(ctx, "counter", 1)

// 分布式锁
token, _ := cache.Lock(ctx, "resource", 10*time.Second)
defer cache.Unlock(ctx, "resource", token)
```

### 5.5 认证 (auth)

JWT认证管理器`auth.Manager`依赖`cache.Cache`存储Refresh Token。

```
import "your-project/pkg/auth"

authManager := auth.NewManager(cache, auth.Config{
Secret:     "xxx",
Issuer:     "myapp",
AccessTTL:  time.Hour,
RefreshTTL: 7 * 24 * time.Hour,
})

// 生成令牌
access, refresh, _ := authManager.Generate(1, "alice", []string{"admin"})

// 解析令牌
claims, _ := authManager.Parse(access)
fmt.Println(claims.UserID, claims.Username)

// 刷新
newAccess, newRefresh, _ := authManager.Refresh(refresh)

// 吊销
authManager.Revoke(tokenID)
```

中间件：`middleware.Auth(authManager)`用于Gin路由认证。

### 5.6 异步任务 (async)

基于优先级队列的协程池，支持重试、超时、取消。

```
import "your-project/pkg/async"

pool := async.NewPool("worker", 10, 100)
pool.Start()
defer pool.Stop()

task := async.NewTask("email", func(ctx context.Context, payload interface{}) (interface{}, error) {
// 业务逻辑
return nil, nil
}, map[string]string{"to": "user@example.com"})
task.WithTimeout(5 * time.Second).WithRetries(3, time.Second)

pool.Submit(task)
task.Wait(10 * time.Second)
result, err := task.Result()
```

### 5.7 文件上传 (upload)

统一存储接口`upload.Storage`，实现local和oss。

```
import "your-project/pkg/upload"

// 由bootstrap初始化好的Manager
uploadManager := app.Upload()

// 上传
file, _ := c.FormFile("file")
info, _ := uploadManager.Upload(ctx, file,
upload.WithCategory("avatar"),
upload.WithPublic(true),
)

// 获取文件URL
url, _ := uploadManager.GetURL(ctx, info.Path, true)
```

### 5.8 监控 (monitor)

Prometheus指标 + 健康检查 + pprof。

```
import "your-project/pkg/monitor"

monitor := monitor.New(monitor.Config{
ServiceName: "myapp",
EnableMetrics: true,
MetricsPath: "/metrics",
})

// 在Gin中使用中间件
engine.Use(monitor.GinMiddleware())

// 注册健康检查路由
engine.GET("/health", gin.WrapH(monitor.HealthHandler()))
```

**指标说明**：

- `http_requests_total`：HTTP请求总数（标签：method, path, status）
- `http_request_duration_seconds`：HTTP请求耗时分布

### 5.9 队列组件 (queue)

支持 RabbitMQ 驱动，提供生产者-消费者模式，自动重连、消息确认、QoS 预取等生产级特性。
#### 配置示例 (`config.yaml`)
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
        connection_timeout: 10
```
#### 初始化

框架已通过 `bootstrap.Init()` 自动初始化，可通过 app.Queue() 获取实例。
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

在服务启动时启动消费者 goroutine：
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
- 声明交换机/队列：DeclareExchange, DeclareQueue
- 绑定/解绑：BindQueue, UnbindQueue
- 队列管理：QueuePurge, QueueInspect, DeleteQueue, DeleteExchange

所有操作均为幂等设计，可重复调用。

### 5.10 中间件 (middleware)

已内置常用中间件：

| 中间件 | 功能 | 依赖 |
| ------ |------ |------ |
| Recovery | panic恢复 | logger |
| RequestID | 生成/传递请求ID | 无 |
| Logger | 访问日志 | logger |
| CORS | 跨域 | 无 |
| Auth | JWT认证 | auth.Manager |
| RateLimiter | IP限流 | 无（内存） |

使用示例：

```
engine.Use(middleware.Recovery(log))
engine.Use(middleware.RequestID())
engine.Use(middleware.CORS([]string{"*"}))
engine.Use(middleware.Logger(log))
engine.Use(middleware.RateLimiter(100, time.Minute))
```

## 🔧 扩展与自定义

### 6.1 添加新组件

1. 在`pkg/`下新建包，定义接口和实现
2. 在`bootstrap.Init()`中初始化该组件，并注入到`app.App`
3. 为`app.App`添加Getter方法

### 6.2 替换默认实现

例如将缓存从Redis替换为Memcached，只需实现`cache.Cache`接口，并在bootstrap中条件选择即可，无需修改其他业务代码。

### 6.3 自定义错误码

在`pkg/errors/errors.go`中添加新的错误码，并在业务中使用：

```
const CodeMyBizError errors.ErrorCode = 3001

err := errors.New(CodeMyBizError, "something wrong")
```

在`app.Response`中会自动转换HTTP状态码。

## ❓ 常见问题 (FAQ)

### Q1: 如何修改默认配置？

**A**: 直接修改`config/config.yaml`，或通过环境变量覆盖。也可以在`bootstrap.Init()`前调用`config.Load("/path")`加载自定义配置文件。

### Q2: Redis不可用时为什么不会panic？

**A**: 框架内置降级策略：Redis连接失败自动切换为内存缓存，不影响服务。可通过日志确认降级状态。

### Q3: 如何实现读写分离？

**A**: 在配置文件中填写`mysql.slaves`列表，业务代码中使用`db.Slave()`获取从库连接，框架自动轮询。

### Q4: JWT密钥长度要求？

**A**: 至少32字符，否则启动时会报错。建议使用`openssl rand -base64 32`生成。

### Q5: 文件上传支持哪些存储？

**A**: 本地文件系统（local）和阿里云OSS。如需支持S3、MinIO等，可扩展`upload.Storage`接口。

### Q6: 如何优雅关闭？

**A**: `bootstrap.Init()`返回的Closer函数会关闭所有组件（数据库、缓存、协程池等）。在收到SIGTERM/SIGINT后调用`app.Shutdown()`即可。

### Q7: 如何在业务代码中获取配置？

**A**: 通过依赖注入：`app.Config()`或直接将配置作为参数传递给业务层。

## 📦 附录：完整配置示例

```
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

jwt:
secret: your-32-character-secret-key
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

---

**DGou Framework** - 让Go微服务开发更简单 ✨

