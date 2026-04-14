package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// FieldService 字段管理业务逻辑
type FieldService struct {
	fieldStore    *storemysql.FieldStore
	fieldRefStore *storemysql.FieldRefStore
	fieldCache    *storeredis.FieldCache
	dictCache     *cache.DictCache
	pagCfg        *config.PaginationConfig
}

// NewFieldService 创建 FieldService
func NewFieldService(fieldStore *storemysql.FieldStore, fieldRefStore *storemysql.FieldRefStore, fieldCache *storeredis.FieldCache, dictCache *cache.DictCache, pagCfg *config.PaginationConfig) *FieldService {
	return &FieldService{
		fieldStore:    fieldStore,
		fieldRefStore: fieldRefStore,
		fieldCache:    fieldCache,
		dictCache:     dictCache,
		pagCfg:        pagCfg,
	}
}

// ---- 业务校验辅助 ----

func (s *FieldService) checkDictExists(group, value string, code int, label string) *errcode.Error {
	if !s.dictCache.Exists(group, value) {
		return errcode.Newf(code, "%s '%s' 不存在", label, value)
	}
	return nil
}

func (s *FieldService) checkTypeExists(typ string) *errcode.Error {
	return s.checkDictExists(util.DictGroupFieldType, typ, errcode.ErrFieldTypeNotFound, "字段类型")
}

func (s *FieldService) checkCategoryExists(category string) *errcode.Error {
	return s.checkDictExists(util.DictGroupFieldCategory, category, errcode.ErrFieldCategoryNotFound, "标签分类")
}

// validatePropertiesConstraints 校验字段 properties 中 constraints 的自洽性
// reference 类型的 refs 校验由 validateReferenceRefs 单独处理，此处跳过
func (s *FieldService) validatePropertiesConstraints(fieldType string, properties json.RawMessage) *errcode.Error {
	if fieldType == util.FieldTypeReference {
		return nil
	}
	props, err := parseProperties(properties)
	if err != nil || props == nil {
		return nil
	}
	if len(props.Constraints) == 0 {
		return nil
	}
	return util.ValidateConstraintsSelf(fieldType, props.Constraints, errcode.ErrBadRequest)
}

// getFieldOrNotFound 按 ID 查字段 + 判空
func (s *FieldService) getFieldOrNotFound(ctx context.Context, id int64) (*model.Field, error) {
	field, err := s.fieldStore.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get field %d: %w", id, err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 ID=%d 不存在", id)
	}
	return field, nil
}

// ---- 业务方法 ----

// List 字段列表（Cache-Aside：Redis → MySQL → 写 Redis）
func (s *FieldService) List(ctx context.Context, q *model.FieldListQuery) (*model.ListData, error) {
	util.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 1. 查 Redis 缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.fieldCache.GetList(ctx, q); err == nil && hit {
		return cached.ToListData(), nil
	}

	// 2. 查 MySQL
	items, total, err := s.fieldStore.List(ctx, q)
	if err != nil {
		slog.Error("service.字段列表查询失败", "error", err, "query", q)
		return nil, err
	}

	for i := range items {
		items[i].TypeLabel = s.dictCache.GetLabel(util.DictGroupFieldType, items[i].Type)
		items[i].CategoryLabel = s.dictCache.GetLabel(util.DictGroupFieldCategory, items[i].Category)
	}

	result := &model.FieldListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}

	// 3. 写 Redis 缓存
	s.fieldCache.SetList(ctx, q, result)

	return result.ToListData(), nil
}

