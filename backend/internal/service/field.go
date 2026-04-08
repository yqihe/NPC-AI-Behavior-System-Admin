package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
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

// ---- 业务校验（需查缓存/DB） ----

// checkDictExists 校验字典值是否存在（通用：type/category 等）
func (s *FieldService) checkDictExists(group, value string, code int, label string) *errcode.Error {
	if !s.dictCache.Exists(group, value) {
		return errcode.Newf(code, "%s '%s' 不存在", label, value)
	}
	return nil
}

func (s *FieldService) checkTypeExists(typ string) *errcode.Error {
	return s.checkDictExists(model.DictGroupFieldType, typ, errcode.ErrFieldTypeNotFound, "字段类型")
}

func (s *FieldService) checkCategoryExists(category string) *errcode.Error {
	return s.checkDictExists(model.DictGroupFieldCategory, category, errcode.ErrFieldCategoryNotFound, "标签分类")
}

// getFieldOrNotFound 查字段 + 判空，通用模式
func (s *FieldService) getFieldOrNotFound(ctx context.Context, name string) (*model.Field, error) {
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get field %s: %w", name, err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 '%s' 不存在", name)
	}
	return field, nil
}

// ---- 业务方法 ----

// List 字段列表（Cache-Aside：Redis → MySQL → 写 Redis）
func (s *FieldService) List(ctx context.Context, q *model.FieldListQuery) (*model.ListData, error) {
	if q.Page <= 0 {
		q.Page = s.pagCfg.DefaultPage
	}
	if q.PageSize <= 0 {
		q.PageSize = s.pagCfg.DefaultPageSize
	}
	if q.PageSize > s.pagCfg.MaxPageSize {
		q.PageSize = s.pagCfg.MaxPageSize
	}

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
		items[i].TypeLabel = s.dictCache.GetLabel(model.DictGroupFieldType, items[i].Type)
		items[i].CategoryLabel = s.dictCache.GetLabel(model.DictGroupFieldCategory, items[i].Category)
	}

	result := &model.FieldListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}

	// 3. 写 Redis 缓存（失败只记日志，不影响响应）
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

	// 业务校验：name 唯一性（含软删除）
	exists, err := s.fieldStore.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建字段-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrFieldNameExists, "字段标识 '%s' 已存在", req.Name)
	}

	// reference 类型：校验引用字段存在性 + 循环引用检测
	var newRefs []string
	if req.Type == model.FieldTypeReference {
		props, _ := parseProperties(req.Properties)
		if props != nil {
			newRefs = parseRefFields(props.Constraints)
			for _, refName := range newRefs {
				f, err := s.fieldStore.GetByName(ctx, refName)
				if err != nil {
					return 0, fmt.Errorf("check ref field %s: %w", refName, err)
				}
				if f == nil {
					return 0, errcode.Newf(errcode.ErrFieldRefNotFound, "引用的字段 '%s' 不存在", refName)
				}
				if !f.Enabled {
					return 0, errcode.Newf(errcode.ErrFieldRefDisabled, "字段 '%s' 已停用，不能引用", refName)
				}
			}
			if len(newRefs) > 0 {
				if err := s.detectCyclicRef(ctx, req.Name, newRefs); err != nil {
					return 0, err
				}
			}
		}
	}

	// 写入
	// TODO: Create + syncFieldRefs 目前非原子操作，reference 字段创建时如果 syncFieldRefs
	// 失败会导致引用关系缺失。模板管理上线时统一重构为事务内操作。
	id, err := s.fieldStore.Create(ctx, req)
	if err != nil {
		slog.Error("service.创建字段失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create field: %w", err)
	}

	// reference 类型：写入引用关系
	if len(newRefs) > 0 {
		affected, err := s.syncFieldRefs(ctx, req.Name, nil, newRefs)
		if err != nil {
			slog.Error("service.创建字段-同步引用失败", "error", err, "name", req.Name)
			return 0, fmt.Errorf("sync field refs: %w", err)
		}
		for _, n := range affected {
			s.fieldCache.DelDetail(ctx, n)
		}
	}

	// 清缓存
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.创建字段成功", "name", req.Name, "id", id)
	return id, nil
}

