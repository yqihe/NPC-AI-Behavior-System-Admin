package validator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/npc-admin/backend/internal/store"
)

// SchemaValidator 基于 JSON Schema 校验 config 字段。
// schema 从指定的 MongoDB 集合中加载。
type SchemaValidator struct {
	store            store.Store
	schemaCollection string
}

// NewSchemaValidator 创建 SchemaValidator。
// schemaCollection 为存储 JSON Schema 的 MongoDB 集合名（如 "component_schemas"）。
func NewSchemaValidator(s store.Store, schemaCollection string) *SchemaValidator {
	return &SchemaValidator{
		store:            s,
		schemaCollection: schemaCollection,
	}
}

// ValidateByName 根据 schema 名称校验 config。
// 从 schemaCollection 中查找 name 对应的文档，取其 config.schema 字段作为 JSON Schema，
// 校验 data 是否符合该 schema。
// 如果 schema 不存在，跳过校验（记 Debug 日志）。
func (v *SchemaValidator) ValidateByName(ctx context.Context, schemaName string, data json.RawMessage) error {
	schemaDoc, err := v.store.Get(ctx, v.schemaCollection, schemaName)
	if err != nil {
		if err == store.ErrNotFound {
			slog.Debug("validator.schema_not_found", "schema", schemaName, "collection", v.schemaCollection)
			return nil // schema 不存在，跳过校验
		}
		return fmt.Errorf("validator.load_schema: %w", err)
	}

	return v.validateWithSchema(schemaDoc.Config, data)
}

// ValidateAll 校验 config，尝试匹配 schemaCollection 中所有 schema。
// 如果集合为空，跳过校验（记 Debug 日志）。
// 此方法用于无法确定具体 schema 名称时的兜底校验。
func (v *SchemaValidator) ValidateAll(ctx context.Context, data json.RawMessage) error {
	docs, err := v.store.List(ctx, v.schemaCollection)
	if err != nil {
		return fmt.Errorf("validator.list_schemas: %w", err)
	}

	if len(docs) == 0 {
		slog.Debug("validator.no_schemas", "collection", v.schemaCollection)
		return nil // 无 schema，跳过校验
	}

	// 有 schema 时暂不做自动匹配校验，等需求 2 实现组件组合校验逻辑
	slog.Debug("validator.schemas_loaded", "collection", v.schemaCollection, "count", len(docs))
	return nil
}

// validateWithSchema 使用给定的 schema 文档校验数据。
func (v *SchemaValidator) validateWithSchema(schemaConfig json.RawMessage, data json.RawMessage) error {
	// 从 schema 文档的 config 中提取 "schema" 字段
	var schemaWrapper struct {
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(schemaConfig, &schemaWrapper); err != nil {
		return &ValidationError{Errors: []string{"Schema 文档格式错误"}}
	}

	// 如果没有 schema 字段，直接用整个 config 作为 schema
	rawSchema := schemaWrapper.Schema
	if rawSchema == nil {
		rawSchema = schemaConfig
	}

	// 解析 schema 为 any
	var schemaObj any
	if err := json.Unmarshal(rawSchema, &schemaObj); err != nil {
		return &ValidationError{Errors: []string{"Schema JSON 解析失败"}}
	}

	// 编译 JSON Schema
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaObj); err != nil {
		return &ValidationError{Errors: []string{fmt.Sprintf("Schema 编译失败: %s", err.Error())}}
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return &ValidationError{Errors: []string{fmt.Sprintf("Schema 编译失败: %s", err.Error())}}
	}

	// 解析待校验数据
	var dataObj any
	if err := json.Unmarshal(data, &dataObj); err != nil {
		return &ValidationError{Errors: []string{"配置数据 JSON 格式错误"}}
	}

	// 校验
	if err := schema.Validate(dataObj); err != nil {
		vErr := &ValidationError{Errors: make([]string, 0)}
		if validationErr, ok := err.(*jsonschema.ValidationError); ok {
			collectErrors(validationErr, vErr)
		} else {
			vErr.Errors = append(vErr.Errors, fmt.Sprintf("校验失败: %s", err.Error()))
		}
		return vErr
	}

	return nil
}

// collectErrors 递归收集 JSON Schema 校验错误。
func collectErrors(ve *jsonschema.ValidationError, result *ValidationError) {
	if len(ve.Causes) == 0 {
		msg := ve.Error()
		if msg != "" {
			result.Errors = append(result.Errors, msg)
		}
		return
	}
	for _, cause := range ve.Causes {
		collectErrors(cause, result)
	}
}
