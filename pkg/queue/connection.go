// file: pkg/queue/connection.go
package queue

import (
	"context"
	"dgou/pkg/config"
	"dgou/pkg/errors"
	"dgou/pkg/logger"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// QueueType 队列类型
type QueueType string

const (
	QueueTypeDirect  QueueType = "direct"  // 直连队列
	QueueTypeFanout  QueueType = "fanout"  // 扇出队列
	QueueTypeTopic   QueueType = "topic"   // 主题队列
	QueueTypeHeaders QueueType = "headers" // 头部队列
)

// DeliveryMode 消息传递模式
type DeliveryMode uint8

const (
	DeliveryModeTransient  DeliveryMode = 1 // 非持久化消息
	DeliveryModePersistent DeliveryMode = 2 // 持久化消息
)

// ExchangeConfig 交换机配置
type ExchangeConfig struct {
	Name       string     `mapstructure:"name"`        // 交换机名称
	Type       QueueType  `mapstructure:"type"`        // 交换机类型
	Durable    bool       `mapstructure:"durable"`     // 是否持久化
	AutoDelete bool       `mapstructure:"auto_delete"` // 是否自动删除
	Internal   bool       `mapstructure:"internal"`    // 是否内部使用
	NoWait     bool       `mapstructure:"no_wait"`     // 是否等待服务器确认
	Args       amqp.Table `mapstructure:"args"`        // 额外参数
}

// QueueConfig 队列配置
type QueueConfig struct {
	Name          string     `mapstructure:"name"`           // 队列名称
	Durable       bool       `mapstructure:"durable"`        // 是否持久化
	AutoDelete    bool       `mapstructure:"auto_delete"`    // 是否自动删除
	Exclusive     bool       `mapstructure:"exclusive"`      // 是否排他
	NoWait        bool       `mapstructure:"no_wait"`        // 是否等待服务器确认
	Args          amqp.Table `mapstructure:"args"`           // 额外参数
	BindingKey    string     `mapstructure:"binding_key"`    // 绑定键
	PrefetchCount int        `mapstructure:"prefetch_count"` // 预取数量
	PrefetchSize  int        `mapstructure:"prefetch_size"`  // 预取大小
	Global        bool       `mapstructure:"global"`         // 是否全局预取
}

// RabbitMQConfig RabbitMQ配置
type RabbitMQConfig struct {
	URL            string        `mapstructure:"url"`             // 连接URL
	Host           string        `mapstructure:"host"`            // 主机地址
	Port           int           `mapstructure:"port"`            // 端口
	Username       string        `mapstructure:"username"`        // 用户名
	Password       string        `mapstructure:"password"`        // 密码
	Vhost          string        `mapstructure:"vhost"`           // 虚拟主机
	Heartbeat      int           `mapstructure:"heartbeat"`       // 心跳间隔（秒）
	ConnectionName string        `mapstructure:"connection_name"` // 连接名称
	MaxReconnect   int           `mapstructure:"max_reconnect"`   // 最大重连次数
	ReconnectDelay time.Duration `mapstructure:"reconnect_delay"` // 重连延迟
	PrefetchCount  int           `mapstructure:"prefetch_count"`  // 预取数量
	PrefetchSize   int           `mapstructure:"prefetch_size"`   // 预取大小
	GlobalPrefetch bool          `mapstructure:"global_prefetch"` // 是否全局预取
	EnableTLS      bool          `mapstructure:"enable_tls"`      // 是否启用TLS
	TLSCertFile    string        `mapstructure:"tls_cert_file"`   // TLS证书文件
	TLSKeyFile     string        `mapstructure:"tls_key_file"`    // TLS密钥文件
	TLSCAFile      string        `mapstructure:"tls_ca_file"`     // TLS CA文件
	EnableMetrics  bool          `mapstructure:"enable_metrics"`  // 是否启用指标
}

// Message 消息结构
type Message struct {
	ID            string                 `json:"id"`             // 消息ID
	Exchange      string                 `json:"exchange"`       // 交换机
	RoutingKey    string                 `json:"routing_key"`    // 路由键
	Body          []byte                 `json:"body"`           // 消息体
	ContentType   string                 `json:"content_type"`   // 内容类型
	DeliveryMode  DeliveryMode           `json:"delivery_mode"`  // 传递模式
	Priority      uint8                  `json:"priority"`       // 优先级
	CorrelationID string                 `json:"correlation_id"` // 关联ID
	ReplyTo       string                 `json:"reply_to"`       // 回复队列
	MessageID     string                 `json:"message_id"`     // 消息ID
	Timestamp     time.Time              `json:"timestamp"`      // 时间戳
	Type          string                 `json:"type"`           // 消息类型
	UserID        string                 `json:"user_id"`        // 用户ID
	AppID         string                 `json:"app_id"`         // 应用ID
	Headers       map[string]interface{} `json:"headers"`        // 消息头
	RetryCount    int                    `json:"retry_count"`    // 重试次数
	MaxRetries    int                    `json:"max_retries"`    // 最大重试次数
	RetryDelay    time.Duration          `json:"retry_delay"`    // 重试延迟
	Expiration    string                 `json:"expiration"`     // 过期时间
}

// Connection 连接管理器
type Connection struct {
	config      *RabbitMQConfig    // 配置
	connection  *amqp.Connection   // AMQP连接
	channel     *amqp.Channel      // AMQP通道
	mu          sync.RWMutex       // 读写锁
	isConnected bool               // 是否已连接
	reconnectCh chan struct{}      // 重连通道
	stopCh      chan struct{}      // 停止通道
	metrics     *ConnectionMetrics // 连接指标
}

// ConnectionMetrics 连接指标
type ConnectionMetrics struct {
	Connections      int64        `json:"connections"`
	Reconnects       int64        `json:"reconnects"`
	MessagesSent     int64        `json:"messages_sent"`
	MessagesReceived int64        `json:"messages_received"`
	Errors           int64        `json:"errors"`
	LastConnectTime  time.Time    `json:"last_connect_time"`
	LastError        string       `json:"last_error"`
	mu               sync.RWMutex `json:"-"`
}

// NewRabbitMQConfig 创建RabbitMQ配置
func NewRabbitMQConfig(cfg *config.Config) *RabbitMQConfig {
	rabbitCfg := &cfg.RabbitMQ

	return &RabbitMQConfig{
		URL:            rabbitCfg.URL,
		Host:           rabbitCfg.Host,
		Port:           rabbitCfg.Port,
		Username:       rabbitCfg.Username,
		Password:       rabbitCfg.Password,
		Vhost:          rabbitCfg.Vhost,
		Heartbeat:      rabbitCfg.Heartbeat,
		ConnectionName: rabbitCfg.ConnectionName,
		MaxReconnect:   rabbitCfg.MaxReconnect,
		ReconnectDelay: rabbitCfg.ReconnectDelay,
		PrefetchCount:  rabbitCfg.PrefetchCount,
		PrefetchSize:   rabbitCfg.PrefetchSize,
		GlobalPrefetch: rabbitCfg.GlobalPrefetch,
		EnableTLS:      rabbitCfg.EnableTLS,
		TLSCertFile:    rabbitCfg.TLSCertFile,
		TLSKeyFile:     rabbitCfg.TLSKeyFile,
		TLSCAFile:      rabbitCfg.TLSCAFile,
		EnableMetrics:  rabbitCfg.EnableMetrics,
	}
}

// NewConnection 创建新的连接管理器
func NewConnection(config *RabbitMQConfig) *Connection {
	if config.ReconnectDelay == 0 {
		config.ReconnectDelay = 5 * time.Second
	}

	if config.MaxReconnect == 0 {
		config.MaxReconnect = 10
	}

	return &Connection{
		config:      config,
		reconnectCh: make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
		metrics:     &ConnectionMetrics{},
	}
}

// Connect 连接到RabbitMQ
func (c *Connection) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已连接，先关闭
	if c.isConnected {
		c.disconnect()
	}

	// 构建连接URL
	url := c.buildURL()

	logger.Info("Connecting to RabbitMQ",
		logger.String("url", c.maskURL(url)),
		logger.String("vhost", c.config.Vhost),
	)

	// 建立连接
	conn, err := amqp.Dial(url)
	if err != nil {
		c.metrics.recordError(fmt.Sprintf("Failed to connect to RabbitMQ: %v", err))
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to connect to RabbitMQ")
	}

	// 创建通道
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		c.metrics.recordError(fmt.Sprintf("Failed to create channel: %v", err))
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to create RabbitMQ channel")
	}

	// 设置QoS
	if err := channel.Qos(
		c.config.PrefetchCount,
		c.config.PrefetchSize,
		c.config.GlobalPrefetch,
	); err != nil {
		conn.Close()
		c.metrics.recordError(fmt.Sprintf("Failed to set QoS: %v", err))
		return errors.Wrap(err, errors.CodeInternalError,
			"Failed to set RabbitMQ QoS")
	}

	c.connection = conn
	c.channel = channel
	c.isConnected = true
	c.metrics.recordConnect()

	logger.Info("RabbitMQ connected successfully",
		logger.String("connection_name", c.config.ConnectionName),
	)

	// 启动连接监控
	go c.monitorConnection()

	return nil
}

