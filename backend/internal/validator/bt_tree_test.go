package validator

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateBtTree_LeafNode(t *testing.T) {
	config := json.RawMessage(`{"type":"stub_action","params":{"name":"patrol","result":"success"}}`)
	if err := ValidateBtTree(config); err != nil {
		t.Errorf("valid leaf node should pass, got: %v", err)
	}
}

func TestValidateBtTree_SequenceNode(t *testing.T) {
	config := json.RawMessage(`{
		"type":"sequence",
		"children":[
			{"type":"set_bb_value","params":{"key":"x","value":"y"}},
			{"type":"stub_action","params":{"name":"run","result":"success"}}
		]
	}`)
	if err := ValidateBtTree(config); err != nil {
		t.Errorf("valid sequence should pass, got: %v", err)
	}
}

func TestValidateBtTree_UnknownType(t *testing.T) {
	config := json.RawMessage(`{"type":"unknown_node"}`)
	err := ValidateBtTree(config)
	if err == nil {
		t.Fatal("expected error for unknown node type")
	}
}

func TestValidateBtTree_CompositeNoChildren(t *testing.T) {
	config := json.RawMessage(`{"type":"sequence"}`)
	err := ValidateBtTree(config)
	if err == nil {
		t.Fatal("expected error for composite without children")
	}
}

func TestValidateBtTree_LeafNoParams(t *testing.T) {
	config := json.RawMessage(`{"type":"stub_action"}`)
	err := ValidateBtTree(config)
	if err == nil {
		t.Fatal("expected error for leaf without params")
	}
}

func TestValidateBtTree_NestedInvalid(t *testing.T) {
	config := json.RawMessage(`{
		"type":"selector",
		"children":[
			{"type":"sequence","children":[
				{"type":"bad_type"}
			]}
		]
	}`)
	err := ValidateBtTree(config)
	if err == nil {
		t.Fatal("expected error for nested invalid node")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestValidateBtTree_EmptyType(t *testing.T) {
	config := json.RawMessage(`{"type":""}`)
	err := ValidateBtTree(config)
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestValidateBtTree_InvalidJSON(t *testing.T) {
	config := json.RawMessage(`{bad}`)
	err := ValidateBtTree(config)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidateBtTree_RealPoliceEngage(t *testing.T) {
	// 来自 configs/bt_trees/police/engage.json
	config := json.RawMessage(`{
		"type":"sequence",
		"children":[
			{"type":"set_bb_value","params":{"key":"current_action","value":"draw_weapon"}},
			{"type":"stub_action","params":{"name":"equip_weapon","result":"success"}},
			{"type":"set_bb_value","params":{"key":"current_action","value":"move_to_threat"}},
			{"type":"stub_action","params":{"name":"approach_target","result":"success"}},
			{"type":"set_bb_value","params":{"key":"current_action","value":"engage_target"}},
			{"type":"stub_action","params":{"name":"engage","result":"success"}}
		]
	}`)
	if err := ValidateBtTree(config); err != nil {
		t.Errorf("real police/engage config should pass, got: %v", err)
	}
}
