package service

import (
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// template.go 纯方法补测：validateFieldsBasic / ParseFieldEntries /
// IsFieldsChanged / DiffFieldIDs。
//
// 方法挂在 *TemplateService 但不访问 struct 字段，zero-value 构造即可。
// ============================================================

// --- validateFieldsBasic ---

func TestValidateFieldsBasic(t *testing.T) {
	s := &TemplateService{}

	cases := []struct {
		name     string
		fields   []model.TemplateFieldEntry
		wantCode int // 0 = 期望 nil
	}{
		{
			name:     "空切片",
			fields:   nil,
			wantCode: errcode.ErrTemplateNoFields,
		},
		{
			name:     "长度 0 切片",
			fields:   []model.TemplateFieldEntry{},
			wantCode: errcode.ErrTemplateNoFields,
		},
		{
			name:     "field_id=0",
			fields:   []model.TemplateFieldEntry{{FieldID: 0, Required: true}},
			wantCode: errcode.ErrBadRequest,
		},
		{
			name:     "field_id<0",
			fields:   []model.TemplateFieldEntry{{FieldID: -1, Required: true}},
			wantCode: errcode.ErrBadRequest,
		},
		{
			name: "field_id 重复",
			fields: []model.TemplateFieldEntry{
				{FieldID: 1, Required: true},
				{FieldID: 2, Required: false},
				{FieldID: 1, Required: false},
			},
			wantCode: errcode.ErrBadRequest,
		},
		{
			name: "合法 — 单字段",
			fields: []model.TemplateFieldEntry{
				{FieldID: 1, Required: true},
			},
			wantCode: 0,
		},
		{
			name: "合法 — 多字段无重复",
			fields: []model.TemplateFieldEntry{
				{FieldID: 1, Required: true},
				{FieldID: 2, Required: false},
				{FieldID: 3, Required: true},
			},
			wantCode: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateFieldsBasic(c.fields)
			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			var codeErr *errcode.Error
			if !errors.As(err, &codeErr) {
				t.Fatalf("want *errcode.Error, got %T: %v", err, err)
			}
			if codeErr.Code != c.wantCode {
				t.Errorf("want code=%d, got code=%d (msg=%q)", c.wantCode, codeErr.Code, codeErr.Message)
			}
		})
	}
}

// --- ParseFieldEntries ---

func TestParseFieldEntries(t *testing.T) {
	s := &TemplateService{}

	t.Run("空 raw 返回空切片非 nil", func(t *testing.T) {
		got, err := s.ParseFieldEntries(nil)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if got == nil {
			t.Fatal("want non-nil empty slice, got nil")
		}
		if len(got) != 0 {
			t.Errorf("want len=0, got len=%d", len(got))
		}
	})

	t.Run("JSON null 也归一化为空切片非 nil", func(t *testing.T) {
		// json.Unmarshal 对 null 会把 slice 置 nil —— 函数必须补回空切片
		got, err := s.ParseFieldEntries(json.RawMessage(`null`))
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if got == nil {
			t.Fatal("want non-nil empty slice, got nil")
		}
	})

	t.Run("合法数组", func(t *testing.T) {
		raw := json.RawMessage(`[{"field_id":1,"required":true},{"field_id":2,"required":false}]`)
		got, err := s.ParseFieldEntries(raw)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if len(got) != 2 || got[0].FieldID != 1 || !got[0].Required || got[1].FieldID != 2 || got[1].Required {
			t.Errorf("解析结果错: %+v", got)
		}
	})

	t.Run("非法 JSON 返错", func(t *testing.T) {
		_, err := s.ParseFieldEntries(json.RawMessage(`{not-array}`))
		if err == nil {
			t.Fatal("want err, got nil")
		}
	})
}

// --- IsFieldsChanged ---

