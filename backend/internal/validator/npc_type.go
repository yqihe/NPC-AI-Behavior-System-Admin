package validator

import (
	"context"
	"encoding/json"

	"github.com/npc-admin/backend/internal/store"
)

type npcConfig struct {
	TypeName   string            `json:"type_name"`
	FsmRef     string            `json:"fsm_ref"`
	BtRefs     map[string]string `json:"bt_refs"`
	Perception *npcPerception    `json:"perception"`
}

type npcPerception struct {
	VisualRange   *float64 `json:"visual_range"`
	AuditoryRange *float64 `json:"auditory_range"`
}

// ValidateNpcType 校验 NPC 类型的 config 字段。
// 需要 store 检查 fsm_ref 和 bt_refs 引用是否存在。
func ValidateNpcType(config json.RawMessage, s store.Store, ctx context.Context) error {
	var c npcConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return &ValidationError{Errors: []string{"配置格式错误，无法解析"}}
	}

	var b validationBuilder

	if c.TypeName == "" {
		b.add("NPC 类型名称不能为空")
	}

	// perception 校验
	if c.Perception == nil {
		b.add("感知配置（perception）不能为空")
	} else {
		if c.Perception.VisualRange == nil {
			b.add("视觉范围（visual_range）不能为空")
		} else if *c.Perception.VisualRange < 0 {
			b.addf("视觉范围不能为负数，当前值: %v", *c.Perception.VisualRange)
		}
		if c.Perception.AuditoryRange == nil {
			b.add("听觉范围（auditory_range）不能为空")
		} else if *c.Perception.AuditoryRange < 0 {
			b.addf("听觉范围不能为负数，当前值: %v", *c.Perception.AuditoryRange)
		}
	}

	// fsm_ref 引用检查
	if c.FsmRef == "" {
		b.add("状态机引用（fsm_ref）不能为空")
	} else if s != nil {
		if _, err := s.Get(ctx, "fsm_configs", c.FsmRef); err != nil {
			b.addf("引用的状态机 \"%s\" 不存在", c.FsmRef)
		}
	}

	// bt_refs 引用检查
	if c.BtRefs == nil || len(c.BtRefs) == 0 {
		b.add("行为树引用（bt_refs）不能为空")
	} else if s != nil {
		for state, btName := range c.BtRefs {
			if btName == "" {
				b.addf("状态 \"%s\" 的行为树引用不能为空", state)
				continue
			}
			if _, err := s.Get(ctx, "bt_trees", btName); err != nil {
				b.addf("状态 \"%s\" 引用的行为树 \"%s\" 不存在", state, btName)
			}
		}
	}

	return b.result()
}
