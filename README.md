# Dgou Framework - 生产级 Go Gin 脚手架
## 项目简介

Dgou Framework 是一个基于 Gin 构建的高性能、高安全性的 Go 语言 Web 开发脚手架。它集成了现代 Web 开发所需的各种组件，旨在为 Go 开发者提供一个快速、可靠的生产级开发起点。
## 核心特性
- 🚀 高性能：基于 Gin 高性能 HTTP 框架，支持连接池、异步处理
- 🛡️ 高安全性：内置 JWT 认证、CSRF 防护、XSS 过滤、速率限制等安全特性
- 📊 完善监控：集成 Prometheus 指标收集、健康检查、分布式追踪
- 🔧 组件化设计：所有组件模块化，可按需使用
- 📦 生产就绪：包含优雅关闭、配置热重载、缓存降级等生产环境特性
- 🧩 易于扩展：清晰的架构设计，便于业务扩展

## 项目结构
```text

dgou-framework/
├── cmd/                   # 应用入口
│   └── server/
│       └── main.go
├── config/                # 配置文件
│   └── config.yaml
├── pkg/                   # 核心组件库
│   ├── app/               # 应用核心
│   ├── auth/              # 认证授权
│   ├── cache/             # 缓存组件
│   ├── config/            # 配置管理
│   ├── database/          # 数据库组件
│   ├── errors/            # 错误处理
│   ├── logger/            # 日志组件
│   ├── middleware/        # 中间件
│   ├── monitor/           # 监控组件
│   ├── queue/             # 消息队列
│   ├── response/          # 统一响应
│   ├── task/              # 定时任务
│   ├── upload/            # 文件上传
│   └── util/              # 工具函数
└── go.mod
```
## 快速开始
### 1. 安装依赖
```bash

# 克隆项目
git clone https://github.com/dgoukj/dgou-framework.git
cd dgou-framework

# 安装依赖
go mod tidy
```
### 2. 配置文件

创建配置文件 `config/config.yaml`：
```yaml

server:
    port: 8080
    mode: release
    enable_gzip: true

mysql:
    host: localhost
    port: 3306
    user: root
    password: password
    dbname: myapp

redis:
    addr: localhost:6379
    password: ""
    db: 0

log:
    level: info
    file: ./logs/app.log

jwt:
    secret: "your-secret-key-at-least-32-chars-long"
    issuer: "myapp"
    expire_hours: 24
```
### 3. 创建应用
```go

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
    
    // 添加业务路由（示例）
    application.AddRouter(&UserRouter{})
    
    // 运行应用
    if err := application.Run(); err != nil {
        log.Fatalf("Failed to run application: %v", err)
    }
}
```
### 4. 启动服务
```bash

go run cmd/server/main.go
```
访问 http://localhost:8080/health 查看健康状态。

## 核心组件
### 1. 配置组件 (pkg/config)

配置组件提供灵活、安全的配置管理，支持多种配置源和环境变量。
#### 主要特性
- ✅ 支持 YAML、JSON 配置文件
- ✅ 环境变量覆盖
- ✅ 配置热重载
- ✅ 配置验证
- ✅ 线程安全单例模式

#### 使用方法

#### 基础用法
```go

import "dgou/pkg/config"

func main() {
    // 加载配置
    cfg := config.LoadConfig("./config/config.yaml")

    // 获取配置值
    port := cfg.Server.Port
    dbHost := cfg.MySQL.Host
    
    // 访问配置
    fmt.Printf("Server running on port: %d\n", port)
}
```
#### 使用环境变量
```bash

# 设置环境变量（优先级高于配置文件）
export APP_SERVER_PORT=9090
export APP_MYSQL_HOST=mysql-prod.example.com
```
#### 配置验证

配置加载时会自动验证：
- 必填字段检查
- JWT Secret 长度验证
- 证书文件存在性检查
- 端口范围验证

#### 配置热重载

当配置文件发生变化时，配置会自动重新加载，无需重启服务：
```go

// 监听配置变化
config.OnConfigChange(func(old, new *config.Config) {
    log.Printf("Configuration changed, new port: %d", new.Server.Port)
})
```
#### 配置结构
```go

type Config struct {
    Server   ServerConfig   // 服务器配置
    MySQL    MySQLConfig    // MySQL配置
    Redis    RedisConfig    // Redis配置
    JWT      JWTConfig      // JWT配置
    Log      LogConfig      // 日志配置
    Monitor  MonitorConfig  // 监控配置
    Security SecurityConfig // 安全配置
    // ... 更多配置
}
```
### 2. 日志组件 (pkg/logger)

日志组件基于 Zap 实现，提供高性能的结构化日志记录。
#### 主要特性
- ✅ 结构化 JSON 日志
- ✅ 日志轮转（按大小和时间）
- ✅ 上下文日志（请求ID、用户ID）
- ✅ 性能监控日志
- ✅ 多级别日志控制

#### 使用方法

#### 初始化日志
```go

import "dgou/pkg/config"
import "dgou/pkg/logger"

func main() {
cfg := config.LoadConfig()

    // 初始化日志组件
    logger.InitLogger(&cfg.Log)
    
    // 确保日志缓冲区刷新
    defer logger.Sync()
}
```
#### 基础日志记录
```go

// 不同级别日志
logger.Info("Application started",
    logger.String("version", "1.0.0"),
    logger.Int("port", 8080),
)

logger.Error("Database connection failed",
    logger.String("host", "localhost"),
    logger.Int("port", 3306),
    logger.ErrorField(err),
)

logger.Debug("Processing request",
    logger.String("method", "GET"),
    logger.String("path", "/api/users"),
)
```
#### 上下文日志
```go

import "context"

func handler(ctx context.Context) {
    // 创建带有请求上下文的日志
    logger.CtxInfo(ctx, "Processing request",
        logger.String("action", "user_login"),
        logger.Duration("elapsed", time.Second),
    )

    logger.CtxError(ctx, "Failed to process",
        logger.String("reason", "invalid input"),
        logger.Any("data", requestData),
    )
}
```
#### 性能监控
```go

func expensiveOperation() {
    defer logger.TimeTrack(context.Background(), time.Now(), "expensiveOperation")

    // 执行耗时操作
    time.Sleep(100 * time.Millisecond)
}

// 记录内存使用情况
logger.MemoryUsage(context.Background())
```
#### Sugar Logger（简化语法）
```go

// 获取 SugaredLogger
sugar := logger.Sugar()

// 格式化日志
sugar.Infof("User %s logged in from %s", username, ip)
sugar.Errorf("Failed to process request: %v", err)
sugar.Warnf("High memory usage: %.2f%%", memoryPercent)
```
#### 日志配置示例
```yaml

log:
    level: info           # 日志级别: debug, info, warn, error
    file: ./logs/app.log  # 日志文件路径
    max_size: 100         # 单个日志文件最大大小(MB)
    max_backups: 3        # 保留的旧日志文件数量
    max_age: 7            # 保留天数
    compress: true        # 是否压缩旧日志
    add_caller: true      # 是否添加调用者信息
```
### 3. 错误处理组件 (pkg/errors)

统一的错误处理机制，支持错误码、HTTP 状态码映射和错误堆栈。
#### 主要特性
- ✅ 自定义错误码体系
- ✅ HTTP 状态码自动映射
- ✅ 错误堆栈追踪
- ✅ 错误包装和解包
- ✅ 错误详情支持

#### 使用方法

#### 创建错误
```go

import "dgou/pkg/errors"

// 创建通用错误
err := errors.New(errors.CodeValidationFailed, "Invalid input data")

// 创建 HTTP 错误
err := errors.BadRequest("Invalid request parameters")
err := errors.Unauthorized("Authentication required")
err := errors.NotFound("Resource not found")
err := errors.InternalServerError("Something went wrong")

// 创建业务错误
err := errors.UserExists("john_doe")
err := errors.UserNotFound("john_doe")
err := errors.InvalidPassword()
err := errors.TokenExpired()
```
#### 包装错误
```go

func processFile(filename string) error {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        // 包装原始错误
        return errors.Wrap(err, errors.CodeInternalError,
        "Failed to read file")
    }

    // 格式化包装
    if len(data) == 0 {
        return errors.Wrapf(nil, errors.CodeValidationFailed,
            "File %s is empty", filename)
    }
    
    return nil
}
```
#### 添加错误详情
```go

err := errors.BadRequest("Validation failed").
WithDetails(
    "Email format is invalid",
    "Password must be at least 8 characters",
    "Username already taken",
).
WithOp("user.Register")
```
#### 错误处理
```go

func handleError(c *gin.Context) {
    err := someOperation()

    if err != nil {
        // 检查错误类型
        if errors.Is(err, errors.CodeUnauthorized) {
            // 处理认证错误
        }
        
        if errors.Is(err, errors.CodeResourceNotFound) {
            // 处理资源不存在错误
        }
        
        // 转换为自定义错误
        var customErr *errors.Error
        if errors.As(err, &customErr) {
            // 使用 customErr.Code, customErr.Message
        }
    }
}
```
#### 错误响应
```go

import "dgou/pkg/response"

func handler(c *gin.Context) {
    err := processRequest(c)
    if err != nil {
        // 自动处理错误响应
        response.Error(c, err)
        return
    }

    response.Success(c, result)
}
```
#### 错误码定义
```go

const (
    // 通用错误码
    CodeUnknown          ErrorCode = 1000
    CodeValidationFailed ErrorCode = 1001
    CodeResourceNotFound ErrorCode = 1002
    CodeUnauthorized     ErrorCode = 1003
    CodeForbidden        ErrorCode = 1004
    CodeTooManyRequests  ErrorCode = 1005
    CodeInternalError    ErrorCode = 1006

    // 业务错误码（可扩展）
    CodeUserExists       ErrorCode = 2001
    CodeUserNotFound     ErrorCode = 2002
    CodeInvalidPassword  ErrorCode = 2003
    CodeTokenExpired     ErrorCode = 2004
    CodeTokenInvalid     ErrorCode = 2005
)
```
### 4. 响应组件 (pkg/response)

统一 API 响应格式，支持成功响应、错误响应和分页响应。
#### 主要特性
- ✅ 统一响应格式
- ✅ 分页支持
- ✅ 自动错误处理
- ✅ 请求ID跟踪
- ✅ 文件下载支持

#### 使用方法

#### 成功响应
```go

import "dgou/pkg/response"

func getUser(c *gin.Context) {
    user := getUserFromDB()

    // 基础成功响应
    response.Success(c, user)
    
    // 或者使用 map
    response.Success(c, gin.H{
        "user": user,
        "meta": "additional info",
    })
}
```
#### 分页响应
```go

func listUsers(c *gin.Context) {
    page := 1
    pageSize := 20
    users, total := getUsersWithPagination(page, pageSize)

    response.SuccessWithPagination(c, users, page, pageSize, total)
}
```
#### 错误响应
```go

func createUser(c *gin.Context) {
    var user User
    if err := c.ShouldBindJSON(&user); err != nil {
    // 自动处理绑定错误
    response.BadRequest(c, "Invalid request data")
        return
    }

    // 业务错误
    if exists := checkUserExists(user.Email); exists {
        response.BadRequest(c, "User already exists")
        return
    }
    
    // 使用自定义错误
    err := saveUser(user)
    if err != nil {
        response.Error(c, err)  // 自动处理错误响应
        return
    }
    
    response.Success(c, gin.H{"message": "User created"})
}
```
#### HTTP 错误快捷方法
```go

// 各种HTTP状态错误
response.BadRequest(c, "Invalid parameters")
response.Unauthorized(c, "Authentication required")
response.Forbidden(c, "Insufficient permissions")
response.NotFound(c, "Resource not found")
response.TooManyRequests(c, "Rate limit exceeded")
response.InternalServerError(c, "Internal server error")

// 验证错误
response.ValidationError(c, "email", "Invalid email format")
```
#### 文件响应
```go

// 文件下载
response.File(c, "/path/to/file.pdf", "document.pdf")

// JSON文件下载
data := map[string]interface{}{"key": "value"}
response.JSONFile(c, data, "data.json")

// CSV文件下载
csvContent := "name,age,email\nJohn,30,john@example.com"
response.CSVFile(c, csvContent, "users.csv")
```
#### 响应格式

#### 成功响应格式
```json

{
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "code": 200,
    "message": "success",
    "data": {
        "id": 1,
        "name": "John Doe",
        "email": "john@example.com"
    },
    "timestamp": 1625097600
}
```
#### 分页响应格式
```json

{
    "code": 200,
    "message": "success",
    "data": {
        "list": [...],
        "pagination": {
            "page": 1,
            "page_size": 20,
            "total": 150,
            "total_page": 8
        }
    }
}
```
#### 错误响应格式
```json

{
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "code": 400,
    "message": "Invalid email format",
    "timestamp": 1625097600
}
```
#### 开发环境错误详情
```json

{
    "code": 500,
    "message": "Database connection failed",
    "data": {
    "details": ["Connection timeout"],
      "stack": ["app.go:45", "handler.go:23", "main.go:17"]
    }
}
```
### 5. 应用核心组件 (pkg/app)

应用核心组件是整个框架的入口，负责协调所有组件并管理应用生命周期。
#### 主要特性
- ✅ 应用生命周期管理
- ✅ 优雅关闭支持
- ✅ 健康检查集成
- ✅ 监控路由自动注册
- ✅ 中间件链配置

#### 使用方法

#### 创建应用
```go

import (
    "dgou/pkg/app"
    "dgou/pkg/config"
    "log"
)

func main() {
    // 1. 加载配置
    cfg := config.LoadConfig("./config/config.yaml")

    // 2. 创建应用实例
    application := app.NewApp(cfg)
    
    // 3. 初始化应用
    if err := application.Initialize(); err != nil {
        log.Fatalf("Failed to initialize application: %v", err)
    }
    
    // 4. 注册业务路由
    application.AddRouter(&UserRouter{})
    application.AddRouter(&ProductRouter{})
    application.AddRouter(&OrderRouter{})
    
    // 5. 运行应用
    if err := application.Run(); err != nil {
        log.Fatalf("Failed to run application: %v", err)
    }
}
```
#### 自定义路由注册器
```go

// 实现 app.Router 接口
type UserRouter struct{}

func (r *UserRouter) Register(router *gin.RouterGroup) {
    // 用户相关路由
    userGroup := router.Group("/users")
    {
        userGroup.GET("", r.listUsers)
        userGroup.POST("", r.createUser)
        userGroup.GET("/:id", r.getUser)
        userGroup.PUT("/:id", r.updateUser)
        userGroup.DELETE("/:id", r.deleteUser)
    }
}

func (r *UserRouter) Priority() int {
    return 10 // 优先级，数字越小越先注册
}

// 路由处理方法
func (r *UserRouter) listUsers(c *gin.Context) {
    // 处理逻辑
    response.Success(c, []User{})
}
```
#### 自定义监控器
```go

// 实现 app.Monitor 接口
type CustomMonitor struct{}

func (m *CustomMonitor) Start() error {
    // 启动监控
    return nil
}

func (m *CustomMonitor) Stop() error {
    // 停止监控
    return nil
}

func (m *CustomMonitor) Name() string {
    return "custom_monitor"
}

// 注册监控器
application.AddMonitor(&CustomMonitor{})
```
#### 获取应用组件
```go

// 获取健康检查器
healthChecker := application.GetHealthChecker()

// 注册自定义健康检查
healthChecker.Register(&CustomHealthCheck{})

// 获取Gin引擎（高级配置）
engine := application.GetEngine()
engine.MaxMultipartMemory = 32 << 20 // 32 MB
```
#### 优雅关闭处理器

应用内置了优雅关闭处理器，支持按优先级执行关闭任务：
```go

// 获取关闭处理器
shutdownHandler := application.GetShutdownHandler()

// 注册关闭任务
shutdownHandler.Register(func() {
// 清理资源
cleanup()
}, app.PriorityHigh, "cleanup")

shutdownHandler.RegisterWithDefault(func() {
// 默认优先级任务
flushLogs()
}, "flush_logs")
```
#### 应用配置

#### 服务器配置
```yaml

server:
    port: 8080                    # 服务端口
    mode: release                 # 运行模式: debug, release, test
    read_timeout: 30              # 读取超时(秒)
    write_timeout: 30             # 写入超时(秒)
    shutdown_timeout: 10          # 关闭超时(秒)
    enable_https: false           # 启用HTTPS
    cert_file: ./cert.pem         # SSL证书文件
    key_file: ./key.pem           # SSL密钥文件
    enable_gzip: true             # 启用Gzip压缩
    max_request_body: 10485760    # 最大请求体大小(10MB)
```
#### 监控配置
```yaml

monitor:
    enable_metrics: true          # 启用指标收集
    metrics_path: /metrics        # 指标端点路径
    enable_health: true           # 启用健康检查
    health_path: /health          # 健康检查端点路径
    enable_profiling: false       # 启用性能分析(仅开发环境)
    profile_path: /debug/pprof    # 性能分析端点路径
```
### 5. 数据库组件 (pkg/database)
### 特性
- ✅ GORM v2 集成
- ✅ MySQL8 完整支持
- ✅ 读写分离与负载均衡
- ✅ 连接池优化配置
- ✅ 慢查询日志监控
- ✅ 事务管理与回滚
- ✅ 数据迁移支持
- ✅ 健康检查与自动重连

### 快速开始

#### 1. 基本配置

