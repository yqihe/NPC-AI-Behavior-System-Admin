package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestFieldCache 用 miniredis 创建测试用 FieldCache
func newTestFieldCache(t *testing.T) (*FieldCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewFieldCache(rdb), mr
}

// TestTryLock_ReturnsNonEmptyLockID 验证获锁成功时返回非空 lockID
func TestTryLock_ReturnsNonEmptyLockID(t *testing.T) {
	cache, mr := newTestFieldCache(t)
	defer mr.Close()

	ctx := context.Background()
	lockID, err := cache.TryLock(ctx, 1, 3*time.Second)
	if err != nil {
		t.Fatalf("TryLock error: %v", err)
	}
	if lockID == "" {
		t.Fatal("expected non-empty lockID, got empty string")
	}
}

// TestTryLock_FailsWhenLockHeld 验证锁被占用时返回空 lockID 而非 error
func TestTryLock_FailsWhenLockHeld(t *testing.T) {
	cache, mr := newTestFieldCache(t)
	defer mr.Close()

	ctx := context.Background()

	// A 先获锁
	lockIDa, err := cache.TryLock(ctx, 1, 3*time.Second)
	if err != nil || lockIDa == "" {
		t.Fatalf("A TryLock failed: err=%v lockID=%q", err, lockIDa)
	}

	// B 尝试获同一 key 的锁，应返回空串
	lockIDb, err := cache.TryLock(ctx, 1, 3*time.Second)
	if err != nil {
		t.Fatalf("B TryLock unexpected error: %v", err)
	}
	if lockIDb != "" {
		t.Fatalf("B should not acquire lock, got lockID=%q", lockIDb)
	}
}

// TestUnlock_CorrectLockIDDeletesKey 验证正确 lockID 解锁后 key 被删除
func TestUnlock_CorrectLockIDDeletesKey(t *testing.T) {
	cache, mr := newTestFieldCache(t)
	defer mr.Close()

	ctx := context.Background()

	lockID, err := cache.TryLock(ctx, 1, 3*time.Second)
	if err != nil || lockID == "" {
		t.Fatalf("TryLock failed: err=%v lockID=%q", err, lockID)
	}

	cache.Unlock(ctx, 1, lockID)

	// key 应被删除，可以重新获锁
	lockID2, err := cache.TryLock(ctx, 1, 3*time.Second)
	if err != nil {
		t.Fatalf("re-TryLock error: %v", err)
	}
	if lockID2 == "" {
		t.Fatal("expected to acquire lock after Unlock, got empty lockID")
	}
}

// TestUnlock_WrongLockIDDoesNotDeleteKey 验证错误 lockID 解锁不会删除他人的锁
//
// 模拟场景：A 获锁 → TTL 超时 → B 获相同 key 锁 → A 用旧 lockID 调 Unlock → B 的锁仍存在
func TestUnlock_WrongLockIDDoesNotDeleteKey(t *testing.T) {
	cache, mr := newTestFieldCache(t)
	defer mr.Close()

	ctx := context.Background()

	// A 获锁（短 TTL，方便测试）
	lockIDA, err := cache.TryLock(ctx, 1, 100*time.Millisecond)
	if err != nil || lockIDA == "" {
		t.Fatalf("A TryLock failed: err=%v lockID=%q", err, lockIDA)
	}

	// 模拟 A 的锁 TTL 超时
	mr.FastForward(200 * time.Millisecond)

	// B 获锁（A 锁已过期）
	lockIDB, err := cache.TryLock(ctx, 1, 3*time.Second)
	if err != nil || lockIDB == "" {
		t.Fatalf("B TryLock failed: err=%v lockID=%q", err, lockIDB)
	}

	// A 用旧 lockID 调 Unlock（不应删 B 的锁）
	cache.Unlock(ctx, 1, lockIDA)

	// 验证 B 的锁仍然存在（使用正确的 key 名 fields:lock:1）
	val, err2 := mr.Get("fields:lock:1")
	if err2 != nil || val == "" {
		t.Fatalf("B's lock was incorrectly deleted by A's stale Unlock: get err=%v val=%q", err2, val)
	}
}
