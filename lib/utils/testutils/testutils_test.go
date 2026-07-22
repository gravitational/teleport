/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package testutils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestExhaustiveNonEmptyBasics tests the basic functionality of ExhaustiveNonEmpty using various
// combinations of simple types.
func TestExhaustiveNonEmptyBasics(t *testing.T) {
	t.Parallel()
	tts := []struct {
		desc   string
		value  any
		expect bool
	}{
		{
			desc:   "basic nil",
			value:  nil,
			expect: false,
		},
		{
			desc:   "nil slice",
			value:  []string(nil),
			expect: false,
		},
		{
			desc:   "empty slice",
			value:  []string{},
			expect: false,
		},
		{
			desc:   "slice with empty element",
			value:  []string{""},
			expect: false,
		},
		{
			desc:   "non-empty slice",
			value:  []string{"a"},
			expect: true,
		},
		{
			desc:   "slice with mix of empty and non-empty elements",
			value:  []string{"", "a"},
			expect: true,
		},
		{
			desc:   "nil pointer",
			value:  (*string)(nil),
			expect: false,
		},
		{
			desc:   "pointer to empty string",
			value:  new(string),
			expect: false,
		},
		{
			desc: "pointer to non-empty string",
			value: func() *string {
				s := "a"
				return &s
			}(),
			expect: true,
		},
		{
			desc:   "zero int",
			value:  int(0),
			expect: false,
		},
		{
			desc:   "non-zero int",
			value:  int(1),
			expect: true,
		},
		{
			desc:   "nil map",
			value:  map[string]string(nil),
			expect: false,
		},
		{
			desc:   "empty map",
			value:  map[string]string{},
			expect: false,
		},
		{
			desc: "map with empty value",
			value: map[string]string{
				"a": "",
			},
			expect: false,
		},
		{
			desc: "map with non-empty value",
			value: map[string]string{
				"a": "b",
			},
			expect: true,
		},
		{
			desc: "map with mix of empty and non-empty values",
			value: map[string]string{
				"a": "",
				"b": "c",
			},
			expect: true,
		},
		{
			desc:   "zero time",
			value:  time.Time{},
			expect: false,
		},
		{
			desc:   "non-zero time",
			value:  time.Now(),
			expect: true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.expect, ExhaustiveNonEmpty(tt.value), "value=%+v", tt.value)
		})
	}
}

// TestExhaustiveNonEmptyStruct tests the basic functionality of ExhaustiveNonEmpty using different struct field/nesting
// scenarios. This test also covers the behavior of struct field ignore options.
func TestExhaustiveNonEmptyStruct(t *testing.T) {
	t.Parallel()
	type Inner struct {
		Field string
	}

	type Outer struct {
		Inner
		Slice   []Inner
		Pointer *Inner
		Value   Inner
		Map     map[string]Inner
	}

	newNonEmpty := func() Outer {
		return Outer{
			Inner: Inner{
				Field: "a",
			},
			Slice: []Inner{
				{Field: "b"},
			},
			Pointer: &Inner{Field: "c"},
			Value:   Inner{Field: "d"},
			Map: map[string]Inner{
				"e": {Field: "f"},
			},
		}
	}

	tts := []struct {
		desc   string
		value  any
		ignore []string
		expect bool
	}{
		{
			desc:   "empty struct",
			value:  Outer{},
			expect: false,
		},
		{
			desc:   "non-empty struct",
			value:  newNonEmpty(),
			expect: true,
		},
		{
			desc:   "pointer to empty struct",
			value:  new(Outer),
			expect: false,
		},
		{
			desc: "pointer to non-empty struct",
			value: func() *Outer {
				v := newNonEmpty()
				return &v
			}(),
			expect: true,
		},
		{
			desc: "struct with empty embed",
			value: func() Outer {
				v := newNonEmpty()
				v.Inner = Inner{}
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with nil slice",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = nil
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with empty slice",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = []Inner{}
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with empty slice element",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = []Inner{{}}
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with nil pointer",
			value: func() Outer {
				v := newNonEmpty()
				v.Pointer = nil
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with empty pointer",
			value: func() Outer {
				v := newNonEmpty()
				v.Pointer = &Inner{}
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with empty value",
			value: func() Outer {
				v := newNonEmpty()
				v.Value = Inner{}
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with nil map",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = nil
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with empty map",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = map[string]Inner{}
				return v
			}(),
			expect: false,
		},
		{
			desc: "struct with empty map value",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = map[string]Inner{"a": {}}
				return v
			}(),
			expect: false,
		},
		{
			desc: "ignore top-level field",
			value: func() Outer {
				v := newNonEmpty()
				v.Value = Inner{}
				return v
			}(),
			ignore: []string{"Outer.Value"},
			expect: true,
		},
		{
			desc: "ignore embedded field",
			value: func() Outer {
				v := newNonEmpty()
				v.Inner = Inner{}
				return v
			}(),
			ignore: []string{"Outer.Field"}, // embedded ignores use the outer type name
			expect: true,
		},
		{
			desc: "ignore slice element field",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = []Inner{{}}
				return v
			}(),
			ignore: []string{"Inner.Field"},
			expect: true,
		},
		{
			desc: "ignore pointer field",
			value: func() Outer {
				v := newNonEmpty()
				v.Pointer = &Inner{}
				return v
			}(),
			ignore: []string{"Inner.Field"},
			expect: true,
		},
		{
			desc: "ignore map value field",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = map[string]Inner{"a": {}}
				return v
			}(),
			ignore: []string{"Inner.Field"},
			expect: true,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			require.Equal(t, tt.expect, ExhaustiveNonEmpty(tt.value, tt.ignore...), "value=%+v", tt.value)
		})
	}
}