```yaml
# config/config.yaml
             mysql:
               master:
                 host: localhost
                 port: 3306
                 user: root
                 password: password123
                 dbname: myapp
                 charset: utf8mb4
                 parse_time: true
                 loc: Local
               slaves:
                 - host: slave1.localhost
                   port: 3306
                   user: root
                   password: password123
                   dbname: myapp
               pool:
                 max_open_conns: 200
                 max_idle_conns: 50
               log:
                 slow_threshold: 200
                 enable_logging: true
                 log_level: warn

  2. 初始化数据库
  go

  import (
  "dgou/pkg/config"
  "dgou/pkg/database"
  )

  func main() {
  // 加载配置
  cfg := config.LoadConfig()

  // 初始化数据库
  db, err := database.InitDB(cfg)
  if err != nil {
  log.Fatal(err)
}
  defer db.Close()

  // 获取主库实例（写操作）
  master := db.GetMaster()

  // 获取从库实例（读操作）
  slave := db.GetSlave()
}

  3. 定义模型
  go

  package models

  import (
  "dgou/pkg/database"
  "time"
  )

  type User struct {
  database.BaseModel
  Username string `gorm:"size:50;uniqueIndex;not null"`
  Email    string `gorm:"size:100;uniqueIndex;not null"`
  Password string `gorm:"size:255;not null"`
  Status   int    `gorm:"default:1"`
}

  type UserProfile struct {
  database.BaseModel
  UserID   uint   `gorm:"uniqueIndex;not null"`
  RealName string `gorm:"size:50"`
  Avatar   string `gorm:"size:255"`
  Phone    string `gorm:"size:20;index"`
}

  4. 数据库操作示例
  go

  // 查询操作
  func GetUserByID(id uint) (*User, error) {
  var user User
  // 使用从库进行读操作
  err := database.Slave().First(&user, id).Error
  if err != nil {
  return nil, err
}
  return &user, nil
}

  // 创建操作
  func CreateUser(user *User) error {
  // 使用主库进行写操作
  return database.Master().Create(user).Error
}

  // 更新操作
  func UpdateUser(user *User) error {
  return database.Master().Save(user).Error
}

  // 事务操作
  func TransferBalance(fromID, toID uint, amount float64) error {
  return database.Transaction(func(tx *gorm.DB) error {
  // 扣减转出账户余额
  if err := tx.Model(&Account{}).
  Where("id = ? AND balance >= ?", fromID, amount).
  Update("balance", gorm.Expr("balance - ?", amount)).
  Error; err != nil {
  return err
  }

  // 增加转入账户余额
  if err := tx.Model(&Account{}).
  Where("id = ?", toID).
  Update("balance", gorm.Expr("balance + ?", amount)).
  Error; err != nil {
  return err
  }

  return nil
})
}

  // 分页查询
  func ListUsers(page, pageSize int) ([]User, *database.PaginationResult, error) {
  var users []User
  pagination := &database.Pagination{
             Page:     page,
             PageSize: pageSize,
}

  db := database.Slave().Model(&User{}).Order("created_at DESC")
  result, err := database.Paginate(db, pagination, &users)

  return users, result, err
}

  5. 数据迁移
  bash

  # 创建新的迁移
  go run cmd/db/main.go --command create --name add_user_role

  # 执行迁移
  go run cmd/db/main.go --command migrate

  # 查看迁移状态
  go run cmd/db/main.go --command status

  # 回滚迁移
  go run cmd/db/main.go --command rollback --steps 1

  6. 监控与统计
  go

  // 获取数据库连接池统计
  stats := db.GetStats()
             fmt.Printf("Connection pool stats: %+v\n", stats)

  // 检查数据库连接状态
  if db.IsConnected() {
  fmt.Println("Database is connected")
}

  // 执行慢查询分析
  slowQueries := db.FindSlowQueries(5 * time.Second)

  高级特性
  读写分离
  go

  // 自动读写分离
  func GetUserWithBalance(userID uint) (*User, error) {
  var user User

  // 读操作自动使用从库
  if err := database.Slave().
  Preload("Accounts").
  First(&user, userID).
  Error; err != nil {
  return nil, err
  }

  return &user, nil
}

  // 强制使用主库读取（用于数据一致性要求高的场景）
  func GetUserForUpdate(userID uint) (*User, error) {
  var user User

  // 使用主库并锁定记录
  if err := database.Master().
             Clauses(clause.Locking{Strength: "UPDATE"}).
  First(&user, userID).
  Error; err != nil {
  return nil, err
}

  return &user, nil
}

  连接池优化
  go

  // 获取连接池统计信息
  sqlDB, _ := db.GetMaster().DB()
  stats := sqlDB.Stats()

  fmt.Printf("连接池统计:\n")
             fmt.Printf("  最大连接数: %d\n", stats.MaxOpenConnections)
             fmt.Printf("  打开连接数: %d\n", stats.OpenConnections)
             fmt.Printf("  使用中连接: %d\n", stats.InUse)
             fmt.Printf("  空闲连接: %d\n", stats.Idle)
             fmt.Printf("  等待次数: %d\n", stats.WaitCount)

  慢查询监控
  go

  // 启用慢查询日志
             // 在配置文件中设置 slow_threshold: 200 (200毫秒)

  // 自定义慢查询处理
  db.GetMaster().Callback().Query().After("gorm:query").
  Register("slow_query_warning", func(db *gorm.DB) {
  if db.Statement.Context != nil {
  elapsed := time.Since(db.Statement.StartTime)
  if elapsed > 200*time.Millisecond {
  logger.CtxWarn(db.Statement.Context, "Slow query detected",
  logger.Duration("elapsed", elapsed),
  logger.String("sql", db.Statement.SQL.String()),
  )
  }
  }
  })

  最佳实践

  始终使用事务处理关联操作

  读写分离提高性能

  合理配置连接池参数

  监控慢查询并及时优化

  定期执行数据迁移维护

  使用软删除保持数据完整性

  启用查询缓存减少数据库压力

  故障排除
  常见问题

  连接池耗尽
  go

  // 增加连接池大小
             max_open_conns: 200
             max_idle_conns: 50

  慢查询优化
  sql

  -- 添加合适的索引
  CREATE INDEX idx_user_email ON users(email);

  -- 优化查询语句
  EXPLAIN SELECT * FROM users WHERE email = 'test@example.com';

  事务死锁
  go

  // 设置事务超时
  ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  defer cancel()

  err := database.TransactionWithContext(ctx, func(tx *gorm.DB) error {
  // 事务操作
})
```
### 7. 缓存组件 (pkg/cache)
#### 特性
 - ✅ Redis客户端封装（支持连接池、健康检查）
 - ✅ 内存缓存降级（Redis不可用时自动降级）
 - ✅ 缓存穿透防护（布隆过滤器、空值缓存）
 - ✅ 缓存击穿防护（分布式锁、双检查锁）
 - ✅ 缓存雪崩防护（随机过期时间、熔断器）
 - ✅ 分布式锁实现（支持重入、自动续期）
 - ✅ 缓存一致性策略（写穿透、写回、双删）
 - ✅ 多种数据结构支持（字符串、哈希、集合、列表）
 - ✅ 发布订阅功能
 - ✅ 详细统计和监控

#### 快速开始

#### 7.1 基本配置
```yaml
# config/config.yaml
         cache:
           type: redis
           prefix: app
           default_ttl: 3600
           enable_stats: true
           max_memory_mb: 100

         redis:
           addr: localhost:6379
           password: password123
           db: 0
```
#### 7.2 初始化缓存
```go

import (
    "dgou/pkg/cache"
    "dgou/pkg/config"
)

func main() {
    // 加载配置
    cfg := config.LoadConfig()
    
    // 初始化缓存
    cacheManager, err := cache.InitCache(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer cacheManager.Close()
    
    // 使用快捷方法
    ctx := context.Background()
    cache.Set(ctx, "user:1", "John Doe", 30*time.Minute)
    value, _ := cache.Get(ctx, "user:1")
    fmt.Println("User:", value)
}
```
#### 7.3 基础操作示例
```go

// 设置缓存
err := cache.Set(ctx, "user:1", map[string]interface{}{
         "id":   1,
         "name": "John Doe",
         "email": "john@example.com",
}, 30*time.Minute)

// 获取缓存
value, err := cache.Get(ctx, "user:1")
    if err != nil {
    // 处理错误
}

// 删除缓存
err = cache.Delete(ctx, "user:1")

// 批量操作
values := map[string]interface{}{
         "user:1": "John",
         "user:2": "Jane",
}
err = cache.MSet(ctx, values, 30*time.Minute)

result, err := cache.MGet(ctx, []string{"user:1", "user:2"})
```
#### 7.4 高级操作示例
```go

// 获取或设置（防缓存击穿）
user, err := cache.GetOrSet(ctx, "user:1", func() (interface{}, error) {
    // 从数据库加载用户
    return db.GetUser(1)
}, 30*time.Minute)

// 使用布隆过滤器防缓存穿透
exists, _ := cache.BloomExists(ctx, "users", "user:1")
if !exists {
    return nil, errors.New("User not found")
}

// 分布式锁
lockKey := "lock:update:user:1"
token, err := cache.Lock(ctx, lockKey, 10*time.Second)
if err != nil {
    return err
}
defer cache.Unlock(ctx, lockKey, token)

// 执行需要锁保护的操作
```
5. 缓存一致性策略
go

// 写穿透（Write-Through）
err = cache.WriteThrough(ctx, "user:1", userData, func() error {
// 先写数据库
return db.UpdateUser(1, userData)
}, 30*time.Minute)

// 读穿透（Read-Through）
user, err := cache.ReadThrough(ctx, "user:1", func() (interface{}, error) {
// 从数据库加载
return db.GetUser(1)
}, 30*time.Minute)

// 双删策略（Double Delete）
err = cache.DoubleDelete(ctx, "user:1", func() error {
// 更新数据库
return db.UpdateUser(1, userData)
}, 1*time.Second)

// 缓存旁路（Cache-Aside）
user, err := cache.CacheAside(ctx, "user:1",
func() (interface{}, error) {
return db.GetUser(1)
},
func() error {
return db.UpdateUser(1, userData)
},
30*time.Minute,
)

6. 分布式锁示例
go

// 基本锁
token, err := cache.Lock(ctx, "resource:1", 10*time.Second)
if err != nil {
return err
}
defer cache.Unlock(ctx, "resource:1", token)

// 执行临界区代码

// 尝试锁（带超时）
token, err := cache.TryLock(ctx, "resource:1", 10*time.Second, 5*time.Second)
if err != nil {
return errors.New("Failed to acquire lock within timeout")
}

// 自动续期锁（适合长时间操作）
token, err := cache.LockWithRenewal(ctx, "resource:1", 10*time.Second, 30*time.Second)
if err != nil {
return err
}
defer cache.Unlock(ctx, "resource:1", token)

// 可重入锁（同一线程可多次获取）
if cache.ReentrantLock(ctx, "resource:1", "thread-1", 10*time.Second) {
defer cache.ReentrantUnlock(ctx, "resource:1", "thread-1")
}

7. 缓存防护策略
go

// 防止缓存穿透
func GetUser(ctx context.Context, userID int) (*User, error) {
key := fmt.Sprintf("user:%d", userID)

// 1. 检查布隆过滤器
exists, _ := cache.BloomExists(ctx, "users", key)
if !exists {
return nil, errors.New("User not found")
}

// 2. 获取缓存
user, err := cache.Get(ctx, key)
if err == nil {
return user, nil
}

// 3. 缓存未命中，使用分布式锁防止击穿
lockKey := fmt.Sprintf("lock:user:%d", userID)
token, err := cache.Lock(ctx, lockKey, 5*time.Second)
if err != nil {
// 等待重试
time.Sleep(100 * time.Millisecond)
return GetUser(ctx, userID)
}
defer cache.Unlock(ctx, lockKey, token)

// 4. 双检查锁
user, err = cache.Get(ctx, key)
if err == nil {
return user, nil
}

// 5. 从数据库加载
user, err = db.GetUser(userID)
if err != nil {
// 防止缓存穿透：缓存空值
_ = cache.Set(ctx, key, "", 30*time.Second)
return nil, err
}

// 6. 设置随机过期时间防止雪崩
ttl := 30*time.Minute + time.Duration(rand.Intn(300))*time.Second
_ = cache.Set(ctx, key, user, ttl)

// 7. 更新布隆过滤器
_ = cache.BloomAdd(ctx, "users", key)

return user, nil
}

// 熔断器保护
func GetWithCircuitBreaker(ctx context.Context, key string) (string, error) {
cb := cache.NewCircuitBreaker(5, 30*time.Second)

var result string
err := cb.Execute(ctx, func() error {
var err error
result, err = cache.Get(ctx, key)
return err
})

return result, err
}

8. 监控和统计
go

// 获取缓存统计
stats := cache.Stats()
         fmt.Printf("Hits: %d, Misses: %d, Hit Rate: %.2f%%\n",
                                          stats.Hits, stats.Misses,
                                          float64(stats.Hits)/float64(stats.Hits+stats.Misses)*100,
)

// 检查Redis是否可用
if cache.IsRedisAvailable() {
fmt.Println("Redis is available")
} else {
fmt.Println("Using memory cache (fallback mode)")
}

// 获取连接池统计
poolStats := cache.GetPoolStats()
         fmt.Printf("Active Connections: %d, Idle Connections: %d\n",
                                            poolStats.TotalConns, poolStats.IdleConns,
)

// 内存使用情况
if cache.IsUsingMemoryCache() {
         fmt.Printf("Memory Usage: %.2f MB\n",
           float64(stats.MemoryUsage)/1024/1024,
)
}

9. 数据结构操作
go

// 哈希操作
err := cache.HSet(ctx, "user:1:profile", "name", "John Doe")
err = cache.HSet(ctx, "user:1:profile", "email", "john@example.com")

name, err := cache.HGet(ctx, "user:1:profile", "name")
profile, err := cache.HGetAll(ctx, "user:1:profile")

// 集合操作
err = cache.SAdd(ctx, "online:users", "user:1", "user:2")
members, err := cache.SMembers(ctx, "online:users")
isOnline, err := cache.SIsMember(ctx, "online:users", "user:1")

// 列表操作
err = cache.LPush(ctx, "messages", "Hello", "World")
message, err := cache.LPop(ctx, "messages")
messages, err := cache.LRange(ctx, "messages", 0, 10)

// 计数器
count, err := cache.Increment(ctx, "page:views", 1)
count, err = cache.Decrement(ctx, "inventory:item:1", 1)

10. 发布订阅
go

// 发布消息
err := cache.Publish(ctx, "notifications", map[string]interface{}{
         "type": "user:registered",
         "user_id": 1,
         "timestamp": time.Now(),
})

// 订阅消息
err := cache.Subscribe(ctx, "notifications", func(message string) {
var notification map[string]interface{}
json.Unmarshal([]byte(message), &notification)

         fmt.Printf("Received notification: %v\n", notification)

// 处理不同类型的通知
switch notification["type"] {
         case "user:registered":
           fmt.Println("New user registered:", notification["user_id"])
         case "order:created":
           fmt.Println("New order created")
}
})

最佳实践
1. 缓存键设计
go

// 使用有意义的键结构
// 格式：<前缀>:<实体>:<ID>:<字段>
userKey := fmt.Sprintf("user:%d", userID)
userProfileKey := fmt.Sprintf("user:%d:profile", userID)
orderKey := fmt.Sprintf("order:%d", orderID)
orderItemsKey := fmt.Sprintf("order:%d:items", orderID)

// 使用一致的命名规范
// 推荐：小写字母、冒号分隔、避免特殊字符

2. 过期时间策略
go

// 不同数据使用不同的TTL
const (
ShortTTL   = 5 * time.Minute    // 频繁变化的数据
MediumTTL  = 30 * time.Minute   // 一般数据
LongTTL    = 2 * time.Hour      // 稳定数据
SessionTTL = 24 * time.Hour     // 会话数据
)

// 添加随机偏移防止雪崩
func getTTLWithJitter(baseTTL time.Duration) time.Duration {
jitter := time.Duration(rand.Intn(300)) * time.Second // 0-5分钟随机偏移
return baseTTL + jitter
}

3. 缓存降级策略
go

// 检查缓存状态，自动降级
func GetData(ctx context.Context, key string) (string, error) {
if !cache.IsRedisAvailable() {
// 使用内存缓存或直接访问数据库
logger.Warn("Redis unavailable, using fallback")
return getFromFallback(ctx, key)
}

return cache.Get(ctx, key)
}

// 多级缓存
func GetWithMultiLevel(ctx context.Context, key string) (string, error) {
// 1. 检查本地内存缓存
if value, err := localCache.Get(key); err == nil {
return value, nil
}

// 2. 检查分布式缓存
if value, err := cache.Get(ctx, key); err == nil {
// 填充本地缓存
localCache.Set(key, value, 1*time.Minute)
return value, nil
}

// 3. 访问数据库
value, err := db.Get(key)
if err != nil {
return "", err
}

// 4. 填充两级缓存
cache.Set(ctx, key, value, 30*time.Minute)
localCache.Set(key, value, 1*time.Minute)

return value, nil
}

4. 缓存预热
go

// 应用启动时预热缓存
func warmUpCache() {
logger.Info("Warming up cache...")

// 预热热点数据
hotKeys := []string{
"config:global",
"stats:daily",
"leaderboard:top10",
}

for _, key := range hotKeys {
cache.GetOrSet(context.Background(), key, func() (interface{}, error) {
return loadHotData(key)
}, 5*time.Minute)
}

logger.Info("Cache warm-up completed")
}

// 定期刷新缓存
func startCacheRefresher() {
ticker := time.NewTicker(5 * time.Minute)
defer ticker.Stop()

for range ticker.C {
refreshCriticalCache()
}
}

故障排除
1. 缓存穿透解决方案
go

// 方案1：布隆过滤器
func GetWithBloomFilter(ctx context.Context, key string) (string, error) {
exists, _ := cache.BloomExists(ctx, "valid_keys", key)
if !exists {
return "", errors.New("Key not in bloom filter")
}
return cache.Get(ctx, key)
}

// 方案2：空值缓存
func GetWithNullCache(ctx context.Context, key string) (string, error) {
value, err := cache.Get(ctx, key)
if err != nil && strings.Contains(err.Error(), "not found") {
// 检查是否是空值占位符
if value == "__NULL__" {
return "", errors.New("Data does not exist")
}
}
return value, err
}

// 方案3：接口限流
func GetWithRateLimit(ctx context.Context, key string) (string, error) {
if !rateLimiter.Allow() {
return "", errors.New("Rate limit exceeded")
}
return cache.Get(ctx, key)
}

2. 缓存击穿解决方案
go

// 方案1：互斥锁
func GetWithMutex(ctx context.Context, key string) (string, error) {
lockKey := "mutex:" + key
token, err := cache.Lock(ctx, lockKey, 5*time.Second)
if err != nil {
// 等待并重试
time.Sleep(100 * time.Millisecond)
return GetWithMutex(ctx, key)
}
defer cache.Unlock(ctx, lockKey, token)

return cache.GetOrSet(ctx, key, loadFromDB, 30*time.Minute)
}

// 方案2：逻辑过期
func GetWithLogicalExpire(ctx context.Context, key string) (string, error) {
data, err := cache.Get(ctx, key)
if err != nil {
return "", err
}

// 解析数据，检查逻辑过期时间
var item CacheItem
json.Unmarshal([]byte(data), &item)

if time.Now().After(item.LogicalExpire) {
// 异步更新缓存
go func() {
newData, _ := loadFromDB()
cache.Set(ctx, key, newData, 30*time.Minute)
}()
}

return item.Data, nil
}

3. 缓存雪崩解决方案
go

// 方案1：随机过期时间
func SetWithRandomTTL(ctx context.Context, key string, value interface{}, baseTTL time.Duration) error {
// 添加随机偏移
jitter := time.Duration(rand.Intn(300)) * time.Second
ttl := baseTTL + jitter
return cache.Set(ctx, key, value, ttl)
}

// 方案2：缓存永不过期，后台更新
func GetWithBackgroundRefresh(ctx context.Context, key string) (string, error) {
value, err := cache.Get(ctx, key)
if err == nil {
// 检查是否需要后台刷新
lastUpdate, _ := cache.Get(ctx, key+":last_update")
if needRefresh(lastUpdate) {
go func() {
newData, _ := loadFromDB()
cache.Set(ctx, key, newData, 0) // 永不过期
cache.Set(ctx, key+":last_update", time.Now(), 0)
}()
}
return value, nil
}

// 缓存未命中，从数据库加载
data, err := loadFromDB()
if err != nil {
return "", err
}

cache.Set(ctx, key, data, 0) // 永不过期
cache.Set(ctx, key+":last_update", time.Now(), 0)

return data, nil
}

// 方案3：服务降级
func GetWithDegradation(ctx context.Context, key string) (string, error) {
// 尝试从缓存获取
value, err := cache.Get(ctx, key)
if err == nil {
return value, nil
}

// 检查缓存是否过载
if cache.IsOverloaded() {
// 直接返回降级数据
return getDegradedData(key)
}

// 正常从数据库加载
return loadFromDB()
}

性能调优
1. Redis连接池优化
yaml

         redis:
           pool_size: 100      # 连接池大小（根据QPS调整）
           min_idle_conns: 20  # 最小空闲连接数
           max_retries: 3      # 最大重试次数
           read_timeout: 3s    # 读取超时
           write_timeout: 3s   # 写入超时

2. 内存缓存优化
go

// 使用适当的数据结构
type OptimizedCache struct {
store sync.Map           // 用于大量数据
lru   *lru.Cache         // 用于热点数据
ttl   time.Duration      // 统一过期时间
}

// 定期清理过期数据
func (c *OptimizedCache) cleanup() {
ticker := time.NewTicker(1 * time.Minute)
for range ticker.C {
c.evictExpired()
}
}

3. 监控指标
go

