// pkg/database/tools.go
package database

import (
	"context"
	"database/sql"
	"dgou/pkg/logger"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ==================== 数据结构定义 ====================

// Pagination 分页参数
type Pagination struct {
	Page     int `form:"page" json:"page" binding:"min=1"`                   // 当前页码
	PageSize int `form:"page_size" json:"page_size" binding:"min=1,max=100"` // 每页大小
}

// PaginationResult 分页结果
type PaginationResult struct {
	Page      int         `json:"page"`       // 当前页码
	PageSize  int         `json:"page_size"`  // 每页大小
	Total     int64       `json:"total"`      // 总记录数
	TotalPage int         `json:"total_page"` // 总页数
	List      interface{} `json:"list"`       // 数据列表
}

// BatchOptions 批量操作选项
type BatchOptions struct {
	BatchSize   int           // 批次大小
	Timeout     time.Duration // 超时时间
	EnableRetry bool          // 是否启用重试
	MaxRetries  int           // 最大重试次数
}

// QueryOptions 查询选项
type QueryOptions struct {
	Select      []string               // 选择字段
	Where       map[string]interface{} // 条件
	Order       []string               // 排序
	Limit       int                    // 限制条数
	Offset      int                    // 偏移量
	Preload     []string               // 预加载关联
	ForUpdate   bool                   // 是否锁定更新
	ForShare    bool                   // 是否锁定共享
	WithContext bool                   // 是否使用上下文
	Timeout     time.Duration          // 超时时间
}

// IndexOptions 索引选项
type IndexOptions struct {
	Name    string   // 索引名称
	Columns []string // 索引列
	Unique  bool     // 是否唯一索引
	Comment string   // 索引注释
}

// ==================== 数据库工具类 ====================

// DatabaseTools 数据库工具类
// 提供通用的数据库操作工具函数，便于应用开发人员使用
type DatabaseTools struct {
	db *gorm.DB
}

// NewDatabaseTools 创建数据库工具实例
func NewDatabaseTools(db *gorm.DB) *DatabaseTools {
	return &DatabaseTools{db: db}
}

// WithDB 设置数据库连接
func (dt *DatabaseTools) WithDB(db *gorm.DB) *DatabaseTools {
	dt.db = db
	return dt
}

// GetDB 获取底层数据库连接
func (dt *DatabaseTools) GetDB() *gorm.DB {
	return dt.db
}

// ==================== 分页查询 ====================

// Paginate 分页查询
func (dt *DatabaseTools) Paginate(pagination *Pagination, result interface{}) (*PaginationResult, error) {
	return dt.PaginateWithOptions(pagination, result, nil)
}

// PaginateWithOptions 带选项的分页查询
func (dt *DatabaseTools) PaginateWithOptions(pagination *Pagination, result interface{}, options *QueryOptions) (*PaginationResult, error) {
	if pagination == nil {
		pagination = &Pagination{
			Page:     1,
			PageSize: 20,
		}
	}

	// 验证分页参数
	if pagination.Page < 1 {
		pagination.Page = 1
	}

	if pagination.PageSize < 1 {
		pagination.PageSize = 20
	}

	if pagination.PageSize > 100 {
		pagination.PageSize = 100
	}

	// 计算偏移量
	offset := (pagination.Page - 1) * pagination.PageSize

	// 构建查询
	query := dt.buildQuery(options)

	// 查询总数
	var total int64
	countDB := query.Session(&gorm.Session{})
	if err := countDB.Count(&total).Error; err != nil {
		return nil, err
	}

	// 计算总页数
	totalPage := int((total + int64(pagination.PageSize) - 1) / int64(pagination.PageSize))

	// 查询数据
	if err := query.Offset(offset).Limit(pagination.PageSize).Find(result).Error; err != nil {
		return nil, err
	}

	return &PaginationResult{
		Page:      pagination.Page,
		PageSize:  pagination.PageSize,
		Total:     total,
		TotalPage: totalPage,
		List:      result,
	}, nil
}

// ==================== 批量操作 ====================

// BatchInsert 批量插入数据
func (dt *DatabaseTools) BatchInsert(data interface{}, options *BatchOptions) error {
	if options == nil {
		options = &BatchOptions{
			BatchSize:   1000,
			EnableRetry: true,
			MaxRetries:  3,
		}
	}

	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice {
		return fmt.Errorf("data must be a slice")
	}

	length := value.Len()
	if length == 0 {
		return nil
	}

	batchSize := options.BatchSize
	if batchSize <= 0 {
		batchSize = 1000
	}

	// 使用事务
	return dt.ExecuteTransaction(func(tx *gorm.DB) error {
		for i := 0; i < length; i += batchSize {
			end := i + batchSize
			if end > length {
				end = length
			}

			batch := value.Slice(i, end).Interface()
			if err := tx.Create(batch).Error; err != nil {
				return err
			}

			logger.Debug("Batch insert progress",
				logger.Int("current", end),
				logger.Int("total", length),
			)
		}

		return nil
	}, &sql.TxOptions{})
}

// BulkUpsert 批量插入或更新
func (dt *DatabaseTools) BulkUpsert(data interface{}, conflictColumns []string, updateColumns []string) error {
	if len(conflictColumns) == 0 {
		return fmt.Errorf("conflictColumns cannot be empty")
	}

	return dt.db.Clauses(clause.OnConflict{
		Columns:   dt.getColumns(conflictColumns),
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(data).Error
}

// ==================== 事务操作 ====================

// ExecuteTransaction 执行事务操作
func (dt *DatabaseTools) ExecuteTransaction(fn func(*gorm.DB) error, opts ...*sql.TxOptions) error {
	return dt.db.Transaction(fn, opts...)
}

// ExecuteTransactionWithRetry 带重试的事务操作
func (dt *DatabaseTools) ExecuteTransactionWithRetry(fn func(*gorm.DB) error, maxRetries int, opts ...*sql.TxOptions) error {
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i*100) * time.Millisecond) // 指数退避
		}

		err := dt.db.Transaction(fn, opts...)
		if err != nil {
			lastErr = err
			// 检查是否是可重试的错误
			if dt.isRetryableError(err) {
				logger.Warn("Transaction failed, retrying",
					logger.ErrorField(err),
					logger.Int("attempt", i+1),
				)
				continue
			}
			return err
		}

		return nil
	}

	return lastErr
}

