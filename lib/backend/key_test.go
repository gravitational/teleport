// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package backend_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/lib/backend"
)

func TestKey(t *testing.T) {
	k1 := backend.NewKey("test")
	k2 := backend.ExactKey("test")

	assert.NotEqual(t, k1, k2)
	assert.Equal(t, "/test", k1.String())
	assert.Equal(t, "/test/", k2.String())
}

func TestKeyString(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		key      backend.Key
	}{
		{
			name: "empty key produces empty string",
		},
		{
			name:     "empty new key produces empty string",
			key:      backend.NewKey(),
			expected: "",
		},
		{
			name:     "key with only empty string produces separator",
			key:      backend.NewKey(""),
			expected: "/",
		},
		{
			name:     "key with contents are separated",
			key:      backend.NewKey("foo", "bar", "baz", "quux"),
			expected: "/foo/bar/baz/quux",
		},
		{
			name:     "empty exact key produces separator",
			key:      backend.ExactKey(),
			expected: "/",
		},
		{
			name:     "empty string exact key produces double separator",
			key:      backend.ExactKey(""),
			expected: "//",
		},
		{
			name:     "exact key adds trailing separator",
			key:      backend.ExactKey("foo", "bar", "baz", "quux"),
			expected: "/foo/bar/baz/quux/",
		},
		{
			name:     "noend key",
			key:      backend.RangeEnd(backend.NewKey("\xFF")),
			expected: "0",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.key.String())

		})
	}
}

func TestKeyComponents(t *testing.T) {
	tests := []struct {
		name     string
		key      backend.Key
		expected []string
	}{
		{
			name: "default value has zero components",
		},
		{
			name: "empty key has zero components",
			key:  backend.NewKey(),
		},
		{
			name:     "empty exact key has empty component",
			key:      backend.ExactKey(),
			expected: []string{""},
		},
		{
			name:     "single value key has a component",
			key:      backend.NewKey("alpha"),
			expected: []string{"alpha"},
		},
		{
			name:     "multiple components",
			key:      backend.NewKey("foo", "bar", "baz"),
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "key without separator",
			key:      backend.ExactKey("foo", "bar", "baz"),
			expected: []string{"foo", "bar", "baz", ""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.key.Components())
		})
	}
}

func TestKeyScan(t *testing.T) {
	tests := []struct {
		name          string
		scan          any
		expectedError string
		expectedKey   backend.Key
	}{
		{
			name:          "invalid type int",
			scan:          123,
			expectedError: "invalid Key type int",
		},
		{
			name:          "invalid type bool",
			scan:          false,
			expectedError: "invalid Key type bool",
		},
		{
			name:        "empty string key",
			scan:        "",
			expectedKey: backend.Key{},
		},
		{
			name:        "empty byte slice key",
			scan:        []byte{},
			expectedKey: backend.Key{},
		},
		{
			name:        "populated string key",
			scan:        backend.NewKey("foo", "bar", "baz").String(),
			expectedKey: backend.NewKey("foo", "bar", "baz"),
		},
		{
			name:        "populated byte slice key",
			scan:        []byte(backend.NewKey("foo", "bar", "baz").String()),
			expectedKey: backend.NewKey("foo", "bar", "baz"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			k := new(backend.Key)
			err := k.Scan(test.scan)
			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
			assert.Equal(t, test.expectedKey, *k)
		})
	}
}

func TestKeyHasSuffix(t *testing.T) {
	tests := []struct {
		name      string
		key       backend.Key
		suffix    backend.Key
		assertion assert.BoolAssertionFunc
	}{
		{
			name:      "default key has no suffixes",
			suffix:    backend.NewKey("test"),
			assertion: assert.False,
		},
		{
			name:      "default key is suffix",
			assertion: assert.True,
		},
		{
			name:      "prefix is not a suffix",
			key:       backend.NewKey("a", "b", "c"),
			suffix:    backend.NewKey("a", "b"),
			assertion: assert.False,
		},
		{
			name:      "empty suffix",
			key:       backend.NewKey("a", "b", "c"),
			assertion: assert.True,
		},
		{
			name:      "valid multi component suffix",
			key:       backend.NewKey("a", "b", "c"),
			suffix:    backend.NewKey("b", "c"),
			assertion: assert.True,
		},
		{
			name:      "valid single component suffix",
			key:       backend.NewKey("a", "b", "c"),
			suffix:    backend.NewKey("c"),
			assertion: assert.True,
		},
		{
			name:      "equivalent keys are suffix",
			key:       backend.NewKey("a", "b", "c"),
			suffix:    backend.NewKey("a", "b", "c"),
			assertion: assert.True,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.assertion(t, test.key.HasSuffix(test.suffix))
		})
	}
}

