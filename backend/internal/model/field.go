package model

import (
	"encoding/json"
	"time"
)

// Field 字段定义
type Field struct {
	ID         int64           `json:"id" db:"id"`
	Name       string          `json:"name" db:"name"`
	Label      string          `json:"label" db:"label"`
	Type       string          `json:"type" db:"type"`
	Category   string          `json:"category" db:"category"`
	Properties json.RawMessage `json:"properties" db:"properties"`

	RefCount  int       `json:"ref_count" db:"ref_count"`
	Version   int       `json:"version" db:"version"`
	Deleted   bool      `json:"-" db:"deleted"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// FieldListItem 列表页展示项（覆盖索引返回，不含 properties）
type FieldListItem struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Label     string    `json:"label" db:"label"`
	Type      string    `json:"type" db:"type"`
	Category  string    `json:"category" db:"category"`
	RefCount  int       `json:"ref_count" db:"ref_count"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// 以下字段由代码层翻译填充，不从 DB 读取
	TypeLabel     string `json:"type_label" db:"-"`
	CategoryLabel string `json:"category_label" db:"-"`
}

// FieldProperties 动态属性（存在 properties JSON 列中）
type FieldProperties struct {
	Description  string          `json:"description,omitempty"`
	ExposeBB     bool            `json:"expose_bb"`
	DefaultValue json.RawMessage `json:"default_value,omitempty"`
	Constraints  json.RawMessage `json:"constraints,omitempty"`
}

// FieldRef 字段引用关系
type FieldRef struct {
	FieldName string `json:"field_name" db:"field_name"`
	RefType   string `json:"ref_type" db:"ref_type"`
	RefName   string `json:"ref_name" db:"ref_name"`
}

// FieldListQuery 列表查询参数
type FieldListQuery struct {
	Label    string `json:"label"`
	Type     string `json:"type"`
	Category string `json:"category"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// CreateFieldRequest 创建字段请求
type CreateFieldRequest struct {
	Name       string          `json:"name"`
	Label      string          `json:"label"`
	Type       string          `json:"type"`
	Category   string          `json:"category"`
	Properties json.RawMessage `json:"properties"`
}

// UpdateFieldRequest 编辑字段请求
type UpdateFieldRequest struct {
	Label      string          `json:"label"`
	Type       string          `json:"type"`
	Category   string          `json:"category"`
	Properties json.RawMessage `json:"properties"`
	Version    int             `json:"version"`
}
