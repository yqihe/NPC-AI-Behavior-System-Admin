package model

import (
	"encoding/json"
	"time"
)

// Template 模板定义（templates 表整行）
//
// 模板是 ADMIN 内部的"字段组合方案"。NPC 创建时选一个模板填值，
// 创建后 NPC 把字段列表+值快照下来，与模板独立。
type Template struct {
	ID          int64           `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Label       string          `json:"label" db:"label"`
	Description string          `json:"description" db:"description"`
	Fields      json.RawMessage `json:"fields" db:"fields"` // [{field_id, required}, ...]

	RefCount  int       `json:"ref_count" db:"ref_count"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	Version   int       `json:"version" db:"version"`
	Deleted   bool      `json:"-" db:"deleted"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TemplateFieldEntry templates.fields JSON 数组的单元
//
// **数组顺序就是 NPC 表单展示顺序** —— 前端"上下移动"按钮即修改此数组顺序。
type TemplateFieldEntry struct {
	FieldID  int64 `json:"field_id"`
	Required bool  `json:"required"`
}

// TemplateListItem 列表项（覆盖索引返回，不含 fields/description）
type TemplateListItem struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Label     string    `json:"label" db:"label"`
	RefCount  int       `json:"ref_count" db:"ref_count"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TemplateLite 给跨模块调用的精简结构（id/name/label）
//
// 用途：字段引用详情接口由 handler 调 templateService.GetByIDsLite 拿到，
// 用于补 template label。
type TemplateLite struct {
	ID    int64  `json:"id" db:"id"`
	Name  string `json:"name" db:"name"`
	Label string `json:"label" db:"label"`
}

// TemplateListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type TemplateListData struct {
	Items    []TemplateListItem `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *TemplateListData) ToListData() *ListData {
	return &ListData{
		Items:    d.Items,
		Total:    d.Total,
		Page:     d.Page,
		PageSize: d.PageSize,
	}
}

// TemplateDetail 详情接口最终响应
//
// **不进缓存**：handler 层从 TemplateService 拿到 *Template 裸行 +
// FieldService.GetByIDsLite 拿到字段精简列表后，在 handler 层组装。
// 这样字段被编辑/停用时不会污染模板缓存。
type TemplateDetail struct {
	ID          int64               `json:"id"`
	Name        string              `json:"name"`
	Label       string              `json:"label"`
	Description string              `json:"description"`
	Enabled     bool                `json:"enabled"`
	Version     int                 `json:"version"`
	RefCount    int                 `json:"ref_count"`
	CreatedAt   time.Time           `json:"created_at"`
	UpdatedAt   time.Time           `json:"updated_at"`
	Fields      []TemplateFieldItem `json:"fields"` // 顺序与 templates.fields JSON 数组一致
}

// TemplateFieldItem 详情中的字段精简信息
//
// 由 handler 把 FieldLite + Required 拼装而成。
type TemplateFieldItem struct {
	FieldID       int64  `json:"field_id"`
	Name          string `json:"name"`
	Label         string `json:"label"`
	Type          string `json:"type"`
	Category      string `json:"category"`
	CategoryLabel string `json:"category_label"` // dictionary 翻译
	Enabled       bool   `json:"enabled"`        // 字段当前是否启用（用于前端标灰停用字段）
	Required      bool   `json:"required"`       // 模板里的必填配置
}

// TemplateListQuery 列表查询参数
type TemplateListQuery struct {
	Label    string `json:"label"`
	Enabled  *bool  `json:"enabled,omitempty"` // nil=不筛选（管理页），true=仅启用（NPC 管理页）
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	Name        string               `json:"name"`
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Fields      []TemplateFieldEntry `json:"fields"`
}

// CreateTemplateResponse 创建模板响应
type CreateTemplateResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// UpdateTemplateRequest 编辑模板请求（无 name，name 创建后不可变）
type UpdateTemplateRequest struct {
	ID          int64                `json:"id"`
	Label       string               `json:"label"`
	Description string               `json:"description"`
	Fields      []TemplateFieldEntry `json:"fields"`
	Version     int                  `json:"version"`
}

// TemplateReferenceItem NPC 引用方
//
// NPC 模块未上线前，引用详情接口返回空 npcs 数组。
type TemplateReferenceItem struct {
	NPCID   int64  `json:"npc_id"`
	NPCName string `json:"npc_name"`
}

// TemplateReferenceDetail 模板引用详情
type TemplateReferenceDetail struct {
	TemplateID    int64                   `json:"template_id"`
	TemplateLabel string                  `json:"template_label"`
	NPCs          []TemplateReferenceItem `json:"npcs"`
}
