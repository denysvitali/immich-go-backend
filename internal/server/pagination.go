package server

// paginationOffset converts a 1-based page number and page size into the SQL
// OFFSET the SQLC queries expect. Page 0 and negative pages are treated as the
// first page, because the Immich clients omit the parameter rather than sending
// 1 explicitly.
func paginationOffset(page, size int32) int32 {
	if page <= 0 {
		return 0
	}
	return (page - 1) * size
}
