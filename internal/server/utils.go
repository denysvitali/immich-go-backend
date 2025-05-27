package server

func ref[T any](v T) *T {
	return &v
}
