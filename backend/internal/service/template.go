package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

// TemplateService 模板管理业务逻辑
//
// 严格遵守"分层职责"硬规则：只持有自己模块的 store/cache，
// 不持有 fieldStore / fieldRefStore / fieldCache / dictCache。
// 跨模块编排（写 field_refs / 调字段补全 / 清字段缓存）由 handler 层负责。
type TemplateService struct {
	store  *storemysql.TemplateStore
	cache  *storeredis.TemplateCache
	pagCfg *config.PaginationConfig
}

// NewTemplateService 创建 TemplateService
func NewTemplateService(store *storemysql.TemplateStore, cache *storeredis.TemplateCache, pagCfg *config.PaginationConfig) *TemplateService {
	return &TemplateService{
		store:  store,
		cache:  cache,
		pagCfg: pagCfg,
	}
}

// DB 暴露数据库连接（handler 层开跨模块事务用）
func (s *TemplateService) DB() *sqlx.DB {
	return s.store.DB()
}

// ---- 业务校验辅助 ----

// validateFieldsBasic 模板自身的 fields 数组基础校验
//
//	非空 + field_id > 0 + 不重复
//
// 注意：字段存在性 / 启用性校验属于"字段管理模块"的职责，
// 由 handler 层调用 fieldService.ValidateFieldsForTemplate 完成。
func (s *TemplateService) validateFieldsBasic(fields []model.TemplateFieldEntry) error {
	if len(fields) == 0 {
		return errcode.New(errcode.ErrTemplateNoFields)
	}
	seen := make(map[int64]bool, len(fields))
	for _, f := range fields {
		if f.FieldID <= 0 {
			return errcode.Newf(errcode.ErrBadRequest, "字段 ID 必须 > 0")
		}
		if seen[f.FieldID] {
			return errcode.Newf(errcode.ErrBadRequest, "字段 ID %d 重复", f.FieldID)
		}
		seen[f.FieldID] = true
	}
	return nil
}

// getTemplateOrNotFound 按 ID 查模板 + 判空
func (s *TemplateService) getTemplateOrNotFound(ctx context.Context, id int64) (*model.Template, error) {
	tpl, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get template %d: %w", id, err)
	}
	if tpl == nil {
		return nil, errcode.Newf(errcode.ErrTemplateNotFound, "模板 ID=%d 不存在", id)
	}
	return tpl, nil
}

// ParseFieldEntries 解析 templates.fields JSON 列
//
// 公开方法供 handler 层使用：handler 拿到 *model.Template 后需要解 fields
// 来拿 fieldIDs，再调 FieldService.GetByIDsLite 拼装 TemplateDetail。
func (s *TemplateService) ParseFieldEntries(raw json.RawMessage) ([]model.TemplateFieldEntry, error) {
	if len(raw) == 0 {
		return make([]model.TemplateFieldEntry, 0), nil
	}
	var entries []model.TemplateFieldEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal template fields: %w", err)
	}
	if entries == nil {
		entries = make([]model.TemplateFieldEntry, 0)
	}
	return entries, nil
}

// ---- 单模块路径 ----

// List 模板列表（Cache-Aside：Redis → MySQL → 写 Redis）
func (s *TemplateService) List(ctx context.Context, q *model.TemplateListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 1. 查 Redis 缓存
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		return cached.ToListData(), nil
	}

	// 2. 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		slog.Error("service.模板列表查询失败", "error", err, "query", q)
		return nil, err
	}

	result := &model.TemplateListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}

	// 3. 写 Redis 缓存
	s.cache.SetList(ctx, q, result)

	return result.ToListData(), nil
}

