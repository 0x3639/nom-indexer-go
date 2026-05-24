package dto

// Page is the envelope every collection endpoint returns:
//
//	{"data": [...], "pagination": {...}}
//
// Single-item endpoints return the object directly (no envelope).
type Page struct {
	Data       interface{} `json:"data"`
	Pagination Pagination  `json:"pagination"`
}

// Pagination mirrors the offset/limit shape documented in
// docs/api/pagination.md. Total is the row count of the entire matching
// set, not just the current page; computed via COUNT(*) OVER () in the
// repository List* methods.
type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// NewPage builds a Page envelope. Callers pass the already-shaped data
// slice plus the (page, page_size, total) tuple.
func NewPage(data interface{}, page, pageSize int, total int64) *Page {
	return &Page{
		Data: data,
		Pagination: Pagination{
			Page:     page,
			PageSize: pageSize,
			Total:    total,
		},
	}
}
