// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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

	for i := range itemCount {
		item := backend.Item{
			Key:   backend.NewKey(fmt.Sprintf("key-%05d", i)),
			Value: fmt.Appendf(nil, "value-%d", i),
		}
		_, err := src.Put(ctx, item)
		require.NoError(t, err)
		items[i] = item
	}

	err = backend.Clone(ctx, src, dst, 10, false)
	require.NoError(t, err)

	start := backend.NewKey("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
	require.Empty(t, diff)
	require.NoError(t, err)
	require.Len(t, result.Items, itemCount)
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

	for i := range itemCount {
		item := backend.Item{
			Key:   backend.NewKey(fmt.Sprintf("key-%05d", i)),
			Value: fmt.Appendf(nil, "value-%d", i),
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

	start := backend.NewKey("")
	result, err := dst.GetRange(ctx, start, backend.RangeEnd(start), 0)
	require.NoError(t, err)

	diff := cmp.Diff(items, result.Items, cmpopts.IgnoreFields(backend.Item{}, "Revision"), cmp.AllowUnexported(backend.Key{}))
	require.Empty(t, diff)
	require.Len(t, result.Items, itemCount)
}
