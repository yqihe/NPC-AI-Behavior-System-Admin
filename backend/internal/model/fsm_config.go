package model

import (
	"encoding/json"
	"time"
)

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// FsmConfig 状态机配置（fsm_configs 表整行）
type FsmConfig struct {
	ID          int64           `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	DisplayName string          `json:"display_name" db:"display_name"`
	ConfigJSON  json.RawMessage `json:"config_json" db:"config_json"`

	Enabled   bool      `json:"enabled" db:"enabled"`
	Version   int       `json:"version" db:"version"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
	Deleted   bool      `json:"-" db:"deleted"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// FsmConfigListItem 列表页展示项
//
// 核心列从 DB 读取，initial_state/state_count 从 config_json unmarshal 后由 service 层填充。
type FsmConfigListItem struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Enabled     bool      `json:"enabled" db:"enabled"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`

	// 以下字段从 config_json unmarshal 后由 service 层填充
	InitialState string `json:"initial_state" db:"-"`
	StateCount   int    `json:"state_count" db:"-"`
}

// FsmConfigListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type FsmConfigListData struct {
	Items    []FsmConfigListItem `json:"items"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *FsmConfigListData) ToListData() *ListData {
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

// FsmConfigDetail 详情接口响应
//
// handler 层组装：FsmConfigService.GetByID 拿 DB 行 + unmarshal config_json。
type FsmConfigDetail struct {
	ID          int64                  `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Enabled     bool                   `json:"enabled"`
	Version     int                    `json:"version"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Config      map[string]interface{} `json:"config"` // config_json 展开
}

// ──────────────────────────────────────────────
// 导出
// ──────────────────────────────────────────────

// FsmConfigExportItem 导出 API 单条
type FsmConfigExportItem struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"` // config_json 原样输出
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// FsmConfigListQuery 列表查询参数
type FsmConfigListQuery struct {
	Label    string `json:"label"`             // display_name 模糊搜索
	Enabled  *bool  `json:"enabled,omitempty"` // nil=不筛选，true=仅启用，false=仅停用
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// CreateFsmConfigRequest 创建状态机配置请求
type CreateFsmConfigRequest struct {
	Name         string          `json:"name"`
	DisplayName  string          `json:"display_name"`
	InitialState string          `json:"initial_state"`
	States       []FsmState      `json:"states"`
	Transitions  []FsmTransition `json:"transitions"`
}

// CreateFsmConfigResponse 创建响应
type CreateFsmConfigResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// UpdateFsmConfigRequest 编辑状态机配置请求（无 name，name 创建后不可变）
type UpdateFsmConfigRequest struct {
	ID           int64           `json:"id"`
	DisplayName  string          `json:"display_name"`
	InitialState string          `json:"initial_state"`
	States       []FsmState      `json:"states"`
	Transitions  []FsmTransition `json:"transitions"`
	Version      int             `json:"version"`
}

// ──────────────────────────────────────────────
// 配置子结构（对齐游戏服务端 fsm.FSMConfig / rule.Condition）
// ──────────────────────────────────────────────

// FsmState 状态定义
type FsmState struct {
	Name string `json:"name"`
}

// FsmTransition 转换规则
type FsmTransition struct {
	From      string       `json:"from"`
	To        string       `json:"to"`
	Priority  int          `json:"priority"`
	Condition FsmCondition `json:"condition"`
}

// FsmCondition 条件树节点
//
// 对齐游戏服务端 rule.Condition：
//   - 叶节点：Key + Op + Value/RefKey
//   - 组合节点：And / Or（可嵌套）
//   - 空条件（所有字段为零值）= 无条件转换，始终 true
type FsmCondition struct {
	// 叶节点字段
	Key    string          `json:"key,omitempty"`
	Op     string          `json:"op,omitempty"`
	Value  json.RawMessage `json:"value,omitempty"`
	RefKey string          `json:"ref_key,omitempty"`

	// 组合节点字段
	And []FsmCondition `json:"and,omitempty"`
	Or  []FsmCondition `json:"or,omitempty"`
}

// IsEmpty 判断条件是否为空（无条件转换，始终为 true）
func (c *FsmCondition) IsEmpty() bool {
	return c.Key == "" && len(c.And) == 0 && len(c.Or) == 0
}
