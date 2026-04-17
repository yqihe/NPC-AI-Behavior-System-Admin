// Package util 提供跨模块共享的常量和工具函数。
//
// 规则：只放真正被 2 个以上模块使用的东西。
// 只有一个调用点的不要往这里塞（red-lines: 禁止过度设计）。
package util

// ──────────────────────────────────────────────
// 感知模式（事件类型系统字段，服务端 struct 直接消费）
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
// 字段类型（字段管理 + 扩展字段共用）
// ──────────────────────────────────────────────

const (
	FieldTypeInteger   = "integer"   // 字段管理用
	FieldTypeFloat     = "float"     // 字段管理 + 扩展字段
	FieldTypeString    = "string"    // 字段管理 + 扩展字段
	FieldTypeBoolean   = "boolean"   // 字段管理用
	FieldTypeSelect    = "select"    // 字段管理 + 扩展字段
	FieldTypeReference = "reference" // 字段管理专用（扩展字段不支持）

	// 扩展字段类型（和字段管理有 int/bool 的命名差异）
	ExtFieldTypeInt    = "int"
	ExtFieldTypeFloat  = "float"
	ExtFieldTypeString = "string"
	ExtFieldTypeBool   = "bool"
	ExtFieldTypeSelect = "select"
)

// ValidExtFieldTypes 扩展字段合法类型（不含 reference）
var ValidExtFieldTypes = map[string]bool{
	ExtFieldTypeInt:    true,
	ExtFieldTypeFloat:  true,
	ExtFieldTypeString: true,
	ExtFieldTypeBool:   true,
	ExtFieldTypeSelect: true,
}

// ──────────────────────────────────────────────
// 引用来源类型
// ──────────────────────────────────────────────

const (
	RefTypeTemplate  = "template"   // 模板引用字段
	RefTypeField     = "field"      // reference 字段引用字段
	RefTypeEventType = "event_type" // 事件类型引用扩展字段（schema_refs）
	RefTypeFsm       = "fsm"        // FSM 条件引用字段 BB Key（field_refs）
	RefTypeBt        = "bt"         // 行为树节点引用字段 BB Key（field_refs）
)

// ──────────────────────────────────────────────
// 字典组名
// ──────────────────────────────────────────────

const (
	DictGroupFieldType        = "field_type"
	DictGroupFieldCategory    = "field_category"
	DictGroupFieldProperties  = "field_properties"
	DictGroupFsmStateCategory = "fsm_state_category"
)
