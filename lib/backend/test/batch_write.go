/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
)

func testBatchWrite(t *testing.T, newBackend Constructor) {
	t.Run("BasicWrite", func(t *testing.T) {
		testBatchWriteBasic(t, newBackend)
	})

	t.Run("LargeBatch", func(t *testing.T) {
		testBatchWriteLarge(t, newBackend)
	})
}

// testBatchWriteBasic tests basic batch write functionality with a small number of items.
func testBatchWriteBasic(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	ctx := context.Background()
	prefix := MakePrefix()

	bw := backend.NewBatchWriter()

	type itemWithSetter struct {
		item   backend.Item
		setter *mockResource
	}

	items := []itemWithSetter{
		{
			item:   backend.Item{Key: prefix("item1"), Value: []byte("value1")},
			setter: &mockResource{},
		},
		{
			item:   backend.Item{Key: prefix("item2"), Value: []byte("value2")},
			setter: &mockResource{},
		},
		{
			item:   backend.Item{Key: prefix("item3"), Value: []byte("value3")},
			setter: &mockResource{},
		},
	}

	for _, iws := range items {
		bw.Add(iws.item, iws.setter)
	}

	err = bw.Execute(ctx, uut)
	require.NoError(t, err)

	for _, iws := range items {
		require.NotEmpty(t, iws.setter.revision, "revision setter should have been updated")

		got, err := uut.Get(ctx, iws.item.Key)
		require.NoError(t, err)
		require.Equal(t, iws.item.Value, got.Value)
		require.Equal(t, iws.setter.revision, got.Revision, "revision from setter should match backend revision")
	}
}

// testBatchWriteLarge tests batch write with more items than MaxAtomicWriteSize,
// ensuring proper chunking and that revisions are properly set across all chunks.
func testBatchWriteLarge(t *testing.T, newBackend Constructor) {
	uut, _, err := newBackend()
	require.NoError(t, err)
	defer func() { require.NoError(t, uut.Close()) }()

	ctx := context.Background()
	prefix := MakePrefix()

	bw := backend.NewBatchWriter()
	const itemCount = backend.MaxAtomicWriteSize * 40

	type itemWithSetter struct {
		item   backend.Item
		setter *mockResource
	}

	items := make([]itemWithSetter, itemCount)
	for i := range itemCount {
		items[i] = itemWithSetter{
			item: backend.Item{
				Key:   prefix("item", strconv.Itoa(i)),
				Value: []byte("value" + strconv.Itoa(i)),
			},
			setter: &mockResource{},
		}
		bw.Add(items[i].item, items[i].setter)
	}

	err = bw.Execute(ctx, uut)
	require.NoError(t, err)

	for i, v := range items {
		require.NotEmpty(t, v.setter.revision, "revision setter for item %d should have been updated", i)

		got, err := uut.Get(ctx, v.item.Key)
		require.NoError(t, err)
		require.Equal(t, v.item.Value, got.Value)
		require.Equal(t, v.setter.revision, got.Revision, "revision from setter for item %d should match backend revision", i)
	}
}

type mockResource struct {
	revision string
}

func (m *mockResource) SetRevision(rev string) {
	m.revision = rev
}
