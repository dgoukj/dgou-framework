package database

import (
	"dgou/pkg/logger"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Migration 迁移结构
type Migration struct {
	ID        string    `gorm:"column:id;primaryKey;type:varchar(255)"`
	Name      string    `gorm:"column:name;type:varchar(255)"`
	Batch     int       `gorm:"column:batch"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

// TableName 指定表名
func (Migration) TableName() string {
	return "migrations"
}

// Migrator 迁移管理器
type Migrator struct {
	db            *gorm.DB        // 数据库连接
	migrations    []MigrationFile // 迁移文件列表
	migrationsDir string          // 迁移文件目录
}

// MigrationFile 迁移文件
type MigrationFile struct {
	ID       string // 迁移ID（时间戳）
	Name     string // 迁移名称
	FilePath string // 文件路径
	UpSQL    string // 升级SQL
	DownSQL  string // 回滚SQL
}

// NewMigrator 创建迁移管理器
func NewMigrator(db *gorm.DB, migrationsDir string) *Migrator {
	return &Migrator{
		db:            db,
		migrationsDir: migrationsDir,
		migrations:    make([]MigrationFile, 0),
	}
}

// LoadMigrations 加载迁移文件
func (m *Migrator) LoadMigrations() error {
	// 检查迁移目录是否存在
	if _, err := os.Stat(m.migrationsDir); os.IsNotExist(err) {
		logger.Warn("Migrations directory does not exist, creating...",
			logger.String("dir", m.migrationsDir),
		)
		if err := os.MkdirAll(m.migrationsDir, 0755); err != nil {
			return fmt.Errorf("failed to create migrations directory: %w", err)
		}
	}

	// 读取迁移文件
	files, err := os.ReadDir(m.migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			migration, err := m.parseMigrationFile(file.Name())
			if err != nil {
				logger.Warn("Failed to parse migration file",
					logger.String("file", file.Name()),
					logger.ErrorField(err),
				)
				continue
			}
			m.migrations = append(m.migrations, migration)
		}
	}

	// 按ID排序
	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].ID < m.migrations[j].ID
	})

	logger.Info("Migrations loaded",
		logger.Int("count", len(m.migrations)),
	)

	return nil
}

// parseMigrationFile 解析迁移文件
func (m *Migrator) parseMigrationFile(filename string) (MigrationFile, error) {
	// 文件名格式: 20230101000000_create_users_table.sql
	parts := strings.Split(strings.TrimSuffix(filename, ".sql"), "_")
	if len(parts) < 2 {
		return MigrationFile{}, fmt.Errorf("invalid migration file name format: %s", filename)
	}

	id := parts[0]
	name := strings.Join(parts[1:], "_")

	filePath := filepath.Join(m.migrationsDir, filename)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return MigrationFile{}, fmt.Errorf("failed to read migration file: %w", err)
	}

	// 解析SQL内容，分隔UP和DOWN部分
	sqlContent := string(content)
	var upSQL, downSQL string

	if strings.Contains(sqlContent, "-- +migrate Up") && strings.Contains(sqlContent, "-- +migrate Down") {
		parts := strings.Split(sqlContent, "-- +migrate Down")
		if len(parts) == 2 {
			upPart := strings.TrimPrefix(parts[0], "-- +migrate Up")
			upSQL = strings.TrimSpace(upPart)
			downSQL = strings.TrimSpace(parts[1])
		}
	} else {
		// 如果没有分隔符，整个文件作为UP，没有DOWN
		upSQL = strings.TrimSpace(sqlContent)
	}

	return MigrationFile{
		ID:       id,
		Name:     name,
		FilePath: filePath,
		UpSQL:    upSQL,
		DownSQL:  downSQL,
	}, nil
}

// CreateMigrationTable 创建迁移表
func (m *Migrator) CreateMigrationTable() error {
	return m.db.AutoMigrate(&Migration{})
}

// GetAppliedMigrations 获取已应用的迁移
func (m *Migrator) GetAppliedMigrations() ([]Migration, error) {
	var migrations []Migration
	result := m.db.Order("id asc").Find(&migrations)
	return migrations, result.Error
}

// GetNextBatchNumber 获取下一个批次号
func (m *Migrator) GetNextBatchNumber() (int, error) {
	var maxBatch int
	result := m.db.Model(&Migration{}).Select("COALESCE(MAX(batch), 0)").Scan(&maxBatch)
	if result.Error != nil {
		return 0, result.Error
	}
	return maxBatch + 1, nil
}

// Migrate 执行迁移
func (m *Migrator) Migrate() error {
	// 创建迁移表（如果不存在）
	if err := m.CreateMigrationTable(); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// 获取已应用的迁移
	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedMap := make(map[string]Migration)
	for _, migration := range appliedMigrations {
		appliedMap[migration.ID] = migration
	}

	// 获取下一个批次号
	batch, err := m.GetNextBatchNumber()
	if err != nil {
		return fmt.Errorf("failed to get next batch number: %w", err)
	}

	// 执行未应用的迁移
	var migrated []string
	for _, migrationFile := range m.migrations {
		if _, applied := appliedMap[migrationFile.ID]; !applied {
			logger.Info("Applying migration",
				logger.String("id", migrationFile.ID),
				logger.String("name", migrationFile.Name),
				logger.Int("batch", batch),
			)

			// 在事务中执行迁移
			err := m.db.Transaction(func(tx *gorm.DB) error {
				// 执行UP SQL
				if migrationFile.UpSQL != "" {
					if err := tx.Exec(migrationFile.UpSQL).Error; err != nil {
						return fmt.Errorf("failed to execute up migration: %w", err)
					}
				}

				// 记录迁移
				migration := Migration{
					ID:        migrationFile.ID,
					Name:      migrationFile.Name,
					Batch:     batch,
					AppliedAt: time.Now(),
				}

				if err := tx.Create(&migration).Error; err != nil {
					return fmt.Errorf("failed to record migration: %w", err)
				}

				return nil
			})

			if err != nil {
				return fmt.Errorf("migration failed: %s - %w", migrationFile.ID, err)
			}

			migrated = append(migrated, migrationFile.ID)
			logger.Info("Migration applied successfully",
				logger.String("id", migrationFile.ID),
			)
		}
	}

	if len(migrated) == 0 {
		logger.Info("No new migrations to apply")
	} else {
		logger.Info("Migrations completed",
			logger.Int("count", len(migrated)),
			logger.String("ids", strings.Join(migrated, ", ")),
		)
	}

	return nil
}

// Rollback 回滚迁移
func (m *Migrator) Rollback(steps int) error {
	// 获取已应用的迁移
	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(appliedMigrations) == 0 {
		logger.Info("No migrations to rollback")
		return nil
	}

	// 按批次分组
	batchGroups := make(map[int][]Migration)
	for _, migration := range appliedMigrations {
		batchGroups[migration.Batch] = append(batchGroups[migration.Batch], migration)
	}

	// 获取批次号列表并排序
	batches := make([]int, 0, len(batchGroups))
	for batch := range batchGroups {
		batches = append(batches, batch)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(batches)))

	// 确定要回滚的批次
	rollbackBatches := batches[:min(steps, len(batches))]

	var rolledBack []string

	// 回滚每个批次
	for _, batch := range rollbackBatches {
		migrations := batchGroups[batch]
		// 按ID倒序回滚（后应用的先回滚）
		sort.Slice(migrations, func(i, j int) bool {
			return migrations[i].ID > migrations[j].ID
		})

		for _, migration := range migrations {
			// 找到对应的迁移文件
			var migrationFile *MigrationFile
			for _, mf := range m.migrations {
				if mf.ID == migration.ID {
					migrationFile = &mf
					break
				}
			}

			if migrationFile == nil {
				logger.Warn("Migration file not found, skipping",
					logger.String("id", migration.ID),
				)
				continue
			}

			if migrationFile.DownSQL == "" {
				logger.Warn("Migration has no down SQL, skipping",
					logger.String("id", migration.ID),
				)
				continue
			}

			logger.Info("Rolling back migration",
				logger.String("id", migration.ID),
				logger.String("name", migration.Name),
				logger.Int("batch", migration.Batch),
			)

			// 在事务中执行回滚
			err := m.db.Transaction(func(tx *gorm.DB) error {
				// 执行DOWN SQL
				if err := tx.Exec(migrationFile.DownSQL).Error; err != nil {
					return fmt.Errorf("failed to execute down migration: %w", err)
				}

				// 删除迁移记录
				if err := tx.Delete(&migration).Error; err != nil {
					return fmt.Errorf("failed to delete migration record: %w", err)
				}

				return nil
			})

			if err != nil {
				return fmt.Errorf("rollback failed: %s - %w", migration.ID, err)
			}

			rolledBack = append(rolledBack, migration.ID)
			logger.Info("Migration rolled back successfully",
				logger.String("id", migration.ID),
			)
		}
	}

	if len(rolledBack) == 0 {
		logger.Info("No migrations were rolled back")
	} else {
		logger.Info("Rollback completed",
			logger.Int("count", len(rolledBack)),
			logger.String("ids", strings.Join(rolledBack, ", ")),
		)
	}

	return nil
}

// CreateMigration 创建新的迁移文件
func (m *Migrator) CreateMigration(name string) (string, error) {
	// 生成时间戳ID
	timestamp := time.Now().Format("20060102150405")
	id := timestamp
	filename := fmt.Sprintf("%s_%s.sql", id, name)
	filepath := filepath.Join(m.migrationsDir, filename)

	// 创建迁移文件模板
	template := `-- +migrate Up
-- SQL in section 'Up' is executed when this migration is applied

-- +migrate Down
-- SQL section 'Down' is executed when this migration is rolled back

`

	if err := os.WriteFile(filepath, []byte(template), 0644); err != nil {
		return "", fmt.Errorf("failed to create migration file: %w", err)
	}

	logger.Info("Migration file created",
		logger.String("filename", filename),
		logger.String("path", filepath),
	)

	return filepath, nil
}

// Status 显示迁移状态
func (m *Migrator) Status() (map[string]interface{}, error) {
	// 获取已应用的迁移
	appliedMigrations, err := m.GetAppliedMigrations()
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]Migration)
	for _, migration := range appliedMigrations {
		appliedMap[migration.ID] = migration
	}

	// 统计信息
	stats := map[string]interface{}{
		"total_files":   len(m.migrations),
		"total_applied": len(appliedMigrations),
		"total_pending": len(m.migrations) - len(appliedMigrations),
		"migrations":    make([]map[string]interface{}, 0),
	}

	// 每个迁移的详细信息
	for _, migrationFile := range m.migrations {
		status := "pending"
		batch := 0
		appliedAt := ""

		if applied, ok := appliedMap[migrationFile.ID]; ok {
			status = "applied"
			batch = applied.Batch
			appliedAt = applied.AppliedAt.Format(time.RFC3339)
		}

		stats["migrations"] = append(stats["migrations"].([]map[string]interface{}), map[string]interface{}{
			"id":         migrationFile.ID,
			"name":       migrationFile.Name,
			"status":     status,
			"batch":      batch,
			"applied_at": appliedAt,
		})
	}

	return stats, nil
}

// min 辅助函数，返回最小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetMigrationSQL 获取迁移的SQL语句
func (m *Migrator) GetMigrationSQL(id string) (map[string]string, error) {
	for _, migration := range m.migrations {
		if migration.ID == id {
			return map[string]string{
				"up":   migration.UpSQL,
				"down": migration.DownSQL,
			}, nil
		}
	}
	return nil, fmt.Errorf("migration not found: %s", id)
}
