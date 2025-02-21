package test

import (
	"iter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AssertSequenceLength asserts that a sequence can be read start-to-finish
// without error, and that the sequence has a specific length.
func AssertSequenceLength[T any](t assert.TestingT, expectedLength int, seq iter.Seq2[T, error]) bool {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}

	length := 0
	for _, err := range seq {
		if !assert.NoError(t, err) {
			return false
		}
		length++
	}
	return assert.Equal(t, expectedLength, length)
}

// RequireSequenceLength asserts that a sequence can be read start-to-finish
// without error, and that the sequence has a specific length.
func RequireSequenceLength[T any](t require.TestingT, expectedLength int, seq iter.Seq2[T, error]) {
	if h, ok := t.(interface{ Helper() }); ok {
		h.Helper()
	}
	if AssertSequenceLength(t, expectedLength, seq) {
		return
	}
	t.FailNow()
}
