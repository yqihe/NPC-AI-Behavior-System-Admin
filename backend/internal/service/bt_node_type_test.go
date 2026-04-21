package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// validateParamSchema 单元测试（纯函数，无 DB/cache 依赖）
//
// 校验 bt_node_types.param_schema 结构：
//   {"params":[{"name":"","label":"","type":"","required":bool,"options":[]}, ...]}
// type ∈ bb_key/string/float/integer/bool/select；select 必须带非空 options。
// 所有失败路径返回 errcode.ErrBtNodeTypeParamSchemaInvalid (44025)。
// ============================================================

func TestValidateParamSchema(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		wantErr bool
	}{
		// ---- 顶层失败路径 ----
		{"空 schema", ``, true},
		{"非 JSON 对象", `[]`, true},
		{"缺 params 字段", `{"other":1}`, true},
		{"params 非数组", `{"params":"not-array"}`, true},

		// ---- 单个 param 字段缺失/类型错 ----
		{"缺 name", `{"params":[{"label":"L","type":"string"}]}`, true},
		{"name 空串", `{"params":[{"name":"","label":"L","type":"string"}]}`, true},
		{"name 非字符串", `{"params":[{"name":123,"label":"L","type":"string"}]}`, true},
		{"缺 label", `{"params":[{"name":"k","type":"string"}]}`, true},
		{"label 空串", `{"params":[{"name":"k","label":"","type":"string"}]}`, true},
		{"label 非字符串", `{"params":[{"name":"k","label":123,"type":"string"}]}`, true},
		{"缺 type", `{"params":[{"name":"k","label":"L"}]}`, true},
		{"type 非字符串", `{"params":[{"name":"k","label":"L","type":123}]}`, true},
		{"type 非法值", `{"params":[{"name":"k","label":"L","type":"foo"}]}`, true},

		// ---- select 专属：options 必须存在且非空 ----
		{"select 缺 options", `{"params":[{"name":"k","label":"L","type":"select"}]}`, true},
		{"select options 非数组", `{"params":[{"name":"k","label":"L","type":"select","options":"x"}]}`, true},
		{"select options 空数组", `{"params":[{"name":"k","label":"L","type":"select","options":[]}]}`, true},

		// ---- 合法用例 ----
		{"空 params 数组合法", `{"params":[]}`, false},
		{"单 bb_key 合法", `{"params":[{"name":"key","label":"键","type":"bb_key"}]}`, false},
		{"所有合法 type 混合", `{"params":[
			{"name":"a","label":"A","type":"bb_key"},
			{"name":"b","label":"B","type":"string"},
			{"name":"c","label":"C","type":"float"},
			{"name":"d","label":"D","type":"integer"},
			{"name":"e","label":"E","type":"bool"},
			{"name":"f","label":"F","type":"select","options":["x","y"]}
		]}`, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateParamSchema(json.RawMessage(c.schema))
			if c.wantErr {
				if err == nil {
					t.Fatalf("want err, got nil")
				}
				var codeErr *errcode.Error
				if !errors.As(err, &codeErr) {
					t.Fatalf("want *errcode.Error, got %T: %v", err, err)
				}
				if codeErr.Code != errcode.ErrBtNodeTypeParamSchemaInvalid {
					t.Errorf("want code=%d, got code=%d (msg=%q)",
						errcode.ErrBtNodeTypeParamSchemaInvalid, codeErr.Code, codeErr.Message)
				}
			} else {
				if err != nil {
					t.Errorf("want nil, got %v", err)
				}
			}
		})
	}
}
