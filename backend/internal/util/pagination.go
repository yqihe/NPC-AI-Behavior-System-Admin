package util

// NormalizePagination 分页参数校正（所有 List 方法共享）
func NormalizePagination(page, pageSize *int, defaultPage, defaultPageSize, maxPageSize int) {
	if *page < 1 {
		*page = defaultPage
	}
	if *pageSize < 1 {
		*pageSize = defaultPageSize
	}
	if *pageSize > maxPageSize {
		*pageSize = maxPageSize
	}
}
