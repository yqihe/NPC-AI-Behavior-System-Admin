package shared

// jsonutil.go — service 层 JSON 提取辅助
//
// 从 json.RawMessage 中提取基础类型，供约束解析/校验使用。
// 无业务语义，纯工具。

import (
	"encoding/json"
	"fmt"
)

// ============================================================
// JSON 解析辅助
// ============================================================

// ParseConstraintsMap 解析 constraints JSON 为 key→RawMessage map
func ParseConstraintsMap(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return make(map[string]json.RawMessage), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal constraints: %w", err)
	}
	return m, nil
}

// GetFloat 从 json.RawMessage 提取 float64
func GetFloat(raw json.RawMessage) (float64, bool) {
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}

// GetString 从 json.RawMessage 提取字符串，失败返回空串
func GetString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// GetBool 从 json.RawMessage 提取 bool
func GetBool(raw json.RawMessage) (bool, bool) {
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, false
	}
	return v, true
}

// IsJSONNull 判断 json.RawMessage 是否为 JSON null（空或 "null"）
func IsJSONNull(v json.RawMessage) bool {
	return len(v) == 0 || string(v) == "null"
}

// ParseSelectOptions 解析 select 类型的 options 数组，返回 value 列表
func ParseSelectOptions(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var options []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil
	}
	values := make([]string, 0, len(options))
	for _, o := range options {
		values = append(values, o.Value)
	}
	return values
}