// ==================== 查询操作 ====================

// QueryWithOptions 带选项的查询
func (dt *DatabaseTools) QueryWithOptions(model interface{}, options *QueryOptions) error {
	if options == nil {
		return dt.db.Find(model).Error
	}

	query := dt.buildQuery(options)
	return query.Find(model).Error
}

// FindByID 根据ID查找记录
func (dt *DatabaseTools) FindByID(model interface{}, id interface{}) error {
	return dt.db.Where("id = ?", id).First(model).Error
}

// FindByCondition 根据条件查找记录
func (dt *DatabaseTools) FindByCondition(model interface{}, condition string, args ...interface{}) error {
	return dt.db.Where(condition, args...).First(model).Error
}

// Exists 检查记录是否存在
func (dt *DatabaseTools) Exists(model interface{}, condition string, args ...interface{}) (bool, error) {
	var count int64
	err := dt.db.Model(model).Where(condition, args...).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CountByCondition 根据条件统计数量
func (dt *DatabaseTools) CountByCondition(model interface{}, condition string, args ...interface{}) (int64, error) {
	var count int64
	err := dt.db.Model(model).Where(condition, args...).Count(&count).Error
	return count, err
}

// FirstOrCreate 查找或创建记录
func (dt *DatabaseTools) FirstOrCreate(model interface{}, where interface{}, defaults ...interface{}) error {
	if len(defaults) > 0 {
		return dt.db.Where(where).Attrs(defaults[0]).FirstOrCreate(model).Error
	}
	return dt.db.Where(where).FirstOrCreate(model).Error
}

// FindInBatches 分批查询
func (dt *DatabaseTools) FindInBatches(result interface{}, batchSize int, fn func(*gorm.DB, int) error) error {
	return dt.db.FindInBatches(result, batchSize, fn).Error
}

// ==================== 更新操作 ====================

// UpdateColumns 安全更新指定列
func (dt *DatabaseTools) UpdateColumns(model interface{}, id interface{}, updates map[string]interface{}) error {
	// 过滤无效的字段名
	validUpdates := make(map[string]interface{})
	for field, value := range updates {
		if dt.isValidFieldName(field) {
			validUpdates[field] = value
		}
	}

	if len(validUpdates) == 0 {
		return nil
	}

	return dt.db.Model(model).Where("id = ?", id).Updates(validUpdates).Error
}

// UpdateByCondition 根据条件更新
func (dt *DatabaseTools) UpdateByCondition(model interface{}, updates map[string]interface{}, condition string, args ...interface{}) error {
	// 过滤无效的字段名
	validUpdates := make(map[string]interface{})
	for field, value := range updates {
		if dt.isValidFieldName(field) {
			validUpdates[field] = value
		}
	}

	if len(validUpdates) == 0 {
		return nil
	}

	return dt.db.Model(model).Where(condition, args...).Updates(validUpdates).Error
}

// ==================== 删除操作 ====================

// SoftDeleteRecord 软删除记录
func (dt *DatabaseTools) SoftDeleteRecord(model interface{}, id interface{}) error {
	return dt.db.Model(model).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}

// SoftDeleteByCondition 根据条件软删除记录
func (dt *DatabaseTools) SoftDeleteByCondition(model interface{}, condition string, args ...interface{}) error {
	return dt.db.Model(model).Where(condition, args...).Update("deleted_at", time.Now()).Error
}

// HardDelete 硬删除记录
func (dt *DatabaseTools) HardDelete(model interface{}, id interface{}) error {
	return dt.db.Unscoped().Where("id = ?", id).Delete(model).Error
}

// HardDeleteByCondition 根据条件硬删除记录
func (dt *DatabaseTools) HardDeleteByCondition(model interface{}, condition string, args ...interface{}) error {
	return dt.db.Unscoped().Where(condition, args...).Delete(model).Error
}

// ==================== 锁操作 ====================

// LockForUpdate 锁定记录用于更新
func (dt *DatabaseTools) LockForUpdate() *gorm.DB {
	return dt.db.Clauses(clause.Locking{Strength: "UPDATE"})
}

// LockForShare 锁定记录用于共享读取
func (dt *DatabaseTools) LockForShare() *gorm.DB {
	return dt.db.Clauses(clause.Locking{Strength: "SHARE"})
}

// ==================== 表维护操作 ====================

// GetTableSize 获取表大小
func (dt *DatabaseTools) GetTableSize(tableName string) (int64, error) {
	// 验证表名安全
	if !dt.isValidFieldName(tableName) {
		return 0, fmt.Errorf("invalid table name: %s", tableName)
	}

	var size int64
	query := fmt.Sprintf("SELECT data_length + index_length FROM information_schema.TABLES WHERE table_schema = DATABASE() AND table_name = ?")

	if err := dt.db.Raw(query, tableName).Scan(&size).Error; err != nil {
		return 0, err
	}

	return size, nil
}

// OptimizeTable 优化表
func (dt *DatabaseTools) OptimizeTable(tableName string) error {
	// 验证表名安全
	if !dt.isValidFieldName(tableName) {
		return fmt.Errorf("invalid table name: %s", tableName)
	}

	query := fmt.Sprintf("OPTIMIZE TABLE `%s`", tableName)
	return dt.db.Exec(query).Error
}

// ExplainQuery 解释查询计划
func (dt *DatabaseTools) ExplainQuery(query string, args ...interface{}) (string, error) {
	var explainRows []map[string]interface{}
	explainQuery := "EXPLAIN " + query

	if err := dt.db.Raw(explainQuery, args...).Scan(&explainRows).Error; err != nil {
		return "", err
	}

	// 格式化解释结果
	var result strings.Builder
	result.WriteString("Query Plan:\n")

	for _, row := range explainRows {
		result.WriteString(fmt.Sprintf("%+v\n", row))
	}

	return result.String(), nil
}

// TruncateTable 清空表
func (dt *DatabaseTools) TruncateTable(tableName string) error {
	if !dt.isValidFieldName(tableName) {
		return fmt.Errorf("invalid table name: %s", tableName)
	}

	query := fmt.Sprintf("TRUNCATE TABLE `%s`", tableName)
	return dt.db.Exec(query).Error
}

// ==================== 索引管理 ====================

// CreateIndex 创建索引
func (dt *DatabaseTools) CreateIndex(tableName string, options *IndexOptions) error {
	if options == nil {
		return fmt.Errorf("index options cannot be nil")
	}

	if !dt.isValidFieldName(tableName) || !dt.isValidFieldName(options.Name) {
		return fmt.Errorf("invalid table or index name")
	}

	indexType := "INDEX"
	if options.Unique {
		indexType = "UNIQUE INDEX"
	}

	columns := strings.Join(options.Columns, ", ")
	query := fmt.Sprintf("CREATE %s `%s` ON `%s` (%s)", indexType, options.Name, tableName, columns)

	if options.Comment != "" {
		query += fmt.Sprintf(" COMMENT '%s'", options.Comment)
	}

	return dt.db.Exec(query).Error
}

// DropIndex 删除索引
func (dt *DatabaseTools) DropIndex(tableName, indexName string) error {
	if !dt.isValidFieldName(tableName) || !dt.isValidFieldName(indexName) {
		return fmt.Errorf("invalid table or index name")
	}

	query := fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, tableName)
	return dt.db.Exec(query).Error
}

