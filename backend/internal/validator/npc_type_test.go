package validator

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateNpcType_ValidNoStore(t *testing.T) {
	// store=nil 跳过引用检查，只做结构校验
	config := json.RawMessage(`{
		"type_name":"civilian",
		"fsm_ref":"civilian",
		"bt_refs":{"Idle":"civilian/idle","Alarmed":"civilian/alarmed"},
		"perception":{"visual_range":200.0,"auditory_range":500.0}
	}`)
	if err := ValidateNpcType(config, nil, nil); err != nil {
		t.Errorf("valid config should pass, got: %v", err)
	}
}

func TestValidateNpcType_EmptyTypeName(t *testing.T) {
	config := json.RawMessage(`{
		"type_name":"",
		"fsm_ref":"x",
		"bt_refs":{"Idle":"x"},
		"perception":{"visual_range":100,"auditory_range":100}
	}`)
	err := ValidateNpcType(config, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty type_name")
	}
}

func TestValidateNpcType_MissingPerception(t *testing.T) {
	config := json.RawMessage(`{"type_name":"x","fsm_ref":"x","bt_refs":{"Idle":"x"}}`)
	err := ValidateNpcType(config, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing perception")
	}
}

func TestValidateNpcType_NegativeRange(t *testing.T) {
	config := json.RawMessage(`{
		"type_name":"x",
		"fsm_ref":"x",
		"bt_refs":{"Idle":"x"},
		"perception":{"visual_range":-10,"auditory_range":100}
	}`)
	err := ValidateNpcType(config, nil, nil)
	if err == nil {
		t.Fatal("expected error for negative visual_range")
	}
}

func TestValidateNpcType_EmptyBtRefs(t *testing.T) {
	config := json.RawMessage(`{
		"type_name":"x",
		"fsm_ref":"x",
		"bt_refs":{},
		"perception":{"visual_range":100,"auditory_range":100}
	}`)
	err := ValidateNpcType(config, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty bt_refs")
	}
}

func TestValidateNpcType_MultipleErrors(t *testing.T) {
	config := json.RawMessage(`{}`)
	err := ValidateNpcType(config, nil, nil)
	if err == nil {
		t.Fatal("expected error for empty config")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	// type_name空 + perception空 + fsm_ref空 + bt_refs空 = 4
	if len(ve.Errors) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidateNpcType_ZeroRanges(t *testing.T) {
	// range=0 是合法值
	config := json.RawMessage(`{
		"type_name":"x",
		"fsm_ref":"x",
		"bt_refs":{"Idle":"x"},
		"perception":{"visual_range":0,"auditory_range":0}
	}`)
	if err := ValidateNpcType(config, nil, nil); err != nil {
		t.Errorf("zero ranges should be valid, got: %v", err)
	}
}
