package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// NPC NPC 实例（npcs 表整行）
//
// NPC 是模板的具体实例。创建时快照模板字段列表+值，与模板后续变更无关。
// behavior 配置（fsm_ref + bt_refs）独立于字段系统，导出 API 按 api-contract.md 输出。
type NPC struct {
	ID           int64           `json:"id"            db:"id"`
	Name         string          `json:"name"          db:"name"`
	Label        string          `json:"label"         db:"label"`
	Description  string          `json:"description"   db:"description"`
	TemplateID   int64           `json:"template_id"   db:"template_id"`
	TemplateName string          `json:"template_name" db:"template_name"`
	Fields       json.RawMessage `json:"fields"        db:"fields"`   // [{field_id, name, required, value}, ...]
	FsmRef       string          `json:"fsm_ref"       db:"fsm_ref"`  // 空串=无行为配置
	BtRefs       json.RawMessage `json:"bt_refs"       db:"bt_refs"`  // {"state_name": "bt_tree_name"}
	Enabled      bool            `json:"enabled"       db:"enabled"`
	Version      int             `json:"version"       db:"version"`
	CreatedAt    time.Time       `json:"created_at"    db:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"    db:"updated_at"`
	Deleted      bool            `json:"-"             db:"deleted"`
}

// NPCFieldEntry fields JSON 数组的单元
//
// field_id：用于编辑时回查字段元数据（type/constraints/label）
// name：字段标识符快照，用于导出 key
// required：来自创建时模板的 required 标记
// value：JSON 原始值，保留类型（number/string/bool/null）
type NPCFieldEntry struct {
	FieldID  int64           `json:"field_id"`
	Name     string          `json:"name"`
	Required bool            `json:"required"`
	Value    json.RawMessage `json:"value"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// NPCListItem 列表页展示项（不含 fields/bt_refs，减少传输量）
//
// TemplateLabel 由 handler 层跨模块调 TemplateService.GetByIDsLite 补全，不从 DB 扫描。
type NPCListItem struct {
	ID            int64     `json:"id"             db:"id"`
	Name          string    `json:"name"           db:"name"`
	Label         string    `json:"label"          db:"label"`
	TemplateID    int64     `json:"template_id"    db:"template_id"`
	TemplateName  string    `json:"template_name"  db:"template_name"`
	TemplateLabel string    `json:"template_label" db:"-"` // 跨模块补全
	FsmRef        string    `json:"fsm_ref"        db:"fsm_ref"`
	Enabled       bool      `json:"enabled"        db:"enabled"`
	CreatedAt     time.Time `json:"created_at"     db:"created_at"`
}

// NPCListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type NPCListData struct {
	Items    []NPCListItem `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *NPCListData) ToListData() *ListData {
	return &ListData{Items: d.Items, Total: d.Total, Page: d.Page, PageSize: d.PageSize}
}

// ──────────────────────────────────────────────
// 详情响应
// ──────────────────────────────────────────────

// NPCDetail 详情接口响应（handler 层组装，不进缓存）
//
// handler 层从 NPCService.GetByID 拿到 *NPC 裸行后：
//  1. fieldService.GetByIDsLite(snapshotFieldIDs) 补全字段元数据
//  2. templateService.GetByIDsLite([templateID]) 补全 TemplateLabel
//  3. 逐字段组装 []NPCDetailField
type NPCDetail struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	Label         string          `json:"label"`
	Description   string          `json:"description"`
	TemplateID    int64           `json:"template_id"`
	TemplateName  string          `json:"template_name"`
	TemplateLabel string          `json:"template_label"` // 跨模块补全
	Enabled       bool            `json:"enabled"`
	Version       int             `json:"version"`
	Fields        []NPCDetailField `json:"fields"`
	FsmRef        string          `json:"fsm_ref"`
	BtRefs        map[string]string `json:"bt_refs"` // 反序列化为 map 方便前端使用
}