// GetIndexes 获取表索引
func (dt *DatabaseTools) GetIndexes(tableName string) ([]map[string]interface{}, error) {
	if !dt.isValidFieldName(tableName) {
		return nil, fmt.Errorf("invalid table name: %s", tableName)
	}

	var indexes []map[string]interface{}
	query := `
		SELECT 
			INDEX_NAME,
			COLUMN_NAME,
			NON_UNIQUE,
			INDEX_TYPE,
			COMMENT
		FROM INFORMATION_SCHEMA.STATISTICS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? 
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`

	if err := dt.db.Raw(query, tableName).Scan(&indexes).Error; err != nil {
		return nil, err
	}

	return indexes, nil
}

// ==================== 列信息 ====================

// GetColumnInfo 获取列信息
func (dt *DatabaseTools) GetColumnInfo(tableName string) ([]map[string]interface{}, error) {
	if !dt.isValidFieldName(tableName) {
		return nil, fmt.Errorf("invalid table name: %s", tableName)
	}

	var columns []map[string]interface{}
	query := `
		SELECT 
			COLUMN_NAME,
			DATA_TYPE,
			IS_NULLABLE,
			COLUMN_DEFAULT,
			COLUMN_COMMENT,
			CHARACTER_MAXIMUM_LENGTH,
			COLUMN_KEY
		FROM INFORMATION_SCHEMA.COLUMNS 
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? 
		ORDER BY ORDINAL_POSITION
	`

	if err := dt.db.Raw(query, tableName).Scan(&columns).Error; err != nil {
		return nil, err
	}

	return columns, nil
}