// TestFindAllEmptyStruct tests the basic functionality of FindAllEmpty using different struct field/nesting
// scenarios. This test also covers the behavior of struct field ignore options.
func TestFindAllEmptyStruct(t *testing.T) {
	t.Parallel()
	type Inner struct {
		Field string
	}

	type Outer struct {
		Inner
		Slice   []Inner
		Pointer *Inner
		Value   Inner
		Map     map[string]Inner
	}

	newNonEmpty := func() Outer {
		return Outer{
			Inner: Inner{
				Field: "a",
			},
			Slice: []Inner{
				{Field: "b"},
			},
			Pointer: &Inner{Field: "c"},
			Value:   Inner{Field: "d"},
			Map: map[string]Inner{
				"e": {Field: "f"},
			},
		}
	}

	tts := []struct {
		desc   string
		value  any
		ignore []string
		expect []string
	}{
		{
			desc:   "empty struct",
			value:  Outer{},
			expect: []string{"Outer.Field", "Outer.Slice", "Outer.Pointer", "Outer.Value.Field", "Outer.Map"},
		},
		{
			desc:   "non-empty struct",
			value:  newNonEmpty(),
			expect: nil,
		},
		{
			desc:   "pointer to empty struct",
			value:  new(Outer),
			expect: []string{"Outer.Field", "Outer.Slice", "Outer.Pointer", "Outer.Value.Field", "Outer.Map"},
		},
		{
			desc: "pointer to non-empty struct",
			value: func() *Outer {
				v := newNonEmpty()
				return &v
			}(),
			expect: nil,
		},
		{
			desc: "struct with empty embed",
			value: func() Outer {
				v := newNonEmpty()
				v.Inner = Inner{}
				return v
			}(),
			expect: []string{"Outer.Field"},
		},
		{
			desc: "struct with nil slice",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = nil
				return v
			}(),
			expect: []string{"Outer.Slice"},
		},
		{
			desc: "struct with empty slice",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = []Inner{}
				return v
			}(),
			expect: []string{"Outer.Slice"},
		},
		{
			desc: "struct with empty slice element",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = []Inner{{}}
				return v
			}(),
			expect: []string{"Outer.Slice.0.Field"},
		},
		{
			desc: "struct with nil pointer",
			value: func() Outer {
				v := newNonEmpty()
				v.Pointer = nil
				return v
			}(),
			expect: []string{"Outer.Pointer"},
		},
		{
			desc: "struct with empty pointer",
			value: func() Outer {
				v := newNonEmpty()
				v.Pointer = &Inner{}
				return v
			}(),
			expect: []string{"Outer.Pointer.Field"},
		},
		{
			desc: "struct with empty value",
			value: func() Outer {
				v := newNonEmpty()
				v.Value = Inner{}
				return v
			}(),
			expect: []string{"Outer.Value.Field"},
		},
		{
			desc: "struct with nil map",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = nil
				return v
			}(),
			expect: []string{"Outer.Map"},
		},
		{
			desc: "struct with empty map",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = map[string]Inner{}
				return v
			}(),
			expect: []string{"Outer.Map"},
		},
		{
			desc: "struct with empty map value",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = map[string]Inner{"a": {}}
				return v
			}(),
			expect: []string{"Outer.Map.a.Field"},
		},
		{
			desc: "ignore top-level field",
			value: func() Outer {
				v := newNonEmpty()
				v.Value = Inner{}
				return v
			}(),
			ignore: []string{"Outer.Value"},
			expect: nil,
		},
		{
			desc: "ignore embedded field",
			value: func() Outer {
				v := newNonEmpty()
				v.Inner = Inner{}
				return v
			}(),
			ignore: []string{"Outer.Field"}, // embedded ignores use the outer type name
			expect: nil,
		},
		{
			desc: "ignore slice element field",
			value: func() Outer {
				v := newNonEmpty()
				v.Slice = []Inner{{}}
				return v
			}(),
			ignore: []string{"Inner.Field"},
			expect: nil,
		},
		{
			desc: "ignore pointer field",
			value: func() Outer {
				v := newNonEmpty()
				v.Pointer = &Inner{}
				return v
			}(),
			ignore: []string{"Inner.Field"},
			expect: nil,
		},
		{
			desc: "ignore map value field",
			value: func() Outer {
				v := newNonEmpty()
				v.Map = map[string]Inner{"a": {}}
				return v
			}(),
			ignore: []string{"Inner.Field"},
			expect: nil,
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			require.ElementsMatch(t, tt.expect, FindAllEmpty(tt.value, tt.ignore...), "value=%+v", tt.value)
		})
	}
}
