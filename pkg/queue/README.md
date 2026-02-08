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
```

#### 2. 初始化队列管理器

```go
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
```

#### 3. 发送消息

```go
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
```

#### 4. 消费消息

```go
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
```

### 高级用法

#### 1. 消息确认机制

```go
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
```

#### 2. 死信队列管理

```go
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
```

#### 3. 延迟队列应用

```go
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
```

#### 4. 消息重试策略

```go
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
```

#### 5. 事务消息

```go
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
```

#### 6. 消息优先级

```go
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
```

#### 7. 消息路由和过滤

```go
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
```

### 监控和管理

#### 1. 性能指标收集

```go
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
```

#### 2. 队列管理API

```go
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
```

#### 3. 告警和通知

```go
// 队列告警管理器
type QueueAlertManager struct {
    manager    *queue.QueueManager
    alertCh    chan *QueueAlert
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
```

### 最佳实践

#### 1. 连接管理最佳实践

```go
// 连接池管理
type ConnectionPool struct {
    config      *queue.RabbitMQConfig
    connections []*queue.Connection
    mu          sync.RWMutex
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
```

#### 2. 消息设计最佳实践

```go
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
```

#### 3. 错误处理和恢复

```go
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
```

这个队列组件提供了完整的生产级 RabbitMQ 解决方案，包含连接管理、消息确认、死信队列、延迟队列和消息重试策略等核心功能。组件设计为纯业务逻辑，不包含任何 Web 框架依赖，可以在任何 Go 项目中使用。