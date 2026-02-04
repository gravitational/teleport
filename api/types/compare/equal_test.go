/*
Copyright 2025 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package compare

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// item is a simple test type that implements IsEqual and Cloner interfaces
type item struct {
	name  string
	value int
}

func (t *item) IsEqual(other *item) bool {
	return t.name == other.name && t.value == other.value
}

func (t *item) Clone() *item {
	if t == nil {
		return nil
	}
	return &item{name: t.name, value: t.value}
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        *item
		b        *item
		opts     []EqualOption[*item]
		expected bool
	}{
		{
			name:     "equal objects",
			a:        &item{name: "test", value: 42},
			b:        &item{name: "test", value: 42},
			expected: true,
		},
		{
			name:     "different values",
			a:        &item{name: "test", value: 42},
			b:        &item{name: "test", value: 43},
			expected: false,
		},
		{
			name:     "different names",
			a:        &item{name: "test1", value: 42},
			b:        &item{name: "test2", value: 42},
			expected: false,
		},
		{
			name: "ignore value with reset function",
			a:    &item{name: "test", value: 42},
			b:    &item{name: "test", value: 100},
			opts: []EqualOption[*item]{
				WithTransform(func(obj *item) { obj.value = 0 }),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Equal(tt.a, tt.b, tt.opts...)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestEqualWithFunc(t *testing.T) {
	t.Run("custom equal func ignores value field", func(t *testing.T) {
		a := &item{name: "test", value: 42}
		b := &item{name: "test", value: 100}

		// Without custom equal func, they are not equal
		require.False(t, Equal(a, b))

		// With custom equal func that only compares names, they are equal
		result := Equal(a, b, WithEqualFunc(func(x, y *item) bool { return x.name == y.name }))
		require.True(t, result)
	})

	t.Run("combine reset fields with custom equal func", func(t *testing.T) {
		c := &item{name: "test", value: 42}
		d := &item{name: "test", value: 100}

		result := Equal(c, d,
			WithTransform(func(obj *item) { obj.value = 0 }),
			WithEqualFunc(func(x, y *item) bool { return x.name == y.name }),
		)
		require.True(t, result)
	})

	t.Run("skip clone modifies originals", func(t *testing.T) {
		a := &item{name: "test", value: 42}
		b := &item{name: "test", value: 100}

		originalValueA := a.value
		originalValueB := b.value

		result := Equal(a, b, WithTransform(func(obj *item) { obj.value = 0 }))

		require.True(t, result)
		require.Equal(t, originalValueA, a.value)
		require.Equal(t, originalValueB, b.value)

		// With WithSkipClone, originals should be modified
		result = Equal(a, b, WithSkipClone[*item](), WithTransform(func(obj *item) { obj.value = 0 }))
		require.True(t, result)
		require.NotEqual(t, originalValueA, a.value)
		require.NotEqual(t, originalValueB, b.value)
	})
}
