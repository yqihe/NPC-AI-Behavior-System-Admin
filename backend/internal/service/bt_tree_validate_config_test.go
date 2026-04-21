package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// validateConfigImpl 单测：注入 fakeBtNodeTypeLookup 避免 DB。
// 核心校验（validateBtNode 及分支）已在 bt_tree_test.go 覆盖，此处
// 只补 validateConfigImpl 自身的 7 条分支：empty / store 错（两处）/
// param_schema 非法 / config 非法 JSON / happy / 带 params 的 happy。
// ============================================================

type fakeBtNodeTypeLookup struct {
	types      map[string]string          // enabled type_name → category
	schemas    map[string]json.RawMessage // type_name → param_schema JSON
	typesErr   error
	schemasErr error
}

func (f *fakeBtNodeTypeLookup) ListEnabledTypes(_ context.Context) (map[string]string, error) {
	if f.typesErr != nil {
		return nil, f.typesErr
	}
	return f.types, nil
}

func (f *fakeBtNodeTypeLookup) ListParamSchemas(_ context.Context) (map[string]json.RawMessage, error) {
	if f.schemasErr != nil {
		return nil, f.schemasErr
	}
	return f.schemas, nil
}

func TestValidateConfigImpl_EmptyConfig(t *testing.T) {
	lookup := &fakeBtNodeTypeLookup{}
	err := validateConfigImpl(context.Background(), lookup, nil)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T", err)
	}
	if codeErr.Code != errcode.ErrBtTreeConfigInvalid {
		t.Errorf("want code=%d, got code=%d", errcode.ErrBtTreeConfigInvalid, codeErr.Code)
	}
}

func TestValidateConfigImpl_ListEnabledTypesError(t *testing.T) {
	lookup := &fakeBtNodeTypeLookup{typesErr: errors.New("db down")}
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`{"type":"x"}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	// 非 *errcode.Error，而是包装后的 fmt.Errorf
	var codeErr *errcode.Error
	if errors.As(err, &codeErr) {
		t.Errorf("不应是 *errcode.Error, got %v", codeErr)
	}
	if !strings.Contains(err.Error(), "load enabled node types") {
		t.Errorf("err 应含 'load enabled node types' 前缀, got %q", err.Error())
	}
}

func TestValidateConfigImpl_ListParamSchemasError(t *testing.T) {
	lookup := &fakeBtNodeTypeLookup{
		types:      map[string]string{},
		schemasErr: errors.New("db down"),
	}
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`{"type":"x"}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "load param schemas") {
		t.Errorf("err 应含 'load param schemas' 前缀, got %q", err.Error())
	}
}

func TestValidateConfigImpl_ParamSchemaInvalidJSONFailFast(t *testing.T) {
	// seed 损坏：某个 type 的 param_schema 不是合法 JSON → fail-fast
	lookup := &fakeBtNodeTypeLookup{
		types: map[string]string{"x": "leaf"},
		schemas: map[string]json.RawMessage{
			"x": json.RawMessage(`{not-json}`),
		},
	}
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`{"type":"x"}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal param_schema") {
		t.Errorf("err 应含 'unmarshal param_schema' 前缀, got %q", err.Error())
	}
}

func TestValidateConfigImpl_ConfigNotJSONObject(t *testing.T) {
	lookup := &fakeBtNodeTypeLookup{
		types:   map[string]string{"x": "leaf"},
		schemas: map[string]json.RawMessage{},
	}
	// 非对象 JSON（数组）
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`["not-object"]`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T", err)
	}
	if codeErr.Code != errcode.ErrBtTreeConfigInvalid {
		t.Errorf("want code=%d, got code=%d", errcode.ErrBtTreeConfigInvalid, codeErr.Code)
	}
}

func TestValidateConfigImpl_UnknownNodeType(t *testing.T) {
	// 顶层节点 type 不在 nodeTypes → ErrBtTreeNodeTypeNotFound
	lookup := &fakeBtNodeTypeLookup{
		types:   map[string]string{}, // 空白白名单
		schemas: map[string]json.RawMessage{},
	}
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`{"type":"ghost"}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T", err)
	}
	if codeErr.Code != errcode.ErrBtTreeNodeTypeNotFound {
		t.Errorf("want code=%d, got code=%d", errcode.ErrBtTreeNodeTypeNotFound, codeErr.Code)
	}
}

func TestValidateConfigImpl_LeafHappyNoParams(t *testing.T) {
	// 合法 leaf 节点，无 params
	lookup := &fakeBtNodeTypeLookup{
		types: map[string]string{"wait_idle": "leaf"},
		schemas: map[string]json.RawMessage{
			"wait_idle": json.RawMessage(`{"params":[]}`),
		},
	}
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`{"type":"wait_idle"}`))
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidateConfigImpl_CompositeWithParamsHappy(t *testing.T) {
	// 合法 composite + leaf 带 bb_key 参数
	lookup := &fakeBtNodeTypeLookup{
		types: map[string]string{
			"sequence":      "composite",
			"check_bb_float": "leaf",
		},
		schemas: map[string]json.RawMessage{
			"sequence": json.RawMessage(`{"params":[]}`),
			"check_bb_float": json.RawMessage(`{"params":[
				{"name":"key","label":"BB key","type":"bb_key","required":true},
				{"name":"op","label":"op","type":"select","required":true,"options":[">","<","=="]},
				{"name":"value","label":"值","type":"float","required":true}
			]}`),
		},
	}
	cfg := json.RawMessage(`{
		"type":"sequence",
		"children":[
			{"type":"check_bb_float","params":{"key":"hp","op":">","value":50}}
		]
	}`)
	err := validateConfigImpl(context.Background(), lookup, cfg)
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidateConfigImpl_EmptySchemasMapAllowed(t *testing.T) {
	// 合法场景：ListParamSchemas 返回空 map（比如所有节点类型都无 params）
	lookup := &fakeBtNodeTypeLookup{
		types:   map[string]string{"leaf_x": "leaf"},
		schemas: nil, // nil 也应正常处理（range nil map 0 次）
	}
	err := validateConfigImpl(context.Background(), lookup, json.RawMessage(`{"type":"leaf_x"}`))
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}
