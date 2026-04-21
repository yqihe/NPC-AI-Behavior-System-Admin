package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// detectCyclicRefImpl 单测：通过注入 fakeFieldLookup 避免 DB。
// 覆盖方法壳 detectCyclicRef 的委托由其他 Create/Update 集成路径间接走到。
// ============================================================

// fakeFieldLookup map-backed FieldStore.GetByID 替身
type fakeFieldLookup struct {
	byID map[int64]*model.Field
	// 可选：返错集合（id → err），模拟 store 层错误
	errByID map[int64]error
}

func (f *fakeFieldLookup) GetByID(_ context.Context, id int64) (*model.Field, error) {
	if err, ok := f.errByID[id]; ok {
		return nil, err
	}
	return f.byID[id], nil
}

// mkRefField 构造 reference 字段（properties.constraints.refs 写 JSON）
func mkRefField(id int64, name string, refs []int64) *model.Field {
	type constraints struct {
		Refs []int64 `json:"refs"`
	}
	props := model.FieldProperties{Constraints: mustJSON(constraints{Refs: refs})}
	return &model.Field{
		ID:         id,
		Name:       name,
		Type:       "reference",
		Enabled:    true,
		Properties: mustJSON(props),
	}
}

// mkScalarField 构造非 reference 字段（DFS 应终止于此）
func mkScalarField(id int64, name, typ string) *model.Field {
	return &model.Field{
		ID:      id,
		Name:    name,
		Type:    typ,
		Enabled: true,
	}
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// --- detectCyclicRefImpl ---

func TestDetectCyclicRef_EmptyRefs(t *testing.T) {
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{}}
	if err := detectCyclicRefImpl(context.Background(), lookup, 0, nil); err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDetectCyclicRef_LinearChainNoCycle(t *testing.T) {
	// A(currentID=1) 要引用 [2]; 2 是 reference 引用 [3]; 3 是 scalar 终止
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkRefField(2, "b", []int64{3}),
		3: mkScalarField(3, "c", "integer"),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDetectCyclicRef_SelfRefViaCurrentID(t *testing.T) {
	// 编辑 ID=1 时试图把自己加入 refs
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		1: mkRefField(1, "a", nil),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{1})
	if err == nil {
		t.Fatal("want cyclic err, got nil")
	}
	if err.Code != errcode.ErrFieldCyclicRef {
		t.Errorf("want code=%d, got code=%d", errcode.ErrFieldCyclicRef, err.Code)
	}
}

func TestDetectCyclicRef_TwoNodeCycle(t *testing.T) {
	// 1 → 2 → 1 （编辑 1 时加入 2，2 又引用 1）
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		1: mkRefField(1, "a", []int64{2}),
		2: mkRefField(2, "b", []int64{1}),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2})
	if err == nil || err.Code != errcode.ErrFieldCyclicRef {
		t.Fatalf("want cyclic err, got %v", err)
	}
}

func TestDetectCyclicRef_ThreeNodeCycle(t *testing.T) {
	// 1 → 2 → 3 → 1
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		1: mkRefField(1, "a", []int64{2}),
		2: mkRefField(2, "b", []int64{3}),
		3: mkRefField(3, "c", []int64{1}),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2})
	if err == nil || err.Code != errcode.ErrFieldCyclicRef {
		t.Fatalf("want cyclic err, got %v", err)
	}
}

func TestDetectCyclicRef_CreateNoCurrentID(t *testing.T) {
	// Create 路径：currentID=0，不把任何 id 预先加入 visited
	// 2 → 3 → 2 （2-3 互循环，与新字段无关也要检出）
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkRefField(2, "b", []int64{3}),
		3: mkRefField(3, "c", []int64{2}),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 0, []int64{2})
	if err == nil || err.Code != errcode.ErrFieldCyclicRef {
		t.Fatalf("want cyclic err, got %v", err)
	}
}

func TestDetectCyclicRef_NonRefTargetStopsDFS(t *testing.T) {
	// 2 不是 reference，DFS 到此就停；即使 3 也有 "2" 的循环也不会误触
	// (此场景实际构造不出，目的只是验证非-reference 早退分支被走到)
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkScalarField(2, "b", "string"),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2})
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
}

func TestDetectCyclicRef_LookupErrorSilentlyContinues(t *testing.T) {
	// store 层失败 → 按现有行为容错（continue），不冒泡。
	// 但 visited[2]=true 已记录；若 [2] 再出现一次仍能检出循环。
	lookup := &fakeFieldLookup{
		byID: map[int64]*model.Field{},
		errByID: map[int64]error{
			2: errors.New("db down"),
		},
	}
	// 单次出现 err → 容错
	if err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2}); err != nil {
		t.Errorf("store 错应被容错, got %v", err)
	}
	// 同一批 [2,2] → 第二次命中 visited 触发 cyclic
	err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2, 2})
	if err == nil || err.Code != errcode.ErrFieldCyclicRef {
		t.Errorf("want cyclic (重复 ID), got %v", err)
	}
}

func TestDetectCyclicRef_InvalidPropertiesJSONContinues(t *testing.T) {
	// 字段 Properties 是非法 JSON → parseProperties 失败 → continue（不递归但不报错）
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: {
			ID:         2,
			Name:       "b",
			Type:       "reference",
			Enabled:    true,
			Properties: json.RawMessage(`{not-json}`),
		},
	}}
	if err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{2}); err != nil {
		t.Errorf("非法 properties 应被容错, got %v", err)
	}
}

func TestDetectCyclicRef_FieldNotFoundContinues(t *testing.T) {
	// lookup 返回 nil（不存在）→ continue，不影响其他分支
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{}} // 全空
	if err := detectCyclicRefImpl(context.Background(), lookup, 1, []int64{99}); err != nil {
		t.Errorf("字段不存在应被容错, got %v", err)
	}
}

func TestDetectCyclicRef_DuplicateInRefsDetected(t *testing.T) {
	// 同一 refIDs 切片里重复出现 → 第二次命中 visited
	lookup := &fakeFieldLookup{byID: map[int64]*model.Field{
		2: mkScalarField(2, "b", "integer"),
	}}
	err := detectCyclicRefImpl(context.Background(), lookup, 0, []int64{2, 2})
	if err == nil || err.Code != errcode.ErrFieldCyclicRef {
		t.Fatalf("want cyclic (duplicate id), got %v", err)
	}
}