// ==================== 统计信息 ====================

// GetDBStats 获取数据库连接池统计
func (dt *DatabaseTools) GetDBStats() (sql.DBStats, error) {
	sqlDB, err := dt.db.DB()
	if err != nil {
		return sql.DBStats{}, err
	}
	return sqlDB.Stats(), nil
}

// GetQueryStats 获取查询统计
func (dt *DatabaseTools) GetQueryStats() (map[string]interface{}, error) {
	var stats []map[string]interface{}
	query := `
		SELECT 
			digest_text AS query,
			count_star AS executions,
			avg_timer_wait AS avg_time,
			max_timer_wait AS max_time,
			sum_lock_time AS total_lock_time
		FROM performance_schema.events_statements_summary_by_digest
		WHERE digest_text IS NOT NULL
		ORDER BY sum_timer_wait DESC
		LIMIT 10
	`

	if err := dt.db.Raw(query).Scan(&stats).Error; err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"top_queries": stats,
	}

	// 获取数据库状态
	var dbStatus []map[string]interface{}
	statusQuery := "SHOW STATUS LIKE 'Threads_connected'"
	if err := dt.db.Raw(statusQuery).Scan(&dbStatus).Error; err == nil {
		for _, row := range dbStatus {
			result[row["Variable_name"].(string)] = row["Value"]
		}
	}

	return result, nil
}

// ==================== 辅助方法 ====================

