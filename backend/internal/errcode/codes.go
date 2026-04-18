package errcode

// 错误码统一定义
// 4xxxx: 业务错误（客户端问题）
// 5xxxx: 系统错误（服务端问题）

// --- 通用 ---

const (
	Success       = 0
	ErrBadRequest = 40000 // 通用参数错误
	ErrInternal   = 50000 // 通用内部错误
)

// --- 字段管理 400xx ---

const (
	ErrFieldNameExists       = 40001 // 字段标识已存在
	ErrFieldNameInvalid      = 40002 // 字段标识格式不合法
	ErrFieldTypeNotFound     = 40003 // 字段类型不存在
	ErrFieldCategoryNotFound = 40004 // 标签分类不存在
	ErrFieldRefDelete        = 40005 // 被引用无法删除
	ErrFieldRefChangeType    = 40006 // 被引用无法修改类型
	ErrFieldRefTighten       = 40007 // 被引用无法收紧约束
	ErrFieldBBKeyInUse       = 40008 // BB Key 被 FSM/BT 引用无法关闭
	ErrFieldCyclicRef        = 40009 // 循环引用
	ErrFieldVersionConflict  = 40010 // 版本冲突（乐观锁）
	ErrFieldNotFound         = 40011 // 字段不存在
	ErrFieldRefNotFound      = 40014 // 引用的字段不存在
	ErrFieldDeleteNotDisabled = 40012 // 删除前必须先停用
	ErrFieldRefDisabled       = 40013 // 不能引用已停用的字段
	ErrFieldEditNotDisabled   = 40015 // 编辑前必须先停用
	ErrFieldRefNested         = 40016 // reference 字段禁止嵌套引用
	ErrFieldRefEmpty          = 40017 // reference 字段 refs 不能为空
)

// --- 模板管理 410xx ---

const (
	ErrTemplateNameExists        = 41001 // 模板标识已存在（含软删除）
	ErrTemplateNameInvalid       = 41002 // 模板标识格式不合法
	ErrTemplateNotFound          = 41003 // 模板不存在
	ErrTemplateNoFields          = 41004 // 未勾选任何字段
	ErrTemplateFieldDisabled     = 41005 // 勾选了停用字段
	ErrTemplateFieldNotFound     = 41006 // 勾选的字段不存在
	ErrTemplateRefDelete         = 41007 // 被 NPC 引用，无法删除
	ErrTemplateRefEditFields     = 41008 // 被 NPC 引用，无法编辑字段列表（含顺序/必填）
	ErrTemplateDeleteNotDisabled = 41009 // 删除前必须先停用
	ErrTemplateEditNotDisabled   = 41010 // 编辑前必须先停用
	ErrTemplateVersionConflict   = 41011 // 版本冲突（乐观锁）
	ErrTemplateFieldIsReference  = 41012 // 模板不能直接挂载 reference 类型字段
)

// --- 事件类型管理 420xx ---

const (
	ErrEventTypeNameExists       = 42001 // 事件标识已存在（含软删除）
	ErrEventTypeNameInvalid      = 42002 // 事件标识格式不合法
	ErrEventTypeModeInvalid      = 42003 // 感知模式枚举非法
	ErrEventTypeSeverityInvalid  = 42004 // 威胁值不在 0-100
	ErrEventTypeTTLInvalid       = 42005 // TTL <= 0
	ErrEventTypeRangeInvalid     = 42006 // 传播范围 < 0
	ErrEventTypeExtValueInvalid  = 42007 // 扩展字段值不符合 schema 约束
	ErrEventTypeRefDelete        = 42008 // 被引用无法删除（占位，本期 ref_count 恒 0）
	ErrEventTypeVersionConflict  = 42010 // 版本冲突（乐观锁）
	ErrEventTypeNotFound         = 42011 // 事件类型不存在
	ErrEventTypeDeleteNotDisabled = 42012 // 删除前必须先停用
	ErrEventTypeEditNotDisabled   = 42015 // 编辑前必须先停用
)

// --- 扩展字段 Schema 420[20-39] ---

