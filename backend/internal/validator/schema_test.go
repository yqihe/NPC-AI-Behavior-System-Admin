package validator

import (
	"encoding/json"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestValidateWithSchema_Valid 验证合法数据通过校验。
func TestValidateWithSchema_Valid(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"required": ["move_type", "move_speed"],
		"properties": {
			"move_type": {"type": "string", "enum": ["wander", "patrol", "follow"]},
			"move_speed": {"type": "number"}
		}
	}`)

	data := json.RawMessage(`{"move_type": "wander", "move_speed": 3.0}`)

	v := &SchemaValidator{}
	err := v.validateWithSchema(schema, data)
	if err != nil {
		t.Errorf("expected valid data to pass, got: %v", err)
	}
}

// TestValidateWithSchema_MissingRequired 验证缺少必填字段返回 ValidationError。
func TestValidateWithSchema_MissingRequired(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"required": ["move_type", "move_speed"],
		"properties": {
			"move_type": {"type": "string"},
			"move_speed": {"type": "number"}
		}
	}`)

	data := json.RawMessage(`{"move_type": "wander"}`)

	v := &SchemaValidator{}
	err := v.validateWithSchema(schema, data)
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Errors) == 0 {
		t.Error("expected at least one error message")
	}
}

// TestValidateWithSchema_WrongType 验证类型错误返回 ValidationError。
func TestValidateWithSchema_WrongType(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"move_speed": {"type": "number"}
		}
	}`)

	data := json.RawMessage(`{"move_speed": "not_a_number"}`)

	v := &SchemaValidator{}
	err := v.validateWithSchema(schema, data)
	if err == nil {
		t.Fatal("expected validation error for wrong type")
	}
	_, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
}

// TestValidateWithSchema_InvalidEnum 验证枚举值不在允许范围内。
func TestValidateWithSchema_InvalidEnum(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"move_type": {"type": "string", "enum": ["wander", "patrol", "follow"]}
		}
	}`)

	data := json.RawMessage(`{"move_type": "teleport"}`)

	v := &SchemaValidator{}
	err := v.validateWithSchema(schema, data)
	if err == nil {
		t.Fatal("expected validation error for invalid enum value")
	}
}

// TestValidateWithSchema_WrappedSchema 验证带 "schema" 包裹层的文档。
func TestValidateWithSchema_WrappedSchema(t *testing.T) {
	// 模拟 component_schemas 集合中的文档格式：config 里有 "schema" 字段
	schemaConfig := json.RawMessage(`{
		"component": "movement",
		"display_name": "移动组件",
		"schema": {
			"type": "object",
			"required": ["move_type"],
			"properties": {
				"move_type": {"type": "string"}
			}
		}
	}`)

	data := json.RawMessage(`{"move_type": "wander"}`)

	v := &SchemaValidator{}
	err := v.validateWithSchema(schemaConfig, data)
	if err != nil {
		t.Errorf("expected valid data to pass with wrapped schema, got: %v", err)
	}
}

// TestValidateWithSchema_InvalidJSON 验证非法 JSON 数据。
func TestValidateWithSchema_InvalidJSON(t *testing.T) {
	schema := json.RawMessage(`{"type": "object"}`)
	data := json.RawMessage(`{invalid`)

	v := &SchemaValidator{}
	err := v.validateWithSchema(schema, data)
	if err == nil {
		t.Fatal("expected error for invalid JSON data")
	}
}

// TestCompilerUsesJsonSchemaLib 验证 jsonschema 库可正常编译和校验。
func TestCompilerUsesJsonSchemaLib(t *testing.T) {
	// 直接使用 jsonschema 库确认它能正常工作
	compiler := jsonschema.NewCompiler()
	schemaObj := map[string]any{
		"type":     "object",
		"required": []any{"name"},
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	if err := compiler.AddResource("test.json", schemaObj); err != nil {
		t.Fatalf("AddResource failed: %v", err)
	}
	schema, err := compiler.Compile("test.json")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// 合法数据
	if err := schema.Validate(map[string]any{"name": "test"}); err != nil {
		t.Errorf("valid data should pass: %v", err)
	}

	// 非法数据
	if err := schema.Validate(map[string]any{}); err == nil {
		t.Error("missing required field should fail")
	}
}
