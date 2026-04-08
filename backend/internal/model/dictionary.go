package model

import (
	"encoding/json"
	"time"
)

// Dictionary 字典条目
type Dictionary struct {
	ID        int64           `json:"id" db:"id"`
	GroupName string          `json:"group_name" db:"group_name"`
	Name      string          `json:"name" db:"name"`
	Label     string          `json:"label" db:"label"`
	SortOrder int             `json:"sort_order" db:"sort_order"`
	Extra     json.RawMessage `json:"extra" db:"extra"`
	Enabled   bool            `json:"enabled" db:"enabled"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// DictionaryItem 前端下拉选项（精简版）
type DictionaryItem struct {
	Name  string          `json:"name"`
	Label string          `json:"label"`
	Extra json.RawMessage `json:"extra,omitempty"`
}

// DictListRequest 字典列表请求
type DictListRequest struct {
	Group string `json:"group"`
}

// DictListResponse 字典列表响应
type DictListResponse struct {
	Items []DictionaryItem `json:"items"`
}
