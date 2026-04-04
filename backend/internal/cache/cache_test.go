package cache

import (
	"errors"
	"testing"
)

func TestErrCacheMiss(t *testing.T) {
	if !errors.Is(ErrCacheMiss, ErrCacheMiss) {
		t.Error("ErrCacheMiss should match itself")
	}
}

// 编译期验证 RedisCache 实现 Cache 接口
var _ Cache = (*RedisCache)(nil)

func TestCacheKeyFormat(t *testing.T) {
	rc := &RedisCache{}
	tests := []struct {
		collection string
		want       string
	}{
		{"event_types", "admin:event_types:list"},
		{"npc_types", "admin:npc_types:list"},
		{"fsm_configs", "admin:fsm_configs:list"},
		{"bt_trees", "admin:bt_trees:list"},
	}
	for _, tt := range tests {
		got := rc.cacheKey(tt.collection)
		if got != tt.want {
			t.Errorf("cacheKey(%q) = %q, want %q", tt.collection, got, tt.want)
		}
	}
}
