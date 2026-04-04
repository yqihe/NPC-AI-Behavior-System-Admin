package validator

import (
	"encoding/json"
)

// eventConfig 是事件类型 config 的校验用临时结构体，不存储。
type eventConfig struct {
	Name           string  `json:"name"`
	DefaultSeverity *int   `json:"default_severity"`
	DefaultTTL     *float64 `json:"default_ttl"`
	PerceptionMode string  `json:"perception_mode"`
	Range          *float64 `json:"range"`
}

var validPerceptionModes = map[string]bool{
	"visual":   true,
	"auditory": true,
	"global":   true,
}

// ValidateEventType 校验事件类型的 config 字段。
func ValidateEventType(config json.RawMessage) error {
	var c eventConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return &ValidationError{Errors: []string{"配置格式错误，无法解析"}}
	}

	var b validationBuilder

	if c.Name == "" {
		b.add("事件名称不能为空")
	}

	if c.DefaultSeverity == nil {
		b.add("威胁等级（default_severity）不能为空")
	} else if *c.DefaultSeverity < 0 || *c.DefaultSeverity > 100 {
		b.addf("威胁等级必须在 0-100 之间，当前值: %d", *c.DefaultSeverity)
	}

	if c.DefaultTTL == nil {
		b.add("持续时间（default_ttl）不能为空")
	} else if *c.DefaultTTL <= 0 {
		b.addf("持续时间必须大于 0，当前值: %v", *c.DefaultTTL)
	}

	if c.PerceptionMode == "" {
		b.add("传播方式（perception_mode）不能为空")
	} else if !validPerceptionModes[c.PerceptionMode] {
		b.addf("传播方式必须是 visual/auditory/global 之一，当前值: %s", c.PerceptionMode)
	}

	if c.Range == nil {
		b.add("传播范围（range）不能为空")
	} else if *c.Range < 0 {
		b.addf("传播范围不能为负数，当前值: %v", *c.Range)
	}

	return b.result()
}