const (
	ErrExtSchemaNameExists         = 42020 // field_name 已存在（含软删除）
	ErrExtSchemaNameInvalid        = 42021 // field_name 格式不合法
	ErrExtSchemaNotFound           = 42022 // 扩展字段定义不存在
	ErrExtSchemaDisabled           = 42023 // 扩展字段已停用，不能被引用
	ErrExtSchemaTypeInvalid        = 42024 // field_type 枚举非法
	ErrExtSchemaConstraintsInvalid = 42025 // constraints 不自洽
	ErrExtSchemaDefaultInvalid     = 42026 // default_value 不符合 constraints
	ErrExtSchemaDeleteNotDisabled  = 42027 // 删除前必须先停用
	ErrExtSchemaRefTighten         = 42028 // 被引用时约束收紧
	ErrExtSchemaRefDelete          = 42029 // 被引用时无法删除
	ErrExtSchemaVersionConflict    = 42030 // 版本冲突（乐观锁）
	ErrExtSchemaEditNotDisabled    = 42031 // 编辑前必须先停用
)

// --- 状态机管理 430xx ---

const (
	ErrFsmConfigNameExists        = 43001 // FSM 标识已存在（含软删除）
	ErrFsmConfigNameInvalid       = 43002 // FSM 标识格式不合法
	ErrFsmConfigNotFound          = 43003 // FSM 配置不存在
	ErrFsmConfigStatesEmpty       = 43004 // 未定义任何状态
	ErrFsmConfigStateNameInvalid  = 43005 // 状态名为空或重复
	ErrFsmConfigInitialInvalid    = 43006 // 初始状态不在状态列表中
	ErrFsmConfigTransitionInvalid = 43007 // 转换规则引用了不存在的状态
	ErrFsmConfigConditionInvalid  = 43008 // 条件表达式不合法
	ErrFsmConfigDeleteNotDisabled = 43009 // 删除前必须先停用
	ErrFsmConfigEditNotDisabled   = 43010 // 编辑前必须先停用
	ErrFsmConfigVersionConflict   = 43011 // 版本冲突（乐观锁）
	ErrFsmConfigRefDelete         = 43012 // 被 NPC 引用，无法删除（占位，本期 ref_count 恒 0）
)

// --- 状态字典管理 430[13-24] ---

const (
	ErrFsmStateDictNameExists        = 43013 // 标识已存在（含软删除）
	ErrFsmStateDictNameInvalid       = 43014 // 标识格式不合法
	ErrFsmStateDictNotFound          = 43015 // 条目不存在
	ErrFsmStateDictDeleteNotDisabled = 43016 // 删除前必须先停用
	ErrFsmStateDictVersionConflict   = 43017 // 版本冲突（乐观锁）
	// 43018-43019 预留
	ErrFsmStateDictInUse = 43020 // 被 FSM 引用，无法删除（携带 referenced_by）
	// 43021-43024 预留
)

// --- 行为树管理 440xx ---

const (
	ErrBtTreeNameExists        = 44001 // 行为树标识已存在（含软删除）
	ErrBtTreeNameInvalid       = 44002 // 行为树标识格式不合法
	ErrBtTreeNotFound          = 44003 // 行为树不存在
	ErrBtTreeConfigInvalid     = 44004 // 树结构不合法
	ErrBtTreeNodeTypeNotFound  = 44005 // 节点类型不存在或已禁用
	ErrBtTreeNodeDepthExceeded = 44006 // 节点嵌套深度超过 20 层
	ErrBtNodeBareFields        = 44007 // 节点字段结构非法（顶层裸字段 / params 缺失或非对象）
	ErrBtNodeParamMissing      = 44008 // 节点缺少必填参数
	ErrBtTreeDeleteNotDisabled = 44009 // 删除前必须先停用
	ErrBtTreeEditNotDisabled   = 44010 // 编辑前必须先停用
	ErrBtTreeVersionConflict   = 44011 // 版本冲突（乐观锁）
	ErrBtTreeRefDelete         = 44012 // 被 NPC 引用，无法删除（占位，NPC 管理完成后激活）
	ErrBtNodeParamType         = 44013 // 节点参数类型不匹配
	ErrBtNodeParamEnum         = 44014 // 节点参数取值不在允许集合
	// 44015 预留
)

