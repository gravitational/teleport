package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSlice tests sync pool holding slices - SliceSyncPool
func TestSlice(t *testing.T) {
	pool := NewSliceSyncPool(1024)
	// having a loop is not a guarantee that the same slice
	// will be reused, but a good enough bet
	for i := 0; i < 10; i++ {
		slice := pool.Get()
		assert.Len(t, slice, 1024, "Returned slice should have zero len and values")
		for i := range slice {
			assert.Equal(t, slice[i], byte(0), "Each slice element is zero byte")
		}
		copy(slice, []byte("just something to fill with"))
		pool.Put(slice)
	}
}
