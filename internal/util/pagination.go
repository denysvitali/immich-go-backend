package util

// Offset computes the SQL offset for paginated queries.
// Page is 1-based; if page <= 0, the result is 0.
func Offset(page, size int32) int32 {
	if page <= 0 {
		return 0
	}
	return (page - 1) * size
}
