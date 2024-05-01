package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllCombinations(t *testing.T) {
	require.Len(t, Combinations([]string{"a"}), 2)
	require.Len(t, Combinations([]string{"a", "b", "c"}), 8)
	require.Len(t, Combinations(make([]string, 5)), 32)
}
