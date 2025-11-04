package events

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemBuffer(t *testing.T) {
	t.Parallel()

	buf := &MemBuffer{}

	n, err := buf.WriteAt([]byte(" likes "), 5)
	require.NoError(t, err)
	assert.Equal(t, 7, n)

	n, err = buf.WriteAt([]byte("Alice"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	n, err = buf.Write([]byte("Bob"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	assert.Equal(t, "Alice likes Bob", string(buf.Bytes()))
}
