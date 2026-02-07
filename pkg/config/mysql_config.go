package config

// MySQLConfigExt 扩展的MySQL配置（支持读写分离）
type MySQLConfigExt struct {
	// 主库配置
	Master MySQLConfig `mapstructure:"master"`

	// 从库配置列表
	Slaves []MySQLConfig `mapstructure:"slaves"`

	// 连接池配置
	Pool struct {
		MaxOpenConns    int `mapstructure:"max_open_conns"`     // 最大打开连接数
		MaxIdleConns    int `mapstructure:"max_idle_conns"`     // 最大空闲连接数
		ConnMaxLifetime int `mapstructure:"conn_max_lifetime"`  // 连接最大生命周期(秒)
		ConnMaxIdleTime int `mapstructure:"conn_max_idle_time"` // 连接最大空闲时间(秒)
	} `mapstructure:"pool"`

	// 日志配置
	Log struct {
		SlowThreshold int    `mapstructure:"slow_threshold"` // 慢查询阈值(毫秒)
		EnableLogging bool   `mapstructure:"enable_logging"` // 是否启用SQL日志
		LogLevel      string `mapstructure:"log_level"`      // 日志级别：silent, error, warn, info
	} `mapstructure:"log"`

	// 性能配置
	Performance struct {
		PrepareStmt       bool `mapstructure:"prepare_stmt"`        // 是否启用预编译语句
		DisableForeignKey bool `mapstructure:"disable_foreign_key"` // 是否禁用外键约束
	} `mapstructure:"performance"`
}

// GetMasterConfig 获取主库配置
func (c *MySQLConfigExt) GetMasterConfig() *MySQLConfig {
	return &c.Master
}

// GetSlaveConfigs 获取从库配置列表
func (c *MySQLConfigExt) GetSlaveConfigs() []MySQLConfig {
	return c.Slaves
}

// HasSlaves 检查是否有从库配置
func (c *MySQLConfigExt) HasSlaves() bool {
	return len(c.Slaves) > 0
}