// GetByName 查询字段详情（Cache-Aside + 分布式锁防击穿）
func (s *FieldService) GetByName(ctx context.Context, name string) (*model.Field, error) {
	// 1. 查 Redis 缓存
	if cached, hit, err := s.fieldCache.GetDetail(ctx, name); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 '%s' 不存在", name)
		}
		return cached, nil
	}

	// 2. 分布式锁防缓存击穿：只放一个请求穿透到 MySQL
	locked, lockErr := s.fieldCache.TryLock(ctx, name, 3*time.Second)
	if lockErr != nil {
		slog.Warn("service.获取锁失败，降级直查MySQL", "error", lockErr, "name", name)
	}
	if locked {
		defer s.fieldCache.Unlock(ctx, name)
	}

	// 获得锁后再查一次缓存（等锁期间可能已被其他请求回填）
	if locked {
		if cached, hit, err := s.fieldCache.GetDetail(ctx, name); err == nil && hit {
			if cached == nil {
				return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 '%s' 不存在", name)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.查询字段详情失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}

	// 4. 写 Redis（field 为 nil 时也缓存，防穿透）
	s.fieldCache.SetDetail(ctx, name, field)

	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 '%s' 不存在", name)
	}
	return field, nil
}

// Update 编辑字段
func (s *FieldService) Update(ctx context.Context, name string, req *model.UpdateFieldRequest) error {
	// 业务校验：type/category 存在性
	if err := s.checkTypeExists(req.Type); err != nil {
		return err
	}
	if err := s.checkCategoryExists(req.Category); err != nil {
		return err
	}

	// 查旧数据
	old, err := s.getFieldOrNotFound(ctx, name)
	if err != nil {
		return err
	}

	// 硬约束：被引用时禁止改 type
	if old.Type != req.Type && old.RefCount > 0 {
		return errcode.Newf(errcode.ErrFieldRefChangeType, "该字段已被 %d 个模板/字段引用，无法修改类型", old.RefCount)
	}

	// 硬约束：被引用时禁止收紧约束
	if old.RefCount > 0 && old.Type == req.Type {
		oldProps, _ := parseProperties(old.Properties)
		newProps, _ := parseProperties(req.Properties)
		if oldProps != nil && newProps != nil {
			if err := checkConstraintTightened(old.Type, oldProps.Constraints, newProps.Constraints); err != nil {
				return err
			}
		}
	}

	// reference 类型：循环引用检测 + 新增引用启用检查
	if req.Type == model.FieldTypeReference {
		newProps, _ := parseProperties(req.Properties)
		if newProps != nil {
			newRefs := parseRefFields(newProps.Constraints)
			if len(newRefs) > 0 {
				// 计算旧引用集合（已有的停用字段可保留）
				oldRefSet := make(map[string]bool)
				if old.Type == model.FieldTypeReference {
					oldProps, _ := parseProperties(old.Properties)
					if oldProps != nil {
						for _, r := range parseRefFields(oldProps.Constraints) {
							oldRefSet[r] = true
						}
					}
				}

				for _, refName := range newRefs {
					f, err := s.fieldStore.GetByName(ctx, refName)
					if err != nil {
						return fmt.Errorf("check ref field %s: %w", refName, err)
					}
					if f == nil {
						return errcode.Newf(errcode.ErrFieldRefNotFound, "引用的字段 '%s' 不存在", refName)
					}
					// 只有新增的引用才检查启用状态
					if !oldRefSet[refName] && !f.Enabled {
						return errcode.Newf(errcode.ErrFieldRefDisabled, "字段 '%s' 已停用，不能新增引用", refName)
					}
				}
				// 循环引用检测
				if err := s.detectCyclicRef(ctx, name, newRefs); err != nil {
					return err
				}
			}
		}
	}

	// 乐观锁写入
	err = s.fieldStore.Update(ctx, name, req)
	if err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrFieldVersionConflict)
		}
		slog.Error("service.编辑字段失败", "error", err, "name", name)
		return fmt.Errorf("update field: %w", err)
	}

	// reference 类型：同步引用关系
	var refAffected []string
	if req.Type == model.FieldTypeReference {
		oldProps, _ := parseProperties(old.Properties)
		newProps, _ := parseProperties(req.Properties)
		var oldRefs, newRefs []string
		if oldProps != nil && old.Type == model.FieldTypeReference {
			oldRefs = parseRefFields(oldProps.Constraints)
		}
		if newProps != nil {
			newRefs = parseRefFields(newProps.Constraints)
		}
		affected, err := s.syncFieldRefs(ctx, name, oldRefs, newRefs)
		if err != nil {
			slog.Error("service.编辑字段-同步引用失败", "error", err, "name", name)
			return fmt.Errorf("sync field refs: %w", err)
		}
		refAffected = affected
	} else if old.Type == model.FieldTypeReference && req.Type != model.FieldTypeReference {
		// 类型从 reference 改为其他：清除所有引用关系
		oldProps, _ := parseProperties(old.Properties)
		if oldProps != nil {
			oldRefs := parseRefFields(oldProps.Constraints)
			affected, err := s.syncFieldRefs(ctx, name, oldRefs, nil)
			if err != nil {
				slog.Error("service.编辑字段-清除引用失败", "error", err, "name", name)
				return fmt.Errorf("clear field refs: %w", err)
			}
			refAffected = affected
		}
	}

	// 清缓存：自身 + 受影响的被引用方
	s.fieldCache.DelDetail(ctx, name)
	for _, n := range refAffected {
		s.fieldCache.DelDetail(ctx, n)
	}
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.编辑字段成功", "name", name)
	return nil
}