// --- 节点类型管理 44016-44025 ---

const (
	ErrBtNodeTypeNameExists          = 44016 // 节点类型标识已存在（含软删除）
	ErrBtNodeTypeNameInvalid         = 44017 // 节点类型标识格式不合法
	ErrBtNodeTypeNotFound            = 44018 // 节点类型不存在
	ErrBtNodeTypeCategoryInvalid     = 44019 // category 枚举非法
	ErrBtNodeTypeDeleteNotDisabled   = 44020 // 删除前必须先停用
	ErrBtNodeTypeEditNotDisabled     = 44021 // 编辑前必须先停用
	ErrBtNodeTypeRefDelete           = 44022 // 被行为树引用，无法删除（携带引用树名列表）
	ErrBtNodeTypeBuiltinDelete       = 44023 // 内置类型不可删除
	ErrBtNodeTypeBuiltinEdit         = 44024 // 内置类型不可编辑
	ErrBtNodeTypeParamSchemaInvalid  = 44025 // param_schema 不合法
	ErrBtNodeTypeVersionConflict     = 44026 // 版本冲突（乐观锁）
)

// --- NPC 管理 450xx ---

const (
	ErrNPCNameExists        = 45001 // NPC 标识已存在（含软删除）
	ErrNPCNameInvalid       = 45002 // NPC 标识格式不合法
	ErrNPCNotFound          = 45003 // NPC 不存在
	ErrNPCTemplateNotFound  = 45004 // 引用的模板不存在
	ErrNPCTemplateDisabled  = 45005 // 引用的模板未启用
	ErrNPCFieldValueInvalid = 45006 // 字段值不符合类型/约束
	ErrNPCFieldRequired     = 45007 // 必填字段未填
	ErrNPCFsmNotFound       = 45008 // 引用的状态机不存在
	ErrNPCFsmDisabled       = 45009 // 引用的状态机未启用
	ErrNPCBtNotFound        = 45010 // 引用的行为树不存在
	ErrNPCBtDisabled        = 45011 // 引用的行为树未启用
	ErrNPCBtStateInvalid    = 45012 // bt_refs 状态名不在 FSM 状态列表中
	ErrNPCDeleteNotDisabled = 45013 // 删除前必须先停用
	ErrNPCVersionConflict   = 45014 // 版本冲突（乐观锁）
	ErrNPCBtWithoutFsm      = 45015 // bt_refs 非空时 fsm_ref 必须设置
	ErrNPCExportDanglingRef = 45016 // 导出 NPC 时发现悬空 FSM/BT 引用
)

// --- 错误消息 ---

