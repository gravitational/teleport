/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSlice tests sync pool holding slices - SliceSyncPool
func TestSlice(t *testing.T) {
	t.Parallel()

	pool := NewSliceSyncPool(1024)
	// having a loop is not a guarantee that the same slice
	// will be reused, but a good enough bet
	for i := 0; i < 10; i++ {
		slice := pool.Get()
		require.Len(t, slice, 1024, "Returned slice should have zero len and values")
		for i := range slice {
			require.Equal(t, slice[i], byte(0), "Each slice element is zero byte")
		}
		copy(slice, []byte("just something to fill with"))
		pool.Put(slice)
	}
}

// TestDuplicateKey tests slice deduplication via key function
func TestDeduplicateKey(t *testing.T) {
	t.Parallel()

	stringTests := []struct {
		name     string
		slice    []string
		keyFn    func(string) string
		expected []string
	}{
		{
			name:     "EmptyStringSlice",
			slice:    []string{},
			keyFn:    func(s string) string { return s },
			expected: []string{},
		},
		{
			name:     "NoStringDuplicates",
			slice:    []string{"foo", "bar", "baz"},
			keyFn:    func(s string) string { return s },
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "StringDuplicates",
			slice:    []string{"foo", "bar", "bar", "bar", "baz", "baz"},
			keyFn:    func(s string) string { return s },
			expected: []string{"foo", "bar", "baz"},
		},
		{
			name:     "StringDuplicatesWeirdKeyFn",
			slice:    []string{"foo", "bar", "bar", "bar", "baz", "baz"},
			keyFn:    func(s string) string { return "huh" },
			expected: []string{"foo"},
		},
	}
	for _, tt := range stringTests {
		t.Run(tt.name, func(t *testing.T) {
			res := DeduplicateKey(tt.slice, tt.keyFn)
			require.Equal(t, tt.expected, res)
		})
	}

	type dedupeStruct struct {
		a string
		b int
		c bool
	}
	dedupeStructKeyFn := func(d dedupeStruct) string { return fmt.Sprintf("%s:%d:%v", d.a, d.b, d.c) }
	structTests := []struct {
		name     string
		slice    []dedupeStruct
		keyFn    func(d dedupeStruct) string
		expected []dedupeStruct
	}{
		{
			name:     "EmptySlice",
			slice:    []dedupeStruct{},
			keyFn:    dedupeStructKeyFn,
			expected: []dedupeStruct{},
		},
		{
			name: "NoStructDuplicates",
			slice: []dedupeStruct{
				{a: "foo", b: 1, c: true},
				{a: "foo", b: 1, c: false},
				{a: "foo", b: 2, c: true},
				{a: "bar", b: 1, c: true},
				{a: "bar", b: 1, c: false},
				{a: "bar", b: 2, c: true},
			},
			keyFn: dedupeStructKeyFn,
			expected: []dedupeStruct{
				{a: "foo", b: 1, c: true},
				{a: "foo", b: 1, c: false},
				{a: "foo", b: 2, c: true},
				{a: "bar", b: 1, c: true},
				{a: "bar", b: 1, c: false},
				{a: "bar", b: 2, c: true},
			},
		},
	}
	for _, tt := range structTests {
		t.Run(tt.name, func(t *testing.T) {
			res := DeduplicateKey(tt.slice, tt.keyFn)
			require.Equal(t, tt.expected, res)
		})
	}
}