// DeleteResult 删除结果
type DeleteResult struct {
	Deleted    bool             `json:"deleted"`
	References []model.FieldRef `json:"references,omitempty"`
}

// Delete 删除字段（硬约束：被引用时禁止删除）
func (s *FieldService) Delete(ctx context.Context, name string) (*DeleteResult, error) {
	// 先查字段是否存在（事务外，快速失败）
	field, err := s.getFieldOrNotFound(ctx, name)
	if err != nil {
		return nil, err
	}

	// 硬约束：必须先停用才能删除
	if field.Enabled {
		return nil, errcode.New(errcode.ErrFieldDeleteNotDisabled)
	}

	// 先查引用详情（事务外，给前端展示用）
	refs, err := s.fieldRefStore.GetByFieldName(ctx, name)
	if err != nil {
		slog.Error("service.删除字段-查引用失败", "error", err, "name", name)
		return nil, fmt.Errorf("get refs: %w", err)
	}
	if len(refs) > 0 {
		return &DeleteResult{Deleted: false, References: refs}, errcode.New(errcode.ErrFieldRefDelete)
	}

	// 事务内原子操作：再次检查引用 + 软删除（防 TOCTOU）
	tx, err := s.fieldStore.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	hasRefs, err := s.fieldRefStore.HasRefsTx(ctx, tx, name)
	if err != nil {
		return nil, fmt.Errorf("check refs in tx: %w", err)
	}
	if hasRefs {
		return &DeleteResult{Deleted: false}, errcode.New(errcode.ErrFieldRefDelete)
	}

	if err := s.fieldStore.SoftDeleteTx(ctx, tx, name); err != nil {
		if errors.Is(err, storemysql.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrFieldNotFound, "字段 '%s' 不存在", name)
		}
		return nil, fmt.Errorf("soft delete: %w", err)
	}

	// reference 类型字段删除时，清除它对其他字段的引用关系
	var affectedFields []string
	if field.Type == model.FieldTypeReference {
		var err error
		affectedFields, err = s.fieldRefStore.RemoveByRef(ctx, tx, model.RefTypeField, name)
		if err != nil {
			return nil, fmt.Errorf("remove field refs: %w", err)
		}
		for _, affectedName := range affectedFields {
			if err := s.fieldStore.DecrRefCount(ctx, tx, affectedName); err != nil {
				return nil, fmt.Errorf("decr ref_count %s: %w", affectedName, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 清缓存（事务提交后）
	s.fieldCache.DelDetail(ctx, name)
	for _, affectedName := range affectedFields {
		s.fieldCache.DelDetail(ctx, affectedName)
	}
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.删除字段成功", "name", name)
	return &DeleteResult{Deleted: true}, nil
}

// CheckName 校验字段标识是否可用
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
func (s *FieldService) GetReferences(ctx context.Context, name string) (*model.ReferenceDetail, error) {
	field, err := s.getFieldOrNotFound(ctx, name)
	if err != nil {
		return nil, err
	}

	refs, err := s.fieldRefStore.GetByFieldName(ctx, name)
	if err != nil {
		slog.Error("service.引用详情-查引用失败", "error", err, "name", name)
		return nil, fmt.Errorf("get refs: %w", err)
	}

	templateNames := make([]string, 0)
	fieldNames := make([]string, 0)
	for _, r := range refs {
		switch r.RefType {
		case model.RefTypeTemplate:
			templateNames = append(templateNames, r.RefName)
		case model.RefTypeField:
			fieldNames = append(fieldNames, r.RefName)
		}
	}

	result := &model.ReferenceDetail{
		FieldName:  name,
		FieldLabel: field.Label,
		Templates:  make([]model.ReferenceItem, 0, len(templateNames)),
		Fields:     make([]model.ReferenceItem, 0, len(fieldNames)),
	}

	if len(fieldNames) > 0 {
		fieldList, err := s.fieldStore.GetByNames(ctx, fieldNames)
		if err != nil {
			slog.Error("service.引用详情-查字段label失败", "error", err)
			return nil, fmt.Errorf("get field labels: %w", err)
		}
		labelMap := make(map[string]string, len(fieldList))
		for _, f := range fieldList {
			labelMap[f.Name] = f.Label
		}
		for _, n := range fieldNames {
			result.Fields = append(result.Fields, model.ReferenceItem{
				RefType: model.RefTypeField,
				RefName: n,
				Label:   labelMap[n],
			})
		}
	}

	for _, n := range templateNames {
		result.Templates = append(result.Templates, model.ReferenceItem{
			RefType: model.RefTypeTemplate,
			RefName: n,
			Label:   n, // TODO: 模板管理完成后 IN 查 templates 拿 label
		})
	}

	return result, nil
}

// BatchDelete 批量删除
func (s *FieldService) BatchDelete(ctx context.Context, names []string) (*model.BatchDeleteResult, error) {
	fields, err := s.fieldStore.GetByNames(ctx, names)
	if err != nil {
		slog.Error("service.批量删除-查字段失败", "error", err)
		return nil, fmt.Errorf("get fields: %w", err)
	}
	fieldMap := make(map[string]*model.Field, len(fields))
	for i := range fields {
		fieldMap[fields[i].Name] = &fields[i]
	}

	deleted := make([]string, 0)
	skipped := make([]model.BatchDeleteSkipped, 0)
	affectedFields := make([]string, 0) // 被引用方（需清缓存）

	for _, name := range names {
		field := fieldMap[name]
		if field == nil {
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Reason: "字段不存在"})
			continue
		}

		if field.Enabled {
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "请先停用再删除"})
			continue
		}

		hasRefs, err := s.fieldRefStore.HasRefs(ctx, name)
		if err != nil {
			slog.Error("service.批量删除-查引用失败", "error", err, "name", name)
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "查询引用失败"})
			continue
		}
		if hasRefs {
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "被引用无法删除"})
			continue
		}

		// 事务内原子操作：FOR SHARE 重新检查引用 + 软删除（防 TOCTOU）
		tx, err := s.fieldStore.DB().BeginTxx(ctx, nil)
		if err != nil {
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "事务启动失败"})
			continue
		}

		hasRefsTx, err := s.fieldRefStore.HasRefsTx(ctx, tx, name)
		if err != nil {
			tx.Rollback()
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "查询引用失败"})
			continue
		}
		if hasRefsTx {
			tx.Rollback()
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "被引用无法删除"})
			continue
		}

		if err := s.fieldStore.SoftDeleteTx(ctx, tx, name); err != nil {
			tx.Rollback()
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "删除失败"})
			continue
		}

		if field.Type == model.FieldTypeReference {
			affected, err := s.fieldRefStore.RemoveByRef(ctx, tx, model.RefTypeField, name)
			if err != nil {
				tx.Rollback()
				skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "清理引用失败"})
				continue
			}
			decrFailed := false
			for _, affectedName := range affected {
				if err := s.fieldStore.DecrRefCount(ctx, tx, affectedName); err != nil {
					tx.Rollback()
					skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "更新引用计数失败"})
					decrFailed = true
					break
				}
			}
			if decrFailed {
				continue
			}
			affectedFields = append(affectedFields, affected...)
		}

		if err := tx.Commit(); err != nil {
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: field.Label, Reason: "提交失败"})
			continue
		}

		deleted = append(deleted, name)
	}

	msg := fmt.Sprintf("%d 项已删除", len(deleted))
	if len(skipped) > 0 {
		msg += fmt.Sprintf("，%d 项因被引用无法删除", len(skipped))
	}

	// 清缓存
	if len(deleted) > 0 {
		for _, name := range deleted {
			s.fieldCache.DelDetail(ctx, name)
		}
		for _, name := range affectedFields {
			s.fieldCache.DelDetail(ctx, name)
		}
		s.fieldCache.InvalidateList(ctx)
	}

	slog.Info("service.批量删除完成", "deleted", len(deleted), "skipped", len(skipped))
	return &model.BatchDeleteResult{Deleted: deleted, Skipped: skipped, Message: msg}, nil
}

