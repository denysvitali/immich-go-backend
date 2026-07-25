// Package ptr provides the one-line pointer helper that Go's standard library
// lacks. It exists so callers can pass an address of a literal or a loop-free
// expression to the many API types that model optional fields as pointers.
package ptr

// To returns a pointer to v. The pointer is always non-nil, including for the
// zero value of T, which is what distinguishes "set to zero" from "unset" in
// the optional-field DTOs this repository exchanges with the Immich clients.
func To[T any](v T) *T {
	return &v
}
