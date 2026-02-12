package queue

import "time"

// ------------------- 发布选项 -------------------
type PublishOptions struct {
	Exchange        string
	RoutingKey      string
	Mandatory       bool
	Immediate       bool
	ContentType     string
	ContentEncoding string
	DeliveryMode    uint8 // 1: transient, 2: persistent
	Priority        uint8
	CorrelationID   string
	ReplyTo         string
	Expiration      string
	MessageID       string
	Timestamp       time.Time
	Type            string
	UserID          string
	AppID           string
	Headers         map[string]interface{}
}

type PublishOption func(*PublishOptions)

func WithExchange(exchange string) PublishOption {
	return func(o *PublishOptions) { o.Exchange = exchange }
}
func WithRoutingKey(key string) PublishOption {
	return func(o *PublishOptions) { o.RoutingKey = key }
}
func WithMandatory(mandatory bool) PublishOption {
	return func(o *PublishOptions) { o.Mandatory = mandatory }
}
func WithImmediate(immediate bool) PublishOption {
	return func(o *PublishOptions) { o.Immediate = immediate }
}
func WithContentType(ct string) PublishOption {
	return func(o *PublishOptions) { o.ContentType = ct }
}
func WithContentEncoding(ce string) PublishOption {
	return func(o *PublishOptions) { o.ContentEncoding = ce }
}
func WithDeliveryMode(mode uint8) PublishOption {
	return func(o *PublishOptions) { o.DeliveryMode = mode }
}
func WithPriority(p uint8) PublishOption {
	return func(o *PublishOptions) { o.Priority = p }
}
func WithCorrelationID(id string) PublishOption {
	return func(o *PublishOptions) { o.CorrelationID = id }
}
func WithReplyTo(replyTo string) PublishOption {
	return func(o *PublishOptions) { o.ReplyTo = replyTo }
}
func WithExpiration(exp string) PublishOption {
	return func(o *PublishOptions) { o.Expiration = exp }
}
func WithMessageID(id string) PublishOption {
	return func(o *PublishOptions) { o.MessageID = id }
}
func WithTimestamp(t time.Time) PublishOption {
	return func(o *PublishOptions) { o.Timestamp = t }
}
func WithType(t string) PublishOption {
	return func(o *PublishOptions) { o.Type = t }
}
func WithUserID(uid string) PublishOption {
	return func(o *PublishOptions) { o.UserID = uid }
}
func WithAppID(aid string) PublishOption {
	return func(o *PublishOptions) { o.AppID = aid }
}
func WithHeaders(h map[string]interface{}) PublishOption {
	return func(o *PublishOptions) { o.Headers = h }
}

// ------------------- 消费选项 -------------------
type ConsumeOptions struct {
	Queue          string
	Consumer       string
	AutoAck        bool
	Exclusive      bool
	NoLocal        bool
	NoWait         bool
	Args           map[string]interface{}
	PrefetchCount  int
	PrefetchSize   int
	GlobalPrefetch bool
}

type ConsumeOption func(*ConsumeOptions)

func WithQueue(queue string) ConsumeOption {
	return func(o *ConsumeOptions) { o.Queue = queue }
}
func WithConsumer(consumer string) ConsumeOption {
	return func(o *ConsumeOptions) { o.Consumer = consumer }
}
func WithAutoAck(autoAck bool) ConsumeOption {
	return func(o *ConsumeOptions) { o.AutoAck = autoAck }
}
func WithExclusive(exclusive bool) ConsumeOption {
	return func(o *ConsumeOptions) { o.Exclusive = exclusive }
}
func WithNoLocal(noLocal bool) ConsumeOption {
	return func(o *ConsumeOptions) { o.NoLocal = noLocal }
}
func WithNoWait(noWait bool) ConsumeOption {
	return func(o *ConsumeOptions) { o.NoWait = noWait }
}
func WithArgs(args map[string]interface{}) ConsumeOption {
	return func(o *ConsumeOptions) { o.Args = args }
}
func WithPrefetch(count, size int, global bool) ConsumeOption {
	return func(o *ConsumeOptions) {
		o.PrefetchCount = count
		o.PrefetchSize = size
		o.GlobalPrefetch = global
	}
}

// ------------------- 连接选项 -------------------
type ConnectionOptions struct {
	URL         string
	Host        string
	Port        int
	Username    string
	Password    string
	Vhost       string
	Heartbeat   int
	DialTimeout time.Duration
	MaxRetries  int
	RetryDelay  time.Duration
}

type ConnectionOption func(*ConnectionOptions)

func WithURL(url string) ConnectionOption {
	return func(o *ConnectionOptions) { o.URL = url }
}
func WithHostPort(host string, port int) ConnectionOption {
	return func(o *ConnectionOptions) { o.Host, o.Port = host, port }
}
func WithAuth(username, password string) ConnectionOption {
	return func(o *ConnectionOptions) { o.Username, o.Password = username, password }
}
func WithVhost(vhost string) ConnectionOption {
	return func(o *ConnectionOptions) { o.Vhost = vhost }
}
func WithHeartbeat(heartbeat int) ConnectionOption {
	return func(o *ConnectionOptions) { o.Heartbeat = heartbeat }
}
func WithDialTimeout(timeout time.Duration) ConnectionOption {
	return func(o *ConnectionOptions) { o.DialTimeout = timeout }
}
