package validator

import (
	"encoding/json"
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
	if _, ok := m["op"]; !ok {
		b.addf("转换 #%d: 条件缺少 op 字段", transIdx)
	}
	// value 或 ref_key 至少有一个
	_, hasValue := m["value"]
	_, hasRefKey := m["ref_key"]
	if !hasValue && !hasRefKey {
		b.addf("转换 #%d: 条件缺少 value 或 ref_key 字段", transIdx)
	}
}

// itoa 简单整数转字符串，避免引入 strconv。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 4)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	// 反转
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
