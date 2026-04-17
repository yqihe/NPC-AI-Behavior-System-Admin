package model

import "time"

// ──────────────────────────────────────────────
// DB 结构体
// ──────────────────────────────────────────────

// FsmStateDict 状态字典条目（fsm_state_dicts 表整行）
type FsmStateDict struct {
	ID          int64     `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Category    string    `json:"category" db:"category"`
	Description string    `json:"description" db:"description"`
	Enabled     bool      `json:"enabled" db:"enabled"`
	Version     int       `json:"version" db:"version"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	Deleted     bool      `json:"-" db:"deleted"`
}

// ──────────────────────────────────────────────
// 列表展示
// ──────────────────────────────────────────────

// FsmStateDictListItem 列表页展示项
type FsmStateDictListItem struct {
	ID            int64     `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	DisplayName   string    `json:"display_name" db:"display_name"`
	Category      string    `json:"category" db:"category"`
	CategoryLabel string    `json:"category_label" db:"-"` // service 层翻译
	Enabled       bool      `json:"enabled" db:"enabled"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// FsmStateDictListData 列表缓存数据（类型安全，避免 any 反序列化丢类型）
type FsmStateDictListData struct {
	Items    []FsmStateDictListItem `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// ToListData 转换为通用 ListData（HTTP 响应用）
func (d *FsmStateDictListData) ToListData() *ListData {
	return &ListData{
		Items:    d.Items,
		Total:    d.Total,
		Page:     d.Page,
		PageSize: d.PageSize,
	}
}

// ──────────────────────────────────────────────
// 请求结构
// ──────────────────────────────────────────────

// FsmStateDictListQuery 列表查询参数
type FsmStateDictListQuery struct {
	Name        string `json:"name"`              // 英文标识模糊搜索
	DisplayName string `json:"display_name"`      // 中文标签模糊搜索
	Category    string `json:"category"`          // 分类精确过滤
	Enabled     *bool  `json:"enabled,omitempty"` // nil=不筛选，true=仅启用，false=仅停用
	Page        int    `json:"page"`
	PageSize    int    `json:"page_size"`
}

// CreateFsmStateDictRequest 创建状态字典条目请求
type CreateFsmStateDictRequest struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// CreateFsmStateDictResponse 创建响应
type CreateFsmStateDictResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// UpdateFsmStateDictRequest 编辑请求（name 创建后不可变）
type UpdateFsmStateDictRequest struct {
	ID          int64  `json:"id"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

// ──────────────────────────────────────────────
// 删除结果（富错误响应）
// ──────────────────────────────────────────────

// FsmConfigRef 被引用的 FSM 配置条目（供 DeleteResult 使用）
type FsmConfigRef struct {
	ID          int64  `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	DisplayName string `json:"display_name" db:"display_name"`
	Enabled     bool   `json:"enabled" db:"enabled"`
}

// FsmStateDictReferenceDetail 状态字典引用详情（被哪些 FSM 配置引用）
type FsmStateDictReferenceDetail struct {
	StateDictID    int64          `json:"state_dict_id"`
	StateDictLabel string         `json:"state_dict_label"`
	FsmConfigs     []FsmConfigRef `json:"fsm_configs"`
}

// FsmStateDictDeleteResult 删除结果
//
// 成功时：ID/Name/DisplayName 有值，ReferencedBy 为空。
// 被引用时：ReferencedBy 有值（最多 20 条），WrapCtx 将 resp 作为 data 携带在错误响应中。
type FsmStateDictDeleteResult struct {
	ID           int64          `json:"id,omitempty"`
	Name         string         `json:"name,omitempty"`
	DisplayName  string         `json:"display_name,omitempty"`
	ReferencedBy []FsmConfigRef `json:"referenced_by,omitempty"`
}
