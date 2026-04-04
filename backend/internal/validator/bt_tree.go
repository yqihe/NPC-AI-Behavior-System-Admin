package validator

import (
	"encoding/json"
)

type btNode struct {
	Type     string          `json:"type"`
	Children json.RawMessage `json:"children"`
	Params   json.RawMessage `json:"params"`
}

// 合法的 BT 节点类型
var compositeNodeTypes = map[string]bool{
	"sequence": true,
	"selector": true,
	"parallel": true,
	"inverter": true,
}

var leafNodeTypes = map[string]bool{
	"check_bb_float":  true,
	"check_bb_string": true,
	"set_bb_value":    true,
	"stub_action":     true,
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
	isLeaf := leafNodeTypes[node.Type]

	if !isComposite && !isLeaf {
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
	}

	if isLeaf {
		if len(node.Params) == 0 || string(node.Params) == "null" {
			b.addf("%s (%s): 叶子节点必须包含 params", path, node.Type)
		}
	}
}
