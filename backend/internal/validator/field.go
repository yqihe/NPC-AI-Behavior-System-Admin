package validator

import (
	"regexp"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FieldValidator 字段校验器
type FieldValidator struct {
	dictCache *cache.DictCache
	cfg       *config.ValidationConfig
}

// NewFieldValidator 创建 FieldValidator
func NewFieldValidator(dictCache *cache.DictCache, cfg *config.ValidationConfig) *FieldValidator {
	return &FieldValidator{dictCache: dictCache, cfg: cfg}
}

// ValidateCreate 校验创建请求
func (v *FieldValidator) ValidateCreate(req *model.CreateFieldRequest) *errcode.Error {
	if req.Name == "" {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "字段标识不能为空")
	}
	if !namePattern.MatchString(req.Name) {
		return errcode.New(errcode.ErrFieldNameInvalid)
	}
	if len(req.Name) > v.cfg.FieldNameMaxLength {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "字段标识长度不能超过 %d 个字符", v.cfg.FieldNameMaxLength)
	}
	if req.Label == "" {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "中文标签不能为空")
	}
	if len(req.Label) > v.cfg.FieldLabelMaxLength {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "中文标签长度不能超过 %d 个字符", v.cfg.FieldLabelMaxLength)
	}
	if req.Type == "" {
		return errcode.Newf(errcode.ErrFieldTypeNotFound, "字段类型不能为空")
	}
	if !v.dictCache.Exists("field_type", req.Type) {
		return errcode.Newf(errcode.ErrFieldTypeNotFound, "字段类型 '%s' 不存在", req.Type)
	}
	if req.Category == "" {
		return errcode.Newf(errcode.ErrFieldCategoryNotFound, "标签分类不能为空")
	}
	if !v.dictCache.Exists("field_category", req.Category) {
		return errcode.Newf(errcode.ErrFieldCategoryNotFound, "标签分类 '%s' 不存在", req.Category)
	}
	if req.Properties == nil {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "properties 不能为空")
	}
	return nil
}

// ValidateUpdate 校验编辑请求（不校验 name，name 不可变）
func (v *FieldValidator) ValidateUpdate(req *model.UpdateFieldRequest) *errcode.Error {
	if req.Label == "" {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "中文标签不能为空")
	}
	if len(req.Label) > v.cfg.FieldLabelMaxLength {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "中文标签长度不能超过 %d 个字符", v.cfg.FieldLabelMaxLength)
	}
	if req.Type == "" {
		return errcode.Newf(errcode.ErrFieldTypeNotFound, "字段类型不能为空")
	}
	if !v.dictCache.Exists("field_type", req.Type) {
		return errcode.Newf(errcode.ErrFieldTypeNotFound, "字段类型 '%s' 不存在", req.Type)
	}
	if req.Category == "" {
		return errcode.Newf(errcode.ErrFieldCategoryNotFound, "标签分类不能为空")
	}
	if !v.dictCache.Exists("field_category", req.Category) {
		return errcode.Newf(errcode.ErrFieldCategoryNotFound, "标签分类 '%s' 不存在", req.Category)
	}
	if req.Properties == nil {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "properties 不能为空")
	}
	if req.Version <= 0 {
		return errcode.Newf(errcode.ErrFieldVersionConflict, "版本号不合法")
	}
	return nil
}
