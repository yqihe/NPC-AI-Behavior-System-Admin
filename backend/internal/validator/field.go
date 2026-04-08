package validator

import (
	"fmt"
	"regexp"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FieldValidator 字段校验器
type FieldValidator struct {
	dictCache *cache.DictCache
}

// NewFieldValidator 创建 FieldValidator
func NewFieldValidator(dictCache *cache.DictCache) *FieldValidator {
	return &FieldValidator{dictCache: dictCache}
}

// ValidateCreate 校验创建请求
func (v *FieldValidator) ValidateCreate(req *model.CreateFieldRequest) (int, string) {
	if req.Name == "" {
		return 40002, "字段标识不能为空"
	}
	if !namePattern.MatchString(req.Name) {
		return 40002, "字段标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线"
	}
	if len(req.Name) > 64 {
		return 40002, "字段标识长度不能超过 64 个字符"
	}
	if req.Label == "" {
		return 40002, "中文标签不能为空"
	}
	if len(req.Label) > 128 {
		return 40002, "中文标签长度不能超过 128 个字符"
	}
	if req.Type == "" {
		return 40003, "字段类型不能为空"
	}
	if !v.dictCache.Exists("field_type", req.Type) {
		return 40003, fmt.Sprintf("字段类型 '%s' 不存在", req.Type)
	}
	if req.Category == "" {
		return 40004, "标签分类不能为空"
	}
	if !v.dictCache.Exists("field_category", req.Category) {
		return 40004, fmt.Sprintf("标签分类 '%s' 不存在", req.Category)
	}
	if req.Properties == nil {
		return 40002, "properties 不能为空"
	}
	return 0, ""
}
