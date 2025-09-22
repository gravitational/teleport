/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package sortmap

import (
	"iter"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIteration verifies the expected behavior of ascending/descending map iteration
// with and without a start key in various scenarios.
func TestIteration(t *testing.T) {
	t.Parallel()

	tts := []struct {
		name                  string
		state                 map[string]string
		start                 string
		ascending, descending []string
	}{
		{
			name:       "empty",
			state:      nil,
			start:      "",
			ascending:  nil,
			descending: nil,
		},
		{
			name:       "empty with start key",
			state:      nil,
			start:      "a",
			ascending:  nil,
			descending: nil,
		},
		{
			name: "single element",
			state: map[string]string{
				"a": "val",
			},
			start:      "",
			ascending:  []string{"a"},
			descending: []string{"a"},
		},
		{
			name:       "single element with lower start key",
			state:      map[string]string{"b": "val"},
			start:      "a",
			ascending:  []string{"b"},
			descending: nil,
		},
		{
			name:       "single element with higher start key",
			state:      map[string]string{"b": "val"},
			start:      "c",
			ascending:  nil,
			descending: []string{"b"},
		},
		{
			name: "multiple elements",
			state: map[string]string{
				"a": "val1",
				"b": "val2",
				"c": "val3",
			},
			start:      "",
			ascending:  []string{"a", "b", "c"},
			descending: []string{"c", "b", "a"},
		},
		{
			name:       "multiple elements with start key",
			state:      map[string]string{"a": "val1", "b": "val2", "c": "val3"},
			start:      "b",
			ascending:  []string{"b", "c"},
			descending: []string{"b", "a"},
		},
		{
			name:       "multiple elements with lower start key",
			state:      map[string]string{"a": "val1", "b": "val2", "c": "val3"},
			start:      "a",
			ascending:  []string{"a", "b", "c"},
			descending: []string{"a"},
		},
		{
			name:       "multiple elements with higher start key",
			state:      map[string]string{"a": "val1", "b": "val2", "c": "val3"},
			start:      "c",
			ascending:  []string{"c"},
			descending: []string{"c", "b", "a"},
		},
		{
			name:       "multiple elements with non-existent start key",
			state:      map[string]string{"a": "val1", "c": "val2", "e": "val3", "g": "val4"},
			start:      "d",
			ascending:  []string{"e", "g"},
			descending: []string{"c", "a"},
		},
		{
			name:       "multiple elements with lower non-existent start key",
			state:      map[string]string{"a": "val1", "c": "val2", "e": "val3", "g": "val4"},
			start:      "b",
			ascending:  []string{"c", "e", "g"},
			descending: []string{"a"},
		},
		{
			name:       "multiple elements with higher non-existent start key",
			state:      map[string]string{"a": "val1", "c": "val2", "e": "val3", "g": "val4"},
			start:      "f",
			ascending:  []string{"g"},
			descending: []string{"e", "c", "a"},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			m := New[string, string]()

			for k, v := range tt.state {
				m.Set(k, v)
			}

			require.Equal(t, tt.ascending, collectKeys(m.Ascend(tt.start)))
			require.Equal(t, tt.descending, collectKeys(m.Descend(tt.start)))
		})
	}
}

// TestWrites verifies the expected behavior of basic write operations.
func TestWrites(t *testing.T) {
	t.Parallel()

	type op struct {
		set    map[string]string
		del    []string
		expect map[string]string
	}

	tts := []struct {
		name string
		ops  []op
	}{
		{
			name: "empty",
			ops: []op{
				{
					set:    nil,
					del:    nil,
					expect: map[string]string{},
				},
			},
		},
		{
			name: "basic sets and delete",
			ops: []op{
				{
					set:    map[string]string{"a": "1", "b": "2"},
					del:    []string{"a"},
					expect: map[string]string{"b": "2"},
				},
			},
		},
		{
			name: "basic multi-operation",
			ops: []op{
				{
					set:    map[string]string{"a": "1", "b": "2"},
					del:    []string{"b"},
					expect: map[string]string{"a": "1"},
				},
				{
					set:    map[string]string{"c": "3"},
					del:    []string{"b"}, // no effect
					expect: map[string]string{"a": "1", "c": "3"},
				},
				{
					set:    map[string]string{"d": "4"},
					del:    []string{"a"},
					expect: map[string]string{"c": "3", "d": "4"},
				},
				{
					set:    nil,
					del:    []string{"c"},
					expect: map[string]string{"d": "4"},
				},
				{
					set:    map[string]string{"e": "5"},
					del:    []string{"d", "e"},
					expect: map[string]string{},
				},
			},
		},
		{
			name: "repeated fill and empty",
			ops: []op{
				{
					set:    map[string]string{"a": "1", "b": "2"},
					del:    nil,
					expect: map[string]string{"a": "1", "b": "2"},
				},
				{
					set:    nil,
					del:    []string{"a", "b", "c"},
					expect: map[string]string{},
				},
				{
					set:    map[string]string{"b": "3", "c": "4"},
					del:    nil,
					expect: map[string]string{"b": "3", "c": "4"},
				},
				{
					set:    nil,
					del:    []string{"a", "b", "c"},
					expect: map[string]string{},
				},
			},
		},
	}

	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			m := New[string, string]()

			for i, op := range tt.ops {
				for k, v := range op.set {
					m.Set(k, v)
				}
				for _, k := range op.del {
					m.Del(k)
				}

				require.Equal(t, op.expect, maps.Collect(m.Ascend("")), "i=%d, op=%+v", i, op)
			}
		})
	}
}

// collectKeys aggregates the keys from a Key-Value sequence, preserving order.
func collectKeys[K, V any](seq iter.Seq2[K, V]) []K {
	var keys []K
	for key, _ := range seq {
		keys = append(keys, key)
	}
	return keys
}
