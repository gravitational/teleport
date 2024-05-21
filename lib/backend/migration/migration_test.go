package migration

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
)

func TestMigration(t *testing.T) {
	ctx := context.Background()
	src, err := memory.New(memory.Config{})
	require.NoError(t, err)

	dst, err := memory.New(memory.Config{})
	require.NoError(t, err)

	itemCount := 1111
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

	migration := Migration{
		src:      src,
		dst:      dst,
		parallel: 10,
		log:      logrus.New(),
	}

	err = migration.Run(ctx)
	require.NoError(t, err)

	start := backend.Key("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision", "ID"))
	require.Empty(t, diff)
	require.Equal(t, itemCount, migration.total)
	require.Equal(t, itemCount, int(migration.migrated.Load()))
}
