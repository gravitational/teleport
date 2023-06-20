/*
Copyright 2023 Gravitational, Inc.

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
package web

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummaryBuffer(t *testing.T) {
	tests := []struct {
		name             string
		outputs          map[string][][]byte
		capacity         int
		expectedOutput   map[string][]byte
		expectedOverflow bool
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
			expectedOverflow: false,
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
			capacity:         8,
			expectedOutput:   nil,
			expectedOverflow: true,
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
			expectedOverflow: false,
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
			capacity:         25,
			expectedOutput:   nil,
			expectedOverflow: true,
		},
		{
			name:             "No output",
			outputs:          nil,
			capacity:         10,
			expectedOutput:   map[string][]byte{},
			expectedOverflow: false,
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
			output, overflow := buffer.Export()
			require.Equal(t, tc.expectedOutput, output)
			require.Equal(t, tc.expectedOverflow, overflow)

		})
	}
}
