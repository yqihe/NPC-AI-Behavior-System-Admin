package validator

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateFsmConfig_Valid(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state": "Idle",
		"states": [{"name":"Idle"},{"name":"Alarmed"},{"name":"Flee"}],
		"transitions": [
			{"from":"Idle","to":"Alarmed","priority":10,"condition":{"key":"last_event_type","op":"!=","value":""}}
		]
	}`)
	if err := ValidateFsmConfig(config); err != nil {
		t.Errorf("valid config should pass, got: %v", err)
	}
}

func TestValidateFsmConfig_EmptyStates(t *testing.T) {
	config := json.RawMessage(`{"initial_state":"Idle","states":[],"transitions":[]}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for empty states")
	}
}

func TestValidateFsmConfig_InitialStateNotInList(t *testing.T) {
	config := json.RawMessage(`{"initial_state":"Missing","states":[{"name":"Idle"}],"transitions":[]}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for initial_state not in states")
	}
}

func TestValidateFsmConfig_TransitionRefMissing(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"}],
		"transitions":[{"from":"Idle","to":"Ghost","priority":5,"condition":{"key":"x","op":"==","value":""}}]
	}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for transition to non-existent state")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestValidateFsmConfig_PriorityZero(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"X"}],
		"transitions":[{"from":"Idle","to":"X","priority":0,"condition":{"key":"x","op":"==","value":""}}]
	}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for priority=0")
	}
}

func TestValidateFsmConfig_AndCondition(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"Flee"}],
		"transitions":[{"from":"Idle","to":"Flee","priority":10,"condition":{"and":[{"key":"threat_level","op":">=","value":50}]}}]
	}`)
	if err := ValidateFsmConfig(config); err != nil {
		t.Errorf("valid AND condition should pass, got: %v", err)
	}
}

func TestValidateFsmConfig_ConditionMissingKey(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"X"}],
		"transitions":[{"from":"Idle","to":"X","priority":5,"condition":{"op":"==","value":""}}]
	}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for condition missing key")
	}
}

func TestValidateFsmConfig_ConditionWithRefKey(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"Flee"}],
		"transitions":[{"from":"Idle","to":"Flee","priority":10,"condition":{"key":"threat_expire_at","op":">","ref_key":"current_time"}}]
	}`)
	if err := ValidateFsmConfig(config); err != nil {
		t.Errorf("condition with ref_key should pass, got: %v", err)
	}
}

func TestValidateFsmConfig_DuplicateStateName(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"Idle"}],
		"transitions":[]
	}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for duplicate state name")
	}
}

func TestValidateFsmConfig_InvalidOp(t *testing.T) {
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"X"}],
		"transitions":[{"from":"Idle","to":"X","priority":5,"condition":{"key":"threat_level","op":"contains","value":"test"}}]
	}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for invalid op 'contains'")
	}
}

func TestValidateFsmConfig_ConditionMixedLeafAndComposite(t *testing.T) {
	// condition 同时有 and 和 key，应该报错
	config := json.RawMessage(`{
		"initial_state":"Idle",
		"states":[{"name":"Idle"},{"name":"X"}],
		"transitions":[{"from":"Idle","to":"X","priority":5,"condition":{"and":[{"key":"threat_level","op":">=","value":50}],"key":"extra","op":"==","value":"bad"}}]
	}`)
	err := ValidateFsmConfig(config)
	if err == nil {
		t.Fatal("expected error for condition with both and and key")
	}
}
