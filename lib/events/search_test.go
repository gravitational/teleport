package events_test

import (
	"testing"

	"github.com/gravitational/teleport/lib/events"
	"github.com/stretchr/testify/require"
)

func TestIndex(t *testing.T) {
	for _, tt := range []struct {
		s             string
		query         string
		expectMatches []int
	}{
		{
			s:             "abcdefghijklmnopqrstuvwxyz",
			query:         "abc",
			expectMatches: []int{0},
		}, {
			s:             "abcdefghijklmnopqrstuvwxyz",
			query:         "xyz",
			expectMatches: []int{23},
		}, {
			s:             "abcabcabcabcabcabc",
			query:         "abc",
			expectMatches: []int{0, 3, 6, 9, 12, 15},
		}, {
			s:             "aaaaaa",
			query:         "aa",
			expectMatches: []int{0, 1, 2, 3, 4},
		}, {
			s:             "abcdefghijklmnopqrstuvwxyz",
			query:         "1",
			expectMatches: nil,
		},
	} {
		matches := events.Index(tt.s, tt.query)
		require.Equal(t, tt.expectMatches, matches)
	}
}
