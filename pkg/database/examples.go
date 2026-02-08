// pkg/database/examples.go
package database

import (
	"dgou/pkg/logger"
	"gorm.io/gorm"
)

// 示例代码，展示如何使用 DatabaseTools
// 在实际项目中，可以删除此文件或将其用作文档

// ExampleUsage 示例用法
func ExampleUsage() {
	// 获取数据库工具实例
	tools := GetTools()
	if tools == nil {
		// 处理错误
		return
	}

	// 示例1：分页查询用户
	type User struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var users []User
	result, err := tools.Paginate(&Pagination{
		Page:     1,
		PageSize: 20,
	}, &users)

	if err != nil {
		// 处理错误
	}

	// 使用分页结果
	_ = result

	// 示例2：批量插入数据
	newUsers := []User{
		{Name: "张三", Age: 25},
		{Name: "李四", Age: 30},
		// ...
	}

	err = tools.BatchInsert(newUsers, &BatchOptions{
		BatchSize:   100,
		EnableRetry: true,
		MaxRetries:  3,
	})

	// 示例3：执行事务
	err = tools.ExecuteTransaction(func(tx *gorm.DB) error {
		// 执行多个数据库操作
		// 如果返回错误，事务会自动回滚
		return nil
	})

	// 示例4：带选项的查询
	err = tools.QueryWithOptions(&users, &QueryOptions{
		Where: map[string]interface{}{
			"age": 25,
		},
		Order:   []string{"created_at DESC"},
		Limit:   10,
		Preload: []string{"Profile", "Roles"},
	})

	// 示例5：表维护
	size, err := tools.GetTableSize("users")
	if err == nil {
		logger.Info("Table size", logger.Int64("size", size))
	}

	// 示例6：获取数据库统计
	stats, err := tools.GetDBStats()
	if err == nil {
		// 使用统计信息
		_ = stats
	}
}
