package validator

import (
	"encoding/json"
	"strconv"
)

type fsmConfig struct {
	InitialState string          `json:"initial_state"`
	States       []fsmState      `json:"states"`
	Transitions  []fsmTransition `json:"transitions"`
}

type fsmState struct {
	Name string `json:"name"`
}

type fsmTransition struct {
	From      string          `json:"from"`
	To        string          `json:"to"`
	Priority  *int            `json:"priority"`
	Condition json.RawMessage `json:"condition"`
}

// ValidateFsmConfig 校验 FSM 状态机的 config 字段。
func ValidateFsmConfig(config json.RawMessage) error {
	var c fsmConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return &ValidationError{Errors: []string{"配置格式错误，无法解析"}}
	}

	var b validationBuilder

	// 收集有效状态名
	stateSet := make(map[string]bool)
	for _, s := range c.States {
		if s.Name == "" {
			b.add("状态名称不能为空")
		} else {
			if stateSet[s.Name] {
				b.addf("状态名 \"%s\" 重复", s.Name)
			}
			stateSet[s.Name] = true
		}
	}

	if len(c.States) == 0 {
		b.add("状态列表不能为空")
	}

	// initial_state 校验
	if c.InitialState == "" {
		b.add("初始状态（initial_state）不能为空")
	} else if !stateSet[c.InitialState] {
		b.addf("初始状态 \"%s\" 不在状态列表中", c.InitialState)
	}

	// transitions 校验
	for i, t := range c.Transitions {
		prefix := func(msg string) string {
			return "转换 #" + itoa(i+1) + ": " + msg
		}

		if t.From == "" {
			b.add(prefix("来源状态（from）不能为空"))
		} else if !stateSet[t.From] {
			b.add(prefix("来源状态 \"" + t.From + "\" 不在状态列表中"))
		}

		if t.To == "" {
			b.add(prefix("目标状态（to）不能为空"))
		} else if !stateSet[t.To] {
			b.add(prefix("目标状态 \"" + t.To + "\" 不在状态列表中"))
		}

		if t.Priority == nil {
			b.add(prefix("优先级（priority）不能为空"))
		} else if *t.Priority <= 0 {
			b.addf("转换 #%d: 优先级必须大于 0，当前值: %d", i+1, *t.Priority)
		}

		if len(t.Condition) > 0 {
			validateCondition(t.Condition, &b, i+1)
		}
	}

	return b.result()
}

// validateCondition 递归校验条件结构。
func validateCondition(raw json.RawMessage, b *validationBuilder, transIdx int) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		b.addf("转换 #%d: 条件格式错误，无法解析", transIdx)
		return
	}

	// 检查组合条件与叶子条件互斥
	_, hasAnd := m["and"]
	_, hasOr := m["or"]
	_, hasKey := m["key"]
	isComposite := hasAnd || hasOr

	if isComposite && hasKey {
		b.addf("转换 #%d: 条件不能同时包含 and/or 和 key，请拆分为组合条件或叶子条件", transIdx)
		return
	}

	// and/or 组合条件
	if andRaw, ok := m["and"]; ok {
		var children []json.RawMessage
		if err := json.Unmarshal(andRaw, &children); err != nil {
			b.addf("转换 #%d: and 条件的子条件列表格式错误", transIdx)
			return
		}
		if len(children) == 0 {
			b.addf("转换 #%d: and 条件的子条件列表不能为空", transIdx)
		}
		for _, child := range children {
			validateCondition(child, b, transIdx)
		}
		return
	}
	if orRaw, ok := m["or"]; ok {
		var children []json.RawMessage
		if err := json.Unmarshal(orRaw, &children); err != nil {
			b.addf("转换 #%d: or 条件的子条件列表格式错误", transIdx)
			return
		}
		if len(children) == 0 {
			b.addf("转换 #%d: or 条件的子条件列表不能为空", transIdx)
		}
		for _, child := range children {
			validateCondition(child, b, transIdx)
		}
		return
	}

	// 叶子条件：必须有 key 和 op
	if _, ok := m["key"]; !ok {
		b.addf("转换 #%d: 条件缺少 key 字段", transIdx)
	}
	if opRaw, ok := m["op"]; !ok {
		b.addf("转换 #%d: 条件缺少 op 字段", transIdx)
	} else {
		var op string
		if err := json.Unmarshal(opRaw, &op); err == nil {
			validOps := map[string]bool{
				"==": true, "!=": true,
				">": true, ">=": true,
				"<": true, "<=": true,
				"in": true,
			}
			if !validOps[op] {
				b.addf("转换 #%d: 不支持的操作符 \"%s\"，合法值: ==, !=, >, >=, <, <=, in", transIdx, op)
			}
		}
	}
	// value 或 ref_key 至少有一个
	_, hasValue := m["value"]
	_, hasRefKey := m["ref_key"]
	if !hasValue && !hasRefKey {
		b.addf("转换 #%d: 条件缺少 value 或 ref_key 字段", transIdx)
	}
}

// itoa 整数转字符串，使用 strconv.Itoa。
func itoa(n int) string {
	return strconv.Itoa(n)
}