// 关键监控指标
type CacheMetrics struct {
HitRate        float64   // 命中率
LatencyP50     time.Duration // P50延迟
LatencyP99     time.Duration // P99延迟
ErrorRate      float64   // 错误率
MemoryUsage    float64   // 内存使用率
ConnectionPool struct {  // 连接池状态
Active int
Idle   int
}
}

// 告警规则
func setupCacheAlerts() {
// 命中率低于80%告警
if metrics.HitRate < 0.8 {
alert("Cache hit rate too low")
}

// P99延迟超过100ms告警
if metrics.LatencyP99 > 100*time.Millisecond {
alert("Cache latency too high")
}

// 错误率超过1%告警
if metrics.ErrorRate > 0.01 {
alert("Cache error rate too high")
}
}

这个缓存组件提供了完整的生产级缓存解决方案，支持Redis和内存缓存降级，并实现了所有高级特性。您可以根据实际需求选择使用不同的策略和配置。         


## 认证组件 (Auth)

### 特性
- ✅ JWT v5 安全实现（支持HS256签名）
- ✅ Token刷新机制（双Token方案）
- ✅ 完整的RBAC权限控制系统
- ✅ 双因子认证支持（TOTP、短信、邮箱）
- ✅ OAuth2.0集成（Google、GitHub、Microsoft等）
- ✅ API密钥认证
- ✅ 会话管理
- ✅ 令牌吊销和黑名单
- ✅ 密码哈希和验证

### 快速开始

#### 1. 基本配置

