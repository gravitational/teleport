package clone

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func TestClone(t *testing.T) {
	ctx := context.Background()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)

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

	cloner := Cloner{
		src:      src,
		dst:      dst,
		parallel: 10,
		log:      logutils.NewPackageLogger(),
	}

	err = cloner.Clone(ctx)
	require.NoError(t, err)

	start := backend.Key("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision", "ID"))
	require.Empty(t, diff)
	require.Equal(t, itemCount, int(cloner.migrated.Load()))
	require.NoError(t, cloner.Close())
}

func TestCloneForce(t *testing.T) {
	ctx := context.Background()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)

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

	cloner := Cloner{
		src:      src,
		dst:      dst,
		parallel: 10,
		log:      logutils.NewPackageLogger(),
	}

	err = cloner.Clone(ctx)
	require.Error(t, err)

	cloner.force = true
	err = cloner.Clone(ctx)
	require.NoError(t, err)

	start := backend.Key("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision", "ID"))
	require.Empty(t, diff)
	require.Equal(t, itemCount, int(cloner.migrated.Load()))
	require.NoError(t, cloner.Close())
}