// BatchUpdateCategory 批量修改分类
func (s *FieldService) BatchUpdateCategory(ctx context.Context, req *model.BatchCategoryRequest) (int64, error) {
	// 业务校验：分类存在性
	if err := s.checkCategoryExists(req.Category); err != nil {
		return 0, err
	}

	affected, err := s.fieldStore.BatchUpdateCategory(ctx, req.Names, req.Category)
	if err != nil {
		slog.Error("service.批量修改分类失败", "error", err)
		return 0, fmt.Errorf("batch update category: %w", err)
	}

	// 清缓存（detail + list 都要清）
	for _, name := range req.Names {
		s.fieldCache.DelDetail(ctx, name)
	}
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.批量修改分类成功", "affected", affected, "category", req.Category)
	return affected, nil
}

// ToggleEnabled 切换启用/停用
func (s *FieldService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	if _, err := s.getFieldOrNotFound(ctx, req.Name); err != nil {
		return err
	}

	err := s.fieldStore.ToggleEnabled(ctx, req.Name, req.Enabled, req.Version)
	if err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrFieldVersionConflict)
		}
		slog.Error("service.切换启用失败", "error", err, "name", req.Name)
		return fmt.Errorf("toggle enabled: %w", err)
	}

	s.fieldCache.DelDetail(ctx, req.Name)
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.切换启用成功", "name", req.Name, "enabled", req.Enabled)
	return nil
}

