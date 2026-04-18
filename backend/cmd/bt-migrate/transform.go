package main

import (
	"fmt"
	"sort"
	"strings"
)

// paramFieldSpec 单个参数的规则化规格
//
// name:         目标 schema 中的字段名（规格化后）
// legacy:       旧字段别名（按顺序尝试；命中则迁移到 name）
// defaultValue: 所有来源都找不到值时填充的默认值（nil 表示无默认）
type paramFieldSpec struct {
	name         string
	legacy       []string
	defaultValue any
}

// paramFields 对每个节点类型声明 params 字段规则
//
// 本表是 seed (T2) param_schema 的"迁移侧镜像"——两者语义对齐但代码解耦
// （迁移脚本一次性执行，不依赖运行时 seed 数据）。
var paramFields = map[string][]paramFieldSpec{
	"stub_action": {
		{name: "name", legacy: []string{"action"}},
		{name: "result", defaultValue: "success"},
	},
	"check_bb_float":  {{name: "key", legacy: []string{"target_key"}}, {name: "op"}, {name: "value"}},
	"check_bb_string": {{name: "key", legacy: []string{"target_key"}}, {name: "op"}, {name: "value"}},
	"set_bb_value":    {{name: "key", legacy: []string{"target_key"}}, {name: "value"}},
	"move_to":         {{name: "target_key_x"}, {name: "target_key_z"}, {name: "speed"}},
	"flee_from":       {{name: "source_key_x"}, {name: "source_key_z"}, {name: "distance"}, {name: "speed"}},
}

// transformNode 规则化单节点并递归 children/child
//
// treeName: 当前树 name（用于 #4 bt/combat/attack 的特判）
// pathHint: 当前节点 JSON 路径（"$" 为根；子节点为 "$.children[N]" 或 "$.child"）
//
// 返回：新节点（深拷贝语义，不共享 input 引用）、迁移日志、致命错误。
// 致命错误场景：type 字段缺失/非字符串、children 非数组、子节点非对象。
func transformNode(node map[string]any, treeName, pathHint string) (map[string]any, []string, error) {
	typeName, ok := node["type"].(string)
	if !ok || typeName == "" {
		return nil, nil, fmt.Errorf("%s: 节点缺少合法的 type 字段", pathHint)
	}

	newNode := map[string]any{"type": typeName}
	var warnings []string

	// 1. 构建 params（仅对有 paramFields 规则的类型）
	if specs, has := paramFields[typeName]; has {
		params, paramWarnings := buildParams(node, typeName, specs, treeName, pathHint)
		if params != nil {
			newNode["params"] = params
		}
		warnings = append(warnings, paramWarnings...)
	}

	// 2. 递归 children
	if rawChildren, has := node["children"]; has {
		childrenList, isArr := rawChildren.([]any)
		if !isArr {
			return nil, nil, fmt.Errorf("%s: children 必须是数组", pathHint)
		}
		newChildren := make([]any, 0, len(childrenList))
		for i, rawChild := range childrenList {
			child, isMap := rawChild.(map[string]any)
			if !isMap {
				return nil, nil, fmt.Errorf("%s.children[%d]: 子节点必须是对象", pathHint, i)
			}
			childPath := fmt.Sprintf("%s.children[%d]", pathHint, i)
			newChild, childWarnings, err := transformNode(child, treeName, childPath)
			if err != nil {
				return nil, nil, err
			}
			newChildren = append(newChildren, newChild)
			warnings = append(warnings, childWarnings...)
		}
		newNode["children"] = newChildren
	}

	// 3. 递归 child
	if rawChild, has := node["child"]; has {
		child, isMap := rawChild.(map[string]any)
		if !isMap {
			return nil, nil, fmt.Errorf("%s.child: 子节点必须是对象", pathHint)
		}
		childPath := pathHint + ".child"
		newChild, childWarnings, err := transformNode(child, treeName, childPath)
		if err != nil {
			return nil, nil, err
		}
		newNode["child"] = newChild
		warnings = append(warnings, childWarnings...)
	}

	// 警告按路径排序，保证输出确定性（便于测试 / 人眼 diff）
	sort.Strings(warnings)
	return newNode, warnings, nil
}

// buildParams 从 node 的多种形态（裸字段 / 半裸 / 已嵌套）构建规范化 params
//
// 查找顺序：node.params[name] → node[name] → node.params[legacy] → node[legacy] → defaultValue
func buildParams(node map[string]any, typeName string, specs []paramFieldSpec, treeName, pathHint string) (map[string]any, []string) {
	var warnings []string
	params := map[string]any{}
	oldParams, _ := node["params"].(map[string]any)

	// #4 特判：bt/combat/attack 的空 stub_action 按位置填占位
	// "空" = 节点无 params 且无 action / name 字段
	if treeName == "bt/combat/attack" && typeName == "stub_action" && isEmptyStubAction(node) {
		placeholder := ""
		if strings.HasSuffix(pathHint, ".children[1]") {
			placeholder = "attack_prepare"
		} else if strings.HasSuffix(pathHint, ".children[2]") {
			placeholder = "attack_strike"
		}
		if placeholder != "" {
			params["name"] = placeholder
			params["result"] = "success"
			warnings = append(warnings, fmt.Sprintf("%s: bt/combat/attack 空 stub_action 填占位 name=%s, result=success", pathHint, placeholder))
			return params, warnings
		}
	}

	for _, spec := range specs {
		// 正规查找
		if v, ok := oldParams[spec.name]; ok {
			params[spec.name] = v
			continue
		}
		if v, ok := node[spec.name]; ok {
			params[spec.name] = v
			continue
		}
		// 旧字段名迁移
		found := false
		for _, legacy := range spec.legacy {
			if v, ok := oldParams[legacy]; ok {
				params[spec.name] = v
				warnings = append(warnings, fmt.Sprintf("%s: %s params.%s → params.%s（旧字段名迁移）", pathHint, typeName, legacy, spec.name))
				found = true
				break
			}
			if v, ok := node[legacy]; ok {
				params[spec.name] = v
				warnings = append(warnings, fmt.Sprintf("%s: %s %s → params.%s（旧字段名迁移）", pathHint, typeName, legacy, spec.name))
				found = true
				break
			}
		}
		if found {
			continue
		}
		// 默认值兜底
		if spec.defaultValue != nil {
			params[spec.name] = spec.defaultValue
			warnings = append(warnings, fmt.Sprintf("%s: %s params.%s 补默认值 %v", pathHint, typeName, spec.name, spec.defaultValue))
		}
	}

	if len(params) == 0 {
		return nil, warnings
	}
	return params, warnings
}

// isEmptyStubAction 判断 stub_action 节点是否"语义上为空"（无任何 params / action / name 信息）
// 专用于 #4 特判，不是通用检查
func isEmptyStubAction(node map[string]any) bool {
	if _, ok := node["params"]; ok {
		return false
	}
	if _, ok := node["action"]; ok {
		return false
	}
	if _, ok := node["name"]; ok {
		return false
	}
	return true
}
