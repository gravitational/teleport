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

package web

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummaryBuffer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		outputs        map[string][][]byte
		capacity       int
		expectedOutput map[string][]byte
		assertValidity require.BoolAssertionFunc
	}{
		{
			name: "Single node",
			outputs: map[string][][]byte{
				"node": {
					[]byte("foo"),
					[]byte("bar"),
					[]byte("baz"),
				},
			},
			capacity: 9,
			expectedOutput: map[string][]byte{
				"node": []byte("foobarbaz"),
			},
			assertValidity: require.True,
		},
		{
			name: "Single node overflow",
			outputs: map[string][][]byte{
				"node": {
					[]byte("foo"),
					[]byte("bar"),
					[]byte("baz"),
				},
			},
			capacity:       8,
			expectedOutput: nil,
			assertValidity: require.False,
		},
		{
			name: "Multiple nodes",
			outputs: map[string][][]byte{
				"node1": {
					[]byte("foo"),
					[]byte("bar"),
					[]byte("baz"),
				},
				"node2": {
					[]byte("baz"),
					[]byte("bar"),
					[]byte("foo"),
				},
				"node3": {
					[]byte("baz"),
					[]byte("baz"),
					[]byte("baz"),
				},
			},
			capacity: 30,
			expectedOutput: map[string][]byte{
				"node1": []byte("foobarbaz"),
				"node2": []byte("bazbarfoo"),
				"node3": []byte("bazbazbaz"),
			},
			assertValidity: require.True,
		},
		{
			name: "Multiple nodes overflow",
			outputs: map[string][][]byte{
				"node1": {
					[]byte("foo"),
					[]byte("bar"),
					[]byte("baz"),
				},
				"node2": {
					[]byte("baz"),
					[]byte("bar"),
					[]byte("foo"),
				},
				"node3": {
					[]byte("baz"),
					[]byte("baz"),
					[]byte("baz"),
				},
			},
			capacity:       25,
			expectedOutput: nil,
			assertValidity: require.False,
		},
		{
			name:           "No output",
			outputs:        nil,
			capacity:       10,
			expectedOutput: map[string][]byte{},
			assertValidity: require.False,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			buffer := newSummaryBuffer(tc.capacity)
			var wg sync.WaitGroup
			for node, output := range tc.outputs {
				node := node
				output := output
				wg.Add(1)
				go func() {
					defer wg.Done()
					for _, chunk := range output {
						buffer.Write(node, chunk)
					}
				}()
			}
			wg.Wait()
			output, isValid := buffer.Export()
			require.Equal(t, tc.expectedOutput, output)
			tc.assertValidity(t, isValid)
		})
	}
}
