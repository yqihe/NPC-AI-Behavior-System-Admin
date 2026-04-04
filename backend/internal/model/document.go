package model

import (
	"encoding/json"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Document 是所有 collection 的通用文档结构。
// 与游戏服务端共享同一 MongoDB，格式必须严格为 {name, config}。
type Document struct {
	Name   string          `json:"name" bson:"name"`
	Config json.RawMessage `json:"config" bson:"config"`
}

// BsonDocument 是写入 MongoDB 时使用的 BSON 文档结构。
// config 字段用 bson.Raw 保证存储为 BSON 文档而非 bytes。
type BsonDocument struct {
	Name   string   `bson:"name"`
	Config bson.Raw `bson:"config"`
}

// ToBsonDocument 将 Document 转换为 BsonDocument，用于写入 MongoDB。
// json.RawMessage → bson.Raw，保证 config 以 BSON 文档形式存储。
func ToBsonDocument(doc Document) (BsonDocument, error) {
	var raw bson.Raw
	if err := bson.UnmarshalExtJSON(doc.Config, false, &raw); err != nil {
		return BsonDocument{}, fmt.Errorf("config JSON 转 BSON 失败: %w", err)
	}
	return BsonDocument{Name: doc.Name, Config: raw}, nil
}

// FromBsonDocument 将 MongoDB 读取的 BsonDocument 转换为 Document。
// bson.Raw → json.RawMessage，供 API 响应使用。
func FromBsonDocument(bdoc BsonDocument) (Document, error) {
	jsonBytes, err := bson.MarshalExtJSON(bdoc.Config, false, false)
	if err != nil {
		return Document{}, fmt.Errorf("config BSON 转 JSON 失败: %w", err)
	}
	return Document{Name: bdoc.Name, Config: json.RawMessage(jsonBytes)}, nil
}

// ListResponse 是列表 API 的统一响应格式。
// items 用 make 初始化，保证序列化为 [] 而非 null。
type ListResponse struct {
	Items []Document `json:"items"`
}

// NewListResponse 创建 ListResponse，保证 Items 不为 nil。
func NewListResponse(docs []Document) ListResponse {
	if docs == nil {
		docs = make([]Document, 0)
	}
	return ListResponse{Items: docs}
}

// ErrorResponse 是错误 API 的统一响应格式。
type ErrorResponse struct {
	Error string `json:"error"`
}