// ---- 约束收紧检查 ----

// parseProperties 解析 properties JSON
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

// parseConstraintsMap 解析 constraints 为通用 map
func parseConstraintsMap(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return make(map[string]json.RawMessage), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal constraints: %w", err)
	}
	return m, nil
}

// getFloat 从 json.RawMessage 中提取数值
func getFloat(raw json.RawMessage) (float64, bool) {
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}

// checkConstraintTightened 检查约束是否被收紧
// 返回 nil 表示未收紧（放宽或不变），返回 error 表示收紧了
func checkConstraintTightened(fieldType string, oldConstraints, newConstraints json.RawMessage) *errcode.Error {
	oldMap, err := parseConstraintsMap(oldConstraints)
	if err != nil {
		return nil // 旧数据解析失败，跳过检查
	}
	newMap, err := parseConstraintsMap(newConstraints)
	if err != nil {
		return nil
	}

	switch fieldType {
	case "integer", "float":
		// min 只能减小或不变，max 只能增大或不变
		if oldMin, ok := getFloat(oldMap["min"]); ok {
			if newMin, ok2 := getFloat(newMap["min"]); ok2 && newMin > oldMin {
				return errcode.Newf(errcode.ErrFieldRefTighten, "最小值从 %v 收紧为 %v，请先移除引用", oldMin, newMin)
			}
		}
		if oldMax, ok := getFloat(oldMap["max"]); ok {
			if newMax, ok2 := getFloat(newMap["max"]); ok2 && newMax < oldMax {
				return errcode.Newf(errcode.ErrFieldRefTighten, "最大值从 %v 收紧为 %v，请先移除引用", oldMax, newMax)
			}
		}

	case "string":
		// minLength 只能减小，maxLength 只能增大
		if oldMinLen, ok := getFloat(oldMap["minLength"]); ok {
			if newMinLen, ok2 := getFloat(newMap["minLength"]); ok2 && newMinLen > oldMinLen {
				return errcode.Newf(errcode.ErrFieldRefTighten, "最小长度从 %v 收紧为 %v，请先移除引用", oldMinLen, newMinLen)
			}
		}
		if oldMaxLen, ok := getFloat(oldMap["maxLength"]); ok {
			if newMaxLen, ok2 := getFloat(newMap["maxLength"]); ok2 && newMaxLen < oldMaxLen {
				return errcode.Newf(errcode.ErrFieldRefTighten, "最大长度从 %v 收紧为 %v，请先移除引用", oldMaxLen, newMaxLen)
			}
		}

	case "select":
		// 只能新增选项，不能删除已有选项
		oldOptions := parseSelectOptions(oldMap["options"])
		newOptions := parseSelectOptions(newMap["options"])
		if len(oldOptions) > 0 {
			newSet := make(map[string]bool, len(newOptions))
			for _, o := range newOptions {
				newSet[o] = true
			}
			for _, o := range oldOptions {
				if !newSet[o] {
					return errcode.Newf(errcode.ErrFieldRefTighten, "选项 '%s' 被删除，请先移除引用", o)
				}
			}
		}
	}

	return nil
}

