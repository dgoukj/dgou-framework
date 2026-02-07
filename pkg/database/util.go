package database

import (
	"context"
	"dgou/pkg/logger"
	"fmt"
	"reflect"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

// Paginate 分页查询
func Paginate(db *gorm.DB, pagination *Pagination, result interface{}) (*PaginationResult, error) {
	if pagination == nil {
		pagination = &Pagination{
			Page:     1,
			PageSize: 20,
		}
	}

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

	// 查询总数
	var total int64
	countDB := db.Session(&gorm.Session{})
	if err := countDB.Count(&total).Error; err != nil {
		return nil, err
	}

	// 计算总页数
	totalPage := int((total + int64(pagination.PageSize) - 1) / int64(pagination.PageSize))

	// 查询数据
	if err := db.Offset(offset).Limit(pagination.PageSize).Find(result).Error; err != nil {
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

// BatchInsert 批量插入数据
func BatchInsert(db *gorm.DB, data interface{}, batchSize int) error {
	if batchSize <= 0 {
		batchSize = 1000 // 默认批次大小
	}

	value := reflect.ValueOf(data)
	if value.Kind() != reflect.Slice {
		return fmt.Errorf("data must be a slice")
	}

	length := value.Len()
	if length == 0 {
		return nil
	}

	// 使用事务
	return db.Transaction(func(tx *gorm.DB) error {
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
	})
}

// SoftDelete 软删除记录
func SoftDelete(db *gorm.DB, model interface{}, id interface{}) error {
	return db.Model(model).Where("id = ?", id).Update("deleted_at", time.Now()).Error
}

// BulkUpsert 批量插入或更新
func BulkUpsert(db *gorm.DB, data interface{}, conflictColumns []string, updateColumns []string) error {
	return db.Clauses(clause.OnConflict{
		Columns:   getColumns(conflictColumns),
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(data).Error
}

// getColumns 将列名转换为clause.Column
func getColumns(columnNames []string) []clause.Column {
	columns := make([]clause.Column, len(columnNames))
	for i, name := range columnNames {
		columns[i] = clause.Column{Name: name}
	}
	return columns
}

// QueryWithTimeout 带超时的查询
func QueryWithTimeout(ctx context.Context, db *gorm.DB, timeout time.Duration, fn func(*gorm.DB) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return fn(db.WithContext(ctx))
}

// LockForUpdate 锁定记录用于更新
func LockForUpdate(db *gorm.DB) *gorm.DB {
	return db.Clauses(clause.Locking{Strength: "UPDATE"})
}

// LockForShare 锁定记录用于共享读取
func LockForShare(db *gorm.DB) *gorm.DB {
	return db.Clauses(clause.Locking{Strength: "SHARE"})
}

// BuildWhereClause 构建WHERE子句（安全的方式）
func BuildWhereClause(db *gorm.DB, conditions map[string]interface{}) *gorm.DB {
	for field, value := range conditions {
		// 防止SQL注入，确保字段名只包含字母、数字和下划线
		if !isValidFieldName(field) {
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
func isValidFieldName(field string) bool {
	// 只允许字母、数字和下划线
	for _, r := range field {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// GetTableSize 获取表大小
func GetTableSize(db *gorm.DB, tableName string) (int64, error) {
	var size int64
	query := fmt.Sprintf("SELECT data_length + index_length FROM information_schema.TABLES WHERE table_schema = DATABASE() AND table_name = '%s'", tableName)

	if err := db.Raw(query).Scan(&size).Error; err != nil {
		return 0, err
	}

	return size, nil
}

// OptimizeTable 优化表
func OptimizeTable(db *gorm.DB, tableName string) error {
	query := fmt.Sprintf("OPTIMIZE TABLE %s", tableName)
	return db.Exec(query).Error
}

// ExplainQuery 解释查询计划
func ExplainQuery(db *gorm.DB, query string, args ...interface{}) (string, error) {
	var explainRows []map[string]interface{}
	explainQuery := "EXPLAIN " + query

	if err := db.Raw(explainQuery, args...).Scan(&explainRows).Error; err != nil {
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