// Create 创建字段
func (s *FieldService) Create(ctx context.Context, req *model.CreateFieldRequest) (int64, error) {
	// 业务校验：type/category 存在性
	if err := s.checkTypeExists(req.Type); err != nil {
		return 0, err
	}
	if err := s.checkCategoryExists(req.Category); err != nil {
		return 0, err
	}

	// 业务校验：constraints 自洽（min<=max, precision>0, select options 非空/不重复 等）
	if err := s.validatePropertiesConstraints(req.Type, req.Properties); err != nil {
		return 0, err
	}

	// 业务校验：name 唯一性（含软删除）
	exists, err := s.fieldStore.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建字段-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrFieldNameExists, "字段标识 '%s' 已存在", req.Name)
	}

	// reference 类型：通过 validateReferenceRefs 统一校验
	// 规则：非空 + 目标存在 + 目标启用 + 目标非 reference（禁嵌套） + 无循环
	var refFieldIDs []int64
	if req.Type == util.FieldTypeReference {
		props, _ := parseProperties(req.Properties)
		if props != nil {
			refFieldIDs = parseRefFieldIDs(props.Constraints)
		}
		if err := s.validateReferenceRefs(ctx, 0, refFieldIDs, nil); err != nil {
			return 0, err
		}
	}

	// 写入
	id, err := s.fieldStore.Create(ctx, req)
	if err != nil {
		slog.Error("service.创建字段失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create field: %w", err)
	}

	// reference 类型：写入引用关系
	if len(refFieldIDs) > 0 {
		affected, err := s.syncFieldRefs(ctx, id, nil, refFieldIDs)
		if err != nil {
			slog.Error("service.创建字段-同步引用失败", "error", err, "id", id)
			return 0, fmt.Errorf("sync field refs: %w", err)
		}
		for _, affectedID := range affected {
			s.fieldCache.DelDetail(ctx, affectedID)
		}
	}

	// 清缓存
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.创建字段成功", "name", req.Name, "id", id)
	return id, nil
}

// GetByID 查询字段详情（Cache-Aside + 分布式锁防击穿）
func (s *FieldService) GetByID(ctx context.Context, id int64) (*model.Field, error) {
	// 1. 查 Redis 缓存
	if cached, hit, err := s.fieldCache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 ID=%d 不存在", id)
		}
		return cached, nil
	}

	// 2. 分布式锁防缓存击穿
	locked, lockErr := s.fieldCache.TryLock(ctx, id, 3*time.Second)
	if lockErr != nil {
		slog.Warn("service.获取锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if locked {
		defer s.fieldCache.Unlock(ctx, id)
	}

	// 获得锁后再查一次缓存（double-check）
	if locked {
		if cached, hit, err := s.fieldCache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 ID=%d 不存在", id)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	field, err := s.fieldStore.GetByID(ctx, id)
	if err != nil {
		slog.Error("service.查询字段详情失败", "error", err, "id", id)
		return nil, fmt.Errorf("get field: %w", err)
	}

	// 4. 写 Redis（field 为 nil 时也缓存，防穿透）
	s.fieldCache.SetDetail(ctx, id, field)

	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 ID=%d 不存在", id)
	}

	// has_refs 不进缓存（引用关系随模板操作变化），每次实时查 field_refs
	hasRefs, err := s.fieldRefStore.HasRefs(ctx, field.ID)
	if err != nil {
		slog.Warn("service.查询字段引用失败，降级为无引用", "error", err, "id", id)
	}
	field.HasRefs = hasRefs

	return field, nil
}