// buildQuery 构建查询
func (dt *DatabaseTools) buildQuery(options *QueryOptions) *gorm.DB {
	query := dt.db

	if options == nil {
		return query
	}

	// 选择字段
	if len(options.Select) > 0 {
		query = query.Select(options.Select)
	}

	// 条件查询
	if len(options.Where) > 0 {
		query = dt.buildWhereClause(query, options.Where)
	}

	// 排序
	if len(options.Order) > 0 {
		for _, order := range options.Order {
			query = query.Order(order)
		}
	}

	// 预加载关联
	if len(options.Preload) > 0 {
		for _, preload := range options.Preload {
			query = query.Preload(preload)
		}
	}

	// 锁定
	if options.ForUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	} else if options.ForShare {
		query = query.Clauses(clause.Locking{Strength: "SHARE"})
	}

	// 限制和偏移
	if options.Limit > 0 {
		query = query.Limit(options.Limit)
	}
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}

	// 上下文和超时
	if options.WithContext && options.Timeout > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
		defer cancel()
		query = query.WithContext(ctx)
	}

	return query
}

// buildWhereClause 构建WHERE子句（安全的方式）
func (dt *DatabaseTools) buildWhereClause(db *gorm.DB, conditions map[string]interface{}) *gorm.DB {
	for field, value := range conditions {
		// 防止SQL注入，确保字段名只包含字母、数字和下划线
		if !dt.isValidFieldName(field) {
			continue
		}

		// 处理不同类型的值
		switch v := value.(type) {
		case []interface{}:
			// IN查询
			db = db.Where(fmt.Sprintf("%s IN ?", field), v)
		case string:
			// LIKE查询
			if strings.Contains(v, "%") {
				db = db.Where(fmt.Sprintf("%s LIKE ?", field), v)
			} else {
				db = db.Where(fmt.Sprintf("%s = ?", field), v)
			}
		default:
			// 等值查询
			db = db.Where(fmt.Sprintf("%s = ?", field), v)
		}
	}

	return db
}

// isValidFieldName 验证字段名是否安全
func (dt *DatabaseTools) isValidFieldName(field string) bool {
	// 只允许字母、数字和下划线
	for _, r := range field {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// getColumns 将列名转换为clause.Column
func (dt *DatabaseTools) getColumns(columnNames []string) []clause.Column {
	columns := make([]clause.Column, len(columnNames))
	for i, name := range columnNames {
		columns[i] = clause.Column{Name: name}
	}
	return columns
}

// isRetryableError 检查是否是可重试的错误
func (dt *DatabaseTools) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// MySQL 可重试错误代码
	errStr := err.Error()
	if strings.Contains(errStr, "deadlock") ||
		strings.Contains(errStr, "lock wait timeout") ||
		strings.Contains(errStr, "try restarting transaction") {
		return true
	}

	return false
}

// QueryWithTimeout 带超时的查询
func (dt *DatabaseTools) QueryWithTimeout(timeout time.Duration, fn func(*gorm.DB) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return fn(dt.db.WithContext(ctx))
}

// ==================== 快捷方法 ====================

// FindAll 查找所有记录
func (dt *DatabaseTools) FindAll(model interface{}) error {
	return dt.db.Find(model).Error
}

// FindByIDs 根据ID列表查找记录
func (dt *DatabaseTools) FindByIDs(model interface{}, ids []interface{}) error {
	return dt.db.Where("id IN ?", ids).Find(model).Error
}

// CreateRecord 创建记录
func (dt *DatabaseTools) CreateRecord(model interface{}) error {
	return dt.db.Create(model).Error
}

// CreateRecords 批量创建记录
func (dt *DatabaseTools) CreateRecords(models interface{}) error {
	return dt.db.Create(models).Error
}

// DeleteByID 根据ID删除记录
func (dt *DatabaseTools) DeleteByID(model interface{}, id interface{}) error {
	return dt.db.Delete(model, "id = ?", id).Error
}

// GetTotalCount 获取总记录数
func (dt *DatabaseTools) GetTotalCount(model interface{}) (int64, error) {
	var count int64
	err := dt.db.Model(model).Count(&count).Error
	return count, err
}

// GetMaxID 获取最大ID
func (dt *DatabaseTools) GetMaxID(model interface{}) (interface{}, error) {
	var maxID interface{}
	err := dt.db.Model(model).Select("MAX(id)").Scan(&maxID).Error
	return maxID, err
}

// GetMinID 获取最小ID
func (dt *DatabaseTools) GetMinID(model interface{}) (interface{}, error) {
	var minID interface{}
	err := dt.db.Model(model).Select("MIN(id)").Scan(&minID).Error
	return minID, err
}
