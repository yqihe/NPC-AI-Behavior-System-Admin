package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// NpcService 纯业务方法测试：
// BuildFieldSnapshot / ValidateBehaviorRefs / BuildDetail。
//
// 均无 store/cache 依赖。Properties 用 `{"constraints":{...}}` 包外层。
// ============================================================

// mkFieldLite 构造字段元数据
func mkFieldLite(id int64, name, typ string, enabled bool, propsJSON string) model.FieldLite {
	fl := model.FieldLite{
		ID:      id,
		Name:    name,
		Label:   name,
		Type:    typ,
		Enabled: enabled,
	}
	if propsJSON != "" {
		fl.Properties = json.RawMessage(propsJSON)
	}
	return fl
}

// --- BuildFieldSnapshot ---

func TestBuildFieldSnapshot_Happy(t *testing.T) {
	s := &NpcService{}

	tplEntries := []model.TemplateFieldEntry{
		{FieldID: 1, Required: true},
		{FieldID: 2, Required: false},
	}
	fieldLites := []model.FieldLite{
		mkFieldLite(1, "hp", "integer", true, `{"constraints":{"min":0,"max":1000}}`),
		mkFieldLite(2, "name", "string", true, ""),
	}
	values := []model.NPCFieldValue{
		{FieldID: 1, Value: json.RawMessage(`100`)},
		{FieldID: 2, Value: json.RawMessage(`"小明"`)},
	}

	snapshot, err := s.BuildFieldSnapshot(tplEntries, fieldLites, values)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(snapshot) != 2 {
		t.Fatalf("want len=2, got %d", len(snapshot))
	}
	// 模板顺序保留
	if snapshot[0].FieldID != 1 || snapshot[0].Name != "hp" || !snapshot[0].Required {
		t.Errorf("snapshot[0] 错: %+v", snapshot[0])
	}
	if string(snapshot[0].Value) != "100" {
		t.Errorf("snapshot[0].Value 错: %s", snapshot[0].Value)
	}
	if snapshot[1].FieldID != 2 || snapshot[1].Name != "name" {
		t.Errorf("snapshot[1] 错: %+v", snapshot[1])
	}
}

