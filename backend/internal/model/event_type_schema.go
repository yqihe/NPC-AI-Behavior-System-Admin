package model

import (
	"encoding/json"
	"time"
)

// 扩展字段类型常量统一定义在 util/const.go
// 使用 util.ExtFieldTypeInt / util.ValidExtFieldTypes 等

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
	HasRefs   bool      `json:"has_refs" db:"-"` // 非 DB 列，service 层通过 schema_refs 填充
	Version   int       `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Deleted   bool      `json:"-" db:"deleted"`
}

// EventTypeSchemaLite 精简版（给详情接口 extension_schema + 内存缓存）
type EventTypeSchemaLite struct {
	ID           int64           `json:"id" db:"id"`
	FieldName    string          `json:"field_name" db:"field_name"`
	FieldLabel   string          `json:"field_label" db:"field_label"`
	FieldType    string          `json:"field_type" db:"field_type"`
	Constraints  json.RawMessage `json:"constraints" db:"constraints"`
	DefaultValue json.RawMessage `json:"default_value" db:"default_value"`
	SortOrder    int             `json:"sort_order" db:"sort_order"`
	Enabled      bool            `json:"enabled" db:"enabled"`
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// EventTypeSchemaListQuery 列表查询参数
type EventTypeSchemaListQuery struct {
	FieldName  string `json:"field_name"`        // 字段标识模糊搜索
	FieldLabel string `json:"field_label"`       // 中文标签模糊搜索
	Enabled    *bool  `json:"enabled,omitempty"` // nil=不筛选
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
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

// ──────────────────────────────────────────────
// 引用关系
// ──────────────────────────────────────────────

// SchemaRef 扩展字段引用关系（schema_refs 表行）
type SchemaRef struct {
	SchemaID int64  `json:"schema_id" db:"schema_id"`
	RefType  string `json:"ref_type" db:"ref_type"`
	RefID    int64  `json:"ref_id" db:"ref_id"`
}

// SchemaReferenceItem 引用详情中的单条引用方
type SchemaReferenceItem struct {
	RefType string `json:"ref_type"` // "event_type"
	RefID   int64  `json:"ref_id"`
	Label   string `json:"label"` // 引用方中文名
}

// SchemaReferenceDetail 扩展字段引用详情
type SchemaReferenceDetail struct {
	SchemaID   int64                 `json:"schema_id"`
	FieldLabel string                `json:"field_label"`
	EventTypes []SchemaReferenceItem `json:"event_types"`
	FsmConfigs []SchemaReferenceItem `json:"fsm_configs"`
	BtTrees    []SchemaReferenceItem `json:"bt_trees"`
}