var messages = map[int]string{
	Success:       "success",
	ErrBadRequest: "请求参数错误",
	ErrInternal:   "服务器内部错误，请稍后重试",

	ErrFieldNameExists:       "字段标识已存在",
	ErrFieldNameInvalid:      "字段标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrFieldTypeNotFound:     "字段类型不存在",
	ErrFieldCategoryNotFound: "标签分类不存在",
	ErrFieldRefDelete:        "该字段正被引用，无法删除",
	ErrFieldRefChangeType:    "该字段已被引用，无法修改类型",
	ErrFieldRefTighten:       "已有数据可能超出新约束范围，请先移除引用",
	ErrFieldBBKeyInUse:       "该 BB Key 正被 FSM/BT 引用，无法关闭暴露",
	ErrFieldCyclicRef:        "检测到循环引用",
	ErrFieldVersionConflict:  "该字段已被其他人修改，请刷新后重试",
	ErrFieldNotFound:         "字段不存在",
	ErrFieldRefNotFound:      "引用的字段不存在",
	ErrFieldDeleteNotDisabled: "请先停用该字段再删除",
	ErrFieldRefDisabled:       "不能引用已停用的字段",
	ErrFieldEditNotDisabled:   "请先停用该字段再编辑",
	ErrFieldRefNested:         "不能引用 reference 类型字段，禁止嵌套",
	ErrFieldRefEmpty:          "reference 字段必须至少引用一个目标字段",

	ErrTemplateNameExists:        "模板标识已存在",
	ErrTemplateNameInvalid:       "模板标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrTemplateNotFound:          "模板不存在",
	ErrTemplateNoFields:          "请至少勾选一个字段",
	ErrTemplateFieldDisabled:     "勾选的字段已停用，请先在字段管理中启用",
	ErrTemplateFieldNotFound:     "勾选的字段不存在",
	ErrTemplateRefDelete:         "该模板正被 NPC 引用，无法删除",
	ErrTemplateRefEditFields:     "该模板已被 NPC 引用，字段勾选与必填配置不可修改",
	ErrTemplateDeleteNotDisabled: "请先停用该模板再删除",
	ErrTemplateEditNotDisabled:   "请先停用该模板再编辑",
	ErrTemplateVersionConflict:   "该模板已被其他人修改，请刷新后重试",
	ErrTemplateFieldIsReference:  "reference 类型字段不能直接加入模板，请点击 reference 字段选择其子字段",

	ErrEventTypeNameExists:       "事件标识已存在",
	ErrEventTypeNameInvalid:      "事件标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrEventTypeModeInvalid:      "感知模式必须是 visual / auditory / global 之一",
	ErrEventTypeSeverityInvalid:  "默认威胁必须在 0-100 之间",
	ErrEventTypeTTLInvalid:       "默认 TTL 必须大于 0",
	ErrEventTypeRangeInvalid:     "传播范围不能小于 0",
	ErrEventTypeExtValueInvalid:  "扩展字段的值不符合约束",
	ErrEventTypeRefDelete:        "当前事件类型仍被引用，不能删除",
	ErrEventTypeVersionConflict:  "该事件类型已被其他人修改，请刷新后重试",
	ErrEventTypeNotFound:         "事件类型不存在",
	ErrEventTypeDeleteNotDisabled: "请先停用该事件类型再删除",
	ErrEventTypeEditNotDisabled:   "请先停用该事件类型再编辑",

	ErrExtSchemaNameExists:         "扩展字段标识已存在",
	ErrExtSchemaNameInvalid:        "扩展字段标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrExtSchemaNotFound:           "扩展字段定义不存在",
	ErrExtSchemaDisabled:           "扩展字段已停用",
	ErrExtSchemaTypeInvalid:        "扩展字段类型必须是 int / float / string / bool / select 之一",
	ErrExtSchemaConstraintsInvalid: "约束配置不自洽",
	ErrExtSchemaDefaultInvalid:     "默认值不符合约束",
	ErrExtSchemaDeleteNotDisabled:  "请先停用该扩展字段再删除",
	ErrExtSchemaRefTighten:         "该扩展字段已被事件类型引用，约束只能放宽不能收紧",
	ErrExtSchemaRefDelete:          "该扩展字段正被事件类型引用，无法删除",
	ErrExtSchemaVersionConflict:    "该扩展字段已被其他人修改，请刷新后重试",
	ErrExtSchemaEditNotDisabled:    "请先停用该扩展字段再编辑",

	ErrFsmConfigNameExists:        "状态机标识已存在",
	ErrFsmConfigNameInvalid:       "状态机标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrFsmConfigNotFound:          "状态机配置不存在",
	ErrFsmConfigStatesEmpty:       "请至少定义一个状态",
	ErrFsmConfigStateNameInvalid:  "状态名不能为空且不能重复",
	ErrFsmConfigInitialInvalid:    "初始状态必须是已定义的状态之一",
	ErrFsmConfigTransitionInvalid: "转换规则引用了不存在的状态",
	ErrFsmConfigConditionInvalid:  "条件表达式不合法",
	ErrFsmConfigDeleteNotDisabled: "请先停用该状态机再删除",
	ErrFsmConfigEditNotDisabled:   "请先停用该状态机再编辑",
	ErrFsmConfigVersionConflict:   "该状态机已被其他人修改，请刷新后重试",
	ErrFsmConfigRefDelete:         "当前状态机仍被引用，不能删除",

	ErrFsmStateDictNameExists:        "状态标识已存在",
	ErrFsmStateDictNameInvalid:       "状态标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrFsmStateDictNotFound:          "状态字典条目不存在",
	ErrFsmStateDictDeleteNotDisabled: "请先停用该状态条目再删除",
	ErrFsmStateDictVersionConflict:   "该状态条目已被其他人修改，请刷新后重试",
	ErrFsmStateDictInUse:             "状态字典条目被 FSM 引用，无法删除",

	ErrBtTreeNameExists:        "行为树标识已存在",
	ErrBtTreeNameInvalid:       "行为树标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线、斜杠",
	ErrBtTreeNotFound:          "行为树不存在",
	ErrBtTreeConfigInvalid:     "行为树结构不合法",
	ErrBtTreeNodeTypeNotFound:  "节点类型不存在或已禁用",
	ErrBtTreeNodeDepthExceeded: "节点嵌套深度超过 20 层",
	ErrBtNodeBareFields:        "节点字段结构非法",
	ErrBtNodeParamMissing:      "节点缺少必填参数",
	ErrBtTreeDeleteNotDisabled: "请先停用该行为树再删除",
	ErrBtTreeEditNotDisabled:   "请先停用该行为树再编辑",
	ErrBtTreeVersionConflict:   "该行为树已被其他人修改，请刷新后重试",
	ErrBtTreeRefDelete:         "当前行为树仍被 NPC 引用，不能删除",
	ErrBtNodeParamType:         "节点参数类型不匹配",
	ErrBtNodeParamEnum:         "节点参数取值不在允许集合",

	ErrBtNodeTypeNameExists:         "节点类型标识已存在",
	ErrBtNodeTypeNameInvalid:        "节点类型标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrBtNodeTypeNotFound:           "节点类型不存在",
	ErrBtNodeTypeCategoryInvalid:    "节点分类必须是 composite / decorator / leaf 之一",
	ErrBtNodeTypeDeleteNotDisabled:  "请先停用该节点类型再删除",
	ErrBtNodeTypeEditNotDisabled:    "请先停用该节点类型再编辑",
	ErrBtNodeTypeRefDelete:          "该节点类型正被行为树引用，无法删除",
	ErrBtNodeTypeBuiltinDelete:      "内置节点类型不可删除",
	ErrBtNodeTypeBuiltinEdit:        "内置节点类型不可编辑",
	ErrBtNodeTypeParamSchemaInvalid: "param_schema 格式不合法",
	ErrBtNodeTypeVersionConflict:    "该节点类型已被其他人修改，请刷新后重试",

	ErrNPCNameExists:        "NPC 标识已存在",
	ErrNPCNameInvalid:       "NPC 标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线",
	ErrNPCNotFound:          "NPC 不存在",
	ErrNPCTemplateNotFound:  "引用的模板不存在",
	ErrNPCTemplateDisabled:  "引用的模板未启用，请先在模板管理中启用",
	ErrNPCFieldValueInvalid: "字段值不符合约束",
	ErrNPCFieldRequired:     "必填字段未填写",
	ErrNPCFsmNotFound:       "引用的状态机不存在",
	ErrNPCFsmDisabled:       "引用的状态机未启用，请先在状态机管理中启用",
	ErrNPCBtNotFound:        "引用的行为树不存在",
	ErrNPCBtDisabled:        "引用的行为树未启用，请先在行为树管理中启用",
	ErrNPCBtStateInvalid:    "行为树绑定的状态名与所选状态机不匹配",
	ErrNPCDeleteNotDisabled: "请先停用该 NPC 再删除",
	ErrNPCVersionConflict:   "该 NPC 已被其他人修改，请刷新后重试",
	ErrNPCBtWithoutFsm:      "配置行为树前请先选择状态机",
	ErrNPCExportDanglingRef: "NPC 导出失败：存在悬空的状态机/行为树引用，请按 details 修复",
}

// Msg 获取错误码对应的默认消息
func Msg(code int) string {
	if msg, ok := messages[code]; ok {
		return msg
	}
	return "未知错误"
}