func TestBuildFieldSnapshot_MissingFieldMeta_LogsAndStubsNull(t *testing.T) {
	s := &NpcService{}

	tplEntries := []model.TemplateFieldEntry{
		{FieldID: 99, Required: true}, // 模板指向一个不存在的字段元数据
	}
	snapshot, err := s.BuildFieldSnapshot(tplEntries, nil, nil)
	if err != nil {
		t.Fatalf("want nil err (warn 路径), got %v", err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("want len=1, got %d", len(snapshot))
	}
	// 缺元数据时 Name 空、Value null
	if snapshot[0].FieldID != 99 || !snapshot[0].Required || snapshot[0].Name != "" {
		t.Errorf("snapshot 错: %+v", snapshot[0])
	}
	if string(snapshot[0].Value) != "null" {
		t.Errorf("want Value=null, got %s", snapshot[0].Value)
	}
}

func TestBuildFieldSnapshot_RequiredNullReturnsError(t *testing.T) {
	s := &NpcService{}

	tplEntries := []model.TemplateFieldEntry{{FieldID: 1, Required: true}}
	fieldLites := []model.FieldLite{mkFieldLite(1, "hp", "integer", true, "")}
	// 无 value → IsJSONNull 命中
	values := []model.NPCFieldValue{{FieldID: 1, Value: nil}}

	_, err := s.BuildFieldSnapshot(tplEntries, fieldLites, values)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if err.Code != errcode.ErrNPCFieldRequired {
		t.Errorf("want code=%d, got code=%d", errcode.ErrNPCFieldRequired, err.Code)
	}
	if !strings.Contains(err.Message, "hp") {
		t.Errorf("错误消息应含字段名 hp, got %q", err.Message)
	}
}

func TestBuildFieldSnapshot_OptionalNullKeeps(t *testing.T) {
	s := &NpcService{}

	tplEntries := []model.TemplateFieldEntry{{FieldID: 1, Required: false}}
	fieldLites := []model.FieldLite{mkFieldLite(1, "nick", "string", true, "")}
	// 无 value 且非必填 → null 占位
	snapshot, err := s.BuildFieldSnapshot(tplEntries, fieldLites, nil)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if string(snapshot[0].Value) != "null" {
		t.Errorf("want null, got %s", snapshot[0].Value)
	}
	if snapshot[0].Name != "nick" {
		t.Errorf("want Name=nick, got %q", snapshot[0].Name)
	}
}

func TestBuildFieldSnapshot_ConstraintViolation(t *testing.T) {
	s := &NpcService{}

	// integer max=100，给 999 应超限
	tplEntries := []model.TemplateFieldEntry{{FieldID: 1, Required: true}}
	fieldLites := []model.FieldLite{
		mkFieldLite(1, "hp", "integer", true, `{"constraints":{"min":0,"max":100}}`),
	}
	values := []model.NPCFieldValue{{FieldID: 1, Value: json.RawMessage(`999`)}}

	_, err := s.BuildFieldSnapshot(tplEntries, fieldLites, values)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if err.Code != errcode.ErrNPCFieldValueInvalid {
		t.Errorf("want code=%d, got code=%d", errcode.ErrNPCFieldValueInvalid, err.Code)
	}
}

func TestBuildFieldSnapshot_EmptyTemplate_ReturnsEmpty(t *testing.T) {
	s := &NpcService{}

	snapshot, err := s.BuildFieldSnapshot(nil, nil, nil)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(snapshot) != 0 {
		t.Errorf("want empty, got %+v", snapshot)
	}
}

func TestBuildFieldSnapshot_FieldLiteIDZeroIgnored(t *testing.T) {
	s := &NpcService{}

	// FieldLite.ID=0 的元数据应被 map 过滤掉
	tplEntries := []model.TemplateFieldEntry{{FieldID: 1, Required: false}}
	fieldLites := []model.FieldLite{
		mkFieldLite(0, "invalid", "string", true, ""), // 被过滤
	}
	snapshot, err := s.BuildFieldSnapshot(tplEntries, fieldLites, nil)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	// id=1 找不到元数据 → Name 空 + null
	if snapshot[0].Name != "" || string(snapshot[0].Value) != "null" {
		t.Errorf("want Name='' Value=null, got %+v", snapshot[0])
	}
}

// --- ValidateBehaviorRefs ---

func TestValidateBehaviorRefs(t *testing.T) {
	s := &NpcService{}

	cases := []struct {
		name     string
		fsmRef   string
		btRefs   map[string]string
		states   map[string]bool
		wantCode int // 0 = nil
	}{
		{
			name:     "空 btRefs 空 fsm 放行",
			fsmRef:   "",
			btRefs:   nil,
			wantCode: 0,
		},
		{
			name:     "btRefs 非空但 fsmRef 为空 → ErrNPCBtWithoutFsm",
			fsmRef:   "",
			btRefs:   map[string]string{"Idle": "bt_idle"},
			wantCode: errcode.ErrNPCBtWithoutFsm,
		},
		{
			name:     "btRefs 里的 state 不在 fsmStates → ErrNPCBtStateInvalid",
			fsmRef:   "fsm_basic",
			btRefs:   map[string]string{"Ghost": "bt_x"},
			states:   map[string]bool{"Idle": true, "Patrol": true},
			wantCode: errcode.ErrNPCBtStateInvalid,
		},
		{
			name:     "btName 空串跳过校验",
			fsmRef:   "fsm_basic",
			btRefs:   map[string]string{"Ghost": ""}, // 空值跳过
			states:   map[string]bool{"Idle": true},
			wantCode: 0,
		},
		{
			name:     "全部合法",
			fsmRef:   "fsm_basic",
			btRefs:   map[string]string{"Idle": "bt_idle", "Patrol": "bt_patrol"},
			states:   map[string]bool{"Idle": true, "Patrol": true, "Combat": true},
			wantCode: 0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.ValidateBehaviorRefs(c.fsmRef, c.btRefs, c.states)
			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want err code=%d, got nil", c.wantCode)
			}
			if err.Code != c.wantCode {
				t.Errorf("want code=%d, got code=%d (msg=%q)", c.wantCode, err.Code, err.Message)
			}
		})
	}
}

// --- BuildDetail ---

