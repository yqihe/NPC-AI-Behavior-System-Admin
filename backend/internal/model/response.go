package model

// Response 统一响应格式
type Response struct {
	Code    int    `json:"code"`
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// Empty 空响应体（写操作无需返回数据时使用）
type Empty struct{}

// ListData 列表数据
type ListData struct {
	Items    any   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}
