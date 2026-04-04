package store

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestIsDuplicateKeyError_WriteException(t *testing.T) {
	// 模拟 MongoDB duplicate key error (code 11000)
	we := mongo.WriteException{
		WriteErrors: mongo.WriteErrors{
			{Code: 11000, Message: "duplicate key error"},
		},
	}
	if !isDuplicateKeyError(we) {
		t.Error("expected isDuplicateKeyError to return true for code 11000")
	}
}

func TestIsDuplicateKeyError_OtherCode(t *testing.T) {
	we := mongo.WriteException{
		WriteErrors: mongo.WriteErrors{
			{Code: 12345, Message: "some other error"},
		},
	}
	if isDuplicateKeyError(we) {
		t.Error("expected isDuplicateKeyError to return false for non-11000 code")
	}
}

func TestIsDuplicateKeyError_FallbackString(t *testing.T) {
	err := errors.New("E11000 duplicate key error collection: npc_ai.event_types")
	if !isDuplicateKeyError(err) {
		t.Error("expected isDuplicateKeyError to return true for string fallback")
	}
}

func TestIsDuplicateKeyError_UnrelatedError(t *testing.T) {
	err := errors.New("connection timeout")
	if isDuplicateKeyError(err) {
		t.Error("expected isDuplicateKeyError to return false for unrelated error")
	}
}

func TestSentinelErrors(t *testing.T) {
	// 确认 ErrNotFound 和 ErrDuplicate 是可用的 sentinel error
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Error("ErrNotFound should match itself")
	}
	if !errors.Is(ErrDuplicate, ErrDuplicate) {
		t.Error("ErrDuplicate should match itself")
	}
	if errors.Is(ErrNotFound, ErrDuplicate) {
		t.Error("ErrNotFound should not match ErrDuplicate")
	}
}

// 编译期验证 MongoStore 实现 Store 接口
var _ Store = (*MongoStore)(nil)
