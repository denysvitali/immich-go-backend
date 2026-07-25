package server

import "testing"

func TestPaginationOffset(t *testing.T) {
	tests := map[string]struct {
		page, size int32
		want       int32
	}{
		"first page":    {page: 1, size: 10, want: 0},
		"second page":   {page: 2, size: 10, want: 10},
		"third page":    {page: 3, size: 25, want: 50},
		"page zero":     {page: 0, size: 10, want: 0},
		"page negative": {page: -5, size: 10, want: 0},
		"size zero":     {page: 3, size: 0, want: 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if got := paginationOffset(tc.page, tc.size); got != tc.want {
				t.Errorf("paginationOffset(%d, %d) = %d, want %d", tc.page, tc.size, got, tc.want)
			}
		})
	}
}
