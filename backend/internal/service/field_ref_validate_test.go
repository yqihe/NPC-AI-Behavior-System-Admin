package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// validateReferenceRefsImpl 单测：共用 fakeFieldLookup / mkRefField /
// mkScalarField（定义在 field_cyclic_test.go）。
// ============================================================

// mkDisabledScalarField 构造 disabled 的非-reference 字段
func mkDisabledScalarField(id int64, name, typ string) *model.Field {
	return &model.Field{ID: id, Name: name, Type: typ, Enabled: false}
}

// asErrCode 抽出 *errcode.Error 并断言 code；不匹配时 t.Fatal
func asErrCode(t *testing.T, err error, wantCode int) {
	t.Helper()
	if err == nil {
		t.Fatalf("want err code=%d, got nil", wantCode)
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T: %v", err, err)
	}
	if codeErr.Code != wantCode {
		t.Fatalf("want code=%d, got code=%d (msg=%q)", wantCode, codeErr.Code, codeErr.Message)
	}
}

func TestValidateReferenceRefs_EmptyRefsRejected(t *testing.T) {
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, nil, nil)
	asErrCode(t, err, errcode.ErrFieldRefEmpty)
}

func TestValidateReferenceRefs_RefNotFound(t *testing.T) {
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{}} // 完全空
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{99}, nil)
	asErrCode(t, err, errcode.ErrFieldRefNotFound)
}

func TestValidateReferenceRefs_NewRefDisabled(t *testing.T) {
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkDisabledScalarField(2, "legacy_hp", "integer"),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, nil)
	asErrCode(t, err, errcode.ErrFieldRefDisabled)
}

func TestValidateReferenceRefs_OldRefDisabledGrandfathered(t *testing.T) {
	// 存量不动：旧 ref 即使 disabled 也放行
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkDisabledScalarField(2, "legacy_hp", "integer"),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, map[int64]bool{2: true})
	if err != nil {
		t.Fatalf("存量 ref 禁用应放行, got %v", err)
	}
}

func TestValidateReferenceRefs_NestedReferenceRejected(t *testing.T) {
	// 新增 ref 指向 reference 类型 → 禁止嵌套
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkRefField(2, "inner_ref", nil),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, nil)
	asErrCode(t, err, errcode.ErrFieldRefNested)
}

func TestValidateReferenceRefs_OldNestedGrandfathered(t *testing.T) {
	// 存量 ref 即使目标类型变成 reference 也放行
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkRefField(2, "was_scalar_now_ref", nil),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, map[int64]bool{2: true})
	if err != nil {
		t.Fatalf("存量 ref 变成 reference 应放行, got %v", err)
	}
}

func TestValidateReferenceRefs_CyclicPropagates(t *testing.T) {
	// 结构校验通过后末尾循环检测被命中
	// 1（编辑）→ 2（reference） → 1（指回自己）
	// 但为了走到循环检测分支，2 不能在第一阶段 ErrFieldRefNested 被拦
	// → 必须让 2 作为 oldRef 放行，再由 detectCyclicRef 检出
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkRefField(2, "b", []int64{1}),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, map[int64]bool{2: true})
	asErrCode(t, err, errcode.ErrFieldCyclicRef)
}

func TestValidateReferenceRefs_LookupErrorWrapped(t *testing.T) {
	// store 层出错 → 非 *errcode.Error 包装返出（区别于 detectCyclicRef 的容错策略）
	lookup := &fakeFieldLookup{
		byID:    map[int64]*model.Field{},
		errByID: map[int64]error{2: errors.New("db connection lost")},
	}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, nil)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if errors.As(err, &codeErr) {
		t.Errorf("不应是 *errcode.Error, got code=%d", codeErr.Code)
	}
	if !strings.Contains(err.Error(), "check ref field") {
		t.Errorf("err 应含 'check ref field' 前缀, got %q", err.Error())
	}
}

func TestValidateReferenceRefs_AllValid(t *testing.T) {
	// 全新 refs 都是 enabled + 非-reference + 无循环
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkScalarField(2, "hp", "integer"),
		3: mkScalarField(3, "mp", "integer"),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 0, []int64{2, 3}, nil)
	if err != nil {
		t.Fatalf("全合法应 nil, got %v", err)
	}
}

func TestValidateReferenceRefs_NilOldRefSetTreatedAsAllNew(t *testing.T) {
	// oldRefSet=nil 等价于"所有 ref 都是新增" → disabled 命中
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkDisabledScalarField(2, "legacy", "integer"),
	}}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2}, nil)
	asErrCode(t, err, errcode.ErrFieldRefDisabled)
}

func TestValidateReferenceRefs_FirstRefFailsStopsEarly(t *testing.T) {
	// 多个 refs 里第一个就 not-found，不应继续
	// 通过 errByID 做"未调用"哨兵：给 id=3 设 err，若被调用即 err
	lookup := &fakeFieldLookup{
		byID:    map[int64]*model.Field{}, // 2 不存在
		errByID: map[int64]error{3: errors.New("should not be called")},
	}
	err := validateReferenceRefsImpl(context.Background(), lookup, 1, []int64{2, 3}, nil)
	asErrCode(t, err, errcode.ErrFieldRefNotFound)
	// 若 err 是 ErrFieldRefNotFound 说明在 id=2 就返了，没继续到 id=3
}
