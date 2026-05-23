package repository

// ListOpts is the canonical pagination + sort shape that every List* method
// in this package accepts. Handlers translate HTTP query parameters into it
// via internal/api/httpx.ParsePagination / ParseSort.
//
// Per-method documentation on each repository explains which column the
// Sort field reorders and what the default direction is.
type ListOpts struct {
	Limit  int    // SQL LIMIT (page_size)
	Offset int    // SQL OFFSET ((page-1)*page_size)
	Sort   string // "asc" / "desc" — anything else is treated as the method's default
}

// orderClause returns "ASC" or "DESC". Any value other than "asc" is
// treated as descending so callers don't have to validate the field.
func orderClause(sort string) string {
	if sort == "asc" {
		return "ASC"
	}
	return "DESC"
}
