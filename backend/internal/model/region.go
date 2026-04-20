package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// 业务子结构（spawn_table JSON 解析用）
// ──────────────────────────────────────────────

// SpawnEntry spawn_table 单条配置。service 层 validateSpawnTable 解码 + 校验用。
//
// 对齐 Server internal/runtime/zone/zone.go 的 SpawnEntry 结构。
// RespawnSeconds 本期 Server 不消费（v3 roadmap 占位），前端 help-text 已标注。
type SpawnEntry struct {
	TemplateRef    string       `json:"template_ref"`
	Count          int          `json:"count"`
	SpawnPoints    []SpawnPoint `json:"spawn_points"`
	WanderRadius   float64      `json:"wander_radius"`
	RespawnSeconds float64      `json:"respawn_seconds"`
}

// SpawnPoint 2D 坐标。对齐 Server zone.go Position{X,Z float64}，**不含 y 维度**。
type SpawnPoint struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// Region DB 行结构体
type Region struct {
	ID          int64           `json:"id"           db:"id"`
	RegionID    string          `json:"region_id"    db:"region_id"`
	DisplayName string          `json:"display_name" db:"display_name"`
	RegionType  string          `json:"region_type"  db:"region_type"`
	SpawnTable  json.RawMessage `json:"spawn_table"  db:"spawn_table"`
	Enabled     bool            `json:"enabled"      db:"enabled"`
	Version     int             `json:"version"      db:"version"`
	CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
	Deleted     bool            `json:"-"            db:"deleted"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// RegionListItem 列表展示项（不含 spawn_table，减少传输量）
type RegionListItem struct {
	ID          int64     `json:"id"           db:"id"`
	RegionID    string    `json:"region_id"    db:"region_id"`
	DisplayName string    `json:"display_name" db:"display_name"`
	RegionType  string    `json:"region_type"  db:"region_type"`
	Enabled     bool      `json:"enabled"      db:"enabled"`
	CreatedAt   time.Time `json:"created_at"   db:"created_at"`
}

// RegionListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type RegionListData struct {
	Items    []RegionListItem `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *RegionListData) ToListData() *ListData {
	return &ListData{Items: d.Items, Total: d.Total, Page: d.Page, PageSize: d.PageSize}
}

// ──────────────────────────────────────────────
// 详情响应
// ──────────────────────────────────────────────

// RegionDetail 详情接口响应（含 spawn_table + version）
type RegionDetail struct {
	ID          int64           `json:"id"`
	RegionID    string          `json:"region_id"`
	DisplayName string          `json:"display_name"`
	RegionType  string          `json:"region_type"`
	SpawnTable  json.RawMessage `json:"spawn_table"`
	Enabled     bool            `json:"enabled"`
	Version     int             `json:"version"`
}

// ──────────────────────────────────────────────
// 导出
// ──────────────────────────────────────────────

// RegionExportConfig 导出 API config 段；enabled/version/id 剥离。
//
// Name 字段对应 Server Zone.Name（显示名），承载 ADMIN 侧的 display_name 值。
// 外层 envelope.Name 则承载 region_id（业务键，HTTPSource 路由用），两者分层不冲突。
type RegionExportConfig struct {
	RegionID   string          `json:"region_id"`
	Name       string          `json:"name"`
	RegionType string          `json:"region_type"`
	SpawnTable json.RawMessage `json:"spawn_table"`
}

// RegionExportItem 导出 API 单条（{name, config} envelope），供 /api/configs/regions 使用
type RegionExportItem struct {
	Name   string             `json:"name"`
	Config RegionExportConfig `json:"config"`
}

// ExportRefTypeNpcTemplate regions 导出悬空引用的 ref_type 枚举值
// Reason 字段复用 ExportRefReasonMissingOrDisabled（npc 域定义），跨文件共享避免重复常量。
const ExportRefTypeNpcTemplate = "npc_template_ref"

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// RegionListQuery 列表查询参数
type RegionListQuery struct {
	RegionID    string `json:"region_id"`    // 业务键模糊搜索
	DisplayName string `json:"display_name"` // 中文名模糊搜索
	RegionType  string `json:"region_type"`  // 精确筛选（空串=全部）
	Enabled     *bool  `json:"enabled"`      // nil=全部
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
}

// CreateRegionResponse 创建区域响应
type CreateRegionResponse struct {
	ID       int64  `json:"id"`
	RegionID string `json:"region_id"`
}

// CreateRegionRequest 创建区域请求
type CreateRegionRequest struct {
	RegionID    string          `json:"region_id"`
	DisplayName string          `json:"display_name"`
	RegionType  string          `json:"region_type"`
	SpawnTable  json.RawMessage `json:"spawn_table"`
}

// UpdateRegionRequest 更新区域请求（含 ID + Version，region_id 不可修改）
type UpdateRegionRequest struct {
	ID          int64           `json:"id"`
	Version     int             `json:"version"`
	DisplayName string          `json:"display_name"`
	RegionType  string          `json:"region_type"`
	SpawnTable  json.RawMessage `json:"spawn_table"`
}
