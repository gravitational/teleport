package web

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSummaryBuffer(t *testing.T) {
	tests := []struct {
		name     string
		outputs  map[string][][]byte
		capacity int
		expected map[string][]byte
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
			expected: map[string][]byte{
				"node": []byte("foobarbaz"),
			},
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
			capacity: 8,
			expected: nil,
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
			expected: map[string][]byte{
				"node1": []byte("foobarbaz"),
				"node2": []byte("bazbarfoo"),
				"node3": []byte("bazbazbaz"),
			},
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
			capacity: 25,
			expected: nil,
		},
		/*
			{name: "Multiple nodes"},
			{name: "No node"},
			{name: "Multiple nodes overflow"},
		*/
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
			require.Equal(t, tc.expected, buffer.Export())

		})
	}
}
