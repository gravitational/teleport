package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShuffleVisit(t *testing.T) {
	const N = 1000
	s := make([]int, 0, N)
	for i := range N {
		s = append(s, i)
	}

	var out []int
	for i, v := range ShuffleVisit(s) {
		require.Equal(t, len(out), i)
		out = append(out, v)
		require.Equal(t, out, s[:len(out)])
	}
	require.Len(t, out, N)

	var fixed int
	for i, v := range s {
		if i == v {
			fixed++
		}
	}
	// this is a check with a HUGE margin of error, seeing that a perfect
	// shuffle of 1000 items would have a 1 in 99_524_607 chance of having more
	// than 10 fixed points, and the chance to have 500 or more is 1 in
	// 3.02e1134
	require.Less(t, fixed, N/2)
}