func TestKeyHasPrefix(t *testing.T) {
	tests := []struct {
		name      string
		key       backend.Key
		prefix    backend.Key
		assertion assert.BoolAssertionFunc
	}{
		{
			name:      "default key has no prexies",
			prefix:    backend.NewKey("test"),
			assertion: assert.False,
		},
		{
			name:      "default key is prefix",
			assertion: assert.True,
		},
		{
			name:      "suffix is not a prefix",
			key:       backend.NewKey("a", "b", "c"),
			prefix:    backend.NewKey("b", "c"),
			assertion: assert.False,
		},
		{
			name:      "empty prefix",
			key:       backend.NewKey("a", "b", "c"),
			assertion: assert.True,
		},
		{
			name:      "valid multi component prefix",
			key:       backend.NewKey("a", "b", "c"),
			prefix:    backend.NewKey("a", "b"),
			assertion: assert.True,
		},
		{
			name:      "valid single component prefix",
			key:       backend.NewKey("a", "b", "c"),
			prefix:    backend.NewKey("a"),
			assertion: assert.True,
		},
		{
			name:      "equivalent keys are prefix",
			key:       backend.NewKey("a", "b", "c"),
			prefix:    backend.NewKey("a", "b", "c"),
			assertion: assert.True,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.assertion(t, test.key.HasPrefix(test.prefix))
		})
	}
}

