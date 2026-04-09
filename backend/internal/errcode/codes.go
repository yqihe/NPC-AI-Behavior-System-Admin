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
	ErrFieldBBKeyInUse       = 40008 // BB Key 被行为树引用无法关闭
	ErrFieldCyclicRef        = 40009 // 循环引用
	ErrFieldVersionConflict  = 40010 // 版本冲突（乐观锁）
	ErrFieldNotFound         = 40011 // 字段不存在
	ErrFieldRefNotFound      = 40014 // 引用的字段不存在
	ErrFieldDeleteNotDisabled = 40012 // 删除前必须先停用
	ErrFieldRefDisabled       = 40013 // 不能引用已停用的字段
	ErrFieldEditNotDisabled   = 40015 // 编辑前必须先停用
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
	ErrFieldBBKeyInUse:       "该 Key 正被行为树使用，无法关闭",
	ErrFieldCyclicRef:        "检测到循环引用",
	ErrFieldVersionConflict:  "该字段已被其他人修改，请刷新后重试",
	ErrFieldNotFound:         "字段不存在",
	ErrFieldRefNotFound:      "引用的字段不存在",
	ErrFieldDeleteNotDisabled: "请先停用该字段再删除",
	ErrFieldRefDisabled:       "不能引用已停用的字段",
	ErrFieldEditNotDisabled:   "请先停用该字段再编辑",
}

// Msg 获取错误码对应的默认消息
func Msg(code int) string {
	if msg, ok := messages[code]; ok {
		return msg
	}
	return "未知错误"
}
