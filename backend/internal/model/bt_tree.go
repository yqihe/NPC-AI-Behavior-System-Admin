package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// BtTree DB 行结构体
type BtTree struct {
	ID          int64           `json:"id"           db:"id"`
	Name        string          `json:"name"         db:"name"`
	DisplayName string          `json:"display_name" db:"display_name"`
	Description string          `json:"description"  db:"description"`
	Config      json.RawMessage `json:"config"       db:"config"`
	Enabled     bool            `json:"enabled"      db:"enabled"`
	Version     int             `json:"version"      db:"version"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
	Deleted     bool            `json:"-"            db:"deleted"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// BtTreeListItem 列表展示项（不含 config，减少传输量）
type BtTreeListItem struct {
	ID          int64     `json:"id"           db:"id"`
	Name        string    `json:"name"         db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Enabled     bool      `json:"enabled"      db:"enabled"`
	CreatedAt   time.Time `json:"created_at"   db:"created_at"`
}

// BtTreeListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type BtTreeListData struct {
	Items    []BtTreeListItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *BtTreeListData) ToListData() *ListData {
	return &ListData{Items: d.Items, Total: d.Total, Page: d.Page, PageSize: d.PageSize}
}

// ──────────────────────────────────────────────
// 详情响应
// ──────────────────────────────────────────────

// BtTreeDetail 详情接口响应（含 config + version）
type BtTreeDetail struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Config      json.RawMessage `json:"config"`
	Enabled     bool            `json:"enabled"`
	Version     int             `json:"version"`
}

// ──────────────────────────────────────────────
// 导出
// ──────────────────────────────────────────────

// BtTreeExportItem 导出 API 单条（仅 name + config，供 /api/configs/bt_trees 使用）
type BtTreeExportItem struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// BtTreeListQuery 列表查询参数
type BtTreeListQuery struct {
	Name        string `json:"name"`         // 前缀匹配
	DisplayName string `json:"display_name"` // 模糊匹配
	Enabled     *bool  `json:"enabled"`      // nil=全部
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
}

// CreateBtTreeResponse 创建行为树响应
type CreateBtTreeResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CreateBtTreeRequest 创建行为树请求
type CreateBtTreeRequest struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Config      json.RawMessage `json:"config"`
}

// UpdateBtTreeRequest 更新行为树请求（含 ID + Version，name 不可修改）
type UpdateBtTreeRequest struct {
	ID          int64           `json:"id"`
	Version     int             `json:"version"`
	DisplayName string          `json:"display_name"`
	Description string          `json:"description"`
	Config      json.RawMessage `json:"config"`
}