// Update 编辑字段（仅未启用时可编辑）
func (s *FieldService) Update(ctx context.Context, req *model.UpdateFieldRequest) error {
	// 业务校验：type/category 存在性
	if err := s.checkTypeExists(req.Type); err != nil {
		return err
	}
	if err := s.checkCategoryExists(req.Category); err != nil {
		return err
	}

	// 业务校验：constraints 自洽
	if err := s.validatePropertiesConstraints(req.Type, req.Properties); err != nil {
		return err
	}

	// 查旧数据
	old, err := s.getFieldOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	// 硬约束：必须未启用才能编辑
	if old.Enabled {
		return errcode.New(errcode.ErrFieldEditNotDisabled)
	}

	// 查询是否有引用（用于类型变更禁止和约束收紧检查）
	hasRefs, err := s.fieldRefStore.HasRefs(ctx, req.ID)
	if err != nil {
		slog.Error("service.查询字段引用失败", "error", err, "id", req.ID)
		return fmt.Errorf("check field refs: %w", err)
	}

	// 硬约束：被引用时禁止改 type
	if old.Type != req.Type && hasRefs {
		return errcode.New(errcode.ErrFieldRefChangeType)
	}

	// 硬约束：被引用时禁止收紧约束（只能放宽）
	if hasRefs && old.Type == req.Type {
		oldProps, _ := parseProperties(old.Properties)
		newProps, _ := parseProperties(req.Properties)
		if oldProps != nil && newProps != nil {
			if err := util.CheckConstraintTightened(old.Type, oldProps.Constraints, newProps.Constraints, errcode.ErrFieldRefTighten); err != nil {
				return err
			}
		}
	}

	// reference 类型：通过 validateReferenceRefs 统一校验
	// 规则：非空 + 目标存在 + 新增 ref 必须启用非 reference + 无循环
	// 已有 ref（在 oldRefSet 中）不重新校验启用/嵌套，保持"存量不动"
	if req.Type == util.FieldTypeReference {
		newProps, _ := parseProperties(req.Properties)
		var newRefIDs []int64
		if newProps != nil {
			newRefIDs = parseRefFieldIDs(newProps.Constraints)
		}
		// 旧 ref 集合（仅旧类型也是 reference 时才有意义）
		var oldRefSet map[int64]bool
		if old.Type == util.FieldTypeReference {
			oldRefSet = make(map[int64]bool)
			oldProps, _ := parseProperties(old.Properties)
			if oldProps != nil {
				for _, rid := range parseRefFieldIDs(oldProps.Constraints) {
					oldRefSet[rid] = true
				}
			}
		}
		if err := s.validateReferenceRefs(ctx, req.ID, newRefIDs, oldRefSet); err != nil {
			return err
		}
	}

	// 硬约束：取消 expose_bb 时，检查是否有 FSM 引用该 BB Key
	oldProps, _ := parseProperties(old.Properties)
	newProps, _ := parseProperties(req.Properties)
	if oldProps != nil && newProps != nil && oldProps.ExposeBB && !newProps.ExposeBB {
		refs, err := s.fieldRefStore.GetByFieldID(ctx, req.ID)
		if err != nil {
			slog.Error("service.查字段引用失败", "error", err, "id", req.ID)
			return fmt.Errorf("get field refs: %w", err)
		}
		for _, r := range refs {
			if r.RefType == util.RefTypeFsm {
				return errcode.New(errcode.ErrFieldBBKeyInUse)
			}
		}
	}

	// 乐观锁写入
	err = s.fieldStore.Update(ctx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrFieldVersionConflict)
		}
		slog.Error("service.编辑字段失败", "error", err, "id", req.ID)
		return fmt.Errorf("update field: %w", err)
	}

	// reference 类型：同步引用关系
	var refAffected []int64
	if req.Type == util.FieldTypeReference {
		oldProps, _ := parseProperties(old.Properties)
		newProps, _ := parseProperties(req.Properties)
		var oldRefIDs, newRefIDs []int64
		if oldProps != nil && old.Type == util.FieldTypeReference {
			oldRefIDs = parseRefFieldIDs(oldProps.Constraints)
		}
		if newProps != nil {
			newRefIDs = parseRefFieldIDs(newProps.Constraints)
		}
		affected, err := s.syncFieldRefs(ctx, req.ID, oldRefIDs, newRefIDs)
		if err != nil {
			slog.Error("service.编辑字段-同步引用失败", "error", err, "id", req.ID)
			return fmt.Errorf("sync field refs: %w", err)
		}
		refAffected = affected
	} else if old.Type == util.FieldTypeReference && req.Type != util.FieldTypeReference {
		// 类型从 reference 改为其他：清除所有引用关系
		oldProps, _ := parseProperties(old.Properties)
		if oldProps != nil {
			oldRefIDs := parseRefFieldIDs(oldProps.Constraints)
			affected, err := s.syncFieldRefs(ctx, req.ID, oldRefIDs, nil)
			if err != nil {
				slog.Error("service.编辑字段-清除引用失败", "error", err, "id", req.ID)
				return fmt.Errorf("clear field refs: %w", err)
			}
			refAffected = affected
		}
	}

	// 清缓存：自身 + 受影响的被引用方
	s.fieldCache.DelDetail(ctx, req.ID)
	for _, affectedID := range refAffected {
		s.fieldCache.DelDetail(ctx, affectedID)
	}
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.编辑字段成功", "id", req.ID)
	return nil
}

