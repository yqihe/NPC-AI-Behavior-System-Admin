package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// 扩展字段类型常量
// ──────────────────────────────────────────────

const (
	ExtFieldTypeInt    = "int"
	ExtFieldTypeFloat  = "float"
	ExtFieldTypeString = "string"
	ExtFieldTypeBool   = "bool"
	ExtFieldTypeSelect = "select"
)

// ValidExtFieldTypes 合法枚举集合（handler 校验用，不含 reference）
var ValidExtFieldTypes = map[string]bool{
	ExtFieldTypeInt:    true,
	ExtFieldTypeFloat:  true,
	ExtFieldTypeString: true,
	ExtFieldTypeBool:   true,
	ExtFieldTypeSelect: true,
}

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// EventTypeSchema 扩展字段定义（event_type_schema 表整行）
type EventTypeSchema struct {
	ID           int64           `json:"id" db:"id"`
	FieldName    string          `json:"field_name" db:"field_name"`
	FieldLabel   string          `json:"field_label" db:"field_label"`
	FieldType    string          `json:"field_type" db:"field_type"`
	Constraints  json.RawMessage `json:"constraints" db:"constraints"`
	DefaultValue json.RawMessage `json:"default_value" db:"default_value"`
	SortOrder    int             `json:"sort_order" db:"sort_order"`

	Enabled   bool      `json:"enabled" db:"enabled"`
	Version   int       `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Deleted   bool      `json:"-" db:"deleted"`
}

// EventTypeSchemaLite 精简版（给详情接口 extension_schema + 内存缓存）
type EventTypeSchemaLite struct {
	FieldName    string          `json:"field_name" db:"field_name"`
	FieldLabel   string          `json:"field_label" db:"field_label"`
	FieldType    string          `json:"field_type" db:"field_type"`
	Constraints  json.RawMessage `json:"constraints" db:"constraints"`
	DefaultValue json.RawMessage `json:"default_value" db:"default_value"`
	SortOrder    int             `json:"sort_order" db:"sort_order"`
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// EventTypeSchemaListQuery 列表查询参数
type EventTypeSchemaListQuery struct {
	Enabled *bool `json:"enabled,omitempty"` // nil=不筛选
}

// CreateEventTypeSchemaRequest 创建扩展字段 Schema 请求
type CreateEventTypeSchemaRequest struct {
	FieldName    string          `json:"field_name"`
	FieldLabel   string          `json:"field_label"`
	FieldType    string          `json:"field_type"`
	Constraints  json.RawMessage `json:"constraints"`
	DefaultValue json.RawMessage `json:"default_value"`
	SortOrder    int             `json:"sort_order"`
}

// CreateEventTypeSchemaResponse 创建响应
type CreateEventTypeSchemaResponse struct {
	ID        int64  `json:"id"`
	FieldName string `json:"field_name"`
}

// UpdateEventTypeSchemaRequest 编辑请求（无 field_name / field_type，不可变）
type UpdateEventTypeSchemaRequest struct {
	ID           int64           `json:"id"`
	FieldLabel   string          `json:"field_label"`
	Constraints  json.RawMessage `json:"constraints"`
	DefaultValue json.RawMessage `json:"default_value"`
	SortOrder    int             `json:"sort_order"`
	Version      int             `json:"version"`
}