func TestKeyTrimSuffix(t *testing.T) {
	tests := []struct {
		name     string
		key      backend.Key
		trim     backend.Key
		expected backend.Key
	}{
		{
			name: "empty key trims nothing",
			trim: backend.NewKey("a", "b"),
		},
		{
			name:     "empty trim trims nothing",
			key:      backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "non-matching trim trims nothing",
			key:      backend.NewKey("a", "b"),
			trim:     backend.NewKey("c", "d"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "prefix trim trims nothing",
			key:      backend.NewKey("a", "b", "c"),
			trim:     backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b", "c"),
		},
		{
			name:     "all trimmed on exact match",
			key:      backend.NewKey("a", "b", "c"),
			trim:     backend.NewKey("a", "b", "c"),
			expected: backend.NewKey(),
		},
		{
			name:     "partial trim",
			key:      backend.NewKey("a", "b", "c"),
			trim:     backend.NewKey("b", "c"),
			expected: backend.NewKey("a"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trimmed := test.key.TrimSuffix(test.trim)
			assert.Equal(t, test.expected, trimmed)
		})
	}
}

func TestKeyTrimPrefix(t *testing.T) {
	tests := []struct {
		name     string
		key      backend.Key
		trim     backend.Key
		expected backend.Key
	}{
		{
			name: "empty key trims nothing",
			trim: backend.NewKey("a", "b"),
		},
		{
			name:     "empty trim trims nothing",
			key:      backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "non-matching trim trims nothing",
			key:      backend.NewKey("a", "b"),
			trim:     backend.NewKey("c", "d"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "suffix trim trims nothing",
			key:      backend.NewKey("a", "b", "c"),
			trim:     backend.NewKey("b", "c"),
			expected: backend.NewKey("a", "b", "c"),
		},
		{
			name:     "all trimmed on exact match",
			key:      backend.NewKey("a", "b", "c"),
			trim:     backend.NewKey("a", "b", "c"),
			expected: backend.NewKey(),
		},
		{
			name:     "partial trim",
			key:      backend.NewKey("a", "b", "c"),
			trim:     backend.NewKey("a", "b"),
			expected: backend.NewKey("c"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trimmed := test.key.TrimPrefix(test.trim)
			assert.Equal(t, test.expected, trimmed)
		})
	}
}

func TestKeyPrependKey(t *testing.T) {
	tests := []struct {
		name     string
		key      backend.Key
		prefix   backend.Key
		expected backend.Key
	}{
		{
			name:     "empty prepend is noop",
			key:      backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "empty key is prepended",
			prefix:   backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "all with leading separators",
			key:      backend.NewKey("a", "b"),
			prefix:   backend.NewKey("1", "2"),
			expected: backend.NewKey("1", "2", "a", "b"),
		},
		{
			name:     "all without leading separators",
			key:      backend.KeyFromString("a/b"),
			prefix:   backend.KeyFromString("1/2"),
			expected: backend.KeyFromString("1/2/a/b"),
		},
		{
			name:     "base without leading separators",
			key:      backend.KeyFromString("a/b"),
			prefix:   backend.NewKey("1", "2"),
			expected: backend.KeyFromString("/1/2/a/b"),
		},
		{
			name:     "base with leading separators",
			key:      backend.NewKey("a", "b"),
			prefix:   backend.KeyFromString("1/2"),
			expected: backend.KeyFromString("1/2/a/b"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prefixed := test.key.PrependKey(test.prefix)
			assert.Equal(t, test.expected, prefixed)
		})
	}
}

func TestKeyAppendKey(t *testing.T) {
	tests := []struct {
		name     string
		key      backend.Key
		suffix   backend.Key
		expected backend.Key
	}{
		{
			name:     "empty append is noop",
			key:      backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "empty key is appended",
			suffix:   backend.NewKey("a", "b"),
			expected: backend.NewKey("a", "b"),
		},
		{
			name:     "all with leading separators",
			key:      backend.NewKey("a", "b"),
			suffix:   backend.NewKey("1", "2"),
			expected: backend.NewKey("a", "b", "1", "2"),
		},
		{
			name:     "all without leading separators",
			key:      backend.KeyFromString("a/b"),
			suffix:   backend.KeyFromString("1/2"),
			expected: backend.KeyFromString("a/b/1/2"),
		},
		{
			name:     "prefix without leading separators",
			key:      backend.KeyFromString("a/b"),
			suffix:   backend.NewKey("1", "2"),
			expected: backend.KeyFromString("a/b/1/2"),
		},
		{
			name:     "suffix with no leading separators",
			key:      backend.NewKey("a", "b"),
			suffix:   backend.KeyFromString("1/2"),
			expected: backend.KeyFromString("/a/b/1/2"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appended := test.key.AppendKey(test.suffix)
			assert.Equal(t, test.expected, appended)
		})
	}
}

func TestKeyCompare(t *testing.T) {
	tests := []struct {
		name     string
		key      backend.Key
		other    backend.Key
		expected int
	}{
		{
			name:     "equal keys",
			key:      backend.NewKey("a", "b", "c"),
			other:    backend.NewKey("a", "b", "c"),
			expected: 0,
		},
		{
			name:     "less",
			key:      backend.NewKey("a", "b", "c"),
			other:    backend.NewKey("a", "b", "d"),
			expected: -1,
		},
		{
			name:     "greater",
			key:      backend.NewKey("d", "b", "c"),
			other:    backend.NewKey("a", "b"),
			expected: 1,
		},
		{
			name:     "empty key is always less",
			other:    backend.NewKey("a", "b"),
			expected: -1,
		},
		{
			name:     "key is always greater than empty",
			key:      backend.NewKey("a", "b"),
			expected: 1,
		},
		{
			name:     "empty keys are equal",
			expected: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.key.Compare(test.other))
		})
	}
}

func TestKeyIsZero(t *testing.T) {
	assert.True(t, backend.Key{}.IsZero())
	assert.True(t, backend.NewKey().IsZero())
	assert.False(t, backend.NewKey("a", "b").IsZero())
	assert.False(t, backend.ExactKey("a", "b").IsZero())
}