// parseSelectOptions 从 options JSON 中提取选项的 value 列表
func parseSelectOptions(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var options []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil
	}
	values := make([]string, 0, len(options))
	for _, o := range options {
		values = append(values, o.Value)
	}
	return values
}

// ---- 循环引用检测 ----

// parseRefFields 从 reference 字段的 constraints 中提取引用列表
func parseRefFields(constraints json.RawMessage) []string {
	if len(constraints) == 0 {
		return nil
	}
	var c struct {
		Refs []string `json:"refs"`
	}
	if err := json.Unmarshal(constraints, &c); err != nil {
		return nil
	}
	return c.Refs
}

// detectCyclicRef 检测循环引用（DFS）
// currentName: 当前正在创建/编辑的字段名
// refs: 当前字段要引用的字段列表
// 返回 nil 表示无环
func (s *FieldService) detectCyclicRef(ctx context.Context, currentName string, refs []string) *errcode.Error {
	visited := map[string]bool{currentName: true}

	var dfs func(names []string) *errcode.Error
	dfs = func(names []string) *errcode.Error {
		for _, name := range names {
			if visited[name] {
				return errcode.Newf(errcode.ErrFieldCyclicRef, "字段 '%s' 与 '%s' 形成循环引用", currentName, name)
			}
			visited[name] = true

			// 查该字段是否也是 reference 类型，递归展开
			field, err := s.fieldStore.GetByName(ctx, name)
			if err != nil || field == nil {
				continue
			}
			if field.Type != model.FieldTypeReference {
				continue
			}

			props, err := parseProperties(field.Properties)
			if err != nil {
				continue
			}
			subRefs := parseRefFields(props.Constraints)
			if len(subRefs) > 0 {
				if err := dfs(subRefs); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return dfs(refs)
}

// syncFieldRefs 同步 reference 字段的引用关系到 field_refs 表
// 比较新旧引用列表，增删 field_refs 记录并维护 ref_count
// 返回 ref_count 发生变化的字段名列表（用于清缓存）
func (s *FieldService) syncFieldRefs(ctx context.Context, fieldName string, oldRefs, newRefs []string) ([]string, error) {
	oldSet := make(map[string]bool, len(oldRefs))
	for _, r := range oldRefs {
		oldSet[r] = true
	}
	newSet := make(map[string]bool, len(newRefs))
	for _, r := range newRefs {
		newSet[r] = true
	}

	// 计算差集
	toAdd := make([]string, 0)
	toRemove := make([]string, 0)
	for _, r := range newRefs {
		if !oldSet[r] {
			toAdd = append(toAdd, r)
		}
	}
	for _, r := range oldRefs {
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

	for _, refName := range toAdd {
		if err := s.fieldRefStore.Add(ctx, tx, &model.FieldRef{
			FieldName: refName,
			RefType:   model.RefTypeField,
			RefName:   fieldName,
		}); err != nil {
			return nil, fmt.Errorf("add field ref %s: %w", refName, err)
		}
		if err := s.fieldStore.IncrRefCount(ctx, tx, refName); err != nil {
			return nil, fmt.Errorf("incr ref_count %s: %w", refName, err)
		}
	}

	for _, refName := range toRemove {
		if err := s.fieldRefStore.Remove(ctx, tx, &model.FieldRef{
			FieldName: refName,
			RefType:   model.RefTypeField,
			RefName:   fieldName,
		}); err != nil {
			return nil, fmt.Errorf("remove field ref %s: %w", refName, err)
		}
		if err := s.fieldStore.DecrRefCount(ctx, tx, refName); err != nil {
			return nil, fmt.Errorf("decr ref_count %s: %w", refName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 返回所有 ref_count 变化的字段名
	affected := make([]string, 0, len(toAdd)+len(toRemove))
	affected = append(affected, toAdd...)
	affected = append(affected, toRemove...)
	return affected, nil
}
