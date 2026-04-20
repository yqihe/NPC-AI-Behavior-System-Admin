package model

import "time"

// RuntimeBbKey 运行时 BB Key 定义
//
// 第三类 BB Key 来源（字段 expose_bb / 事件扩展字段 / 运行时注册表三路之一），
// 对齐游戏服务端 internal/core/blackboard/keys.go 的 31 个静态声明。
// FSM 条件 / BT check_bb_* 节点可引用本表的 key，引用关系记录在 runtime_bb_key_refs。
type RuntimeBbKey struct {
	ID          int64     `json:"id"          db:"id"`
	Name        string    `json:"name"        db:"name"`        // ^[a-z][a-z0-9_]*$，对齐 keys.go NewKey 第一参数
	Type        string    `json:"type"        db:"type"`        // integer / float / string / bool 四枚举
	Label       string    `json:"label"       db:"label"`       // 中文标签（UI 下拉展示）
	Description string    `json:"description" db:"description"` // 中文描述（UI tooltip）
	GroupName   string    `json:"group_name"  db:"group_name"`  // 分组：threat/event/fsm/npc/action/need/emotion/memory/social/decision/move

	Enabled   bool      `json:"enabled"    db:"enabled"`
	Version   int       `json:"version"    db:"version"`
	Deleted   bool      `json:"-"          db:"deleted"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// 非 DB 列，service 层 detail 路径填充
	HasRefs  bool `json:"has_refs"  db:"-"`
	RefCount int  `json:"ref_count" db:"-"`
}

// RuntimeBbKeyRef 运行时 BB Key 引用关系（FSM/BT 配置 → 运行时 key）
type RuntimeBbKeyRef struct {
	RuntimeKeyID int64     `json:"runtime_key_id" db:"runtime_key_id"`
	RefType      string    `json:"ref_type"       db:"ref_type"` // fsm | bt
	RefID        int64     `json:"ref_id"         db:"ref_id"`
	CreatedAt    time.Time `json:"created_at"     db:"created_at"`
}

// RuntimeBbKeyListItem 列表页展示项（覆盖索引返回，不含 description/updated_at）
type RuntimeBbKeyListItem struct {
	ID        int64     `json:"id"         db:"id"`
	Name      string    `json:"name"       db:"name"`
	Type      string    `json:"type"       db:"type"`
	Label     string    `json:"label"      db:"label"`
	GroupName string    `json:"group_name" db:"group_name"`
	Enabled   bool      `json:"enabled"    db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// RuntimeBbKeyListQuery 列表查询参数
type RuntimeBbKeyListQuery struct {
	Name      string `json:"name"`       // 英文标识模糊搜索
	Label     string `json:"label"`      // 中文标签模糊搜索
	Type      string `json:"type"`       // 精确筛选 integer/float/string/bool
	GroupName string `json:"group_name"` // 精确筛选组名
	Enabled   *bool  `json:"enabled,omitempty"` // nil=不筛选；true=仅启用（BBKeySelector 调用时用）
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
}

// CreateRuntimeBbKeyRequest 创建请求（无 ID，无 version）
type CreateRuntimeBbKeyRequest struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description"`
	GroupName   string `json:"group_name"`
}

// UpdateRuntimeBbKeyRequest 编辑请求（有 ID + version；name 不可变，对齐 bt_trees/fsm_configs）
type UpdateRuntimeBbKeyRequest struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Description string `json:"description"`
	GroupName   string `json:"group_name"`
	Version     int    `json:"version"`
}

// RuntimeBbKeyListData 列表数据（类型安全，缓存序列化/反序列化用）
type RuntimeBbKeyListData struct {
	Items    []RuntimeBbKeyListItem `json:"items"`
	Total    int64                  `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// ToListData 转为 handler 层通用响应结构（沿用 field 模块 pattern）
func (d *RuntimeBbKeyListData) ToListData() *ListData {
	return &ListData{
		Items:    d.Items,
		Total:    d.Total,
		Page:     d.Page,
		PageSize: d.PageSize,
	}
}

// RuntimeBbKeyReferenceDetail 引用详情响应（GET /:id/references）
type RuntimeBbKeyReferenceDetail struct {
	KeyID    int64           `json:"key_id"`
	KeyName  string          `json:"key_name"`
	KeyLabel string          `json:"key_label"`
	Fsms     []ReferenceItem `json:"fsms"`
	Bts      []ReferenceItem `json:"bts"`
}