// NPCDetailField 详情中的字段项（字段元数据 + 快照 required + 快照 value）
type NPCDetailField struct {
	FieldID       int64           `json:"field_id"`
	Name          string          `json:"name"`
	Label         string          `json:"label"`          // 来自 FieldLite（当前状态）
	Type          string          `json:"type"`
	Category      string          `json:"category"`
	CategoryLabel string          `json:"category_label"` // DictCache 翻译
	Enabled       bool            `json:"enabled"`        // 字段当前是否启用（停用时前端标灰 + 警告图标）
	Required      bool            `json:"required"`       // 来自快照
	Value         json.RawMessage `json:"value"`          // 保留原始 JSON 类型
}

// ──────────────────────────────────────────────
// 跨模块精简结构
// ──────────────────────────────────────────────

// NPCLite 给跨模块调用的精简结构（TemplateHandler.GetReferences 使用）
type NPCLite struct {
	ID    int64  `json:"npc_id"    db:"id"`
	Name  string `json:"npc_name"  db:"name"`
	Label string `json:"npc_label" db:"label"`
}

// ──────────────────────────────────────────────
// 导出
// ──────────────────────────────────────────────

// NPCExportItem 导出 API 单条（/api/configs/npc_templates 使用）
type NPCExportItem struct {
	Name   string          `json:"name"`
	Config NPCExportConfig `json:"config"`
}

// NPCExportConfig 导出配置（对齐 api-contract.md §3）
type NPCExportConfig struct {
	TemplateRef string                     `json:"template_ref"`
	Fields      map[string]json.RawMessage `json:"fields"`
	Behavior    NPCExportBehavior          `json:"behavior"`
}

// NPCExportBehavior 行为配置（omitempty：空串/空 map 时省略对应键）
type NPCExportBehavior struct {
	FsmRef string            `json:"fsm_ref,omitempty"`
	BtRefs map[string]string `json:"bt_refs,omitempty"`
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// NPCListQuery 列表查询参数
type NPCListQuery struct {
	Name         string `json:"name"`              // NPC 标识，模糊匹配
	Label        string `json:"label"`             // 中文标签，模糊匹配
	TemplateName string `json:"template_name"`     // 模板标识，精确匹配
	Enabled      *bool  `json:"enabled,omitempty"` // nil=不筛选，true=仅启用，false=仅停用
	Page         int    `json:"page"`
	PageSize     int    `json:"page_size"`
}

// CreateNPCRequest 创建 NPC 请求
type CreateNPCRequest struct {
	Name        string            `json:"name"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	TemplateID  int64             `json:"template_id"`
	FieldValues []NPCFieldValue   `json:"field_values"` // 按模板字段顺序传入
	FsmRef      string            `json:"fsm_ref"`       // 空串=无行为配置
	BtRefs      map[string]string `json:"bt_refs"`       // 空 map=无

	// Handler 层在校验完成后填入（不参与 JSON 反序列化）
	TemplateName   string         `json:"-"` // 来自 templateService.GetByID
	FieldsSnapshot []NPCFieldEntry `json:"-"` // 按模板字段顺序组装的快照
}

// NPCFieldValue 字段值（创建/编辑请求中的单个字段）
type NPCFieldValue struct {
	FieldID int64           `json:"field_id"`
	Value   json.RawMessage `json:"value"` // 原始 JSON 值，保留类型
}

// UpdateNPCRequest 编辑 NPC 请求（template_id 不可修改）
type UpdateNPCRequest struct {
	ID          int64             `json:"id"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	FieldValues []NPCFieldValue   `json:"field_values"` // 全量传入（按快照字段顺序）
	FsmRef      string            `json:"fsm_ref"`
	BtRefs      map[string]string `json:"bt_refs"`
	Version     int               `json:"version"`

	// Handler 层在校验完成后填入（不参与 JSON 反序列化）
	FieldsSnapshot []NPCFieldEntry `json:"-"` // 重新组装的字段快照
}

// CreateNPCResponse 创建 NPC 响应
type CreateNPCResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