// GetByID 查询模板裸行（Cache-Aside + 分布式锁防击穿）
//
// 返回 *model.Template 裸行，不含字段补全。
// 字段补全由 handler 层调 fieldService.GetByIDsLite 完成。
func (s *TemplateService) GetByID(ctx context.Context, id int64) (*model.Template, error) {
	// 1. 查 Redis 缓存
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrTemplateNotFound, "模板 ID=%d 不存在", id)
		}
		return cached, nil
	}

	// 2. 分布式锁防缓存击穿
	lockID, lockErr := s.cache.TryLock(ctx, id, 3*time.Second)
	if lockErr != nil {
		slog.Warn("service.获取模板锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
	}

	// 获得锁后 double-check 缓存
	if lockID != "" {
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.Newf(errcode.ErrTemplateNotFound, "模板 ID=%d 不存在", id)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	tpl, err := s.store.GetByID(ctx, id)
	if err != nil {
		slog.Error("service.查询模板详情失败", "error", err, "id", id)
		return nil, fmt.Errorf("get template: %w", err)
	}

	// 4. 写 Redis（tpl 为 nil 时也缓存空标记，防穿透）
	s.cache.SetDetail(ctx, id, tpl)

	if tpl == nil {
		return nil, errcode.Newf(errcode.ErrTemplateNotFound, "模板 ID=%d 不存在", id)
	}
	return tpl, nil
}

// ExistsByName 校验 name 是否已存在（含软删除）
func (s *TemplateService) ExistsByName(ctx context.Context, name string) (bool, error) {
	return s.store.ExistsByName(ctx, name)
}

// CheckName 校验模板标识是否可用
func (s *TemplateService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验模板名失败", "error", err, "name", name)
		return nil, fmt.Errorf("check template name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该模板标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// ToggleEnabled 切换启用/停用（单模块写 + 乐观锁 + 清缓存）
func (s *TemplateService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	if _, err := s.getTemplateOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	err := s.store.ToggleEnabled(ctx, req.ID, req.Enabled, req.Version)
	if err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrTemplateVersionConflict)
		}
		slog.Error("service.切换模板启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle template enabled: %w", err)
	}

	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换模板启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// ---- 跨模块路径（接收外部 tx）----

// CreateTx 事务内创建模板
//
// service 层做：
//   - fields 基础校验（非空 / field_id > 0 / 不重复）→ 41004 / ErrBadRequest
//   - name 唯一性校验（含软删除）→ 41001
//
// 不做：
//   - 字段存在性 / 启用性校验（属字段管理模块，handler 调 FieldService）
//   - field_refs 写入（属字段管理模块）
func (s *TemplateService) CreateTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateTemplateRequest) (int64, error) {
	if err := s.validateFieldsBasic(req.Fields); err != nil {
		return 0, err
	}

	// name 唯一性（在事务外预查，事务内不再做）
	// 注意：理论上有 TOCTOU 风险（预查后到 INSERT 之间被插入），
	// 但 uk_name 唯一约束会在 INSERT 时拦截重复，service 把 driver 的
	// duplicate key 错误转成 41001 即可。这里先做预查给前端友好提示。
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, fmt.Errorf("check template name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrTemplateNameExists, "模板标识 '%s' 已存在", req.Name)
	}

	// 序列化 fields JSON
	fieldsJSON, err := json.Marshal(req.Fields)
	if err != nil {
		return 0, fmt.Errorf("marshal template fields: %w", err)
	}

	id, err := s.store.CreateTx(ctx, tx, req, fieldsJSON)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrTemplateNameExists, "模板标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建模板失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create template: %w", err)
	}

	slog.Info("service.创建模板成功", "id", id, "name", req.Name)
	return id, nil
}

