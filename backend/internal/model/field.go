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

	ExposeBB bool `json:"expose_bb" db:"expose_bb"` // 是否暴露给 BB 系统（独立列）
	Enabled  bool `json:"enabled" db:"enabled"`
	HasRefs  bool `json:"has_refs" db:"-"` // 非 DB 列，service 层通过 field_refs 填充
	Version   int       `json:"version" db:"version"`
	Deleted   bool      `json:"-" db:"deleted"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// FieldLite 给跨模块调用的字段精简结构
//
// 用途：模板管理详情接口由 handler 调 fieldService.GetByIDsLite 拿到，
// 用于拼装 TemplateFieldItem。CategoryLabel 由 service 层翻译填充。
type FieldLite struct {
	ID            int64  `json:"id" db:"id"`
	Name          string `json:"name" db:"name"`
	Label         string `json:"label" db:"label"`
	Type          string `json:"type" db:"type"`
	Category      string `json:"category" db:"category"`
	CategoryLabel string `json:"category_label" db:"-"` // service 层翻译
	Enabled       bool   `json:"enabled" db:"enabled"`
}

// FieldListItem 列表页展示项（覆盖索引返回，不含 properties）
type FieldListItem struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Label     string    `json:"label" db:"label"`
	Type      string    `json:"type" db:"type"`
	Category  string    `json:"category" db:"category"`
	Enabled   bool      `json:"enabled" db:"enabled"`
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

// 字段类型常量、引用来源类型常量统一定义在 util/const.go
// 使用 util.FieldTypeReference / util.RefTypeTemplate / util.RefTypeField

// FieldRef 字段引用关系（改用 ID 关联）
type FieldRef struct {
	FieldID int64  `json:"field_id" db:"field_id"`
	RefType string `json:"ref_type" db:"ref_type"`
	RefID   int64  `json:"ref_id" db:"ref_id"`
}

// FieldListQuery 列表查询参数
type FieldListQuery struct {
	Label     string `json:"label"`
	Type      string `json:"type"`
	Category  string `json:"category"`
	Enabled   *bool  `json:"enabled,omitempty"`   // nil=不筛选（字段管理页），true=仅启用（其他模块选字段）
	ExposesBB *bool  `json:"bb_exposed,omitempty" form:"bb_exposed"` // nil=不筛选，true=仅暴露 BB 的字段
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
}

// CreateFieldRequest 创建字段请求（无 ID）
type CreateFieldRequest struct {
	Name       string          `json:"name"`
	Label      string          `json:"label"`
	Type       string          `json:"type"`
	Category   string          `json:"category"`
	Properties json.RawMessage `json:"properties"`
}

// UpdateFieldRequest 编辑字段请求（有 ID，无 name）
type UpdateFieldRequest struct {
	ID         int64           `json:"id"`
	Label      string          `json:"label"`
	Type       string          `json:"type"`
	Category   string          `json:"category"`
	Properties json.RawMessage `json:"properties"`
	Version    int             `json:"version"`
}

// IDRequest 通用的按 ID 查询请求
type IDRequest struct {
	ID int64 `json:"id"`
}

// IDVersionRequest 通用的 ID + 乐观锁版本号请求（用于需要乐观锁的 Delete 操作）
type IDVersionRequest struct {
	ID      int64 `json:"id"`
	Version int   `json:"version"`
}

// ReferenceItem 引用详情中的单条引用方
type ReferenceItem struct {
	RefType string `json:"ref_type"` // "template" / "field" / "fsm"
	RefID   int64  `json:"ref_id"`   // 引用方 ID
	Label   string `json:"label"`    // 引用方中文名
}

// ReferenceDetail 字段引用详情
type ReferenceDetail struct {
	FieldID    int64           `json:"field_id"`
	FieldLabel string          `json:"field_label"`
	Templates  []ReferenceItem `json:"templates"`
	Fields     []ReferenceItem `json:"fields"`
	Fsms       []ReferenceItem `json:"fsms"`
}

// ToggleEnabledRequest 启用/停用请求（改用 ID）
type ToggleEnabledRequest struct {
	ID      int64 `json:"id"`
	Enabled bool  `json:"enabled"`
	Version int   `json:"version"`
}

// FieldListData 字段列表数据（类型安全，用于缓存序列化/反序列化）
type FieldListData struct {
	Items    []FieldListItem `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *FieldListData) ToListData() *ListData {
	return &ListData{
		Items:    d.Items,
		Total:    d.Total,
		Page:     d.Page,
		PageSize: d.PageSize,
	}
}

// CreateFieldResponse 创建字段响应
type CreateFieldResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CheckNameRequest 唯一性校验请求（保留 name，创建前校验）
type CheckNameRequest struct {
	Name string `json:"name"`
}

// CheckNameResult 唯一性校验结果
type CheckNameResult struct {
	Available bool   `json:"available"`
	Message   string `json:"message"`
}

// DeleteResult 删除结果
type DeleteResult struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
}
