package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// 纯包级辅助的 unit test：无 DB/service 依赖。
// 目标：checkTemplateFields（4 分支）+ extractFieldIDs（2 分支）
// ============================================================

func TestCheckTemplateFields_Empty(t *testing.T) {
	err := checkTemplateFields(nil)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if err.Code != errcode.ErrTemplateNoFields {
		t.Errorf("want code=%d, got code=%d", errcode.ErrTemplateNoFields, err.Code)
	}

	err = checkTemplateFields([]model.TemplateFieldEntry{})
	if err == nil {
		t.Fatal("empty slice: want err, got nil")
	}
	if err.Code != errcode.ErrTemplateNoFields {
		t.Errorf("empty slice: want code=%d, got code=%d", errcode.ErrTemplateNoFields, err.Code)
	}
}

func TestCheckTemplateFields_NonPositiveID(t *testing.T) {
	cases := []struct {
		name    string
		entries []model.TemplateFieldEntry
	}{
		{"zero", []model.TemplateFieldEntry{{FieldID: 0}}},
		{"negative", []model.TemplateFieldEntry{{FieldID: -1}}},
		{"mixed good then bad", []model.TemplateFieldEntry{{FieldID: 5}, {FieldID: 0}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkTemplateFields(c.entries)
			if err == nil {
				t.Fatal("want err, got nil")
			}
			if err.Code != errcode.ErrBadRequest {
				t.Errorf("want code=%d, got code=%d", errcode.ErrBadRequest, err.Code)
			}
			if !strings.Contains(err.Error(), "field_id 必须 > 0") {
				t.Errorf("err 应含 'field_id 必须 > 0', got %q", err.Error())
			}
		})
	}
}

func TestCheckTemplateFields_DuplicateID(t *testing.T) {
	err := checkTemplateFields([]model.TemplateFieldEntry{
		{FieldID: 1},
		{FieldID: 2},
		{FieldID: 1},
	})
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if err.Code != errcode.ErrBadRequest {
		t.Errorf("want code=%d, got code=%d", errcode.ErrBadRequest, err.Code)
	}
	if !strings.Contains(err.Error(), "重复") {
		t.Errorf("err 应含 '重复', got %q", err.Error())
	}
}

func TestCheckTemplateFields_Happy(t *testing.T) {
	// 合法：多字段 + 有 required true/false 混合
	entries := []model.TemplateFieldEntry{
		{FieldID: 10, Required: true},
		{FieldID: 20, Required: false},
		{FieldID: 30, Required: true},
	}
	if err := checkTemplateFields(entries); err != nil {
		t.Errorf("want nil, got %v", err)
	}

	// 单元素也 OK
	if err := checkTemplateFields([]model.TemplateFieldEntry{{FieldID: 1}}); err != nil {
		t.Errorf("单元素: want nil, got %v", err)
	}
}

func TestCheckTemplateFields_IsErrcodeError(t *testing.T) {
	// 返回类型是 *errcode.Error，确保上层 errors.As 链路正确
	err := checkTemplateFields(nil)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) {
		t.Fatalf("want *errcode.Error, got %T", err)
	}
	if codeErr.Code != errcode.ErrTemplateNoFields {
		t.Errorf("want code=%d, got code=%d", errcode.ErrTemplateNoFields, codeErr.Code)
	}
}

func TestExtractFieldIDs_Empty(t *testing.T) {
	ids := extractFieldIDs(nil)
	if len(ids) != 0 {
		t.Errorf("nil input: want empty, got %v", ids)
	}
	ids = extractFieldIDs([]model.TemplateFieldEntry{})
	if len(ids) != 0 {
		t.Errorf("empty slice: want empty, got %v", ids)
	}
}

func TestExtractFieldIDs_PreservesOrder(t *testing.T) {
	entries := []model.TemplateFieldEntry{
		{FieldID: 30, Required: true},
		{FieldID: 10, Required: false},
		{FieldID: 20, Required: true},
	}
	ids := extractFieldIDs(entries)
	want := []int64{30, 10, 20}
	if len(ids) != len(want) {
		t.Fatalf("len mismatch: want %d, got %d", len(want), len(ids))
	}
	for i, v := range want {
		if ids[i] != v {
			t.Errorf("ids[%d]: want %d, got %d", i, v, ids[i])
		}
	}
}