// Delete 软删除字段
func (s *FieldService) Delete(ctx context.Context, id int64) (*model.DeleteResult, error) {
	field, err := s.getFieldOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 硬约束：必须先停用
	if field.Enabled {
		return nil, errcode.New(errcode.ErrFieldDeleteNotDisabled)
	}

	// 事务内原子操作：FOR SHARE 检查引用 + 软删除（防 TOCTOU）
	tx, err := s.fieldStore.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	hasRefs, err := s.fieldRefStore.HasRefsTx(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("check refs in tx: %w", err)
	}
	if hasRefs {
		return nil, errcode.New(errcode.ErrFieldRefDelete)
	}

	if err := s.fieldStore.SoftDeleteTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 ID=%d 不存在", id)
		}
		return nil, fmt.Errorf("soft delete: %w", err)
	}

	// reference 类型字段删除时，清除它对其他字段的引用关系
	var affectedIDs []int64
	if field.Type == util.FieldTypeReference {
		affectedIDs, err = s.fieldRefStore.RemoveBySource(ctx, tx, util.RefTypeField, id)
		if err != nil {
			return nil, fmt.Errorf("remove field refs: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 清缓存
	s.fieldCache.DelDetail(ctx, id)
	for _, affectedID := range affectedIDs {
		s.fieldCache.DelDetail(ctx, affectedID)
	}
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.删除字段成功", "id", id, "name", field.Name)
	return &model.DeleteResult{ID: id, Name: field.Name, Label: field.Label}, nil
}

// CheckName 校验字段标识是否可用（保留 name，创建前校验）
func (s *FieldService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.fieldStore.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验字段名失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该字段标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// GetReferences 查询字段引用详情
//
// 分层职责：本方法只负责字段模块内的数据（field_refs 关系 + 字段自身 label），
// 不查询模板表。模板 label 的填充由 handler 跨模块编排（调用 TemplateService）。
// 返回的 Templates 数组中，每项 Label 字段为空，由 handler 负责补齐。
func (s *FieldService) GetReferences(ctx context.Context, id int64) (*model.ReferenceDetail, error) {
	field, err := s.getFieldOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	refs, err := s.fieldRefStore.GetByFieldID(ctx, id)
	if err != nil {
		slog.Error("service.引用详情-查引用失败", "error", err, "id", id)
		return nil, fmt.Errorf("get refs: %w", err)
	}

	templateIDs := make([]int64, 0)
	fieldIDs := make([]int64, 0)
	fsmIDs := make([]int64, 0)
	for _, r := range refs {
		switch r.RefType {
		case util.RefTypeTemplate:
			templateIDs = append(templateIDs, r.RefID)
		case util.RefTypeField:
			fieldIDs = append(fieldIDs, r.RefID)
		case util.RefTypeFsm:
			fsmIDs = append(fsmIDs, r.RefID)
		}
	}

	result := &model.ReferenceDetail{
		FieldID:    id,
		FieldLabel: field.Label,
		Templates:  make([]model.ReferenceItem, 0, len(templateIDs)),
		Fields:     make([]model.ReferenceItem, 0, len(fieldIDs)),
		Fsms:       make([]model.ReferenceItem, 0, len(fsmIDs)),
	}

	if len(fieldIDs) > 0 {
		fieldList, err := s.fieldStore.GetByIDs(ctx, fieldIDs)
		if err != nil {
			slog.Error("service.引用详情-查字段label失败", "error", err)
			return nil, fmt.Errorf("get field labels: %w", err)
		}
		labelMap := make(map[int64]string, len(fieldList))
		for _, f := range fieldList {
			labelMap[f.ID] = f.Label
		}
		for _, fid := range fieldIDs {
			result.Fields = append(result.Fields, model.ReferenceItem{
				RefType: util.RefTypeField,
				RefID:   fid,
				Label:   labelMap[fid],
			})
		}
	}

	// 模板引用：只填 ID，Label 留空由 handler 跨模块补齐
	for _, tid := range templateIDs {
		result.Templates = append(result.Templates, model.ReferenceItem{
			RefType: util.RefTypeTemplate,
			RefID:   tid,
		})
	}

	// FSM 引用：只填 ID，Label 留空由 handler 跨模块补齐
	for _, fid := range fsmIDs {
		result.Fsms = append(result.Fsms, model.ReferenceItem{
			RefType: util.RefTypeFsm,
			RefID:   fid,
		})
	}

	return result, nil
}

// ToggleEnabled 切换启用/停用
func (s *FieldService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	if _, err := s.getFieldOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	err := s.fieldStore.ToggleEnabled(ctx, req.ID, req.Enabled, req.Version)
	if err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrFieldVersionConflict)
		}
		slog.Error("service.切换启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle enabled: %w", err)
	}

	s.fieldCache.DelDetail(ctx, req.ID)
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.切换启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// ---- 跨模块对外方法（供 handler 跨模块编排调用） ----

// ValidateFieldsForTemplate 校验字段列表对模板的可用性
//
// 用途：模板创建/编辑时由 handler 调用，校验勾选的 field_ids 全部存在 + 启用 + 非 reference。
// 返回：
//   - errcode.ErrTemplateFieldNotFound (41006) — 任一字段不存在
//   - errcode.ErrTemplateFieldDisabled (41005) — 任一字段已停用
//   - errcode.ErrTemplateFieldIsReference (41012) — 任一字段是 reference 类型
//   - nil — 全部通过
//
// 41005/41006/41012 归在模板段位（41xxx），因为这些错误由模板管理页消费，
// 与字段管理自身的 40011/40013 语义不混用（go-red-lines）。
//
// 禁 reference 的原因：reference 字段只是前端"快捷选择器"，模板侧只存展开后的 leaf
// 字段 ID；允许 reference 进入模板 fields 会让 templates.fields JSON 与
// "模板只存 leaf"的全局约定相悖，也会让 NPC 渲染/导出路径出现歧义。
func (s *FieldService) ValidateFieldsForTemplate(ctx context.Context, fieldIDs []int64) error {
	if len(fieldIDs) == 0 {
		return nil
	}
	fields, err := s.fieldStore.GetByIDs(ctx, fieldIDs)
	if err != nil {
		return fmt.Errorf("get fields by ids: %w", err)
	}
	foundMap := make(map[int64]model.Field, len(fields))
	for _, f := range fields {
		foundMap[f.ID] = f
	}
	for _, fid := range fieldIDs {
		f, ok := foundMap[fid]
		if !ok {
			return errcode.Newf(errcode.ErrTemplateFieldNotFound, "字段 ID=%d 不存在", fid)
		}
		if !f.Enabled {
			return errcode.Newf(errcode.ErrTemplateFieldDisabled, "字段 '%s' 已停用，请先在字段管理中启用", f.Name)
		}
		if f.Type == util.FieldTypeReference {
			return errcode.Newf(errcode.ErrTemplateFieldIsReference, "字段 '%s' 是 reference 类型，请展开其子字段后加入模板", f.Name)
		}
	}
	return nil
}

// AttachToTemplateTx 把字段列表挂到模板上（事务内）
//
// 用途：模板创建 / 编辑模板新增字段时由 handler 调用。
// 行为：对每个 fieldID 写 field_refs(field_id, 'template', templateID)
//
// 返回：fieldIDs 副本，handler 在 commit 后用它清字段方 detail 缓存。
func (s *FieldService) AttachToTemplateTx(ctx context.Context, tx *sqlx.Tx, templateID int64, fieldIDs []int64) ([]int64, error) {
	if len(fieldIDs) == 0 {
		return make([]int64, 0), nil
	}
	for _, fieldID := range fieldIDs {
		if err := s.fieldRefStore.Add(ctx, tx, fieldID, util.RefTypeTemplate, templateID); err != nil {
			return nil, fmt.Errorf("add field ref %d → template %d: %w", fieldID, templateID, err)
		}
	}
	affected := make([]int64, len(fieldIDs))
	copy(affected, fieldIDs)
	return affected, nil
}

// DetachFromTemplateTx 把字段列表从模板上卸下（事务内）
//
// 用途：模板删除 / 编辑模板移除字段时由 handler 调用。
// 行为：对每个 fieldID 删 field_refs(field_id, 'template', templateID)
//
// 返回：fieldIDs 副本，handler 在 commit 后用它清字段方 detail 缓存。
func (s *FieldService) DetachFromTemplateTx(ctx context.Context, tx *sqlx.Tx, templateID int64, fieldIDs []int64) ([]int64, error) {
	if len(fieldIDs) == 0 {
		return make([]int64, 0), nil
	}
	for _, fieldID := range fieldIDs {
		if err := s.fieldRefStore.Remove(ctx, tx, fieldID, util.RefTypeTemplate, templateID); err != nil {
			return nil, fmt.Errorf("remove field ref %d → template %d: %w", fieldID, templateID, err)
		}
	}
	affected := make([]int64, len(fieldIDs))
	copy(affected, fieldIDs)
	return affected, nil
}

// GetByIDsLite 批量查字段精简信息（跨模块）
//
// 用途：模板详情接口由 handler 调用拼装 TemplateFieldItem。
// 行为：
//   - 调 fieldStore.GetByIDs 批量取
//   - service 层用 dictCache 翻译 CategoryLabel
//   - 保持 fieldIDs 顺序对齐：缺失的位置返回 zero FieldLite{ID: 0}
//     handler 拼装时识别 ID=0 跳过并 slog.Warn
func (s *FieldService) GetByIDsLite(ctx context.Context, fieldIDs []int64) ([]model.FieldLite, error) {
	result := make([]model.FieldLite, len(fieldIDs))
	if len(fieldIDs) == 0 {
		return result, nil
	}
	fields, err := s.fieldStore.GetByIDs(ctx, fieldIDs)
	if err != nil {
		return nil, fmt.Errorf("get fields by ids: %w", err)
	}
	foundMap := make(map[int64]model.Field, len(fields))
	for _, f := range fields {
		foundMap[f.ID] = f
	}
	for i, fid := range fieldIDs {
		f, ok := foundMap[fid]
		if !ok {
			// 缺失：保持 zero value（ID=0），handler 识别后 warn + skip
			continue
		}
		result[i] = model.FieldLite{
			ID:            f.ID,
			Name:          f.Name,
			Label:         f.Label,
			Type:          f.Type,
			Category:      f.Category,
			CategoryLabel: s.dictCache.GetLabel(util.DictGroupFieldCategory, f.Category),
			Enabled:       f.Enabled,
		}
	}
	return result, nil
}

// InvalidateDetails 批量清字段详情缓存
//
// 用途：模板写操作 commit 后由 handler 调用，保证字段引用关系变更后缓存一致。
// 不返回 error：缓存清理失败仅 slog.Error，不阻塞业务。
func (s *FieldService) InvalidateDetails(ctx context.Context, fieldIDs []int64) {
	for _, fid := range fieldIDs {
		s.fieldCache.DelDetail(ctx, fid)
	}
}

// ---- 内部辅助 ----

func parseProperties(raw json.RawMessage) (*model.FieldProperties, error) {
	if len(raw) == 0 {
		return &model.FieldProperties{}, nil
	}
	var props model.FieldProperties
	if err := json.Unmarshal(raw, &props); err != nil {
		return nil, fmt.Errorf("unmarshal properties: %w", err)
	}
	return &props, nil
}

// ---- 循环引用检测 ----

// parseRefFieldIDs 从 reference 字段的 constraints 中提取引用字段 ID 列表
func parseRefFieldIDs(constraints json.RawMessage) []int64 {
	if len(constraints) == 0 {
		return nil
	}
	var c struct {
		Refs []int64 `json:"refs"`
	}
	if err := json.Unmarshal(constraints, &c); err != nil {
		return nil
	}
	return c.Refs
}

// validateReferenceRefs 校验 reference 字段的 refs 业务规则
//
// currentID: 正在创建/编辑的字段 ID（Create 传 0）
// newRefIDs: 新的 refs 列表
// oldRefSet: 旧 refs 集合（Create 或旧类型非 reference 时传 nil）
//
// 规则：
//  1. newRefIDs 不能为空 → 40017 ErrFieldRefEmpty
//  2. 每个 refID 必须存在 → 40014 ErrFieldRefNotFound
//  3. 对新增的 refID（不在 oldRefSet 中）：
//     - 必须启用 → 40013 ErrFieldRefDisabled
//     - 不能是 reference 类型（禁止嵌套） → 40016 ErrFieldRefNested
//  4. 末尾检测循环引用 → 40009 ErrFieldCyclicRef
//
// 存量不动：已有的 ref 即使后来被停用或目标类型变成 reference 也保留，
// 只有新增/替换产生的新 ref 才走严格校验。nil oldRefSet 等价于"所有 ref 都是新增"。
func (s *FieldService) validateReferenceRefs(ctx context.Context, currentID int64, newRefIDs []int64, oldRefSet map[int64]bool) error {
	if len(newRefIDs) == 0 {
		return errcode.New(errcode.ErrFieldRefEmpty)
	}
	for _, refID := range newRefIDs {
		f, err := s.fieldStore.GetByID(ctx, refID)
		if err != nil {
			return fmt.Errorf("check ref field %d: %w", refID, err)
		}
		if f == nil {
			return errcode.Newf(errcode.ErrFieldRefNotFound, "引用的字段 ID=%d 不存在", refID)
		}
		if oldRefSet[refID] {
			continue // 存量不动
		}
		if !f.Enabled {
			return errcode.Newf(errcode.ErrFieldRefDisabled, "字段 '%s' 已停用，不能引用", f.Name)
		}
		if f.Type == util.FieldTypeReference {
			return errcode.Newf(errcode.ErrFieldRefNested, "字段 '%s' 是 reference 类型，不允许嵌套引用", f.Name)
		}
	}
	if err := s.detectCyclicRef(ctx, currentID, newRefIDs); err != nil {
		return err
	}
	return nil
}

// detectCyclicRef 检测循环引用（DFS）
// currentID: 当前正在创建/编辑的字段 ID（新建时为 0）
// refIDs: 当前字段要引用的字段 ID 列表
func (s *FieldService) detectCyclicRef(ctx context.Context, currentID int64, refIDs []int64) *errcode.Error {
	visited := make(map[int64]bool)
	if currentID > 0 {
		visited[currentID] = true
	}

	var dfs func(ids []int64) *errcode.Error
	dfs = func(ids []int64) *errcode.Error {
		for _, id := range ids {
			if visited[id] {
				return errcode.Newf(errcode.ErrFieldCyclicRef, "字段 ID=%d 形成循环引用", id)
			}
			visited[id] = true

			field, err := s.fieldStore.GetByID(ctx, id)
			if err != nil || field == nil {
				continue
			}
			if field.Type != util.FieldTypeReference {
				continue
			}

			props, err := parseProperties(field.Properties)
			if err != nil {
				continue
			}
			subRefs := parseRefFieldIDs(props.Constraints)
			if len(subRefs) > 0 {
				if err := dfs(subRefs); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return dfs(refIDs)
}

// syncFieldRefs 同步 reference 字段的引用关系
// sourceFieldID: 引用方字段 ID
// oldRefIDs, newRefIDs: 旧/新被引用字段 ID 列表
// 返回引用关系发生变化的字段 ID 列表（用于清缓存）
func (s *FieldService) syncFieldRefs(ctx context.Context, sourceFieldID int64, oldRefIDs, newRefIDs []int64) ([]int64, error) {
	oldSet := make(map[int64]bool, len(oldRefIDs))
	for _, r := range oldRefIDs {
		oldSet[r] = true
	}
	newSet := make(map[int64]bool, len(newRefIDs))
	for _, r := range newRefIDs {
		newSet[r] = true
	}

	toAdd := make([]int64, 0)
	toRemove := make([]int64, 0)
	for _, r := range newRefIDs {
		if !oldSet[r] {
			toAdd = append(toAdd, r)
		}
	}
	for _, r := range oldRefIDs {
		if !newSet[r] {
			toRemove = append(toRemove, r)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil, nil
	}

	tx, err := s.fieldStore.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, targetID := range toAdd {
		if err := s.fieldRefStore.Add(ctx, tx, targetID, util.RefTypeField, sourceFieldID); err != nil {
			return nil, fmt.Errorf("add field ref %d: %w", targetID, err)
		}
	}

	for _, targetID := range toRemove {
		if err := s.fieldRefStore.Remove(ctx, tx, targetID, util.RefTypeField, sourceFieldID); err != nil {
			return nil, fmt.Errorf("remove field ref %d: %w", targetID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	affected := make([]int64, 0, len(toAdd)+len(toRemove))
	affected = append(affected, toAdd...)
	affected = append(affected, toRemove...)
	return affected, nil
}

// ---- FSM BB Key 引用维护 ----

// SyncFsmBBKeyRefs 同步 FSM 条件中 BB Key 对字段的引用关系（事务内）
//
// oldKeys/newKeys: 条件树中提取的 BB Key name 集合。
// 内部解析 name→field ID，只追踪来自字段表的 Key（运行时 Key 跳过）。
// 返回 affected field IDs（用于清缓存）。
func (s *FieldService) SyncFsmBBKeyRefs(ctx context.Context, tx *sqlx.Tx, fsmID int64, oldKeys, newKeys map[string]bool) ([]int64, error) {
	toAdd := make([]string, 0)
	toRemove := make([]string, 0)
	for k := range newKeys {
		if !oldKeys[k] {
			toAdd = append(toAdd, k)
		}
	}
	for k := range oldKeys {
		if !newKeys[k] {
			toRemove = append(toRemove, k)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil, nil
	}

	// 批量查所有涉及的 name → field ID（合并查一次）
	allNames := make([]string, 0, len(toAdd)+len(toRemove))
	allNames = append(allNames, toAdd...)
	allNames = append(allNames, toRemove...)
	fields, err := s.fieldStore.GetByNames(ctx, allNames)
	if err != nil {
		return nil, fmt.Errorf("get fields by names: %w", err)
	}
	nameToID := make(map[string]int64, len(fields))
	for _, f := range fields {
		nameToID[f.Name] = f.ID
	}

	affected := make([]int64, 0)

	for _, name := range toAdd {
		fieldID, ok := nameToID[name]
		if !ok {
			continue // 运行时 Key，不来自字段表，跳过
		}
		if err := s.fieldRefStore.Add(ctx, tx, fieldID, util.RefTypeFsm, fsmID); err != nil {
			return nil, fmt.Errorf("add fsm bb key ref %s: %w", name, err)
		}
		affected = append(affected, fieldID)
	}

	for _, name := range toRemove {
		fieldID, ok := nameToID[name]
		if !ok {
			continue
		}
		if err := s.fieldRefStore.Remove(ctx, tx, fieldID, util.RefTypeFsm, fsmID); err != nil {
			return nil, fmt.Errorf("remove fsm bb key ref %s: %w", name, err)
		}
		affected = append(affected, fieldID)
	}

	return affected, nil
}

// CleanFsmBBKeyRefs 清理 FSM 删除时的所有 BB Key 引用（事务内）
//
// 返回被引用的 field IDs（用于清缓存）。
func (s *FieldService) CleanFsmBBKeyRefs(ctx context.Context, tx *sqlx.Tx, fsmID int64) ([]int64, error) {
	return s.fieldRefStore.RemoveBySource(ctx, tx, util.RefTypeFsm, fsmID)
}
