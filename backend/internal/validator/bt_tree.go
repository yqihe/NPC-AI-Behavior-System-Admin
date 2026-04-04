package validator

import (
	"encoding/json"
)

type btNode struct {
	Type     string          `json:"type"`
	Children json.RawMessage `json:"children"`
	Child    json.RawMessage `json:"child"`
	Params   json.RawMessage `json:"params"`
}

// 合法的 BT 节点类型
var compositeNodeTypes = map[string]bool{
	"sequence": true,
	"selector": true,
	"parallel": true,
}

var decoratorNodeTypes = map[string]bool{
	"inverter": true,
}

var leafNodeTypes = map[string]bool{
	"check_bb_float":  true,
	"check_bb_string": true,
	"set_bb_value":    true,
	"stub_action":     true,
}

// validBBKeys 是游戏服务端 Blackboard 注册表的白名单（来源：internal/core/blackboard/keys.go）。
// 运营平台产出的 set_bb_value / check_bb_float / check_bb_string 节点的 key 必须在此列表内，
// 否则游戏服务端加载时会 panic。
var validBBKeys = map[string]bool{
	"threat_level":      true,
	"threat_source":     true,
	"threat_expire_at":  true,
	"last_event_type":   true,
	"current_time":      true,
	"fsm_state":         true,
	"npc_type":          true,
	"npc_pos_x":         true,
	"npc_pos_z":         true,
	"current_action":    true,
	"alert_start_tick":  true,
	"exit_cleanup_done": true,
}

// validStubResults 是 stub_action 节点合法的 result 值。
var validStubResults = map[string]bool{
	"success": true,
	"failure": true,
	"running": true,
}

// ValidateBtTree 校验行为树的 config 字段（递归校验节点树）。
func ValidateBtTree(config json.RawMessage) error {
	var b validationBuilder
	validateBtNode(config, &b, "根节点")
	return b.result()
}

func validateBtNode(raw json.RawMessage, b *validationBuilder, path string) {
	var node btNode
	if err := json.Unmarshal(raw, &node); err != nil {
		b.addf("%s: 节点格式错误，无法解析", path)
		return
	}

	if node.Type == "" {
		b.addf("%s: 节点类型（type）不能为空", path)
		return
	}

	isComposite := compositeNodeTypes[node.Type]
	isDecorator := decoratorNodeTypes[node.Type]
	isLeaf := leafNodeTypes[node.Type]

	if !isComposite && !isDecorator && !isLeaf {
		b.addf("%s: 未知的节点类型 \"%s\"", path, node.Type)
		return
	}

	if isComposite {
		if len(node.Children) == 0 || string(node.Children) == "null" {
			b.addf("%s (%s): 复合节点必须包含 children", path, node.Type)
			return
		}
		var children []json.RawMessage
		if err := json.Unmarshal(node.Children, &children); err != nil {
			b.addf("%s (%s): children 格式错误", path, node.Type)
			return
		}
		if len(children) == 0 {
			b.addf("%s (%s): children 不能为空数组", path, node.Type)
		}
		for i, child := range children {
			childPath := path + " → 子节点 #" + itoa(i+1)
			validateBtNode(child, b, childPath)
		}

		// parallel 节点的 policy 参数校验
		if node.Type == "parallel" && len(node.Params) > 0 && string(node.Params) != "null" {
			var pParams map[string]string
			if err := json.Unmarshal(node.Params, &pParams); err != nil {
				b.addf("%s (parallel): params 格式错误，无法解析", path)
			} else if policy, ok := pParams["policy"]; ok {
				if policy != "require_all" && policy != "require_one" {
					b.addf("%s (parallel): policy 必须是 require_all 或 require_one，当前值: \"%s\"", path, policy)
				}
			}
		}
	}

	if isDecorator {
		if len(node.Child) == 0 || string(node.Child) == "null" {
			b.addf("%s (%s): 装饰节点必须包含 child（单个子节点）", path, node.Type)
			return
		}
		childPath := path + " → 子节点"
		validateBtNode(node.Child, b, childPath)
	}

	if isLeaf {
		if len(node.Params) == 0 || string(node.Params) == "null" {
			b.addf("%s (%s): 叶子节点必须包含 params", path, node.Type)
			return
		}
		validateLeafParams(node.Type, node.Params, b, path)
	}
}

// validateLeafParams 校验叶子节点的 params 内容：BB key 白名单、stub_action result 枚举。
func validateLeafParams(nodeType string, params json.RawMessage, b *validationBuilder, path string) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(params, &m); err != nil {
		b.addf("%s (%s): params 格式错误", path, nodeType)
		return
	}

	switch nodeType {
	case "check_bb_float", "check_bb_string", "set_bb_value":
		keyRaw, ok := m["key"]
		if !ok {
			b.addf("%s (%s): params 缺少 key 字段", path, nodeType)
			return
		}
		var key string
		if err := json.Unmarshal(keyRaw, &key); err != nil {
			b.addf("%s (%s): params.key 格式错误", path, nodeType)
			return
		}
		if key == "" {
			b.addf("%s (%s): params.key 不能为空", path, nodeType)
		} else if !validBBKeys[key] {
			b.addf("%s (%s): 未注册的 Blackboard Key \"%s\"", path, nodeType, key)
		}

	case "stub_action":
		resultRaw, ok := m["result"]
		if !ok {
			return // result 可选，服务端默认 success
		}
		var result string
		if err := json.Unmarshal(resultRaw, &result); err != nil {
			b.addf("%s (stub_action): result 字段格式错误", path)
		} else if !validStubResults[result] {
			b.addf("%s (stub_action): result 必须是 success/failure/running，当前值: \"%s\"", path, result)
		}
	}
}
