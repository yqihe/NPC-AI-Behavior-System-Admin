package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// BtNodeType DB 行结构体
type BtNodeType struct {
	ID          int64           `json:"id"           db:"id"`
	TypeName    string          `json:"type_name"    db:"type_name"`
	Category    string          `json:"category"     db:"category"`
	Label       string          `json:"label"        db:"label"`
	Description string          `json:"description"  db:"description"`
	ParamSchema json.RawMessage `json:"param_schema" db:"param_schema"`
	IsBuiltin   bool            `json:"is_builtin"   db:"is_builtin"`
	Enabled     bool            `json:"enabled"      db:"enabled"`
	Version     int             `json:"version"      db:"version"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
	Deleted     bool            `json:"-"            db:"deleted"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// BtNodeTypeListItem 列表展示项（不含 param_schema）
type BtNodeTypeListItem struct {
	ID        int64  `json:"id"         db:"id"`
	TypeName  string `json:"type_name"  db:"type_name"`
	Category  string `json:"category"   db:"category"`
	Label     string `json:"label"      db:"label"`
	IsBuiltin bool   `json:"is_builtin" db:"is_builtin"`
	Enabled   bool   `json:"enabled"    db:"enabled"`
}

// BtNodeTypeListData 列表分页数据（含 ToListData 方法）
type BtNodeTypeListData struct {
	Items    []BtNodeTypeListItem `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *BtNodeTypeListData) ToListData() *ListData {
	return &ListData{Items: d.Items, Total: d.Total, Page: d.Page, PageSize: d.PageSize}
}

// ──────────────────────────────────────────────
// 详情响应
// ──────────────────────────────────────────────

// BtNodeTypeDetail 详情（含 param_schema + version）
type BtNodeTypeDetail struct {
	ID          int64           `json:"id"`
	TypeName    string          `json:"type_name"`
	Category    string          `json:"category"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	ParamSchema json.RawMessage `json:"param_schema"`
	IsBuiltin   bool            `json:"is_builtin"`
	Enabled     bool            `json:"enabled"`
	Version     int             `json:"version"`
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// CreateBtNodeTypeRequest 创建请求
type CreateBtNodeTypeRequest struct {
	TypeName    string          `json:"type_name"`
	Category    string          `json:"category"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	ParamSchema json.RawMessage `json:"param_schema"`
}

// UpdateBtNodeTypeRequest 更新请求（含 ID + Version）
type UpdateBtNodeTypeRequest struct {
	ID          int64           `json:"id"`
	Version     int             `json:"version"`
	Label       string          `json:"label"`
	Description string          `json:"description"`
	ParamSchema json.RawMessage `json:"param_schema"`
}

// BtNodeTypeListQuery 列表查询参数
type BtNodeTypeListQuery struct {
	TypeName string `json:"type_name"` // 前缀匹配
	Category string `json:"category"`  // 精确匹配
	Enabled  *bool  `json:"enabled"`   // nil=全部
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}
