package database

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 基础模型（包含常用字段）
type BaseModel struct {
	ID        uint           `gorm:"primarykey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index;not null" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// TimestampModel 时间戳模型（不含ID）
type TimestampModel struct {
	CreatedAt time.Time      `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index;not null" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// UUIDModel UUID主键模型
type UUIDModel struct {
	ID        string         `gorm:"primarykey;type:char(36);default:(UUID())" json:"id"`
	CreatedAt time.Time      `gorm:"index;not null" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index;not null" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// AuditModel 审计模型（包含创建者和更新者）
type AuditModel struct {
	BaseModel
	CreatedBy uint `gorm:"index;not null;default:0" json:"created_by"`
	UpdatedBy uint `gorm:"index;not null;default:0" json:"updated_by"`
}

// SoftDelete 软删除接口
type SoftDelete interface {
	GetDeletedAt() gorm.DeletedAt
	SetDeletedAt(time.Time)
}

// BeforeCreate 创建前的钩子
func (m *BaseModel) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = now
	}
	return nil
}

// BeforeUpdate 更新前的钩子
func (m *BaseModel) BeforeUpdate(tx *gorm.DB) error {
	m.UpdatedAt = time.Now()
	return nil
}

// BeforeDelete 删除前的钩子
func (m *BaseModel) BeforeDelete(tx *gorm.DB) error {
	// 如果是软删除，设置删除时间
	if tx.Statement.Unscoped {
		return nil
	}
	m.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	return nil
}