// buildURL 构建连接URL
func (c *Connection) buildURL() string {
	if c.config.URL != "" {
		return c.config.URL
	}

	protocol := "amqp"
	if c.config.EnableTLS {
		protocol = "amqps"
	}

	return fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		protocol,
		c.config.Username,
		c.config.Password,
		c.config.Host,
		c.config.Port,
		c.config.Vhost,
	)
}

// maskURL 隐藏URL中的密码
func (c *Connection) maskURL(url string) string {
	// 简单的密码隐藏，实际应用中可能需要更复杂的处理
	return url
}

// disconnect 断开连接
func (c *Connection) disconnect() {
	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}

	if c.connection != nil {
		c.connection.Close()
		c.connection = nil
	}

	c.isConnected = false
}

// IsConnected 检查是否已连接
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isConnected
}

// GetChannel 获取通道
func (c *Connection) GetChannel() (*amqp.Channel, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isConnected || c.channel == nil {
		return nil, errors.New(errors.CodeInternalError,
			"RabbitMQ channel is not available")
	}

	return c.channel, nil
}

// Reconnect 重新连接
func (c *Connection) Reconnect() error {
	logger.Info("Attempting to reconnect to RabbitMQ...")

	// 尝试重连
	for i := 0; i < c.config.MaxReconnect; i++ {
		if err := c.Connect(); err == nil {
			logger.Info("Reconnected to RabbitMQ successfully")
			return nil
		}

		logger.Warn("Reconnect attempt failed",
			logger.Int("attempt", i+1),
			logger.Int("max_attempts", c.config.MaxReconnect),
		)

		// 等待重试延迟
		select {
		case <-time.After(c.config.ReconnectDelay):
			continue
		case <-c.stopCh:
			return errors.New(errors.CodeInternalError, "Reconnect cancelled")
		}
	}

	return errors.New(errors.CodeInternalError,
		fmt.Sprintf("Failed to reconnect after %d attempts", c.config.MaxReconnect))
}

