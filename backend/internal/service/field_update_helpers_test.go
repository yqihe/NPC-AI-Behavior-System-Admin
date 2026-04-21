package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// FieldService.Update 拆分后的纯辅助 enforceRefIntegrityOnUpdate 单测。
// 其余三个辅助（checkExposeBBRevocation / validateReferenceRefsForUpdate /
// syncFieldRefsAfterCommit）依赖 *FieldService 的 store/cache，归入集成范畴。
// ============================================================

// mkProps 构造 FieldProperties（constraints 传 JSON 字符串）
func mkProps(constraintsJSON string) *model.FieldProperties {
	p := &model.FieldProperties{}
	if constraintsJSON != "" {
		p.Constraints = json.RawMessage(constraintsJSON)
	}
	return p
}

func TestEnforceRefIntegrityOnUpdate_NoRefsAlwaysPass(t *testing.T) {
	// hasRefs=false → 无论类型/约束是否变化都放行
	err := enforceRefIntegrityOnUpdate(false, "integer", "string",
		mkProps(`{"min":0}`), mkProps(`{"min":100}`))
	if err != nil {
		t.Errorf("未被引用应放行, got %v", err)
	}
}

func TestEnforceRefIntegrityOnUpdate_HasRefsTypeChangeRejected(t *testing.T) {
	err := enforceRefIntegrityOnUpdate(true, "integer", "string", nil, nil)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T", err)
	}
	if codeErr.Code != errcode.ErrFieldRefChangeType {
		t.Errorf("want code=%d, got code=%d", errcode.ErrFieldRefChangeType, codeErr.Code)
	}
}

func TestEnforceRefIntegrityOnUpdate_HasRefsSameTypeTightenedRejected(t *testing.T) {
	// 被引用 + 同类型 + integer max 从 200 收紧到 100 → ErrFieldRefTighten
	err := enforceRefIntegrityOnUpdate(true, "integer", "integer",
		mkProps(`{"min":0,"max":200}`), mkProps(`{"min":0,"max":100}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T", err)
	}
	if codeErr.Code != errcode.ErrFieldRefTighten {
		t.Errorf("want code=%d, got code=%d", errcode.ErrFieldRefTighten, codeErr.Code)
	}
}

func TestEnforceRefIntegrityOnUpdate_HasRefsSameTypeRelaxedAllowed(t *testing.T) {
	// 被引用 + 同类型 + integer max 从 100 放宽到 200 → 放行
	err := enforceRefIntegrityOnUpdate(true, "integer", "integer",
		mkProps(`{"min":0,"max":100}`), mkProps(`{"min":0,"max":200}`))
	if err != nil {
		t.Errorf("放宽约束应放行, got %v", err)
	}
}

func TestEnforceRefIntegrityOnUpdate_HasRefsNilPropsAllowed(t *testing.T) {
	// 被引用 + 同类型 + oldProps/newProps 任一为 nil → 跳过约束检查（放行）
	if err := enforceRefIntegrityOnUpdate(true, "integer", "integer", nil, mkProps(`{"min":0}`)); err != nil {
		t.Errorf("oldProps=nil 应放行, got %v", err)
	}
	if err := enforceRefIntegrityOnUpdate(true, "integer", "integer", mkProps(`{"min":0}`), nil); err != nil {
		t.Errorf("newProps=nil 应放行, got %v", err)
	}
}

func TestEnforceRefIntegrityOnUpdate_HasRefsTypeChangeTakesPrecedence(t *testing.T) {
	// 被引用时：type 变化优先拦截，不触发收紧判断
	err := enforceRefIntegrityOnUpdate(true, "integer", "string",
		mkProps(`{"min":0,"max":200}`), mkProps(`{"min":0,"max":100}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) || codeErr.Code != errcode.ErrFieldRefChangeType {
		t.Errorf("want ErrFieldRefChangeType, got %v", err)
	}
}

func TestEnforceRefIntegrityOnUpdate_HasRefsSameTypeEmptyConstraintsAllowed(t *testing.T) {
	// 空 constraints → CheckConstraintTightened 直接放行
	err := enforceRefIntegrityOnUpdate(true, "integer", "integer", mkProps(""), mkProps(""))
	if err != nil {
		t.Errorf("空 constraints 应放行, got %v", err)
	}
}
