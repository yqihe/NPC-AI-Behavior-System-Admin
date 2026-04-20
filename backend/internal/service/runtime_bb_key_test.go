package service

import (
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// RuntimeBbKeyService 静态校验单元测试（T17）
//
// 覆盖 validateName / validateType / validateGroupName 三个零依赖 validator。
// 与 bt_tree_test / npc_service_test 同 pattern —— 不引入 sqlmock，
// 依赖 store/cache 的业务路径（Create/Delete/Toggle/Sync）归 T18 手动 smoke。
//
// 测试目的：锁住 design §0（type 4 枚举 / group 11 枚举 / name 格式）
// 与 Server keys.go 对齐契约，未来任何误改都会立即 fire。
// ============================================================

func TestValidateRuntimeBbKeyName(t *testing.T) {
	s := &RuntimeBbKeyService{}

	cases := []struct {
		name     string
		input    string
		wantCode int // 0 = 期望 nil
	}{
		// 合法：对齐 Server keys.go 31 条常见形态
		{"合法-威胁 key", "threat_level", 0},
		{"合法-含数字", "npc_pos_x", 0},
		{"合法-最短 2 字符", "ab", 0},
		{"合法-最长 64 字符", "a" + repeat("b", 63), 0},
		{"合法-纯下划线尾缀", "exit_cleanup_done", 0},

		// 非法：首字符
		{"非法-大写开头", "ThreatLevel", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-数字开头", "1_bad", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-下划线开头", "_private", errcode.ErrRuntimeBBKeyNameInvalid},

		// 非法：字符集
		{"非法-含连字符", "threat-level", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-含点号", "threat.level", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-含空格", "threat level", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-中文", "威胁等级", errcode.ErrRuntimeBBKeyNameInvalid},

		// 非法：长度
		{"非法-空串", "", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-单字符", "a", errcode.ErrRuntimeBBKeyNameInvalid},
		{"非法-超 64 字符", "a" + repeat("b", 64), errcode.ErrRuntimeBBKeyNameInvalid},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateName(c.input)
			if c.wantCode == 0 {
				if err != nil {
					t.Errorf("want nil, got %v (code=%d)", err, err.Code)
				}
				return
			}
			if err == nil {
				t.Fatalf("want code=%d, got nil", c.wantCode)
			}
			if err.Code != c.wantCode {
				t.Errorf("want code=%d, got code=%d (msg=%q)", c.wantCode, err.Code, err.Message)
			}
		})
	}
}

func TestValidateRuntimeBbKeyType(t *testing.T) {
	s := &RuntimeBbKeyService{}

	cases := []struct {
		name     string
		input    string
		wantCode int
	}{
		// design §0 锁定 4 枚举（与 Server keys.go 泛型参数一一映射）
		{"合法-integer", "integer", 0},
		{"合法-float", "float", 0},
		{"合法-string", "string", 0},
		{"合法-bool", "bool", 0},

		// 非法：常见误写 / 历史别名
		{"非法-int（应为 integer）", "int", errcode.ErrRuntimeBBKeyTypeInvalid},
		{"非法-boolean（应为 bool）", "boolean", errcode.ErrRuntimeBBKeyTypeInvalid},
		{"非法-number", "number", errcode.ErrRuntimeBBKeyTypeInvalid},
		{"非法-object（FSM/BT 不支持）", "object", errcode.ErrRuntimeBBKeyTypeInvalid},
		{"非法-大小写敏感 Float", "Float", errcode.ErrRuntimeBBKeyTypeInvalid},
		{"非法-空串", "", errcode.ErrRuntimeBBKeyTypeInvalid},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateType(c.input)
			if c.wantCode == 0 {
				if err != nil {
					t.Errorf("want nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want code=%d, got nil", c.wantCode)
			}
			if err.Code != c.wantCode {
				t.Errorf("want code=%d, got code=%d", c.wantCode, err.Code)
			}
		})
	}
}

func TestValidateRuntimeBbKeyGroupName(t *testing.T) {
	s := &RuntimeBbKeyService{}

	// design §0 锁定 11 组（与 Server keys.go 分节注释逐字对齐）
	validGroups := []string{
		"threat", "event", "fsm", "npc", "action",
		"need", "emotion", "memory", "social", "decision", "move",
	}
	for _, g := range validGroups {
		t.Run("合法-"+g, func(t *testing.T) {
			if err := s.validateGroupName(g); err != nil {
				t.Errorf("group %q should be valid, got %v", g, err)
			}
		})
	}

	invalidCases := []struct {
		name  string
		input string
	}{
		{"非法-空串", ""},
		{"非法-大小写敏感 Threat", "Threat"},
		{"非法-带空格", "threat "},
		{"非法-未登记 combat", "combat"},
		{"非法-复数 threats", "threats"},
	}
	for _, c := range invalidCases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateGroupName(c.input)
			if err == nil {
				t.Fatalf("want invalid, got nil for %q", c.input)
			}
			if err.Code != errcode.ErrRuntimeBBKeyGroupNameInvalid {
				t.Errorf("want code=%d, got code=%d", errcode.ErrRuntimeBBKeyGroupNameInvalid, err.Code)
			}
		})
	}
}

// repeat 辅助：生成 n 个重复字符（标准库 strings.Repeat 的无依赖替代）
func repeat(ch string, n int) string {
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = ch[0]
	}
	return string(buf)
}
