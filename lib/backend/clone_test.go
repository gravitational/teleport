package backend_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestClone(t *testing.T) {
	ctx := context.Background()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer src.Close()

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer dst.Close()

	itemCount := 11111
	items := make([]backend.Item, itemCount)

	for i := 0; i < itemCount; i++ {
		item := backend.Item{
			Key:   backend.Key(fmt.Sprintf("key-%05d", i)),
			Value: []byte(fmt.Sprintf("value-%d", i)),
		}
		_, err := src.Put(ctx, item)
		require.NoError(t, err)
		items[i] = item
	}

	err = backend.Clone(ctx, src, dst, 10, false)
	require.NoError(t, err)

	start := backend.Key("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision"))
	require.Empty(t, diff)
	require.NoError(t, err)
	require.Equal(t, itemCount, len(result.Items))
}

func TestCloneForce(t *testing.T) {
	ctx := context.Background()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer src.Close()

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)
	defer dst.Close()

	itemCount := 100
	items := make([]backend.Item, itemCount)

	for i := 0; i < itemCount; i++ {
		item := backend.Item{
			Key:   backend.Key(fmt.Sprintf("key-%05d", i)),
			Value: []byte(fmt.Sprintf("value-%d", i)),
		}
		_, err := src.Put(ctx, item)
		require.NoError(t, err)
		items[i] = item
	}

	_, err = dst.Put(ctx, items[0])
	require.NoError(t, err)

	err = backend.Clone(ctx, src, dst, 10, false)
	require.Error(t, err)

	err = backend.Clone(ctx, src, dst, 10, true)
	require.NoError(t, err)

	start := backend.Key("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision"))
	require.Empty(t, diff)
	require.Equal(t, itemCount, len(result.Items))
}
