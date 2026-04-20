package mysql

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

// TestListParamSchemas_VerifyLiveDB 对 docker MySQL 的实时 verify。
// 若本地未启 docker compose，测试会在 Connect 时 Skip。
func TestListParamSchemas_VerifyLiveDB(t *testing.T) {
	const dsn = "root:root@tcp(127.0.0.1:3306)/npc_ai_admin?charset=utf8mb4&parseTime=true&loc=Local"
	dialCtx, dialCancel := ctxDial()
	defer dialCancel()
	db, err := sqlx.ConnectContext(dialCtx, "mysql", dsn)
	if err != nil {
		t.Skipf("docker MySQL 未就绪，跳过：%v", err)
	}
	defer db.Close()

	store := NewBtNodeTypeStore(db)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m, err := store.ListParamSchemas(ctx)
	if err != nil {
		t.Fatalf("ListParamSchemas error: %v", err)
	}

	// R: 非 nil（即使空也应是 non-nil empty map）
	if m == nil {
		t.Fatal("ListParamSchemas returned nil map, want non-nil")
	}

	// R: 10 个 type
	expectedTypes := []string{
		"sequence", "selector", "parallel", "inverter",
		"check_bb_float", "check_bb_string", "set_bb_value", "stub_action",
		"move_to", "flee_from",
	}
	if len(m) != len(expectedTypes) {
		t.Errorf("len = %d, want %d; keys=%v", len(m), len(expectedTypes), keysOf(m))
	}
	for _, tn := range expectedTypes {
		if _, ok := m[tn]; !ok {
			t.Errorf("type %q missing from ListParamSchemas result", tn)
		}
	}

	// R: 每条 RawMessage 都是合法 JSON 且含 params 数组
	for tn, raw := range m {
		var parsed struct {
			Params []map[string]any `json:"params"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Errorf("type %q: param_schema unmarshal failed: %v; raw=%s", tn, err, string(raw))
		}
	}

	// R: move_to 必含 target_key_x + target_key_z（T2 落库内容的回归断言）
	if mt, ok := m["move_to"]; ok {
		s := string(mt)
		if !strings.Contains(s, `"target_key_x"`) || !strings.Contains(s, `"target_key_z"`) {
			t.Errorf("move_to param_schema missing expected keys; got: %s", s)
		}
	}

	// R: flee_from 必含 source_key_x + source_key_z
	if ff, ok := m["flee_from"]; ok {
		s := string(ff)
		if !strings.Contains(s, `"source_key_x"`) || !strings.Contains(s, `"source_key_z"`) {
			t.Errorf("flee_from param_schema missing expected keys; got: %s", s)
		}
	}
}

func ctxDial() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
