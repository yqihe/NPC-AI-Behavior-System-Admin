package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// 感知模式枚举常量
// ──────────────────────────────────────────────

const (
	PerceptionModeVisual   = "visual"
	PerceptionModeAuditory = "auditory"
	PerceptionModeGlobal   = "global"
)

// ValidPerceptionModes 合法枚举集合（handler 校验用）
var ValidPerceptionModes = map[string]bool{
	PerceptionModeVisual:   true,
	PerceptionModeAuditory: true,
	PerceptionModeGlobal:   true,
}

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// EventType 事件类型定义（event_types 表整行）
type EventType struct {
	ID             int64           `json:"id" db:"id"`
	Name           string          `json:"name" db:"name"`
	DisplayName    string          `json:"display_name" db:"display_name"`
	PerceptionMode string          `json:"perception_mode" db:"perception_mode"`
	ConfigJSON     json.RawMessage `json:"config_json" db:"config_json"`

	Enabled   bool      `json:"enabled" db:"enabled"`
	Version   int       `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Deleted   bool      `json:"-" db:"deleted"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// EventTypeListItem 列表页展示项
//
// 核心列从 DB 读取，severity/ttl/range 从 config_json unmarshal 后由 service 层填充。
type EventTypeListItem struct {
	ID             int64     `json:"id" db:"id"`
	Name           string    `json:"name" db:"name"`
	DisplayName    string    `json:"display_name" db:"display_name"`
	PerceptionMode string    `json:"perception_mode" db:"perception_mode"`
	Enabled        bool      `json:"enabled" db:"enabled"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`

	// 以下字段从 config_json unmarshal 后由 service 层填充
	DefaultSeverity float64 `json:"default_severity" db:"-"`
	DefaultTTL      float64 `json:"default_ttl" db:"-"`
	Range           float64 `json:"range" db:"-"`
}

// EventTypeListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type EventTypeListData struct {
	Items    []EventTypeListItem `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *EventTypeListData) ToListData() *ListData {
	return &ListData{
		Items:    d.Items,
		Total:    d.Total,
		Page:     d.Page,
		PageSize: d.PageSize,
	}
}

// ──────────────────────────────────────────────
// 详情响应
// ──────────────────────────────────────────────

// EventTypeDetail 详情接口响应
//
// handler 层组装：EventTypeService.GetByID 拿 DB 行 +
// EventTypeSchemaService.ListEnabled 拿扩展字段定义 + unmarshal config_json。
type EventTypeDetail struct {
	ID             int64                  `json:"id"`
	Name           string                 `json:"name"`
	DisplayName    string                 `json:"display_name"`
	Enabled        bool                   `json:"enabled"`
	Version        int                    `json:"version"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Config         map[string]interface{} `json:"config"`            // config_json 展开
	ExtensionSchema []EventTypeSchemaLite `json:"extension_schema"` // 当前启用的扩展字段定义
}

// ──────────────────────────────────────────────
// 导出
// ──────────────────────────────────────────────

// EventTypeExportItem 导出 API 单条
type EventTypeExportItem struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"` // config_json 原样输出
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// EventTypeListQuery 列表查询参数
type EventTypeListQuery struct {
	Label          string `json:"label"`                     // display_name 模糊搜索
	PerceptionMode string `json:"perception_mode,omitempty"` // 精确筛选
	Enabled        *bool  `json:"enabled,omitempty"`         // nil=不筛选，true=仅启用，false=仅停用
	Page           int    `json:"page"`
	PageSize       int    `json:"page_size"`
}

// CreateEventTypeRequest 创建事件类型请求
type CreateEventTypeRequest struct {
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"display_name"`
	PerceptionMode  string                 `json:"perception_mode"`
	DefaultSeverity float64                `json:"default_severity"`
	DefaultTTL      float64                `json:"default_ttl"`
	Range           float64                `json:"range"`
	Extensions      map[string]interface{} `json:"extensions"` // 扩展字段 key→value（可选）
}

// CreateEventTypeResponse 创建响应
type CreateEventTypeResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// UpdateEventTypeRequest 编辑事件类型请求（无 name，name 创建后不可变）
type UpdateEventTypeRequest struct {
	ID              int64                  `json:"id"`
	DisplayName     string                 `json:"display_name"`
	PerceptionMode  string                 `json:"perception_mode"`
	DefaultSeverity float64                `json:"default_severity"`
	DefaultTTL      float64                `json:"default_ttl"`
	Range           float64                `json:"range"`
	Extensions      map[string]interface{} `json:"extensions"`
	Version         int                    `json:"version"`
}