func TestIsFieldsChanged(t *testing.T) {
	s := &TemplateService{}

	cases := []struct {
		name string
		old  []model.TemplateFieldEntry
		curr []model.TemplateFieldEntry
		want bool
	}{
		{
			name: "完全相同",
			old:  []model.TemplateFieldEntry{{FieldID: 1, Required: true}, {FieldID: 2, Required: false}},
			curr: []model.TemplateFieldEntry{{FieldID: 1, Required: true}, {FieldID: 2, Required: false}},
			want: false,
		},
		{
			name: "长度不同",
			old:  []model.TemplateFieldEntry{{FieldID: 1, Required: true}},
			curr: []model.TemplateFieldEntry{{FieldID: 1, Required: true}, {FieldID: 2, Required: false}},
			want: true,
		},
		{
			name: "field_id 不同",
			old:  []model.TemplateFieldEntry{{FieldID: 1, Required: true}},
			curr: []model.TemplateFieldEntry{{FieldID: 2, Required: true}},
			want: true,
		},
		{
			name: "required 不同",
			old:  []model.TemplateFieldEntry{{FieldID: 1, Required: true}},
			curr: []model.TemplateFieldEntry{{FieldID: 1, Required: false}},
			want: true,
		},
		{
			name: "顺序不同但集合相同 — 视为变更",
			// 当前实现按索引对比，顺序变化 = 变更
			old:  []model.TemplateFieldEntry{{FieldID: 1, Required: true}, {FieldID: 2, Required: false}},
			curr: []model.TemplateFieldEntry{{FieldID: 2, Required: false}, {FieldID: 1, Required: true}},
			want: true,
		},
		{
			name: "都空",
			old:  nil,
			curr: nil,
			want: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := s.IsFieldsChanged(c.old, c.curr); got != c.want {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

// --- DiffFieldIDs ---

func TestDiffFieldIDs(t *testing.T) {
	s := &TemplateService{}

	mk := func(ids ...int64) []model.TemplateFieldEntry {
		out := make([]model.TemplateFieldEntry, len(ids))
		for i, id := range ids {
			out[i] = model.TemplateFieldEntry{FieldID: id}
		}
		return out
	}

	normalize := func(s []int64) []int64 {
		if s == nil {
			return []int64{}
		}
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		return s
	}

	cases := []struct {
		name       string
		old        []model.TemplateFieldEntry
		curr       []model.TemplateFieldEntry
		wantAdd    []int64
		wantRemove []int64
	}{
		{
			name:       "完全相同",
			old:        mk(1, 2, 3),
			curr:       mk(1, 2, 3),
			wantAdd:    []int64{},
			wantRemove: []int64{},
		},
		{
			name:       "顺序改变集合相同",
			old:        mk(1, 2, 3),
			curr:       mk(3, 1, 2),
			wantAdd:    []int64{},
			wantRemove: []int64{},
		},
		{
			name:       "新增 4 删除 2",
			old:        mk(1, 2, 3),
			curr:       mk(1, 3, 4),
			wantAdd:    []int64{4},
			wantRemove: []int64{2},
		},
		{
			name:       "全部替换",
			old:        mk(1, 2),
			curr:       mk(3, 4),
			wantAdd:    []int64{3, 4},
			wantRemove: []int64{1, 2},
		},
		{
			name:       "空 → 有",
			old:        nil,
			curr:       mk(1, 2),
			wantAdd:    []int64{1, 2},
			wantRemove: []int64{},
		},
		{
			name:       "有 → 空",
			old:        mk(1, 2),
			curr:       nil,
			wantAdd:    []int64{},
			wantRemove: []int64{1, 2},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			add, remove := s.DiffFieldIDs(c.old, c.curr)
			gotAdd := normalize(add)
			gotRemove := normalize(remove)
			wantAdd := normalize(c.wantAdd)
			wantRemove := normalize(c.wantRemove)
			if len(gotAdd) != len(wantAdd) {
				t.Errorf("add: want %v, got %v", wantAdd, gotAdd)
			} else {
				for i := range wantAdd {
					if gotAdd[i] != wantAdd[i] {
						t.Errorf("add[%d]: want %d, got %d", i, wantAdd[i], gotAdd[i])
					}
				}
			}
			if len(gotRemove) != len(wantRemove) {
				t.Errorf("remove: want %v, got %v", wantRemove, gotRemove)
			} else {
				for i := range wantRemove {
					if gotRemove[i] != wantRemove[i] {
						t.Errorf("remove[%d]: want %d, got %d", i, wantRemove[i], gotRemove[i])
					}
				}
			}
		})
	}
}