```yaml
# config/config.yaml
         auth:
           type: jwt
           jwt_secret: "your-super-secret-jwt-key-at-least-32-characters-long"
           jwt_expire: 60
           refresh_expire: 7
           issuer: "dgou-app"
           audience: "dgou-client"
           enable_2fa: false
           enable_rbac: true

  2. 用户提供者接口实现
  go

  package services

  import (
  "context"
  "dgou/pkg/auth"
  "dgou/pkg/database"
  )

  type UserService struct {
  db *database.DB
}

  func (us *UserService) GetUserByID(ctx context.Context, userID uint64) (*auth.User, error) {
  var user auth.User
  if err := us.db.First(&user, userID).Error; err != nil {
  return nil, err
  }
  return &user, nil
}

  func (us *UserService) VerifyCredentials(ctx context.Context, username, password string) (*auth.User, error) {
  var user auth.User
  if err := us.db.Where("username = ? OR email = ?", username, username).First(&user).Error; err != nil {
  return nil, auth.ErrInvalidCredentials
  }

  if err := auth.VerifyPassword(user.PasswordHash, password); err != nil {
  return nil, err
  }

  return &user, nil
}

  // 实现其他接口方法...

  3. 初始化认证
  go

  import (
  "dgou/pkg/auth"
  "dgou/pkg/config"
  "your-app/services"
  )

  func main() {
  cfg := config.LoadConfig()

  // 创建用户服务
  userService := &services.UserService{}

  // 初始化认证
  authManager, err := auth.InitAuth(cfg, userService)
  if err != nil {
  log.Fatal(err)
  }

  // 注册OAuth2提供商
  oauth2Manager := auth.NewOAuth2Manager()
  googleConfig := &auth.OAuth2ProviderConfig{
         ClientID:     cfg.OAuth2.Google.ClientID,
         ClientSecret: cfg.OAuth2.Google.ClientSecret,
         AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
         TokenURL:     "https://oauth2.googleapis.com/token",
         UserInfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
         RedirectURL:  cfg.OAuth2.Google.RedirectURL,
         Scopes:       cfg.OAuth2.Google.Scopes,
}
  oauth2Manager.RegisterProvider(auth.OAuth2Google, googleConfig)
}

  4. 认证路由示例
  go

  package api

  import (
  "dgou/pkg/auth"
  "dgou/pkg/response"
  "net/http"

  "github.com/gin-gonic/gin"
  )

  type AuthController struct {
  authManager *auth.AuthManager
}

  func NewAuthController(authManager *auth.AuthManager) *AuthController {
  return &AuthController{
         authManager: authManager,
}
}

  // Login 用户名密码登录
  func (ac *AuthController) Login(c *gin.Context) {
  var req struct {
  Username string `json:"username" binding:"required"`
  Password string `json:"password" binding:"required"`
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  ipAddress := c.ClientIP()
  userAgent := c.Request.UserAgent()

  authResult, err := ac.authManager.Authenticate(c.Request.Context(), req.Username, req.Password, ipAddress, userAgent)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  // 检查是否需要双因子认证
  if authResult.Metadata != nil && authResult.Metadata["requires_2fa"].(bool) {
  response.Success(c, gin.H{
         "requires_2fa": true,
         "method":       authResult.Metadata["2fa_method"],
         "user_id":      authResult.UserID,
})
  return
}

  response.Success(c, authResult)
}

  // Login2FA 双因子认证登录
  func (ac *AuthController) Login2FA(c *gin.Context) {
  var req struct {
  UserID uint64 `json:"user_id" binding:"required"`
  Code   string `json:"code" binding:"required"`
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  ipAddress := c.ClientIP()
  userAgent := c.Request.UserAgent()

  // 这里需要根据实际情况获取用户名和密码
  // 实际项目中可能需要修改接口设计
  authResult, err := ac.authManager.AuthenticateWith2FA(c.Request.Context(), "", "", req.Code, ipAddress, userAgent)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  response.Success(c, authResult)
}

  // RefreshToken 刷新令牌
  func (ac *AuthController) RefreshToken(c *gin.Context) {
  var req struct {
  RefreshToken string `json:"refresh_token" binding:"required"`
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  tokenPair, err := ac.authManager.RefreshToken(c.Request.Context(), req.RefreshToken)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  response.Success(c, tokenPair)
}

  // Logout 登出
  func (ac *AuthController) Logout(c *gin.Context) {
  // 从上下文中获取用户ID
  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  // 吊销用户所有令牌
  if err := ac.authManager.RevokeAllUserTokens(c.Request.Context(), userClaims.UserID); err != nil {
  response.InternalServerError(c, "Failed to logout")
  return
}

  response.Success(c, gin.H{
         "message": "Logged out successfully",
})
}

  // GetProfile 获取用户资料
  func (ac *AuthController) GetProfile(c *gin.Context) {
  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  response.Success(c, gin.H{
         "user_id":    userClaims.UserID,
         "username":   userClaims.Username,
         "email":      userClaims.Email,
         "roles":      userClaims.Roles,
         "permissions": userClaims.Permissions,
})
}

  5. 权限控制示例
  go

  // 需要认证的路由
  func setupAuthRoutes(router *gin.Engine, authManager *auth.AuthManager) {
  authGroup := router.Group("/api")

  // 使用认证中间件
  authGroup.Use(authManager.AuthMiddleware())

  // 用户相关路由
  userGroup := authGroup.Group("/users")
  {
    // 所有用户都可以访问自己的资料
    userGroup.GET("/profile", userController.GetProfile)

    // 需要user:read权限
    userGroup.GET("", userController.ListUsers,
    authManager.RBACMiddleware(auth.PermissionUserRead))

    // 需要user:create权限
    userGroup.POST("", userController.CreateUser,
    authManager.RBACMiddleware(auth.PermissionUserCreate))

    // 需要user:update权限
    userGroup.PUT("/:id", userController.UpdateUser,
    authManager.RBACMiddleware(auth.PermissionUserUpdate))

    // 需要user:delete权限
    userGroup.DELETE("/:id", userController.DeleteUser,
    authManager.RBACMiddleware(auth.PermissionUserDelete))
  }

  // 文章相关路由
  articleGroup := authGroup.Group("/articles")
  {
  // 所有用户都可以查看文章
  articleGroup.GET("/:id", articleController.GetArticle)

  // 需要article:create权限
  articleGroup.POST("", articleController.CreateArticle,
  authManager.RBACMiddleware(auth.PermissionArticleCreate))

  // 需要article:update权限
  articleGroup.PUT("/:id", articleController.UpdateArticle,
  authManager.RBACMiddleware(auth.PermissionArticleUpdate))

  // 需要article:delete权限
  articleGroup.DELETE("/:id", articleController.DeleteArticle,
  authManager.RBACMiddleware(auth.PermissionArticleDelete))
  }

  // 管理员路由
  adminGroup := authGroup.Group("/admin")
  {
  // 需要admin角色
  adminGroup.Use(authManager.RoleMiddleware([]auth.UserRole{auth.RoleAdmin, auth.RoleSuperAdmin}))

  adminGroup.GET("/dashboard", adminController.Dashboard)
  adminGroup.GET("/users", adminController.ListAllUsers)

  // 需要super_admin角色
  superAdminGroup := adminGroup.Group("/system")
  superAdminGroup.Use(authManager.RoleMiddleware([]auth.UserRole{auth.RoleSuperAdmin}))
  {
  superAdminGroup.GET("/config", adminController.GetSystemConfig)
  superAdminGroup.PUT("/config", adminController.UpdateSystemConfig)
  }
  }
}

  // 双因子认证路由
  func setup2FARoutes(router *gin.Engine, authManager *auth.AuthManager) {
  authGroup := router.Group("/api/2fa")
  authGroup.Use(authManager.AuthMiddleware())

  // 启用双因子认证
  authGroup.POST("/enable", func(c *gin.Context) {
  userClaims, _ := auth.GetUserFromContext(c)

  // 生成TOTP密钥
  twoFactorManager := auth.NewTwoFactorManager("dgou-app")
  secret, qrURL, err := twoFactorManager.GenerateTOTPSecret(userClaims.Username)
  if err != nil {
  response.InternalServerError(c, "Failed to generate TOTP secret")
  return
  }

  // 生成备份代码
  backupCodes, hashedCodes, err := twoFactorManager.GenerateBackupCodes(10)
  if err != nil {
  response.InternalServerError(c, "Failed to generate backup codes")
  return
  }

  // 保存到数据库
         // TODO: 保存secret和hashedCodes到用户记录

  response.Success(c, gin.H{
         "secret":       secret,
         "qr_url":       qrURL,
         "backup_codes": backupCodes, // 只返回一次，用户需要安全保存
})
})

  // 验证并启用双因子认证
  authGroup.POST("/verify", func(c *gin.Context) {
  var req struct {
  Code string `json:"code" binding:"required"`
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  userClaims, _ := auth.GetUserFromContext(c)

  // 从数据库获取用户和TOTP密钥
         // TODO: 获取用户和TOTP密钥

  twoFactorManager := auth.NewTwoFactorManager("dgou-app")
  valid, err := twoFactorManager.VerifyTOTPCode(secret, req.Code)
  if err != nil || !valid {
  response.BadRequest(c, "Invalid verification code")
  return
}

  // 启用双因子认证
         // TODO: 更新用户记录，启用双因子认证

  response.Success(c, gin.H{
         "message": "Two-factor authentication enabled successfully",
})
})

  // 禁用双因子认证
  authGroup.POST("/disable", func(c *gin.Context) {
  // 需要验证密码或其他安全措施
  var req struct {
  Password string `json:"password" binding:"required"`
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  userClaims, _ := auth.GetUserFromContext(c)

  // 验证密码
         // TODO: 验证用户密码

  // 禁用双因子认证
         // TODO: 更新用户记录，禁用双因子认证

  response.Success(c, gin.H{
         "message": "Two-factor authentication disabled successfully",
})
})
}

  6. OAuth2.0集成示例
  go

  // OAuth2路由
  func setupOAuth2Routes(router *gin.Engine, authManager *auth.AuthManager, oauth2Manager *auth.OAuth2Manager) {
  // OAuth2认证开始
  router.GET("/auth/:provider", func(c *gin.Context) {
  provider := auth.OAuth2Provider(c.Param("provider"))

  // 生成状态令牌，防止CSRF攻击
  state, err := generateStateToken()
  if err != nil {
  response.InternalServerError(c, "Failed to generate state token")
  return
}

  // 保存状态到会话
         // TODO: 保存state到会话

  // 获取认证URL
  authURL, err := oauth2Manager.GetAuthURL(provider, state)
  if err != nil {
  response.InternalServerError(c, "Failed to get auth URL")
  return
}

  // 重定向到OAuth2提供商
  c.Redirect(http.StatusFound, authURL)
})

  // OAuth2回调
  router.GET("/auth/:provider/callback", func(c *gin.Context) {
  provider := auth.OAuth2Provider(c.Param("provider"))
  code := c.Query("code")
  state := c.Query("state")

  // 验证状态令牌
         // TODO: 从会话中获取并验证state

  // 交换代码获取令牌
  token, err := oauth2Manager.ExchangeCode(provider, code)
  if err != nil {
  response.Unauthorized(c, "Failed to exchange code for token")
  return
}

  // 获取用户信息
  userInfo, err := oauth2Manager.GetUserInfo(provider, token.AccessToken)
  if err != nil {
  response.Unauthorized(c, "Failed to get user info")
  return
}

  // 查找或创建用户
  user, err := findOrCreateOAuth2User(c.Request.Context(), provider, userInfo)
  if err != nil {
  response.InternalServerError(c, "Failed to find or create user")
  return
}

  // 生成JWT令牌
  tokenPair, err := authManager.GenerateTokenPair(user)
  if err != nil {
  response.InternalServerError(c, "Failed to generate token")
  return
}

  // 重定向到前端，携带令牌
  // 或者设置cookie，根据你的应用架构决定
  frontendURL := fmt.Sprintf("%s?token=%s", frontendCallbackURL, tokenPair.AccessToken)
  c.Redirect(http.StatusFound, frontendURL)
})
}

  func generateStateToken() (string, error) {
  bytes := make([]byte, 32)
  if _, err := rand.Read(bytes); err != nil {
  return "", err
}
  return base64.URLEncoding.EncodeToString(bytes), nil
}

  7. API密钥认证示例
  go

  // API密钥路由
  func setupAPIKeyRoutes(router *gin.Engine, apiKeyManager *auth.APIKeyManager) {
  apiGroup := router.Group("/api/keys")

  // 使用JWT认证中间件
  apiGroup.Use(authManager.AuthMiddleware())

  // 生成API密钥
  apiGroup.POST("", func(c *gin.Context) {
  var req struct {
  Name        string               `json:"name" binding:"required"`
  Permissions []auth.Permission    `json:"permissions"`
  ExpiresIn   *time.Duration       `json:"expires_in"` // 可选，例如：720h (30天)
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  userClaims, _ := auth.GetUserFromContext(c)

  // 生成API密钥
  apiKey, fullKey, err := apiKeyManager.GenerateAPIKey(
  c.Request.Context(),
  userClaims.UserID,
  req.Name,
  req.Permissions,
  req.ExpiresIn,
  )

  if err != nil {
  response.InternalServerError(c, "Failed to generate API key")
  return
}

  // 返回API密钥（只显示一次）
  response.Success(c, gin.H{
         "api_key": apiKey,
         "full_key": fullKey, // 注意：这个只返回一次！
         "warning": "Save this API key now! It will not be shown again.",
})
})

  // 列出API密钥
  apiGroup.GET("", func(c *gin.Context) {
  userClaims, _ := auth.GetUserFromContext(c)

  keys, err := apiKeyManager.ListUserAPIKeys(c.Request.Context(), userClaims.UserID)
  if err != nil {
  response.InternalServerError(c, "Failed to list API keys")
  return
}

  response.Success(c, keys)
})

  // 吊销API密钥
  apiGroup.DELETE("/:id", func(c *gin.Context) {
  keyID := c.Param("id")

  if err := apiKeyManager.RevokeAPIKey(c.Request.Context(), keyID); err != nil {
  response.InternalServerError(c, "Failed to revoke API key")
  return
}

  response.Success(c, gin.H{
         "message": "API key revoked successfully",
})
})
}

  // API路由使用API密钥认证
  func setupAPIRoutes(router *gin.Engine, apiKeyManager *auth.APIKeyManager) {
  apiGroup := router.Group("/api/v1")

  // 使用API密钥认证中间件
  apiGroup.Use(apiKeyManager.APIKeyMiddleware())

  // 公共API端点
  apiGroup.GET("/health", func(c *gin.Context) {
  response.Success(c, gin.H{
         "status": "ok",
         "timestamp": time.Now().Unix(),
})
})

  apiGroup.GET("/data", func(c *gin.Context) {
  // 获取API密钥信息
  apiKeyInfo, exists := c.Get("api_key_info")
  if !exists {
  response.Unauthorized(c, "API key information not found")
  return
}

  keyInfo := apiKeyInfo.(*auth.APIKey)

  // 检查权限
  hasPermission := false
  for _, perm := range keyInfo.Permissions {
  if perm == auth.PermissionArticleRead {
  hasPermission = true
  break
}
}

  if !hasPermission {
  response.Forbidden(c, "Insufficient permissions")
  return
}

  // 返回数据
  data := fetchDataForAPIKey(keyInfo.ID)
  response.Success(c, data)
})
}

  高级特性
  1. 自定义用户声明
  go

  // 添加自定义声明
  claims := &auth.UserClaims{
         UserID:   user.ID,
         Username: user.Username,
         Email:    user.Email,
         Roles:    user.Roles,
         CustomClaims: map[string]interface{}{
           "department": "engineering",
           "team":       "backend",
           "employee_id": "EMP-12345",
},
}

  // 在中间件中访问自定义声明
  func CustomMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
         c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
  return
}

  // 访问自定义声明
  if department, ok := userClaims.CustomClaims["department"].(string); ok {
  c.Set("user_department", department)
}

  c.Next()
}
}

  2. 令牌黑名单
  go

  // 吊销令牌
  func revokeTokenHandler(c *gin.Context) {
  token, err := authManager.ExtractTokenFromHeader(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  if err := authManager.RevokeToken(c.Request.Context(), token, "user_logout"); err != nil {
  response.InternalServerError(c, "Failed to revoke token")
  return
}

  response.Success(c, gin.H{
         "message": "Token revoked successfully",
})
}

  // 批量吊销用户所有令牌
  func revokeAllUserTokensHandler(c *gin.Context) {
  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  if err := authManager.RevokeAllUserTokens(c.Request.Context(), userClaims.UserID); err != nil {
  response.InternalServerError(c, "Failed to revoke tokens")
  return
}

  response.Success(c, gin.H{
         "message": "All tokens revoked successfully",
})
}

  3. 会话管理
  go

  // 获取用户所有会话
  func listSessionsHandler(c *gin.Context) {
  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  sessions, err := authManager.GetUserSessions(c.Request.Context(), userClaims.UserID)
  if err != nil {
  response.InternalServerError(c, "Failed to get sessions")
  return
}

  response.Success(c, sessions)
}

  // 终止特定会话
  func terminateSessionHandler(c *gin.Context) {
  sessionID := c.Param("id")

  if err := authManager.TerminateSession(c.Request.Context(), sessionID); err != nil {
  response.InternalServerError(c, "Failed to terminate session")
  return
}

  response.Success(c, gin.H{
         "message": "Session terminated successfully",
})
}

  // 终止除当前会话外的所有会话
  func terminateOtherSessionsHandler(c *gin.Context) {
  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  sessions, err := authManager.GetUserSessions(c.Request.Context(), userClaims.UserID)
  if err != nil {
  response.InternalServerError(c, "Failed to get sessions")
  return
}

  currentSessionID, _ := c.Get("session_id")

  for _, session := range sessions {
  if session.ID != currentSessionID {
  _ = authManager.TerminateSession(c.Request.Context(), session.ID)
}
}

  response.Success(c, gin.H{
         "message": "Other sessions terminated successfully",
})
}

  4. 密码策略
  go

  // 密码验证器
  type PasswordValidator struct {
  MinLength     int
  RequireUpper  bool
  RequireLower  bool
  RequireNumber bool
  RequireSpecial bool
}

  func NewPasswordValidator() *PasswordValidator {
  return &PasswordValidator{
         MinLength:     8,
         RequireUpper:  true,
         RequireLower:  true,
         RequireNumber: true,
         RequireSpecial: true,
}
}

  func (pv *PasswordValidator) Validate(password string) error {
  if len(password) < pv.MinLength {
  return fmt.Errorf("password must be at least %d characters long", pv.MinLength)
}

  if pv.RequireUpper && !containsUpper(password) {
  return errors.New("password must contain at least one uppercase letter")
}

  if pv.RequireLower && !containsLower(password) {
  return errors.New("password must contain at least one lowercase letter")
}

  if pv.RequireNumber && !containsNumber(password) {
  return errors.New("password must contain at least one number")
}

  if pv.RequireSpecial && !containsSpecial(password) {
  return errors.New("password must contain at least one special character")
}

  return nil
}

  // 密码重置
  func resetPasswordHandler(c *gin.Context) {
  var req struct {
  CurrentPassword string `json:"current_password" binding:"required"`
  NewPassword     string `json:"new_password" binding:"required"`
}

  if err := c.ShouldBindJSON(&req); err != nil {
  response.BadRequest(c, "Invalid request")
  return
}

  userClaims, err := auth.GetUserFromContext(c)
  if err != nil {
  response.Unauthorized(c, err.Error())
  return
}

  // 验证当前密码
  user, err := userService.GetUserByID(c.Request.Context(), userClaims.UserID)
  if err != nil {
  response.InternalServerError(c, "Failed to get user")
  return
}

  if err := auth.VerifyPassword(user.PasswordHash, req.CurrentPassword); err != nil {
  response.BadRequest(c, "Current password is incorrect")
  return
}

  // 验证新密码强度
  validator := NewPasswordValidator()
  if err := validator.Validate(req.NewPassword); err != nil {
  response.BadRequest(c, err.Error())
  return
}

  // 哈希新密码
  newHash, err := auth.HashPassword(req.NewPassword)
  if err != nil {
  response.InternalServerError(c, "Failed to hash password")
  return
}

  // 更新密码
  if err := userService.UpdatePassword(c.Request.Context(), userClaims.UserID, newHash); err != nil {
  response.InternalServerError(c, "Failed to update password")
  return
}

  // 吊销所有现有令牌（安全最佳实践）
  _ = authManager.RevokeAllUserTokens(c.Request.Context(), userClaims.UserID)

  response.Success(c, gin.H{
         "message": "Password updated successfully. Please login again.",
})
}

  安全最佳实践
  1. JWT安全配置
  go

  // 使用强密钥
  jwtSecret := generateStrongSecret(64)

  // 设置合理的过期时间
  jwtExpire := 15 * time.Minute     // 访问令牌：15分钟
  refreshExpire := 7 * 24 * time.Hour // 刷新令牌：7天

  // 使用HTTPS传输
  // 启用HttpOnly和Secure Cookie

  2. 防止令牌泄露
  go

  // 令牌轮换
  func rotateTokens(refreshToken string) (*auth.TokenPair, error) {
  // 验证刷新令牌
  // 生成新的访问令牌和刷新令牌
  // 吊销旧的刷新令牌
  // 返回新的令牌对
}

  // 令牌绑定（绑定到IP、User-Agent等）
  func bindTokenToContext(token string, ipAddress, userAgent string) bool {
  // 在令牌中存储上下文信息
  // 验证时检查上下文是否匹配
  // 不匹配则拒绝访问
}

  3. 速率限制
  go

  // 登录尝试限制
  func loginRateLimitMiddleware() gin.HandlerFunc {
  return func(c *gin.Context) {
  ip := c.ClientIP()
  key := fmt.Sprintf("login_attempts:%s", ip)

  attempts, _ := cache.Get(c.Request.Context(), key)
  if attempts >= 5 {
  c.JSON(http.StatusTooManyRequests, gin.H{
         "error": "Too many login attempts. Please try again later.",
})
  c.Abort()
  return
}

  cache.Increment(c.Request.Context(), key, 1)
  cache.Expire(c.Request.Context(), key, 15*time.Minute)

  c.Next()
}
}

  故障排除
  1. 令牌验证失败
  go

  // 检查令牌格式
  token := strings.TrimPrefix(authHeader, "Bearer ")

  // 检查令牌是否过期
  claims, err := authManager.ParseToken(token)
  if err != nil {
  if strings.Contains(err.Error(), "token is expired") {
  return errors.New("Token has expired. Please refresh or login again.")
}
  return errors.New("Invalid token")
}

  // 检查令牌是否被吊销
  revoked, err := authManager.IsTokenRevoked(token)
  if err != nil || revoked {
  return errors.New("Token has been revoked")
}

  2. 权限检查失败
  go

  // 检查用户角色
  userClaims := GetUserFromContext(c)
  if !authManager.HasRole(userClaims, auth.RoleAdmin) {
  return errors.New("Insufficient role privileges")
}

  // 检查具体权限
  if !authManager.HasPermission(userClaims, auth.PermissionUserDelete) {
  return errors.New("Insufficient permissions")
}

  // 检查多个权限
  requiredPermissions := []auth.Permission{
  auth.PermissionUserRead,
  auth.PermissionUserUpdate,
}

  if !authManager.HasAllPermissions(userClaims, requiredPermissions) {
  return errors.New("Missing required permissions")
}

  3. 双因子认证问题
  go

  // 处理TOTP代码验证
  func verifyTOTPCode(secret, code string) error {
  // 验证代码
  valid, err := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
         Period:    30,
         Skew:      1, // 允许前后一个周期
         Digits:    otp.DigitsSix,
         Algorithm: otp.AlgorithmSHA1,
})

  if err != nil {
  return errors.Wrap(err, "Failed to validate TOTP code")
}

  if !valid {
  return errors.New("Invalid TOTP code")
}

  return nil
}

  // 处理备份代码
  func verifyBackupCode(code string, hashedCodes []string) (bool, error) {
  for _, hashed := range hashedCodes {
  if err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(code)); err == nil {
  return true, nil
}
}
  return false, nil
}

  这个认证组件提供了完整的生产级认证解决方案，支持多种认证方式和安全特性。您可以根据实际需求选择使用不同的认证策略。


  # Dgou Framework - 生产级 Go Gin 脚手架

  ## 异步任务组件 (pkg/async)

  ### 特性
         - ✅ **协程池管理**：可配置的协程池，支持动态调整工作协程数
         - ✅ **任务优先级**：4级优先级（低、正常、高、关键），支持优先队列
         - ✅ **失败重试机制**：可配置的重试次数和重试延迟，支持指数退避
         - ✅ **任务状态追踪**：完整的任务生命周期管理，实时状态监控
         - ✅ **任务结果查询**：支持任务结果持久化和查询
         - ✅ **任务超时控制**：每个任务可配置独立超时时间
         - ✅ **优雅关闭**：支持优雅关闭，确保任务完成不丢失
         - ✅ **详细指标监控**：全面的性能指标和统计信息
         - ✅ **任务取消支持**：可随时取消正在执行的任务
         - ✅ **防内存泄漏**：自动清理过期任务，防止内存泄漏

  ### 快速开始

  #### 1. 基本配置

  ```yaml
  # config/config.yaml
         async:
           max_workers: 100           # 最大工作协程数
           max_queue_size: 10000      # 最大队列大小
           worker_idle_time: 30s      # 工作协程空闲时间
           enable_metrics: true       # 是否启用指标

  2. 初始化任务管理器
  go

  import (
  "dgou/pkg/async"
  "dgou/pkg/config"
  )

  func main() {
  // 加载配置
  cfg := config.LoadConfig()

  // 初始化任务管理器
  _, err := async.InitTaskManager(cfg)
  if err != nil {
  log.Fatal(err)
  }
}

  3. 创建并提交任务
  go

  // 定义任务处理函数
  func processImage(ctx context.Context, params interface{}) (interface{}, error) {
  imageData, ok := params.([]byte)
  if !ok {
  return nil, errors.New("Invalid image data")
  }

  // 处理图片
  result, err := resizeImage(imageData)
  if err != nil {
  return nil, err
  }

  return result, nil
}

  // 创建任务
  task := async.NewTask("process_image", processImage, imageBytes).
  WithPriority(async.PriorityHigh).    // 设置高优先级
  WithRetries(3, time.Second).         // 设置重试3次，每次间隔1秒
  WithTimeout(30 * time.Second).       // 设置30秒超时
  WithMetadata("user_id", userID)      // 添加元数据

  // 提交任务
  taskID, err := async.SubmitTask(task)
  if err != nil {
         log.Printf("Failed to submit task: %v", err)
  return
}

         log.Printf("Task submitted: %s", taskID)

  4. 等待任务完成并获取结果
  go

  // 等待任务完成（阻塞方式）
  result, err := async.SubmitAndWait(task, 60*time.Second)
  if err != nil {
         log.Printf("Task failed: %v", err)
  return
}

         log.Printf("Task completed: %v", result.Value)

  // 或者异步等待
  go func() {
  // 等待任务完成
  if task.Wait(60 * time.Second) {
  result, err := task.GetResult()
  if err != nil {
         log.Printf("Task failed: %v", err)
} else {
         log.Printf("Task completed successfully: %v", result.Value)
}
} else {
  log.Println("Task wait timeout")
}
}()

  5. 查询任务状态
  go

  // 根据任务ID查询任务
  task, err := async.GetTaskByID(taskID)
  if err != nil {
         log.Printf("Failed to get task: %v", err)
  return
}

  // 获取任务状态
  state := task.GetState()
         log.Printf("Task state: %s", state)

  // 检查任务是否完成
  if task.IsCompleted() {
  result, err := task.GetResult()
  if err != nil {
         log.Printf("Task failed: %v", err)
} else {
         log.Printf("Task result: %v", result.Value)
}
}

  高级用法
  1. 创建自定义协程池
  go

  // 创建协程池配置
  config := &async.PoolConfig{
         MaxWorkers:     50,               // 50个工作协程
         MaxQueueSize:   5000,             // 最大队列大小5000
         WorkerIdleTime: 60 * time.Second, // 空闲60秒后回收
         EnableMetrics:  true,             // 启用指标
}

  // 创建自定义协程池
  pool, err := async.GetTaskManager().CreatePool("image_processing", config)
  if err != nil {
  log.Fatal(err)
}

  // 提交任务到自定义协程池
  taskID, err := async.SubmitTaskToPool("image_processing", task)
  if err != nil {
         log.Printf("Failed to submit task: %v", err)
  return
}

  2. 任务优先级示例
  go

  // 创建不同优先级的任务
  lowPriorityTask := async.NewTask("low_task", handler, params).
  WithPriority(async.PriorityLow)   // 低优先级

  normalPriorityTask := async.NewTask("normal_task", handler, params).
  WithPriority(async.PriorityNormal) // 正常优先级

  highPriorityTask := async.NewTask("high_task", handler, params).
  WithPriority(async.PriorityHigh)   // 高优先级

  criticalPriorityTask := async.NewTask("critical_task", handler, params).
  WithPriority(async.PriorityCritical) // 关键优先级

  // 高优先级任务会先执行，即使后提交

  3. 任务重试策略
  go

  // 指数退避重试
  task := async.NewTask("retry_task", handler, params).
  WithRetries(5, time.Second). // 重试5次，每次间隔1秒
  WithMetadata("max_retries", 5)

  // 或者在任务处理函数中实现自定义重试逻辑
  func handlerWithRetry(ctx context.Context, params interface{}) (interface{}, error) {
  maxRetries := 3
  for i := 0; i < maxRetries; i++ {
  result, err := doSomething(params)
  if err == nil {
  return result, nil
  }

  // 指数退避
  delay := time.Duration(math.Pow(2, float64(i))) * time.Second
  select {
         case <-time.After(delay):
           continue
         case <-ctx.Done():
           return nil, ctx.Err()
}
}
  return nil, errors.New("Max retries exceeded")
}

  4. 任务依赖关系
  go

  // 创建有依赖关系的任务链
  func createTaskChain() {
  // 第一步任务
  task1 := async.NewTask("step1", step1Handler, params1)
  task1ID, _ := async.SubmitTask(task1)

  // 等待第一步完成
  if task1.Wait(30 * time.Second) {
  // 第二步任务，依赖第一步的结果
  task2 := async.NewTask("step2", step2Handler, params2)
  task2.WithMetadata("depends_on", task1ID)
  async.SubmitTask(task2)
}
}

  // 或者使用任务编排器
  func createWorkflow() {
  tasks := []*async.Task{
  async.NewTask("step1", step1Handler, params1),
  async.NewTask("step2", step2Handler, params2),
  async.NewTask("step3", step3Handler, params3),
}

  // 顺序执行
  for _, task := range tasks {
  if _, err := async.SubmitAndWait(task, 30*time.Second); err != nil {
         log.Printf("Task failed, stopping workflow: %v", err)
  break
}
}
}

  5. 批量任务处理
  go

  // 批量提交任务
  func processBatch(items []Item) []string {
  taskIDs := make([]string, 0, len(items))

  for _, item := range items {
  task := async.NewTask("process_item", processItem, item)
  taskID, err := async.SubmitTask(task)
  if err != nil {
         log.Printf("Failed to submit task for item %v: %v", item.ID, err)
  continue
}
  taskIDs = append(taskIDs, taskID)
}

  return taskIDs
}

  // 等待批量任务完成
  func waitForBatch(taskIDs []string, timeout time.Duration) map[string]*async.TaskResult {
  results := make(map[string]*async.TaskResult)

  for _, taskID := range taskIDs {
  task, err := async.GetTaskByID(taskID)
  if err != nil {
         results[taskID] = &async.TaskResult{Error: err}
  continue
}

  if task.Wait(timeout) {
  result, err := task.GetResult()
  if err != nil {
         results[taskID] = &async.TaskResult{Error: err}
} else {
  results[taskID] = result
}
} else {
         results[taskID] = &async.TaskResult{Error: errors.New("Timeout")}
}
}

  return results
}

  6. 任务取消和超时处理
  go

  // 取消任务
  func cancelTask(taskID string) error {
  return async.CancelTask(taskID)
}

  // 带超时的任务执行
  func executeWithTimeout(handler async.TaskHandler, params interface{}, timeout time.Duration) (interface{}, error) {
  task := async.NewTask("timeout_task", handler, params).
  WithTimeout(timeout)

  return async.SubmitAndWait(task, timeout+5*time.Second)
}

  // 在任务处理函数中检查上下文
  func handlerWithContextCheck(ctx context.Context, params interface{}) (interface{}, error) {
  // 定期检查上下文是否被取消
  for i := 0; i < 10; i++ {
  select {
         case <-ctx.Done():
           return nil, ctx.Err() // 任务被取消或超时
         default:
           // 继续执行
}

  // 执行一部分工作
  if err := doPartialWork(params, i); err != nil {
  return nil, err
}

  // 等待一段时间
  select {
         case <-time.After(100 * time.Millisecond):
           continue
         case <-ctx.Done():
           return nil, ctx.Err()
}
}

  return "completed", nil
}

  7. 监控和指标
  go

  // 获取协程池统计信息
  func monitorPool() {
  manager := async.GetTaskManager()
  stats := manager.GetStats()

  fmt.Printf("Pool Statistics:\n")
  for poolName, poolStats := range stats {
         fmt.Printf("  Pool: %s\n", poolName)
         fmt.Printf("    Active Workers: %v\n", poolStats["active_workers"])
         fmt.Printf("    Queue Size: %v/%v\n", poolStats["queue_size"], poolStats["max_queue_size"])
         fmt.Printf("    Completed Tasks: %v\n", poolStats["completed_tasks"])
         fmt.Printf("    Failed Tasks: %v\n", poolStats["failed_tasks"])
         fmt.Printf("    Average Process Time: %v\n", poolStats["avg_process_time"])
}
}

  // 获取任务存储统计
  func monitorTasks() {
  // 获取所有任务状态
  manager := async.GetTaskManager()
  defaultPool := manager.defaultPool
  taskStore := defaultPool.taskStore

  stats := taskStore.GetStats()
  fmt.Printf("Task Statistics:\n")
         fmt.Printf("  Total Tasks: %d\n", stats.Total)
         fmt.Printf("  Pending Tasks: %d\n", stats.Pending)
         fmt.Printf("  Running Tasks: %d\n", stats.Running)
         fmt.Printf("  Completed Tasks: %d\n", stats.Completed)
         fmt.Printf("  Failed Tasks: %d\n", stats.Failed)
}

  8. 优雅关闭
  go

  // 在应用关闭时优雅停止协程池
  func gracefulShutdown() {
  manager := async.GetTaskManager()

  // 停止接收新任务
  log.Println("Stopping task submission...")

  // 等待一段时间让现有任务完成
  time.Sleep(10 * time.Second)

  // 停止所有协程池
  if err := manager.StopAll(); err != nil {
         log.Printf("Error stopping worker pools: %v", err)
}

  log.Println("All worker pools stopped")
}

  配置说明
  协程池配置
  yaml

         async:
           # 默认协程池配置
           default:
             max_workers: 100           # 最大工作协程数，根据CPU核心数调整
             max_queue_size: 10000      # 任务队列最大大小，防止内存溢出
             worker_idle_time: 30s      # 工作协程空闲时间，超时后回收
             enable_metrics: true       # 启用性能指标收集

           # 自定义协程池配置
           pools:
             image_processing:
               max_workers: 50          # 图片处理专用池
               max_queue_size: 5000
               worker_idle_time: 60s

             email_sending:
               max_workers: 20          # 邮件发送专用池
               max_queue_size: 2000
               worker_idle_time: 30s

  任务配置
  配置项	默认值	说明
  Priority	PriorityNormal	任务优先级
  MaxRetries	3	最大重试次数
  RetryDelay	1s	重试延迟时间
  Timeout	30s	任务执行超时时间
  最佳实践
  1. 合理设置协程池大小
  go

  // 根据CPU核心数设置协程池大小
  func calculatePoolSize() int {
  // 通常设置为CPU核心数的2-4倍
  cpuCores := runtime.NumCPU()
  return cpuCores * 3
}

  // I/O密集型任务可以设置更大的协程池
  func getIOPoolConfig() *async.PoolConfig {
  return &async.PoolConfig{
         MaxWorkers:     200,  // I/O密集型任务可以更多
         MaxQueueSize:   10000,
         WorkerIdleTime: 60 * time.Second,
}
}

  2. 任务幂等性设计
  go

  // 确保任务可以安全重试
  func idempotentHandler(ctx context.Context, params interface{}) (interface{}, error) {
  taskID := ctx.Value("task_id").(string)

  // 检查任务是否已经执行过
  if executed, err := checkTaskExecuted(taskID); err != nil {
  return nil, err
} else if executed {
  // 已经执行过，直接返回结果
  return getTaskResult(taskID), nil
}

  // 执行任务
  result, err := doRealWork(params)
  if err != nil {
  return nil, err
}

  // 保存执行结果
  if err := saveTaskResult(taskID, result); err != nil {
  return nil, err
}

  return result, nil
}

  3. 资源限制和熔断
  go

  // 使用信号量限制并发
  var semaphore = make(chan struct{}, 10) // 最大10个并发

  func handlerWithLimiter(ctx context.Context, params interface{}) (interface{}, error) {
  select {
         case semaphore <- struct{}{}:
           defer func() { <-semaphore }()
         case <-ctx.Done():
           return nil, ctx.Err()
}

  return doWork(params)
}

  // 熔断器实现
  type CircuitBreaker struct {
  failures     int
  maxFailures  int
  resetTimeout time.Duration
  lastFailure  time.Time
  mu           sync.RWMutex
}

  func (cb *CircuitBreaker) Execute(handler async.TaskHandler, params interface{}) (interface{}, error) {
  cb.mu.RLock()
  if cb.failures >= cb.maxFailures && time.Since(cb.lastFailure) < cb.resetTimeout {
  cb.mu.RUnlock()
  return nil, errors.New("Circuit breaker open")
}
  cb.mu.RUnlock()

  result, err := handler(context.Background(), params)

  cb.mu.Lock()
  defer cb.mu.Unlock()

  if err != nil {
  cb.failures++
  cb.lastFailure = time.Now()
} else {
  if time.Since(cb.lastFailure) > cb.resetTimeout {
  cb.failures = 0
  }
}

  return result, err
}

  4. 错误处理和日志
  go

  // 详细的错误处理和日志记录
  func handlerWithLogging(ctx context.Context, params interface{}) (interface{}, error) {
  taskID := ctx.Value("task_id").(string)
  startTime := time.Now()

  logger.Info("Task started",
  logger.String("task_id", taskID),
  logger.String("handler", "process_data"),
  logger.Time("start_time", startTime),
  )

  defer func() {
  duration := time.Since(startTime)
  logger.Info("Task completed",
  logger.String("task_id", taskID),
  logger.Duration("duration", duration),
  )
}()

  // 执行实际工作
  result, err := doWork(params)
  if err != nil {
  logger.Error("Task failed",
  logger.String("task_id", taskID),
  logger.ErrorField(err),
  logger.Any("params", params),
  )
  return nil, err
}

  return result, nil
}

  故障排除
  1. 任务队列积压
  go

  // 监控队列积压
  func monitorQueueBacklog() {
  manager := async.GetTaskManager()
  stats := manager.GetStats()

  for poolName, poolStats := range stats {
  queueSize := poolStats["queue_size"].(int)
  maxQueueSize := poolStats["max_queue_size"].(int)

  if float64(queueSize) > float64(maxQueueSize)*0.8 {
  logger.Warn("Queue approaching capacity",
  logger.String("pool", poolName),
  logger.Int("queue_size", queueSize),
  logger.Int("max_queue_size", maxQueueSize),
  )

  // 可以考虑动态调整协程池大小
  // 或者实现负载均衡到其他协程池
  }
}
}

  // 自动扩容策略
  func autoScalePool(pool *async.WorkerPool) {
  stats := pool.GetStats()
  queueSize := stats["queue_size"].(int)
  activeWorkers := stats["active_workers"].(int)

  // 如果队列积压且工作协程全忙，考虑扩容
  if queueSize > 100 && activeWorkers == pool.config.MaxWorkers {
  // 记录日志，建议管理员调整配置
  logger.Warn("Pool may need scaling",
  logger.Int("queue_size", queueSize),
  logger.Int("active_workers", activeWorkers),
  logger.Int("max_workers", pool.config.MaxWorkers),
  )
}
}

  2. 内存泄漏检测
  go

  // 定期检查任务存储
  func checkMemoryLeak() {
  manager := async.GetTaskManager()
  defaultPool := manager.defaultPool
  taskStore := defaultPool.taskStore

  // 清理24小时前的已完成任务
  cleaned := taskStore.Cleanup(24 * time.Hour)
  if cleaned > 0 {
  logger.Info("Cleaned up expired tasks",
  logger.Int("count", cleaned),
  )
}

  // 检查任务存储大小
  stats := taskStore.GetStats()
  if stats.Total > 10000 {
  logger.Warn("Task store growing large",
  logger.Int64("total_tasks", stats.Total),
  logger.Int64("pending_tasks", stats.Pending),
  logger.Int64("completed_tasks", stats.Completed),
  )
}
}

  3. 死锁检测
  go

  // 监控工作协程状态
  func monitorWorkerHealth() {
  manager := async.GetTaskManager()

  for poolName, pool := range manager.pools {
  stats := pool.GetStats()
  activeWorkers := stats["active_workers"].(int)

  // 检查是否有工作协程长时间处于活动状态
  // 这可能表示死锁或长时间运行的任务
  if activeWorkers > 0 {
  // 实现更详细的监控逻辑
  logger.Debug("Worker health check",
  logger.String("pool", poolName),
  logger.Int("active_workers", activeWorkers),
  )
  }
}
}

  性能优化建议

  协程池大小调优：根据任务类型调整协程池大小，CPU密集型任务使用较小池，I/O密集型任务使用较大池。

  合理设置队列大小：根据内存容量和任务特点设置队列大小，避免内存溢出。

  使用适当的数据结构：优先队列使用二叉堆实现，确保插入和删除的复杂度为O(log n)。

  批量操作优化：对于大量小任务，考虑批量提交以提高效率。

  内存复用：对于频繁创建的任务对象，考虑使用对象池。

  异步I/O：在任务处理函数中使用异步I/O，避免阻塞工作协程。

  监控和告警：设置关键指标的告警阈值，如队列长度、任务失败率等。

  这个异步任务组件提供了完整的生产级解决方案，支持高并发、高可靠性的异步任务处理。您可以根据实际需求调整配置和使用方式。

  ## 文件上传组件 (pkg/upload)

  ### 特性
         - ✅ **多存储后端**：支持本地存储和阿里云OSS，易于扩展其他存储
         - ✅ **文件类型验证**：基于扩展名、MIME类型和文件类型的完整验证
         - ✅ **病毒扫描集成**：支持病毒扫描接口，可集成ClamAV等扫描引擎
         - ✅ **分片上传支持**：大文件分片上传，支持断点续传和进度查询
         - ✅ **CDN集成**：支持CDN加速，自动生成CDN URL
         - ✅ **安全性**：文件名安全检查，防止目录遍历攻击
         - ✅ **高性能**：流式处理，内存高效，支持并发上传
         - ✅ **可配置性**：灵活的配置选项，支持多种上传场景

  ### 快速开始

  #### 1. 基本配置

  ```yaml
  # config/config.yaml
         upload:
           # 存储配置
           storage_type: "local"  # local 或 oss
           base_path: "./uploads" # 本地存储基础路径
           base_url: "http://localhost:8080/uploads" # 基础访问URL
           cdn_url: "" # CDN URL（可选）

           # OSS配置（如果使用OSS）
           access_key_id: ""
           access_key_secret: ""
           endpoint: "oss-cn-hangzhou.aliyuncs.com"
           bucket: "my-bucket"
           region: "cn-hangzhou"

           # 验证配置
           allowed_types: ["image", "document"] # 允许的文件类型
           allowed_extensions: ["jpg", "jpeg", "png", "pdf", "doc", "docx"]
           allowed_mime_types: ["image/jpeg", "image/png", "application/pdf"]
           max_file_size: 10485760 # 10MB
           min_file_size: 1024     # 1KB
           max_file_name_length: 255
           validate_virus: false   # 是否启用病毒扫描
           scan_timeout: 30        # 病毒扫描超时（秒）

           # 分片上传配置
           chunk_enabled: true     # 是否启用分片上传
           chunk_size: 5242880    # 分片大小（5MB）
           max_chunks: 1000       # 最大分片数
           temp_dir: "/tmp/uploads" # 临时目录
           cleanup_interval: 1800 # 清理间隔（秒）
           max_temp_file_age: 86400 # 临时文件最大年龄（秒）
           enable_resumable: true # 是否启用断点续传

           # 其他配置
           enable_virus_scan: false # 是否启用病毒扫描
           use_https: true        # 是否使用HTTPS
           enable_cdn: false      # 是否启用CDN

  2. 初始化上传管理器
  go

  import (
  "dgou/pkg/upload"
  "dgou/pkg/config"
  )

  func main() {
  // 加载配置
  cfg := config.LoadConfig()

  // 初始化上传管理器
  manager, err := upload.InitUploadManager(cfg)
  if err != nil {
  log.Fatal(err)
  }
  defer manager.Stop()

  // 创建上传处理器
  handler := upload.NewUploadHandler(manager)

  // 在Gin中注册路由
  router := gin.Default()
  handler.RegisterRoutes(router.Group("/api"))
}

  3. 上传单个文件
  go

  // 使用Gin处理文件上传
  func uploadHandler(c *gin.Context) {
  // 获取文件
  file, err := c.FormFile("file")
  if err != nil {
         c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
  return
}

  // 获取上传管理器
  manager := upload.GetUploadManager()

  // 配置上传选项
  options := &upload.UploadOptions{
         UploaderID:   c.GetString("user_id"),
         UploaderIP:   c.ClientIP(),
         IsPublic:     true,
         Category:     "avatar",
         SubDirectory: "users",
}

  // 上传文件
  fileInfo, err := manager.Upload(c.Request.Context(), file, options)
  if err != nil {
         c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  return
}

  c.JSON(http.StatusOK, gin.H{
         "message": "File uploaded successfully",
         "file":    fileInfo,
         "url":     fileInfo.URL,
})
}

  4. 上传多个文件
  go

  func uploadMultipleHandler(c *gin.Context) {
  form, err := c.MultipartForm()
  if err != nil {
         c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form"})
  return
}

  files := form.File["files"]
  manager := upload.GetUploadManager()
  options := &upload.UploadOptions{
         UploaderID:   c.GetString("user_id"),
         UploaderIP:   c.ClientIP(),
         Category:     "documents",
}

  results := make([]gin.H, 0)
  errors := make([]string, 0)

  for _, file := range files {
  fileInfo, err := manager.Upload(c.Request.Context(), file, options)
  if err != nil {
         errors = append(errors, fmt.Sprintf("%s: %v", file.Filename, err))
  continue
}

  results = append(results, gin.H{
         "filename": file.Filename,
         "url":      fileInfo.URL,
         "size":     fileInfo.Size,
})
}

  c.JSON(http.StatusOK, gin.H{
         "success": results,
         "errors":  errors,
         "count":   len(results),
})
}

  高级用法
  1. 分片上传（大文件）
  go

  // 前端JavaScript示例
  const uploadLargeFile = async (file) => {
  const chunkSize = 5 * 1024 * 1024; // 5MB
  const totalChunks = Math.ceil(file.size / chunkSize);

  // 1. 开始上传会话
  const startResponse = await fetch('/api/upload/chunk/start', {
         method: 'POST',
         headers: {'Content-Type': 'application/json'},
         body: JSON.stringify({
           file_name: file.name,
           file_size: file.size,
           category: 'videos'
})
});

  const { upload_id, total_chunks, chunk_size } = await startResponse.json();

  // 2. 上传所有分片
  for (let chunkNumber = 1; chunkNumber <= totalChunks; chunkNumber++) {
  const start = (chunkNumber - 1) * chunkSize;
  const end = Math.min(start + chunkSize, file.size);
  const chunk = file.slice(start, end);

  const formData = new FormData();
  formData.append('chunk', chunk);

  await fetch(`/api/upload/chunk/${upload_id}/chunk/${chunkNumber}`, {
         method: 'POST',
         body: formData
});

  // 更新进度
  const progress = (chunkNumber / totalChunks) * 100;
         console.log(`Upload progress: ${progress.toFixed(2)}%`);
}

  // 3. 完成上传
  const completeResponse = await fetch(`/api/upload/chunk/${upload_id}/complete`, {
         method: 'POST'
});

  const result = await completeResponse.json();
  console.log('Upload completed:', result);
};

  // Go后端处理
  func handleChunkUpload(c *gin.Context) {
  // 处理器已内置，直接使用即可
}

  2. 文件验证和病毒扫描
  go

  // 自定义验证器
  func createCustomValidator() *upload.Validator {
  config := &upload.ValidationConfig{
         AllowedTypes: []upload.FileType{
                         upload.FileTypeImage,
                         upload.FileTypeDocument,
                         upload.FileTypeVideo,
  },
         AllowedExtensions: []string{
                              "jpg", "jpeg", "png", "gif", "pdf", "doc", "docx", "mp4", "avi",
  },
         MaxFileSize: 100 * 1024 * 1024, // 100MB
         ValidateVirus: true,
         ScanTimeout: 60,
}

  return upload.NewValidator(config)
}

  // 集成ClamAV病毒扫描
  func integrateClamAV() {
  validator := createCustomValidator()

  // 重写ScanVirus方法
  type CustomValidator struct {
  *upload.Validator
}

         customValidator := &CustomValidator{Validator: validator}

  // 实现ClamAV扫描
  func (v *CustomValidator) ScanVirus(filePath string) (bool, string, error) {
  // 调用ClamAV进行扫描
  // 这里需要安装并运行clamd服务
  cmd := exec.Command("clamdscan", "--no-summary", filePath)
  output, err := cmd.CombinedOutput()

  if err != nil {
  // 检查是否为病毒检测错误
  if strings.Contains(string(output), "FOUND") {
  return false, "Virus detected", nil
  }
  return false, "Scan failed", err
  }

  return true, "Clean", nil
}
}

  3. CDN集成
  go

  // 配置CDN
  func configureCDN() {
  config := &upload.StorageConfig{
         Type:        upload.StorageTypeOSS,
         Endpoint:    "oss-cn-hangzhou.aliyuncs.com",
         Bucket:      "my-bucket",
         CDNURL:      "cdn.example.com", // CDN域名
         EnableCDN:   true,
         UseHTTPS:    true,
}

  // 使用CDN后，文件URL会自动使用CDN域名
  // 例如：https://cdn.example.com/images/avatar.jpg
}

  // 动态切换CDN
  func getFileURLWithCDN(manager *upload.UploadManager, path string, useCDN bool) (string, error) {
  // 临时启用/禁用CDN
  originalCDN := manager.config.StorageConfig.EnableCDN
  manager.config.StorageConfig.EnableCDN = useCDN

  url, err := manager.GetFileURL(context.Background(), path, true)

  // 恢复原设置
  manager.config.StorageConfig.EnableCDN = originalCDN

  return url, err
}

  4. 文件处理（缩略图、水印等）
  go

  // 图片处理示例
  func processImage(fileInfo *upload.FileInfo) error {
  // 读取图片
  reader, err := uploadManager.GetFile(context.Background(), fileInfo.Path)
  if err != nil {
  return err
}
  defer reader.Close()

  // 解码图片
  img, _, err := image.Decode(reader)
  if err != nil {
  return err
}

  // 生成缩略图
  thumbnail := resize.Thumbnail(200, 200, img, resize.Lanczos3)

  // 保存缩略图
  thumbnailPath := fmt.Sprintf("%s_thumb.jpg", strings.TrimSuffix(fileInfo.Path, filepath.Ext(fileInfo.Path)))

  thumbnailFile, err := os.Create(thumbnailPath)
  if err != nil {
  return err
}
  defer thumbnailFile.Close()

         if err := jpeg.Encode(thumbnailFile, thumbnail, &jpeg.Options{Quality: 85}); err != nil {
           return err
}

  // 上传缩略图到存储
  thumbnailReader, _ := os.Open(thumbnailPath)
  defer thumbnailReader.Close()
  defer os.Remove(thumbnailPath)

  thumbnailInfo := &upload.FileInfo{
         ID:          uuid.New().String(),
         Name:        fmt.Sprintf("%s_thumb%s", strings.TrimSuffix(fileInfo.Name, filepath.Ext(fileInfo.Name)), filepath.Ext(fileInfo.Name)),
         StorageName: fmt.Sprintf("%s_thumb%s", strings.TrimSuffix(fileInfo.StorageName, filepath.Ext(fileInfo.StorageName)), filepath.Ext(fileInfo.StorageName)),
         Path:        thumbnailPath,
         MimeType:    "image/jpeg",
         Size:        0, // 实际大小需要计算
}

  return uploadManager.storage.Put(context.Background(), thumbnailInfo, thumbnailReader)
}

  5. 权限控制
  go

  // 基于角色的文件访问控制
  func checkFilePermission(c *gin.Context, filePath string) bool {
  userRole := c.GetString("user_role")

  // 解析文件路径获取分类
  parts := strings.Split(filePath, "/")
  if len(parts) == 0 {
  return false
}

  category := parts[0]

  // 定义权限规则
  permissionRules := map[string][]string{
         "admin":      {"*"},                     // 管理员可以访问所有
         "user":       {"avatar", "documents"},   // 普通用户可以访问头像和文档
         "guest":      {"public"},                // 访客只能访问公开文件
  }

  // 检查权限
  allowedCategories, exists := permissionRules[userRole]
  if !exists {
  return false
  }

  // 检查通配符
  for _, allowed := range allowedCategories {
  if allowed == "*" || allowed == category {
  return true
  }
  }

  return false
}

  // 私有文件访问
  func servePrivateFile(c *gin.Context) {
  filePath := c.Param("path")

  // 检查权限
  if !checkFilePermission(c, filePath) {
         c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
  return
}

  // 生成临时访问URL（带签名）
  manager := upload.GetUploadManager()
  url, err := manager.GetFileURL(c.Request.Context(), filePath, false)
  if err != nil {
         c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  return
}

  // 重定向到签名URL
  c.Redirect(http.StatusFound, url)
}

  6. 文件元数据管理
  go

  // 添加上下文元数据
  func uploadWithMetadata(c *gin.Context) {
  file, _ := c.FormFile("file")

  options := &upload.UploadOptions{
         UploaderID: c.GetString("user_id"),
         Metadata: map[string]interface{}{
           "upload_context": c.Query("context"),
           "tags":           strings.Split(c.Query("tags"), ","),
           "description":    c.Query("description"),
           "custom_field":   c.Query("custom_field"),
           "timestamp":      time.Now().Unix(),
},
}

  manager := upload.GetUploadManager()
  fileInfo, _ := manager.Upload(c.Request.Context(), file, options)

  // 保存元数据到数据库
  saveFileMetadataToDB(fileInfo)
}

  // 文件搜索和过滤
  func searchFiles(c *gin.Context) {
  query := c.Query("q")
  category := c.Query("category")
  startDate := c.Query("start_date")
  endDate := c.Query("end_date")

  // 从数据库查询文件元数据
  files := queryFileMetadataFromDB(query, category, startDate, endDate)

  c.JSON(http.StatusOK, gin.H{
         "files": files,
         "count": len(files),
})
}

  配置说明
  存储配置
  yaml

         upload:
           # 本地存储配置
           storage_type: "local"
           base_path: "./uploads"           # 存储根目录
           base_url: "http://localhost:8080/uploads"

           # OSS存储配置
           storage_type: "oss"
           access_key_id: "your-access-key-id"
           access_key_secret: "your-access-key-secret"
           endpoint: "oss-cn-hangzhou.aliyuncs.com"
           bucket: "your-bucket-name"
           region: "cn-hangzhou"
           use_https: true

           # CDN配置
           cdn_url: "https://cdn.example.com"
           enable_cdn: true

  验证配置
  yaml

         upload:
           validation:
             # 文件类型限制
             allowed_types: ["image", "document", "video"]

             # 扩展名限制
             allowed_extensions: ["jpg", "png", "pdf", "mp4"]

             # MIME类型限制
             allowed_mime_types: ["image/jpeg", "image/png", "application/pdf", "video/mp4"]

             # 大小限制
             max_file_size: 104857600  # 100MB
             min_file_size: 1024       # 1KB

             # 文件名限制
             max_file_name_length: 255

             # 病毒扫描
             validate_virus: true
             scan_timeout: 30

  分片上传配置
  yaml

         upload:
           chunk:
             enabled: true
             chunk_size: 5242880      # 5MB
             max_chunks: 1000
             temp_dir: "/tmp/uploads"
             cleanup_interval: 1800   # 30分钟
             max_temp_file_age: 86400 # 24小时
             enable_resumable: true   # 断点续传

  最佳实践
  1. 安全性建议
  go

  // 1. 文件名安全处理
  func sanitizeFilename(filename string) string {
  // 移除路径分隔符
  filename = filepath.Base(filename)

  // 移除危险字符
  dangerousChars := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\x00"}
  for _, char := range dangerousChars {
  filename = strings.ReplaceAll(filename, char, "")
}

  // 限制长度
  if len(filename) > 255 {
  ext := filepath.Ext(filename)
  name := filename[:255-len(ext)]
  filename = name + ext
}

  return filename
}

  // 2. 文件类型白名单
  func validateFileTypeByContent(fileHeader *multipart.FileHeader) error {
  // 读取文件前512字节检测真实类型
  file, err := fileHeader.Open()
  if err != nil {
  return err
}
  defer file.Close()

  buffer := make([]byte, 512)
  n, err := file.Read(buffer)
  if err != nil && err != io.EOF {
  return err
}

  // 检测MIME类型
  mimeType := http.DetectContentType(buffer[:n])

  // 验证允许的MIME类型
  allowedMimes := []string{"image/jpeg", "image/png", "application/pdf"}
  for _, allowed := range allowedMimes {
  if mimeType == allowed {
  return nil
  }
}

  return errors.New("File type not allowed")
}

  // 3. 文件大小限制
  func limitFileSize(c *gin.Context) {
  // 设置请求体大小限制
  c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 100<<20) // 100MB

  // 在Gin中间件中限制
  router.MaxMultipartMemory = 8 << 20 // 8MB
}

  2. 性能优化
  go

  // 1. 使用缓冲区池
  var bufferPool = sync.Pool{
         New: func() interface{} {
           return make([]byte, 32*1024) // 32KB缓冲区
},
}

  func uploadWithBufferPool(file io.Reader) error {
  buffer := bufferPool.Get().([]byte)
  defer bufferPool.Put(buffer)

  for {
  n, err := file.Read(buffer)
  if err != nil && err != io.EOF {
  return err
  }
  if n == 0 {
  break
  }
  // 处理数据...
}
  return nil
}

  // 2. 并发上传控制
  type UploadLimiter struct {
  semaphore chan struct{}
}

  func NewUploadLimiter(maxConcurrent int) *UploadLimiter {
  return &UploadLimiter{
         semaphore: make(chan struct{}, maxConcurrent),
}
}

  func (ul *UploadLimiter) Upload(file *multipart.FileHeader) error {
  ul.semaphore <- struct{}{}
  defer func() { <-ul.semaphore }()

  // 执行上传
  return doUpload(file)
}

  // 3. 内存优化
  func streamUpload(c *gin.Context) {
  // 使用流式处理，避免内存中保存整个文件
  file, _ := c.FormFile("file")
  src, _ := file.Open()
  defer src.Close()

  // 直接流式传输到存储
  dst, _ := os.CreateTemp("", "upload-*")
  defer dst.Close()
  defer os.Remove(dst.Name())

  if _, err := io.Copy(dst, src); err != nil {
         c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  return
}
}

  3. 错误处理和重试
  go

  // 1. 上传重试机制
  func uploadWithRetry(manager *upload.UploadManager, file *multipart.FileHeader, options *upload.UploadOptions, maxRetries int) (*upload.FileInfo, error) {
  var lastErr error

  for i := 0; i < maxRetries; i++ {
  fileInfo, err := manager.Upload(context.Background(), file, options)
  if err == nil {
  return fileInfo, nil
}

  lastErr = err

  // 指数退避
  delay := time.Duration(math.Pow(2, float64(i))) * time.Second
  time.Sleep(delay)

  // 重新打开文件（如果需要）
  if i < maxRetries-1 {
  // 重新获取文件句柄
}
}

         return nil, fmt.Errorf("upload failed after %d retries: %v", maxRetries, lastErr)
}

  // 2. 监控和日志
  func uploadWithMonitoring(c *gin.Context) {
  startTime := time.Now()

  file, _ := c.FormFile("file")

  // 记录上传开始
  logger.Info("Upload started",
  logger.String("filename", file.Filename),
  logger.Int64("filesize", file.Size),
  logger.String("client_ip", c.ClientIP()),
  )

  // 执行上传
  fileInfo, err := uploadManager.Upload(c.Request.Context(), file, &upload.UploadOptions{})

  duration := time.Since(startTime)

  if err != nil {
  // 记录失败
  logger.Error("Upload failed",
  logger.String("filename", file.Filename),
  logger.Duration("duration", duration),
  logger.ErrorField(err),
  )
         c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
  return
}

  // 记录成功
  logger.Info("Upload completed",
  logger.String("filename", file.Filename),
  logger.String("file_id", fileInfo.ID),
  logger.Int64("filesize", fileInfo.Size),
  logger.Duration("duration", duration),
  logger.Float64("speed_mbps", float64(fileInfo.Size)/duration.Seconds()/1024/1024),
  )

         c.JSON(http.StatusOK, gin.H{"file": fileInfo})
}

  4. 存储策略
  go

  // 1. 多存储后端切换
  type MultiStorageManager struct {
  primary   upload.Storage
  secondary upload.Storage
  usePrimary bool
  mu        sync.RWMutex
}

  func (m *MultiStorageManager) Put(ctx context.Context, file *upload.FileInfo, src io.Reader) error {
  m.mu.RLock()
  storage := m.primary
  if !m.usePrimary {
  storage = m.secondary
}
  m.mu.RUnlock()

  err := storage.Put(ctx, file, src)
  if err != nil && m.usePrimary {
  // 主存储失败，切换到备用存储
  m.mu.Lock()
  m.usePrimary = false
  m.mu.Unlock()

  logger.Warn("Primary storage failed, switching to secondary",
  logger.String("file_id", file.ID),
  logger.ErrorField(err),
  )

  return m.secondary.Put(ctx, file, src)
}

  return err
}

  // 2. 存储分层策略
  type TieredStorage struct {
  hotStorage  upload.Storage  // 热存储（SSD/内存）
  warmStorage upload.Storage  // 温存储（普通磁盘）
  coldStorage upload.Storage  // 冷存储（对象存储/磁带）
}

  func (t *TieredStorage) Put(ctx context.Context, file *upload.FileInfo, src io.Reader) error {
  // 根据文件类型和大小选择存储层
  if file.Size < 10*1024*1024 { // 小于10MB
  return t.hotStorage.Put(ctx, file, src)
} else if file.Size < 100*1024*1024 { // 小于100MB
  return t.warmStorage.Put(ctx, file, src)
} else {
  return t.coldStorage.Put(ctx, file, src)
}
}

  // 3. 自动清理旧文件
  func autoCleanup(manager *upload.UploadManager) {
  ticker := time.NewTicker(24 * time.Hour)
  defer ticker.Stop()

  for range ticker.C {
  // 清理30天前的临时文件
  cleanupTempFiles(30 * 24 * time.Hour)

  // 清理过期的用户上传文件
  cleanupExpiredFiles()

  // 清理未完成的断点上传
  cleanupAbandonedUploads(7 * 24 * time.Hour)
}
}

  故障排除
  1. 上传失败处理
  go

  // 常见错误处理
  func handleUploadError(err error) string {
  if err == nil {
  return "Success"
}

  // 检查错误类型
  if strings.Contains(err.Error(), "file size exceeds") {
  return "File too large. Please reduce file size."
}

  if strings.Contains(err.Error(), "invalid file type") {
  return "File type not allowed. Please upload allowed file types only."
}

  if strings.Contains(err.Error(), "virus detected") {
  return "File contains virus. Upload blocked for security."
}

  if strings.Contains(err.Error(), "disk full") {
  return "Storage space full. Please contact administrator."
}

  if strings.Contains(err.Error(), "network error") {
  return "Network error. Please check your connection and try again."
}

  if strings.Contains(err.Error(), "timeout") {
  return "Upload timeout. Please try again with smaller file or better network."
}

  return "Upload failed. Please try again later."
}

  // 重试逻辑
  func uploadWithSmartRetry(manager *upload.UploadManager, file *multipart.FileHeader, options *upload.UploadOptions) (*upload.FileInfo, error) {
  maxRetries := 3
  var lastErr error

  for attempt := 1; attempt <= maxRetries; attempt++ {
  fileInfo, err := manager.Upload(context.Background(), file, options)
  if err == nil {
  return fileInfo, nil
  }

  lastErr = err

  // 根据错误类型决定是否重试
  if shouldRetry(err) {
  // 等待重试
  delay := calculateRetryDelay(attempt)
  time.Sleep(delay)
  continue
  }

  // 不可重试的错误
  break
}

  return nil, lastErr
}

  func shouldRetry(err error) bool {
  // 网络错误、超时错误可以重试
  retryableErrors := []string{
  "network error",
  "timeout",
  "connection reset",
  "temporary failure",
}

  errStr := strings.ToLower(err.Error())
  for _, retryable := range retryableErrors {
  if strings.Contains(errStr, retryable) {
  return true
  }
}

  return false
}

  func calculateRetryDelay(attempt int) time.Duration {
  // 指数退避 + 随机抖动
  baseDelay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
  jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
  return baseDelay + jitter
}

  2. 性能问题诊断
  go

  // 性能监控
  type UploadMetrics struct {
  TotalRequests   int64
  SuccessfulUploads int64
  FailedUploads   int64
  TotalBytes      int64
  AverageDuration time.Duration
  ErrorRate       float64
}

  func monitorUploadPerformance() {
  metrics := &UploadMetrics{}
  startTime := time.Now()

  // 在中间件中收集指标
  router.Use(func(c *gin.Context) {
  if strings.Contains(c.Request.URL.Path, "/upload") {
  atomic.AddInt64(&metrics.TotalRequests, 1)
  uploadStart := time.Now()

  c.Next()

  duration := time.Since(uploadStart)

  if c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
  atomic.AddInt64(&metrics.SuccessfulUploads, 1)
  } else {
  atomic.AddInt64(&metrics.FailedUploads, 1)
  }

  // 更新平均持续时间
  oldAvg := metrics.AverageDuration
  count := metrics.SuccessfulUploads + metrics.FailedUploads
  metrics.AverageDuration = time.Duration(
  (float64(oldAvg)*float64(count-1) + float64(duration)) / float64(count),
  )
  }
})

  // 定期报告指标
  go func() {
  ticker := time.NewTicker(30 * time.Second)
  defer ticker.Stop()

  for range ticker.C {
  total := metrics.TotalRequests
  success := metrics.SuccessfulUploads
  failed := metrics.FailedUploads

  errorRate := 0.0
  if total > 0 {
  errorRate = float64(failed) / float64(total) * 100
  }

  logger.Info("Upload performance metrics",
  logger.Int64("total_requests", total),
  logger.Int64("successful", success),
  logger.Int64("failed", failed),
  logger.Float64("error_rate", errorRate),
  logger.Duration("avg_duration", metrics.AverageDuration),
  logger.Int64("total_bytes", metrics.TotalBytes),
  )
  }
}()
}

  这个文件上传组件提供了完整的生产级解决方案，支持多种存储后端、文件验证、病毒扫描、分片上传等功能。您可以根据实际需求调整配置和使用方式。

  ## 队列组件 (pkg/queue)

  ### 特性
         - ✅ **RabbitMQ连接管理**：自动重连、连接池、心跳检测
         - ✅ **消息确认机制**：手动/自动确认、批量确认、事务支持
         - ✅ **死信队列**：消息重试失败处理、死信路由、死信监控
         - ✅ **延迟队列**：延迟消息发送、基于TTL的延迟、延迟队列管理
         - ✅ **消息重试策略**：多种重试策略（固定、线性、指数、斐波那契）
         - ✅ **高可用性**：连接故障自动恢复、消息持久化、集群支持
         - ✅ **监控指标**：完整的性能指标和统计信息
         - ✅ **安全性**：TLS支持、连接认证、权限控制

  ### 快速开始

  #### 1. 基本配置

  ```yaml
  # config/config.yaml
         queue:
           # RabbitMQ连接配置
           rabbitmq:
             url: "amqp://guest:guest@localhost:5672/"
             host: "localhost"
             port: 5672
             username: "guest"
             password: "guest"
             vhost: "/"
             heartbeat: 30
             connection_name: "dgou-app"
             max_reconnect: 10
             reconnect_delay: 5s
             prefetch_count: 10
             prefetch_size: 0
             global_prefetch: false
             enable_tls: false

           # 死信队列配置
           enable_dead_letter: true
           dead_letter_exchange: "dlx.exchange"
           dead_letter_queue: "dlx.queue"
           dead_letter_routing_key: "#"
           dead_letter_ttl: 168h  # 7天
           dead_letter_max_length: 10000

           # 延迟队列配置
           enable_delayed_queue: true
           delayed_exchange: "delayed.exchange"
           delayed_queue_prefix: "delayed."
           delayed_max_delay: 720h  # 30天

           # 重试策略配置
           enable_retry: true
           max_retries: 3
           retry_initial_delay: 1s
           retry_max_delay: 60s
           retry_strategy: "exponential"
           retry_backoff_factor: 2.0
           retry_jitter: true
           retry_jitter_factor: 0.1

           # 监控配置
           enable_metrics: true

  2. 初始化队列管理器
  go

  import (
  "dgou/pkg/queue"
  "dgou/pkg/config"
  "context"
  )

  func main() {
  // 加载配置
  cfg := config.LoadConfig()

  // 初始化队列管理器
  queueManager, err := queue.InitQueueManager(cfg)
  if err != nil {
  log.Fatal(err)
  }
  defer queueManager.Stop()

  // 使用队列管理器...
}

  3. 发送消息
  go

  // 创建消息生产者
  func createProducer(manager *queue.QueueManager) *queue.Producer {
  return manager.GetProducer("order.exchange", "order.created")
}

  // 发送消息
  func sendOrderMessage(producer *queue.Producer, orderID string) error {
  // 创建消息
  msg, err := queue.CreateMessage(
  "order-created-"+orderID,
  map[string]interface{}{
         "order_id": orderID,
         "user_id":  "user-123",
         "amount":   99.99,
         "items":    []string{"item1", "item2"},
  },
  )
  if err != nil {
  return err
  }

  // 设置消息属性
  msg.DeliveryMode = queue.DeliveryModePersistent
  msg.Priority = 5
  msg.Headers = map[string]interface{}{
         "x-trace-id": "trace-123456",
         "x-user-id":  "user-123",
}

  // 发送消息
  ctx := context.Background()
  return producer.Publish(ctx, msg)
}

  // 发送延迟消息
  func sendDelayedMessage(manager *queue.QueueManager, msg *queue.Message, delay time.Duration) error {
  delayedMgr := manager.GetDelayedQueueManager()
  if delayedMgr == nil {
  return fmt.Errorf("delayed queue is disabled")
}

  ctx := context.Background()
  return delayedMgr.SendDelayed(ctx, msg, delay)
}

  4. 消费消息
  go

  // 定义消息处理器
  type OrderHandler struct{}

  func (h *OrderHandler) Handle(ctx context.Context, msg *queue.Message) error {
  // 解析消息
  var orderData map[string]interface{}
  if err := json.Unmarshal(msg.Body, &orderData); err != nil {
  return err
}

  // 处理订单
  orderID := orderData["order_id"].(string)
  logger.Info("Processing order",
  logger.String("order_id", orderID),
  logger.String("message_id", msg.ID),
  )

  // 执行业务逻辑
  if err := processOrder(orderData); err != nil {
  return err
}

  return nil
}

  // 创建消费者
  func createConsumer(manager *queue.QueueManager) error {
  handler := &OrderHandler{}

  options := &queue.ConsumerOptions{
         ConsumerTag: "order-processor",
         AutoAck:     false, // 手动确认
         Exclusive:   false,
}

  return manager.StartConsumer("order.queue", handler, options)
}

  高级用法
  1. 消息确认机制
  go

  // 手动确认模式
  type ManualAckHandler struct {
  maxRetries int
}

  func (h *ManualAckHandler) Handle(ctx context.Context, msg *queue.Message) error {
  // 获取投递对象
  delivery, _ := ctx.Value("delivery").(amqp.Delivery)

  // 执行业务逻辑
  err := processMessage(msg)

  // 根据处理结果确认消息
  if err != nil {
  // 处理失败，检查是否重试
  retryCount := msg.RetryCount
  if retryCount < h.maxRetries {
  // 需要重试，拒绝并重新入队
  if delivery != nil {
  delivery.Nack(false, true)
  }
  return err
} else {
  // 重试次数用尽，拒绝并丢弃
  if delivery != nil {
  delivery.Nack(false, false)
  }

  // 发送到死信队列
  deadLetterMgr := getDeadLetterManager()
  if deadLetterMgr != nil {
  deadLetterMgr.SendToDeadLetter(msg, "max retries exceeded")
  }
  return err
}
}

  // 处理成功，确认消息
  if delivery != nil {
  delivery.Ack(false)
}
  return nil
}

  // 批量确认模式
  type BatchAckHandler struct {
  batchSize int
  messages  []*queue.Message
  mu        sync.Mutex
}

  func (h *BatchAckHandler) Handle(ctx context.Context, msg *queue.Message) error {
  h.mu.Lock()
  h.messages = append(h.messages, msg)

  if len(h.messages) >= h.batchSize {
  // 批量处理
  if err := h.processBatch(); err != nil {
  h.mu.Unlock()
  return err
}

  // 批量确认
  if delivery, ok := ctx.Value("delivery").(amqp.Delivery); ok {
  delivery.Ack(true) // multiple=true 批量确认
}

  h.messages = nil
}
  h.mu.Unlock()

  return nil
}

  func (h *BatchAckHandler) processBatch() error {
  // 批量处理逻辑
  for _, msg := range h.messages {
  if err := processSingleMessage(msg); err != nil {
  return err
}
}
  return nil
}

  2. 死信队列管理
  go

  // 配置死信队列
  func setupDeadLetter(manager *queue.QueueManager) error {
  deadLetterMgr := manager.GetDeadLetterManager()
  if deadLetterMgr == nil {
  return fmt.Errorf("dead letter manager is not available")
}

  // 为队列配置死信
  if err := deadLetterMgr.SetupDeadLetterForQueue("order.queue", 3); err != nil {
  return err
}

  // 处理死信队列中的消息
  dlHandler := &DeadLetterHandler{}
  if err := deadLetterMgr.ProcessDeadLetter(dlHandler); err != nil {
  return err
}

  return nil
}

  // 死信处理器
  type DeadLetterHandler struct{}

  func (h *DeadLetterHandler) Handle(ctx context.Context, msg *queue.Message) error {
  // 记录死信消息
  reason, _ := msg.Headers["x-dead-letter-reason"].(string)
  timestamp, _ := msg.Headers["x-dead-letter-timestamp"].(string)

  logger.Error("Dead letter message received",
  logger.String("message_id", msg.ID),
  logger.String("reason", reason),
  logger.String("timestamp", timestamp),
  logger.Int("retry_count", msg.RetryCount),
  )

  // 可以在这里进行告警、日志记录、人工干预等
  sendAlert(msg, reason)

  // 或者尝试重新处理
  // return retryProcessing(msg)

  return nil // 确认消息，从死信队列中移除
}

  func sendAlert(msg *queue.Message, reason string) {
  // 发送告警通知
  alertData := map[string]interface{}{
         "type":        "dead_letter",
         "message_id":  msg.ID,
         "reason":      reason,
         "queue":       msg.Headers["x-original-queue"],
         "retry_count": msg.RetryCount,
         "timestamp":   time.Now().Format(time.RFC3339),
}

  // 发送到监控系统
  // ...
}

  3. 延迟队列应用
  go

  // 订单超时取消
  func scheduleOrderCancel(manager *queue.QueueManager, orderID string, timeout time.Duration) error {
  delayedMgr := manager.GetDelayedQueueManager()
  if delayedMgr == nil {
  return fmt.Errorf("delayed queue is disabled")
}

  // 创建取消消息
  msg, err := queue.CreateMessage(
  "order-cancel-"+orderID,
  map[string]interface{}{
         "order_id": orderID,
         "action":   "cancel",
         "reason":   "timeout",
},
  )
  if err != nil {
  return err
}

  // 发送延迟消息
  ctx := context.Background()
  return delayedMgr.SendDelayed(ctx, msg, timeout)
}

  // 处理延迟消息
  func processDelayedMessages(manager *queue.QueueManager) error {
  delayedMgr := manager.GetDelayedQueueManager()
  if delayedMgr == nil {
  return fmt.Errorf("delayed queue is disabled")
}

  // 消费延迟队列
  handler := &DelayedMessageHandler{}
  _, err := delayedMgr.ConsumeDelayed("delayed.order.cancel", handler)
  return err
}

  type DelayedMessageHandler struct{}

  func (h *DelayedMessageHandler) Handle(ctx context.Context, msg *queue.Message) error {
  // 解析消息
  var data map[string]interface{}
  if err := json.Unmarshal(msg.Body, &data); err != nil {
  return err
}

  action := data["action"].(string)
  orderID := data["order_id"].(string)

  switch action {
         case "cancel":
           return cancelOrder(orderID, data["reason"].(string))
         case "remind":
           return sendReminder(orderID)
         case "release":
           return releaseResources(orderID)
}

  return nil
}

  4. 消息重试策略
  go

  // 自定义重试策略
  func createCustomRetryStrategy() *queue.RetryConfig {
  return &queue.RetryConfig{
         Enabled:       true,
         MaxRetries:    5,
         InitialDelay:  1 * time.Second,
         MaxDelay:      60 * time.Second,
         Strategy:      queue.RetryStrategyExponential,
         BackoffFactor: 2.0,
         Jitter:        true,
         JitterFactor:  0.2,
}
}

  // 使用重试管理器
  func processWithRetry(manager *queue.QueueManager, operation func() error) error {
  retryMgr := manager.GetRetryManager()

  result, err := retryMgr.Execute(operation)
  if err != nil {
  logger.Error("Operation failed after retries",
  logger.Int("attempts", result.Attempts),
  logger.Duration("total_delay", result.TotalDelay),
  logger.ErrorField(err),
  )
  return err
}

  logger.Info("Operation succeeded",
  logger.Int("attempts", result.Attempts),
  logger.Duration("total_delay", result.TotalDelay),
  )

  return nil
}

  // 消息级别的重试控制
  type SmartRetryHandler struct {
  retryMgr *queue.RetryManager
}

  func (h *SmartRetryHandler) Handle(ctx context.Context, msg *queue.Message) error {
  // 检查是否应该重试
  if !h.retryMgr.ShouldRetry(ctx.Err(), msg.RetryCount) {
  // 不重试，发送到死信队列
  return fmt.Errorf("no retry allowed for this error")
}

  // 更新重试信息
  h.retryMgr.UpdateRetryHeaders(msg, msg.RetryCount+1)

  // 执行操作
  err := processMessage(msg)
  if err != nil {
  // 计算下一次重试延迟
  delay := h.retryMgr.GetRetryDelay(msg.RetryCount)

  logger.Warn("Message processing failed, will retry",
  logger.String("message_id", msg.ID),
  logger.Int("retry_count", msg.RetryCount),
  logger.Duration("next_retry_delay", delay),
  logger.ErrorField(err),
  )
}

  return err
}

  5. 事务消息
  go

  // 事务性生产者
  type TransactionalProducer struct {
  manager  *queue.QueueManager
  channel  *amqp.Channel
  txActive bool
}

  func NewTransactionalProducer(manager *queue.QueueManager) (*TransactionalProducer, error) {
  channel, err := manager.connection.GetChannel()
  if err != nil {
  return nil, err
}

  // 启用事务模式
  if err := channel.Tx(); err != nil {
  channel.Close()
  return nil, err
}

  return &TransactionalProducer{
         manager:  manager,
         channel:  channel,
         txActive: true,
}, nil
}

  func (p *TransactionalProducer) PublishInTransaction(ctx context.Context, exchange, routingKey string, msg *queue.Message) error {
  if !p.txActive {
  return fmt.Errorf("transaction is not active")
}

  publishing := amqp.Publishing{
         ContentType:  msg.ContentType,
         Body:         msg.Body,
         DeliveryMode: uint8(msg.DeliveryMode),
         Headers:      amqp.Table{},
}

  // 复制消息头
  for k, v := range msg.Headers {
  publishing.Headers[k] = v
}

  // 在事务中发布
  err := p.channel.PublishWithContext(ctx, exchange, routingKey, false, false, publishing)
  if err != nil {
  // 回滚事务
  p.channel.TxRollback()
  p.txActive = false
  return err
}

  return nil
}

  func (p *TransactionalProducer) Commit() error {
  if !p.txActive {
  return fmt.Errorf("transaction is not active")
}

  if err := p.channel.TxCommit(); err != nil {
  return err
}

  p.txActive = false
  return nil
}

  func (p *TransactionalProducer) Rollback() error {
  if !p.txActive {
  return nil
}

  if err := p.channel.TxRollback(); err != nil {
  return err
}

  p.txActive = false
  return nil
}

  func (p *TransactionalProducer) Close() error {
  return p.channel.Close()
}

  // 使用事务
  func processOrderWithTransaction(manager *queue.QueueManager, order *Order) error {
  // 创建事务生产者
  txProducer, err := NewTransactionalProducer(manager)
  if err != nil {
  return err
}
  defer txProducer.Close()

  // 开始数据库事务
  dbTx := beginDBTransaction()
  defer func() {
  if err != nil {
  dbTx.Rollback()
}
}()

  // 保存订单到数据库
  if err := saveOrder(dbTx, order); err != nil {
  txProducer.Rollback()
  return err
}

  // 发送订单创建消息（在事务中）
  msg, _ := queue.CreateMessage("order-created", order)
  if err := txProducer.PublishInTransaction(context.Background(),
  "order.exchange", "order.created", msg); err != nil {
  dbTx.Rollback()
  return err
}

  // 提交数据库事务
  if err := dbTx.Commit(); err != nil {
  txProducer.Rollback()
  return err
}

  // 提交消息事务
  if err := txProducer.Commit(); err != nil {
  // 这里需要补偿逻辑，因为数据库已提交但消息未发送
  // 可以记录日志、发送告警、尝试重新发送等
  logCompensatingAction(order, err)
  return err
}

  return nil
}

  6. 消息优先级
  go

  // 优先级队列
  func setupPriorityQueue(manager *queue.QueueManager) error {
  channel, err := manager.connection.GetChannel()
  if err != nil {
  return err
}

  // 声明支持优先级的队列
  args := amqp.Table{
         "x-max-priority": 10, // 支持10个优先级级别
}

  _, err = channel.QueueDeclare(
  "priority.queue",
  true,  // durable
  false, // autoDelete
  false, // exclusive
  false, // noWait
  args,
  )

  return err
}

  // 发送优先级消息
  func sendPriorityMessage(producer *queue.Producer, msg *queue.Message, priority uint8) error {
  // 设置消息优先级（0-9，0最低，9最高）
  msg.Priority = priority

  ctx := context.Background()
  return producer.Publish(ctx, msg)
}

  // 紧急订单处理
  func processUrgentOrder(manager *queue.QueueManager, order *UrgentOrder) error {
  // 创建高优先级消息
  msg, err := queue.CreateMessage("urgent-order", order)
  if err != nil {
  return err
}

  // 设置高优先级
  msg.Priority = 9 // 最高优先级

  producer := manager.GetProducer("order.exchange", "order.urgent")
  ctx := context.Background()

  // 使用带重试的发布
  return producer.PublishWithRetry(ctx, msg, 3, 1*time.Second)
}

  7. 消息路由和过滤
  go

  // 主题交换机路由
  func setupTopicRouting(manager *queue.QueueManager) error {
  // 声明主题交换机
  exchangeConfig := &queue.ExchangeConfig{
         Name:    "logs.topic",
         Type:    queue.QueueTypeTopic,
         Durable: true,
}

  if err := manager.DeclareExchange(exchangeConfig); err != nil {
  return err
}

  // 创建不同级别的日志队列
  queues := []struct {
  name       string
  routingKey string
}{
  {"logs.error", "logs.error.#"},
  {"logs.warn", "logs.warn.#"},
  {"logs.info", "logs.info.#"},
  {"logs.debug", "logs.debug.#"},
}

  for _, q := range queues {
  queueConfig := &queue.QueueConfig{
         Name:       q.name,
         Durable:    true,
         BindingKey: q.routingKey,
}

  if err := manager.DeclareQueue(queueConfig); err != nil {
  return err
}

  // 绑定到交换机
  if err := manager.BindQueue(q.name, "logs.topic", q.routingKey, nil); err != nil {
  return err
}
}

  return nil
}

  // 发送日志消息
  func sendLogMessage(manager *queue.QueueManager, level, service, message string) error {
  msg, err := queue.CreateMessage("log-message", map[string]string{
         "service": service,
         "message": message,
         "level":   level,
         "time":    time.Now().Format(time.RFC3339),
})
  if err != nil {
  return err
}

  // 根据级别选择路由键
  routingKey := fmt.Sprintf("logs.%s.%s", level, service)

  producer := manager.GetProducer("logs.topic", routingKey)
  ctx := context.Background()

  return producer.Publish(ctx, msg)
}

  // 头部队列过滤
  func setupHeadersRouting(manager *queue.QueueManager) error {
  // 声明头部队列
  args := amqp.Table{
         "x-match": "all", // 所有头部必须匹配
}

  // 创建区域特定的队列
  regions := []string{"us-east", "us-west", "eu-central", "ap-southeast"}

  for _, region := range regions {
  queueName := fmt.Sprintf("orders.%s", region)

  // 声明队列
  _, err := manager.connection.channel.QueueDeclare(
  queueName,
  true,
  false,
  false,
  false,
  nil,
  )
  if err != nil {
  return err
}

  // 使用头部绑定
  headers := amqp.Table{
         "region": region,
         "type":   "order",
}

  err = manager.connection.channel.QueueBind(
  queueName,
  "", // 路由键为空
  "orders.headers",
  false,
  headers,
  )
  if err != nil {
  return err
}
}

  return nil
}

  监控和管理
  1. 性能指标收集
  go

  // 监控队列性能
  type QueueMonitor struct {
  manager *queue.QueueManager
  ticker  *time.Ticker
}

  func NewQueueMonitor(manager *queue.QueueManager) *QueueMonitor {
  return &QueueMonitor{
         manager: manager,
         ticker:  time.NewTicker(30 * time.Second),
}
}

  func (m *QueueMonitor) Start() {
  go m.monitor()
}

  func (m *QueueMonitor) monitor() {
  for range m.ticker.C {
  metrics := m.manager.GetMetrics()

  // 记录指标
  logger.Info("Queue metrics",
  logger.Any("metrics", metrics),
  )

  // 检查健康状态
  m.checkHealth(metrics)

  // 发送到监控系统
  m.sendToMonitoringSystem(metrics)
}
}

  func (m *QueueMonitor) checkHealth(metrics map[string]interface{}) {
  // 检查连接状态
  connectionMetrics, _ := metrics["connection"].(*queue.ConnectionMetrics)
  if connectionMetrics != nil && connectionMetrics.Errors > 10 {
  logger.Warn("High error rate detected in queue connection",
  logger.Int64("errors", connectionMetrics.Errors),
  )
}

  // 检查消费者状态
  consumers, _ := metrics["consumers"].(map[string]interface{})
  for queueName, consumerMetrics := range consumers {
  if cm, ok := consumerMetrics.(*queue.ConsumerMetrics); ok {
  if cm.ProcessingErrors > 5 {
  logger.Warn("High processing error rate",
  logger.String("queue", queueName),
  logger.Int64("errors", cm.ProcessingErrors),
  )
  }
}
}
}

  func (m *QueueMonitor) sendToMonitoringSystem(metrics map[string]interface{}) {
  // 集成Prometheus、Datadog等监控系统
  // ...
}

  func (m *QueueMonitor) Stop() {
  m.ticker.Stop()
}

  2. 队列管理API
  go

  // 队列管理服务
  type QueueManagementService struct {
  manager *queue.QueueManager
}

  func NewQueueManagementService(manager *queue.QueueManager) *QueueManagementService {
  return &QueueManagementService{
         manager: manager,
}
}

  // 获取队列统计
  func (s *QueueManagementService) GetQueueStats(queueName string) (map[string]interface{}, error) {
  return s.manager.GetQueueStats(queueName)
}

  // 清空队列
  func (s *QueueManagementService) PurgeQueue(queueName string) (int, error) {
  return s.manager.PurgeQueue(queueName)
}

  // 重新投递死信消息
  func (s *QueueManagementService) RedeliverDeadLetter(queueName string, count int) (int, error) {
  deadLetterMgr := s.manager.GetDeadLetterManager()
  if deadLetterMgr == nil {
  return 0, fmt.Errorf("dead letter manager not available")
}

  // 从死信队列消费消息并重新投递
  // 实际实现需要从死信队列读取消息并重新发布到原始队列
  return 0, nil
}

  // 调整消费者预取值
  func (s *QueueManagementService) UpdatePrefetch(queueName string, prefetchCount int) error {
  // 停止现有消费者
  s.manager.StopConsumer(queueName)

  // 使用新的预取值重新启动消费者
  // 实际实现需要更新消费者配置并重新启动
  return nil
}

  3. 告警和通知
  go

  // 队列告警管理器
  type QueueAlertManager struct {
  manager   *queue.QueueManager
  alertCh   chan *QueueAlert
  thresholds map[string]AlertThreshold
}

  type QueueAlert struct {
  Type      string                 `json:"type"`
  Queue     string                 `json:"queue"`
  Severity  string                 `json:"severity"` // critical, warning, info
  Message   string                 `json:"message"`
  Metrics   map[string]interface{} `json:"metrics"`
  Timestamp time.Time              `json:"timestamp"`
}

  type AlertThreshold struct {
  MaxQueueLength    int           `json:"max_queue_length"`
  MaxConsumerLag    time.Duration `json:"max_consumer_lag"`
  MaxErrorRate      float64       `json:"max_error_rate"`
  MinConsumerCount  int           `json:"min_consumer_count"`
}

  func NewQueueAlertManager(manager *queue.QueueManager) *QueueAlertManager {
  return &QueueAlertManager{
         manager: manager,
         alertCh: make(chan *QueueAlert, 100),
         thresholds: map[string]AlertThreshold{
           "default": {
             MaxQueueLength:   10000,
             MaxConsumerLag:   5 * time.Minute,
             MaxErrorRate:     0.05, // 5%
             MinConsumerCount: 1,
           },
},
}
}

  func (a *QueueAlertManager) Start() {
  go a.monitorAlerts()
  go a.processAlerts()
}

  func (a *QueueAlertManager) monitorAlerts() {
  ticker := time.NewTicker(1 * time.Minute)
  defer ticker.Stop()

  for range ticker.C {
  a.checkQueueAlerts()
  a.checkConnectionAlerts()
  a.checkConsumerAlerts()
}
}

  func (a *QueueAlertManager) checkQueueAlerts() {
  // 获取所有队列状态
  // 检查队列长度、消费者数量、消息积压等
  // 触发相应的告警
}

  func (a *QueueAlertManager) checkConnectionAlerts() {
  metrics := a.manager.connection.GetMetrics()

  // 检查连接错误率
  if metrics.Errors > 10 {
  a.alertCh <- &QueueAlert{
         Type:      "connection_error",
         Severity:  "warning",
         Message:   fmt.Sprintf("High connection error rate: %d errors", metrics.Errors),
         Metrics:   map[string]interface{}{"errors": metrics.Errors},
         Timestamp: time.Now(),
}
}

  // 检查最近连接时间
  if time.Since(metrics.LastConnectTime) > 5*time.Minute {
  a.alertCh <- &QueueAlert{
         Type:      "connection_stale",
         Severity:  "critical",
         Message:   "No recent connection activity",
         Metrics:   map[string]interface{}{"last_connect": metrics.LastConnectTime},
         Timestamp: time.Now(),
}
}
}

  func (a *QueueAlertManager) processAlerts() {
  for alert := range a.alertCh {
  // 记录日志
  logAlert(alert)

  // 发送通知
  sendNotification(alert)

  // 触发自动化操作
  triggerAutomation(alert)
}
}

  func logAlert(alert *QueueAlert) {
  switch alert.Severity {
         case "critical":
           logger.Error("Queue alert",
           logger.String("type", alert.Type),
           logger.String("queue", alert.Queue),
           logger.String("severity", alert.Severity),
           logger.String("message", alert.Message),
           logger.Any("metrics", alert.Metrics),
           )
         case "warning":
           logger.Warn("Queue alert",
           logger.String("type", alert.Type),
           logger.String("queue", alert.Queue),
           logger.String("severity", alert.Severity),
           logger.String("message", alert.Message),
           )
         default:
           logger.Info("Queue alert",
           logger.String("type", alert.Type),
           logger.String("queue", alert.Queue),
           logger.String("severity", alert.Severity),
           logger.String("message", alert.Message),
           )
}
}

  func sendNotification(alert *QueueAlert) {
  // 发送到Slack、Email、短信等
  // ...
}

  func triggerAutomation(alert *QueueAlert) {
  // 触发自动化操作，如自动扩容、重启消费者等
  // ...
}

  最佳实践
  1. 连接管理最佳实践
  go

  // 连接池管理
  type ConnectionPool struct {
  config     *queue.RabbitMQConfig
  connections []*queue.Connection
  mu         sync.RWMutex
  maxPoolSize int
}

  func NewConnectionPool(config *queue.RabbitMQConfig, maxPoolSize int) *ConnectionPool {
  return &ConnectionPool{
         config:      config,
         maxPoolSize: maxPoolSize,
         connections: make([]*queue.Connection, 0, maxPoolSize),
}
}

  func (p *ConnectionPool) GetConnection() (*queue.Connection, error) {
  p.mu.Lock()
  defer p.mu.Unlock()

  // 查找可用的连接
  for _, conn := range p.connections {
  if conn.IsConnected() {
  return conn, nil
}
}

  // 创建新连接
  if len(p.connections) < p.maxPoolSize {
  conn := queue.NewConnection(p.config)
  if err := conn.Connect(); err != nil {
  return nil, err
}
  p.connections = append(p.connections, conn)
  return conn, nil
}

  // 连接池已满，等待或返回错误
  return nil, fmt.Errorf("connection pool exhausted")
}

  // 优雅关闭
  func gracefulShutdown(manager *queue.QueueManager) {
  // 停止接收新消息
  logger.Info("Stopping queue consumers...")

  // 等待正在处理的消息完成
  time.Sleep(30 * time.Second)

  // 停止队列管理器
  if err := manager.Stop(); err != nil {
  logger.Error("Failed to stop queue manager",
  logger.ErrorField(err),
  )
}

  logger.Info("Queue manager stopped gracefully")
}

  2. 消息设计最佳实践
  go

  // 消息信封模式
  type MessageEnvelope struct {
  ID        string                 `json:"id"`
  Type      string                 `json:"type"`
  Version   string                 `json:"version"`
  Timestamp time.Time              `json:"timestamp"`
  Source    string                 `json:"source"`
  Payload   map[string]interface{} `json:"payload"`
  Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

  func CreateEnvelopedMessage(msgType, source string, payload interface{}) (*queue.Message, error) {
  envelope := &MessageEnvelope{
         ID:        uuid.New().String(),
         Type:      msgType,
         Version:   "1.0",
         Timestamp: time.Now(),
         Source:    source,
         Payload:   make(map[string]interface{}),
}

  // 序列化payload
  if payload != nil {
  data, err := json.Marshal(payload)
  if err != nil {
  return nil, err
}

  var payloadMap map[string]interface{}
  if err := json.Unmarshal(data, &payloadMap); err != nil {
  return nil, err
}
  envelope.Payload = payloadMap
}

  // 创建队列消息
  return queue.CreateMessage(envelope.ID, envelope)
}

  // 消息版本控制
  type MessageVersion string

  const (
  MessageVersionV1 MessageVersion = "v1"
  MessageVersionV2 MessageVersion = "v2"
  )

  func CreateVersionedMessage(version MessageVersion, payload interface{}) (*queue.Message, error) {
  var exchange, routingKey string

  switch version {
         case MessageVersionV1:
           exchange = "messages.v1"
           routingKey = "message.v1"
         case MessageVersionV2:
           exchange = "messages.v2"
           routingKey = "message.v2"
         default:
           return nil, fmt.Errorf("unsupported message version: %s", version)
}

  msg, err := queue.CreateMessage(uuid.New().String(), payload)
  if err != nil {
  return nil, err
}

  // 添加版本信息
  msg.Headers = map[string]interface{}{
         "x-version": string(version),
}

  return msg, nil
}

  3. 错误处理和恢复
  go

  // 错误恢复策略
  type ErrorRecoveryStrategy struct {
  manager *queue.QueueManager
}

  func (s *ErrorRecoveryStrategy) RecoverFromError(err error, msg *queue.Message) error {
  // 根据错误类型采取不同的恢复策略
  if isNetworkError(err) {
  return s.recoverFromNetworkError(err, msg)
}

  if isResourceError(err) {
  return s.recoverFromResourceError(err, msg)
}

  if isBusinessError(err) {
  return s.recoverFromBusinessError(err, msg)
}

  // 默认恢复策略
  return s.defaultRecovery(err, msg)
}

  func (s *ErrorRecoveryStrategy) recoverFromNetworkError(err error, msg *queue.Message) error {
  // 网络错误：等待并重试
  logger.Warn("Network error detected, waiting before retry",
  logger.String("message_id", msg.ID),
  logger.ErrorField(err),
  )

  time.Sleep(5 * time.Second)

  // 重新发布消息
  producer := s.manager.GetProducer(msg.Exchange, msg.RoutingKey)
  ctx := context.Background()
  return producer.PublishWithRetry(ctx, msg, 3, 1*time.Second)
}

  func (s *ErrorRecoveryStrategy) recoverFromResourceError(err error, msg *queue.Message) error {
  // 资源错误：发送到死信队列并告警
  deadLetterMgr := s.manager.GetDeadLetterManager()
  if deadLetterMgr != nil {
         deadLetterMgr.SendToDeadLetter(msg, "resource error: "+err.Error())
    }
    
    // 发送告警
    sendResourceAlert(err, msg)
    
    return nil // 错误已处理
}

func (s *ErrorRecoveryStrategy) defaultRecovery(err error, msg *queue.Message) error {
    // 默认恢复：记录日志并发送到死信队列
    logger.Error("Unrecoverable error",
  logger.String("message_id", msg.ID),
  logger.ErrorField(err),
  )

  deadLetterMgr := s.manager.GetDeadLetterManager()
  if deadLetterMgr != nil {
         deadLetterMgr.SendToDeadLetter(msg, "unrecoverable error: "+err.Error())
    }
    
    return nil
}

这个队列组件提供了完整的生产级 RabbitMQ 解决方案，包含连接管理、消息确认、死信队列、延迟队列和消息重试策略等核心功能。组件设计为纯业务逻辑，不包含任何 Web 框架依赖，可以在任何 Go 项目中使用。

## 9. 监控组件 (pkg/monitor)

### 特性
- ✅ **Prometheus指标收集**：完整的HTTP、数据库、缓存、业务指标
- ✅ **分布式追踪**：OpenTelemetry集成，支持Jaeger导出
- ✅ **告警规则管理**：YAML配置，支持Prometheus告警规则
- ✅ **性能分析工具**：pprof集成，内存、CPU、协程分析
- ✅ **健康检查系统**：多维度健康检查，支持自定义检查器
- ✅ **运行时监控**：自动收集Go运行时指标
- ✅ **中间件支持**：监控中间件，追踪中间件，恢复中间件
- ✅ **高性能**：零锁设计，异步处理，内存高效
- ✅ **生产就绪**：完善的错误处理，优雅关闭，安全防护

### 快速开始

#### 1. 基本配置
```yaml
# config/config.yaml
monitor:
  service_name: "myapp"
         service_version: "1.0.0"
         environment: "production"

  # Prometheus配置
         enable_metrics: true
         metrics_path: /metrics

  # 健康检查配置
         enable_health: true
         health_path: /health

  # 性能分析配置
         enable_profiling: false  # 生产环境建议关闭
         profile_path: /debug/pprof

  # 分布式追踪配置
         enable_tracing: true
         jaeger_endpoint: "http://jaeger:14268/api/traces"
         trace_sampling_rate: 0.1

  # 告警配置
         enable_alerts: true
         alert_rules: "./config/alert_rules.yaml"

  # 性能阈值配置
         performance_thresholds:
           max_request_duration: 2.0
           max_memory_usage: 0.8
           max_db_query_duration: 1.0
           max_goroutines: 1000
           error_rate_threshold: 0.05

  2. 初始化监控
  go

  import (
  "dgou/pkg/app"
  "dgou/pkg/config"
  "dgou/pkg/monitor"
  )

  func main() {
  // 加载配置
  cfg := config.LoadConfig()

  // 创建应用
  app := app.NewApp(cfg)

  // 初始化监控
  monitor, err := monitor.InitMonitor(cfg.Monitor, app.GetEngine())
  if err != nil {
  log.Fatal(err)
}
  defer monitor.Stop()

  // 使用监控中间件
  app.GetEngine().Use(monitor.MetricsMiddleware())
  app.GetEngine().Use(monitor.TracingMiddleware())
  app.GetEngine().Use(monitor.RecoveryMiddleware())

  // 启动应用
  app.Run()
}

  3. 指标收集示例
  go

  // HTTP请求自动收集（通过中间件）
  // 无需额外代码

  // 数据库查询指标
  func getUserByID(id int) (*User, error) {
  start := time.Now()

  var user User
  err := database.Master().First(&user, id).Error

  duration := time.Since(start)
  monitor.RecordDBQuery("mysql", "users", "select", duration, err == nil)

  return &user, err
}

  // 业务事件指标
  func processOrder(order *Order) error {
  start := time.Now()

  // 处理订单逻辑
  err := doProcessOrder(order)

  duration := time.Since(start)
  status := "success"
  if err != nil {
  status = "error"
}

  monitor.RecordBusinessEvent("order_processing", status)
  monitor.RecordBusinessProcessing("order_process", status, duration)

  return err
}

  // 自定义指标
  func registerCustomMetrics(monitor *monitor.Monitor) error {
  // 创建自定义计数器
  customCounter := prometheus.NewCounter(prometheus.CounterOpts{
         Name: "custom_events_total",
         Help: "Total custom events",
})

  return monitor.RegisterCustomMetric("custom_events", customCounter)
}

  4. 分布式追踪
  go

  import (
  "context"
  "go.opentelemetry.io/otel"
  )

  func processWithTracing(ctx context.Context) error {
  tracer := otel.Tracer("business")

  ctx, span := tracer.Start(ctx, "process_business")
  defer span.End()

  // 执行业务逻辑
  result, err := doBusiness(ctx)

  if err != nil {
  span.RecordError(err)
  span.SetStatus(codes.Error, err.Error())
}

  return err
}

  5. 健康检查
  go

  // 自定义健康检查
  type DatabaseHealthCheck struct {
  db *gorm.DB
}

  func (c *DatabaseHealthCheck) Name() string {
  return "database"
}

  func (c *DatabaseHealthCheck) Check(ctx context.Context) monitor.HealthStatus {
  var result int
  err := c.db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error

  status := "healthy"
  message := "Database is healthy"

  if err != nil {
  status = "unhealthy"
         message = fmt.Sprintf("Database connection failed: %v", err)
}

  return monitor.HealthStatus{
         Name:    c.Name(),
         Status:  status,
         Message: message,
}
}

  // 注册健康检查
         monitor.RegisterHealthCheck(&DatabaseHealthCheck{db: database.Master()})

  6. 性能分析
  go

  // 手动触发性能分析
  func triggerProfiling() {
  profiler := monitor.NewProfiler(monitor.GetMonitor())

  // 开始CPU分析（30秒）
  cpuFile, _ := profiler.StartCPUProfile(30 * time.Second)

  // 捕获堆内存快照
  heapFile, _ := profiler.CaptureHeapProfile()

  // 获取性能报告
  report := profiler.PerformanceReport()

  logger.Info("Profiling completed",
  logger.String("cpu_profile", cpuFile),
  logger.String("heap_profile", heapFile),
  logger.Any("report", report),
  )
}

  // 通过HTTP端点访问
  // GET /debug/pprof/          # pprof首页
  // GET /debug/pprof/heap      # 堆内存分析
  // GET /debug/pprof/profile   # CPU分析（30秒）
  // GET /debug/pprof/goroutine # 协程分析
  // GET /debug/pprof/trace     # 追踪分析（5秒）

  7. 告警管理
  yaml

  # config/alert_rules.yaml
         groups:
           - name: application_alerts
             rules:
               - alert: HighHttpErrorRate
                 expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
                 for: 2m
                 labels:
                   severity: critical
                 annotations:
                   summary: "High HTTP error rate detected"
                   description: "HTTP error rate is {{ printf \"%.2f\" $value }}%"

               - alert: HighRequestLatency
                 expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
                 for: 2m
                 labels:
                   severity: warning
                 annotations:
                   summary: "High request latency detected"
                   description: "95th percentile request latency is {{ printf \"%.2f\" $value }}s"

  8. 监控端点
  bash

  # Prometheus指标
  GET /metrics

  # 健康检查
  GET /health
  GET /health?detailed=true

  # 性能分析（需开启）
  GET /debug/pprof/
  GET /debug/pprof/heap
  GET /debug/pprof/profile?seconds=30

  # 调试信息
  GET /debug/vars
  GET /debug/metrics

  高级特性
  1. 数据库监控集成
  go

  // 包装GORM数据库
  db := database.Master()
  monitor := monitor.GetMonitor()

  gormMonitor := monitor.NewGormMonitor(monitor)
  monitoredDB := gormMonitor.WrapDB(db)

  // 自动收集所有查询指标
  // 更新连接池统计
  gormMonitor.UpdateDBStats(db)

  2. 缓存监控集成
  go

  monitor := monitor.GetMonitor()
  cacheMonitor := monitor.NewCacheMonitor(monitor)

  // 在缓存操作前后记录指标
  start := time.Now()
  value, err := cache.Get(ctx, "key")
  duration := time.Since(start)

  cacheMonitor.RecordGet(ctx, "redis", duration, err == nil && value != nil)

  3. 自定义告警处理器
  go

  type SlackAlertHandler struct {
  webhookURL string
}

  func (h *SlackAlertHandler) Handle(ctx context.Context, alert monitor.Alert) error {
         message := fmt.Sprintf("Alert: %s (Severity: %s)\nValue: %f\nMessage: %s",
                                                                     alert.Rule.Name,
                                                                     alert.Rule.Severity,
                                                                     alert.Value,
                                                                     alert.Annotations["message"],
  )

  // 发送到Slack
  return sendSlackMessage(h.webhookURL, message)
}

  // 注册告警处理器
  alertManager := monitor.GetMonitor().AlertManager()
  alertManager.AddHandler(&SlackAlertHandler{
         webhookURL: "https://hooks.slack.com/services/...",
})

  4. 性能阈值检查
  go

  func checkPerformanceThresholds(monitor *monitor.Monitor) {
  metrics := monitor.GetMetrics()

  // 检查内存使用率
  if memUsage := metrics["memory"].(map[string]interface{})["usage"].(float64);
  memUsage > 0.8 {
  logger.Warn("High memory usage detected",
  logger.Float64("usage", memUsage),
  )
}

  // 检查错误率
  if errorRate := metrics["error_rate"].(float64);
  errorRate > 0.05 {
  logger.Error("High error rate detected",
  logger.Float64("rate", errorRate),
  )
}
}

  最佳实践
  1. 生产环境配置建议
  yaml

         monitor:
           enable_metrics: true      # 始终启用指标
           enable_health: true       # 始终启用健康检查
           enable_profiling: false   # 生产环境关闭性能分析
           enable_tracing: true      # 根据需求开启追踪
           enable_alerts: true       # 始终启用告警

           # 安全建议
           profile_path: /internal/debug/pprof  # 使用非标准路径
           metrics_path: /internal/metrics      # 使用非标准路径

  2. 指标标签设计
  go

  // 好的标签设计
  labels := []string{
  "method",      // HTTP方法
  "path",        // 请求路径
  "status",      // HTTP状态码
  "handler",     // 处理函数
  "service",     // 服务名称
  "version",     // 服务版本
  "environment", // 环境
}

  // 避免标签爆炸
  // 不要使用用户ID、会话ID等高基数标签

  3. 告警策略
  yaml

  # 告警级别定义
  # critical - 需要立即处理，服务不可用
  # warning  - 需要关注，可能影响性能
  # info     - 信息性告警，无需立即处理

  # 告警抑制规则
  # 避免重复告警，设置合适的for持续时间
  # 使用标签分组，相关告警一起通知

  4. 性能优化
  go

  // 使用异步处理
  go func() {
  monitor.RecordBusinessEvent("async_event", "processed")
}()

  // 批量记录指标
  func batchRecordMetrics(events []BusinessEvent) {
  for _, event := range events {
  monitor.RecordBusinessEvent(event.Type, event.Status)
  }
}

  // 避免在热点路径中创建大量标签

  故障排除
  1. 指标不显示
  bash

  # 检查Prometheus配置
         scrape_configs:
           - job_name: 'myapp'
             static_configs:
               - targets: ['localhost:8080']
             metrics_path: '/metrics'

  # 检查服务端点
  curl http://localhost:8080/metrics
  curl http://localhost:8080/health

  2. 内存泄漏检测
  go

  // 定期检查内存使用
  func checkMemoryLeaks() {
  profiler := monitor.NewProfiler(monitor.GetMonitor())

  // 定期捕获堆内存
  ticker := time.NewTicker(1 * time.Hour)
  for range ticker.C {
  heapFile, _ := profiler.CaptureHeapProfile()
  logger.Info("Heap profile captured",
  logger.String("file", heapFile),
  )
}
}

  3. 高延迟问题
  bash

  # 使用pprof分析
  go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

  # 火焰图生成
  go tool pprof -http=:8081 profile.pprof

  4. 追踪数据缺失
  yaml

  # 检查Jaeger配置
         jaeger:
           endpoint: "http://jaeger:14268/api/traces"
           sampler_type: "probabilistic"
           sampler_param: 0.1  # 10%采样率

  # 检查网络连接
  telnet jaeger 14268

  这个监控组件提供了完整的生产级监控解决方案，支持指标收集、分布式追踪、告警管理和性能分析。组件设计为模块化，可以按需启用各个功能，并且与现有组件完美集成。

## 完整示例
### 创建用户服务
```go

package main

import (
"dgou/pkg/app"
"dgou/pkg/config"
"dgou/pkg/response"
"github.com/gin-gonic/gin"
"log"
)

// User 用户结构体
type User struct {
ID       int    `json:"id"`
Name     string `json:"name"`
Email    string `json:"email"`
Password string `json:"-"`
}

// UserRouter 用户路由
type UserRouter struct{}

func (r *UserRouter) Register(router *gin.RouterGroup) {
userGroup := router.Group("/api/users")
{
    userGroup.GET("", r.listUsers)
    userGroup.POST("", r.createUser)
    userGroup.GET("/:id", r.getUser)
}
}

func (r *UserRouter) Priority() int {
return 10
}

func (r *UserRouter) listUsers(c *gin.Context) {
// 模拟数据
users := []User{
    {ID: 1, Name: "John Doe", Email: "john@example.com"},
    {ID: 2, Name: "Jane Smith", Email: "jane@example.com"},
}

// 分页参数
page := 1
pageSize := 10
total := int64(100)

response.SuccessWithPagination(c, users, page, pageSize, total)
}

func (r *UserRouter) createUser(c *gin.Context) {
var user User
if err := c.ShouldBindJSON(&user); err != nil {
    response.BadRequest(c, "Invalid user data")
    return
}

// 模拟创建用户
user.ID = 100

response.Success(c, gin.H{
    "message": "User created successfully",
    "user":    user,
})
}

func (r *UserRouter) getUser(c *gin.Context) {
id := c.Param("id")

// 模拟获取用户
user := User{
    ID:    1,
    Name:  "John Doe",
    Email: "john@example.com",
}

response.Success(c, user)
}

func main() {
// 加载配置
cfg := config.LoadConfig("./config/config.yaml")

// 创建应用
application := app.NewApp(cfg)

// 初始化应用
if err := application.Initialize(); err != nil {
    log.Fatalf("Failed to initialize application: %v", err)
}

// 注册用户路由
application.AddRouter(&UserRouter{})

// 运行应用
if err := application.Run(); err != nil {
    log.Fatalf("Failed to run application: %v", err)
}
}
```
## 部署建议
### 1. 生产环境配置
```yaml

server:
    port: 8080
    mode: release
    enable_https: true
    cert_file: /etc/ssl/cert.pem
    key_file: /etc/ssl/key.pem
    enable_gzip: true

mysql:
    host: mysql-cluster
    port: 3306
    user: app_user
    password: ${DB_PASSWORD}  # 使用环境变量
    dbname: app_prod
    max_open_conns: 200
    max_idle_conns: 50

redis:
    addr: redis-cluster:6379
    password: ${REDIS_PASSWORD}
    db: 0
    pool_size: 200
    min_idle_conns: 20

log:
    level: warn
    file: /var/log/app/app.log
    max_size: 500
    max_backups: 10
    max_age: 30
    compress: true
```
### 2. Docker 部署
```dockerfile

FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
COPY --from=builder /app/config/config.yaml ./config/
COPY --from=builder /app/cert.pem /app/key.pem ./ssl/

EXPOSE 8080
CMD ["./server"]
```
### 3. Kubernetes 部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app-deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
        - name: app
          image: myapp:latest
          ports:
            - containerPort: 8080
          env:
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: db-secret
                  key: password
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```
## 监控与维护
### 1. 健康检查端点
- `GET /health` - 完整健康检查
- `GET /ready` - 就绪检查
- `GET /live` - 存活检查

### 2. 性能监控
- `GET /metrics` - Prometheus 指标
- `GET /debug/pprof` - Go 性能分析

### 3. 日志分析
```bash

# 查看错误日志
tail -f logs/app.log | grep '"level":"error"'

# 统计API请求
cat logs/app.log | grep '"path":"/api' | wc -l

# 查找慢请求
cat logs/app.log | jq 'select(.elapsed > 1000)'
```
## 贡献指南

我们欢迎各种形式的贡献！

1. **提交 Issue：** 报告 bug 或提出新功能建议
2. **提交 Pull Request：** 修复 bug 或添加新功能
3. **完善文档：** 改进 README 或添加示例

## 开发规范
1. 遵循 Go 代码规范
2. 添加单元测试
3. 更新相关文档
4. 保持向后兼容性

## 许可证

本项目采用 MIT 许可证。详见 LICENSE 文件。
## 联系与支持
- GitHub Issues: [提交问题](https://github.com/dgoukj/dgou-framework/issues)

- 文档: [查看完整文档](https://github.com/dgoukj/dgou-framework/wiki)

感谢使用 Dgou Framework！希望这个脚手架能帮助您快速构建可靠的 Go 应用。如果有任何问题或建议，请随时联系我们。