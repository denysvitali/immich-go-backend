package ptr

import "testing"

func TestTo(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		got := To(42)
		if got == nil || *got != 42 {
			t.Fatalf("To(42) = %v, want a pointer to 42", got)
		}
	})

	t.Run("string", func(t *testing.T) {
		got := To("hello")
		if got == nil || *got != "hello" {
			t.Fatalf(`To("hello") = %v, want a pointer to "hello"`, got)
		}
	})

	t.Run("zero value is still non-nil", func(t *testing.T) {
		got := To(false)
		if got == nil {
			t.Fatal("To(false) = nil, want a pointer to false")
		}
		if *got {
			t.Error("*To(false) = true, want false")
		}
	})
}