func TestBuildDetail_Full(t *testing.T) {
	s := &NpcService{}

	npc := &model.NPC{
		ID:           42,
		Name:         "merchant_01",
		Label:        "商人",
		Description:  "城镇商店主",
		TemplateID:   7,
		TemplateName: "villager_merchant",
		Enabled:      true,
		Version:      3,
		Fields:       json.RawMessage(`[{"field_id":1,"name":"hp","required":true,"value":100},{"field_id":2,"name":"name","required":false,"value":"小张"}]`),
		FsmRef:       "fsm_civilian",
		BtRefs:       json.RawMessage(`{"Idle":"bt_chat","Flee":"bt_run"}`),
	}
	fieldLites := []model.FieldLite{
		{ID: 1, Name: "hp", Label: "生命", Type: "integer", Category: "stat", CategoryLabel: "属性", Enabled: true},
		{ID: 2, Name: "name", Label: "姓名", Type: "string", Category: "basic", CategoryLabel: "基础", Enabled: true},
	}

	detail, err := s.BuildDetail(npc, fieldLites, "商人模板")
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if detail.ID != 42 || detail.Name != "merchant_01" || detail.TemplateLabel != "商人模板" {
		t.Errorf("顶层字段错: %+v", detail)
	}
	if len(detail.Fields) != 2 {
		t.Fatalf("want fields len=2, got %d", len(detail.Fields))
	}
	if detail.Fields[0].Label != "生命" || detail.Fields[0].CategoryLabel != "属性" {
		t.Errorf("fields[0] 元数据回填失败: %+v", detail.Fields[0])
	}
	if len(detail.BtRefs) != 2 || detail.BtRefs["Idle"] != "bt_chat" {
		t.Errorf("BtRefs 解析错: %v", detail.BtRefs)
	}
}

func TestBuildDetail_FieldLiteMissingSkipped(t *testing.T) {
	s := &NpcService{}

	npc := &model.NPC{
		Fields: json.RawMessage(`[{"field_id":1,"name":"hp","value":100},{"field_id":99,"name":"ghost","value":null}]`),
		BtRefs: json.RawMessage(`{}`),
	}
	// 只给 id=1 的元数据，id=99 应被跳过
	fieldLites := []model.FieldLite{{ID: 1, Name: "hp", Type: "integer", Enabled: true}}

	detail, err := s.BuildDetail(npc, fieldLites, "")
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(detail.Fields) != 1 {
		t.Fatalf("want fields len=1 (99 应跳过), got %d", len(detail.Fields))
	}
	if detail.Fields[0].FieldID != 1 {
		t.Errorf("want id=1 留存, got %d", detail.Fields[0].FieldID)
	}
}

func TestBuildDetail_EmptyBtRefs_NormalizedToMap(t *testing.T) {
	s := &NpcService{}

	// BtRefs 为空 JSON（长度 0）→ 归一化为空 map 非 nil
	npc := &model.NPC{
		Fields: json.RawMessage(`[]`),
		BtRefs: nil,
	}
	detail, err := s.BuildDetail(npc, nil, "")
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if detail.BtRefs == nil {
		t.Fatal("want non-nil empty map")
	}
	if len(detail.BtRefs) != 0 {
		t.Errorf("want empty, got %v", detail.BtRefs)
	}
}

func TestBuildDetail_InvalidFieldsJSON_ReturnsErr(t *testing.T) {
	s := &NpcService{}

	npc := &model.NPC{
		Fields: json.RawMessage(`{not-array}`),
		BtRefs: json.RawMessage(`{}`),
	}
	_, err := s.BuildDetail(npc, nil, "")
	if err == nil {
		t.Fatal("want err, got nil")
	}
}

func TestBuildDetail_FieldLiteIDZeroIgnored(t *testing.T) {
	s := &NpcService{}

	npc := &model.NPC{
		Fields: json.RawMessage(`[{"field_id":1,"name":"hp","value":100}]`),
		BtRefs: json.RawMessage(`{}`),
	}
	// ID=0 的 lite 应不进 map → id=1 查不到 → 跳过
	fieldLites := []model.FieldLite{{ID: 0, Name: "invalid", Type: "string"}}

	detail, err := s.BuildDetail(npc, fieldLites, "")
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(detail.Fields) != 0 {
		t.Errorf("want skipped, got %+v", detail.Fields)
	}
}
