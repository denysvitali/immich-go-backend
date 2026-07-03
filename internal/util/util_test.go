package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPtr(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		v := 42
		p := Ptr(v)
		assert.Equal(t, &v, p)
	})

	t.Run("string", func(t *testing.T) {
		v := "hello"
		p := Ptr(v)
		assert.Equal(t, &v, p)
	})

	t.Run("bool", func(t *testing.T) {
		v := true
		p := Ptr(v)
		assert.Equal(t, &v, p)
	})
}

func TestOptionalBool(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := OptionalBool(nil)
		assert.False(t, result.Valid)
	})

	t.Run("true", func(t *testing.T) {
		b := true
		result := OptionalBool(&b)
		assert.True(t, result.Valid)
		assert.True(t, result.Bool)
	})

	t.Run("false", func(t *testing.T) {
		b := false
		result := OptionalBool(&b)
		assert.True(t, result.Valid)
		assert.False(t, result.Bool)
	})
}

func TestOptionalText(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := OptionalText(nil)
		assert.False(t, result.Valid)
	})

	t.Run("value", func(t *testing.T) {
		s := "hello"
		result := OptionalText(&s)
		assert.True(t, result.Valid)
		assert.Equal(t, "hello", result.String)
	})
}

func TestOptionalInt8(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result := OptionalInt8(nil)
		assert.False(t, result.Valid)
	})

	t.Run("value", func(t *testing.T) {
		i := int32(42)
		result := OptionalInt8(&i)
		assert.True(t, result.Valid)
		assert.Equal(t, int64(42), result.Int64)
	})
}

func TestOptionalUUID(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		result, err := OptionalUUID(nil)
		assert.NoError(t, err)
		assert.False(t, result.Valid)
	})

	t.Run("valid", func(t *testing.T) {
		s := "550e8400-e29b-41d4-a716-446655440000"
		result, err := OptionalUUID(&s)
		assert.NoError(t, err)
		assert.True(t, result.Valid)
	})

	t.Run("invalid", func(t *testing.T) {
		s := "not-a-uuid"
		_, err := OptionalUUID(&s)
		assert.Error(t, err)
	})
}

func TestOffset(t *testing.T) {
	tests := []struct {
		name     string
		page     int32
		size     int32
		expected int32
	}{
		{"zero page", 0, 10, 0},
		{"negative page", -1, 10, 0},
		{"page 1", 1, 10, 0},
		{"page 2", 2, 10, 10},
		{"page 3 size 20", 3, 20, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Offset(tt.page, tt.size)
			assert.Equal(t, tt.expected, result)
		})
	}
}
