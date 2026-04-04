package validator

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateEventType_Valid(t *testing.T) {
	config := json.RawMessage(`{"name":"explosion","default_severity":80,"default_ttl":15.0,"perception_mode":"auditory","range":500.0}`)
	if err := ValidateEventType(config); err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidateEventType_ZeroSeverity(t *testing.T) {
	// severity=0 是合法值，不应被拒绝
	config := json.RawMessage(`{"name":"silence","default_severity":0,"default_ttl":1.0,"perception_mode":"global","range":0}`)
	if err := ValidateEventType(config); err != nil {
		t.Errorf("severity=0 should be valid, got: %v", err)
	}
}

func TestValidateEventType_ZeroRange(t *testing.T) {
	config := json.RawMessage(`{"name":"self","default_severity":10,"default_ttl":1.0,"perception_mode":"global","range":0}`)
	if err := ValidateEventType(config); err != nil {
		t.Errorf("range=0 should be valid, got: %v", err)
	}
}

func TestValidateEventType_InvalidJSON(t *testing.T) {
	config := json.RawMessage(`{bad json}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestValidateEventType_EmptyName(t *testing.T) {
	config := json.RawMessage(`{"name":"","default_severity":50,"default_ttl":5.0,"perception_mode":"visual","range":100}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(ve.Errors) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidateEventType_SeverityOutOfRange(t *testing.T) {
	config := json.RawMessage(`{"name":"x","default_severity":101,"default_ttl":5.0,"perception_mode":"visual","range":100}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for severity > 100")
	}
}

func TestValidateEventType_NegativeTTL(t *testing.T) {
	config := json.RawMessage(`{"name":"x","default_severity":50,"default_ttl":-1.0,"perception_mode":"visual","range":100}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for negative TTL")
	}
}

func TestValidateEventType_InvalidPerceptionMode(t *testing.T) {
	config := json.RawMessage(`{"name":"x","default_severity":50,"default_ttl":5.0,"perception_mode":"telepathy","range":100}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for invalid perception mode")
	}
}

func TestValidateEventType_NegativeRange(t *testing.T) {
	config := json.RawMessage(`{"name":"x","default_severity":50,"default_ttl":5.0,"perception_mode":"visual","range":-10}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for negative range")
	}
}

func TestValidateEventType_MultipleErrors(t *testing.T) {
	// 所有字段缺失
	config := json.RawMessage(`{}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	// name空 + severity空 + ttl空 + perception_mode空 + range空 = 5个错误
	if len(ve.Errors) != 5 {
		t.Errorf("expected 5 errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidateEventType_MissingSeverity(t *testing.T) {
	// severity 字段完全缺失（nil pointer）
	config := json.RawMessage(`{"name":"x","default_ttl":5.0,"perception_mode":"visual","range":100}`)
	err := ValidateEventType(config)
	if err == nil {
		t.Fatal("expected error for missing severity")
	}
}