// monitorConnection 监控连接状态
func (c *Connection) monitorConnection() {
	// 监听连接关闭事件
	closeCh := c.connection.NotifyClose(make(chan *amqp.Error))

	for {
		select {
		case err := <-closeCh:
			if err != nil {
				c.metrics.recordError(fmt.Sprintf("Connection closed: %v", err))
				logger.Error("RabbitMQ connection closed",
					logger.String("reason", err.Reason),
					logger.Int("code", err.Code),
				)

				// 设置连接状态为断开
				c.mu.Lock()
				c.isConnected = false
				c.mu.Unlock()

				// 触发重连
				select {
				case c.reconnectCh <- struct{}{}:
					logger.Info("Triggering reconnect...")
				default:
					// 通道已满，忽略
				}
			}

		case <-c.reconnectCh:
			// 执行重连
			if err := c.Reconnect(); err != nil {
				logger.Error("Failed to reconnect",
					logger.ErrorField(err),
				)
			}

		case <-c.stopCh:
			return
		}
	}
}

// Stop 停止连接管理器
func (c *Connection) Stop() error {
	close(c.stopCh)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.disconnect()

	logger.Info("RabbitMQ connection stopped")
	return nil
}

// GetMetrics 获取连接指标
func (c *Connection) GetMetrics() *ConnectionMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	// 返回副本
	return &ConnectionMetrics{
		Connections:      c.metrics.Connections,
		Reconnects:       c.metrics.Reconnects,
		MessagesSent:     c.metrics.MessagesSent,
		MessagesReceived: c.metrics.MessagesReceived,
		Errors:           c.metrics.Errors,
		LastConnectTime:  c.metrics.LastConnectTime,
		LastError:        c.metrics.LastError,
	}
}

// recordConnect 记录连接事件
func (m *ConnectionMetrics) recordConnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Connections++
	m.Reconnects++
	m.LastConnectTime = time.Now()
}

// recordError 记录错误事件
func (m *ConnectionMetrics) recordError(err string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Errors++
	m.LastError = err
}

// recordMessageSent 记录消息发送
func (m *ConnectionMetrics) recordMessageSent() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MessagesSent++
}

// recordMessageReceived 记录消息接收
func (m *ConnectionMetrics) recordMessageReceived() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.MessagesReceived++
}