// UpdateTx 事务内编辑模板
//
// 调用方（handler）必须先调 GetByID 拿到旧 tpl 并解析 oldEntries 传入。
// service 层做：
//   - fields 基础校验
//   - enabled 状态前置校验 → 41010
//   - 乐观锁错误转换 → 41011
//
// 不做：字段存在性/启用性校验、field_refs 写入。
func (s *TemplateService) UpdateTx(
	ctx context.Context,
	tx *sqlx.Tx,
	req *model.UpdateTemplateRequest,
	old *model.Template,
	oldEntries []model.TemplateFieldEntry,
) (fieldsChanged bool, toAdd []int64, toRemove []int64, err error) {
	if err = s.validateFieldsBasic(req.Fields); err != nil {
		return false, nil, nil, err
	}

	// 必须未启用才能编辑
	if old.Enabled {
		return false, nil, nil, errcode.New(errcode.ErrTemplateEditNotDisabled)
	}

	// diff fields
	fieldsChanged = isFieldsChanged(oldEntries, req.Fields)

	// 计算 toAdd / toRemove（仅字段集合维度，required-only 变化也归到 fieldsChanged 但不会有 add/remove）
	if fieldsChanged {
		toAdd, toRemove = diffFieldIDs(oldEntries, req.Fields)
	}

	// 序列化新 fields JSON
	fieldsJSON, err := json.Marshal(req.Fields)
	if err != nil {
		return false, nil, nil, fmt.Errorf("marshal template fields: %w", err)
	}

	if err = s.store.UpdateTx(ctx, tx, req, fieldsJSON); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return false, nil, nil, errcode.New(errcode.ErrTemplateVersionConflict)
		}
		slog.Error("service.编辑模板失败", "error", err, "id", req.ID)
		return false, nil, nil, fmt.Errorf("update template: %w", err)
	}

	slog.Info("service.编辑模板成功", "id", req.ID, "fields_changed", fieldsChanged)
	return fieldsChanged, toAdd, toRemove, nil
}

// SoftDeleteTx 事务内软删除模板
//
// 调用方（handler）必须先调 GetByID 校验存在 + enabled=0。
func (s *TemplateService) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	if err := s.store.SoftDeleteTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return errcode.Newf(errcode.ErrTemplateNotFound, "模板 ID=%d 不存在", id)
		}
		slog.Error("service.软删除模板失败", "error", err, "id", id)
		return fmt.Errorf("soft delete template: %w", err)
	}
	slog.Info("service.软删除模板成功", "id", id)
	return nil
}

// ---- 缓存失效（跨模块编排 commit 后由 handler 调用）----

// InvalidateDetail 清单条模板缓存
func (s *TemplateService) InvalidateDetail(ctx context.Context, id int64) {
	s.cache.DelDetail(ctx, id)
}

// InvalidateList 清模板列表缓存
func (s *TemplateService) InvalidateList(ctx context.Context) {
	s.cache.InvalidateList(ctx)
}

// ---- 跨模块对外查询接口 ----

// GetByIDsLite 批量查模板精简信息
//
// 给字段管理 handler 跨模块编排时补 template label 用。
func (s *TemplateService) GetByIDsLite(ctx context.Context, ids []int64) ([]model.TemplateLite, error) {
	return s.store.GetByIDs(ctx, ids)
}

// ---- 内部 diff 算法 ----

// isFieldsChanged 模板 fields 是否变更
//
// 集合 + 顺序 + required 任一不同都视为变更。
func isFieldsChanged(old, new []model.TemplateFieldEntry) bool {
	if len(old) != len(new) {
		return true
	}
	for i := range old {
		if old[i].FieldID != new[i].FieldID {
			return true
		}
		if old[i].Required != new[i].Required {
			return true
		}
	}
	return false
}

// diffFieldIDs 计算字段集合的增删（顺序变化但集合相同时返回空切片）
func diffFieldIDs(old, new []model.TemplateFieldEntry) (toAdd, toRemove []int64) {
	oldSet := make(map[int64]bool, len(old))
	for _, e := range old {
		oldSet[e.FieldID] = true
	}
	newSet := make(map[int64]bool, len(new))
	for _, e := range new {
		newSet[e.FieldID] = true
	}
	toAdd = make([]int64, 0)
	toRemove = make([]int64, 0)
	for _, e := range new {
		if !oldSet[e.FieldID] {
			toAdd = append(toAdd, e.FieldID)
		}
	}
	for _, e := range old {
		if !newSet[e.FieldID] {
			toRemove = append(toRemove, e.FieldID)
		}
	}
	return toAdd, toRemove
}
