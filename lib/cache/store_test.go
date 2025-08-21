// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

package cache

import (
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/sortcache"
)

func TestResourceStore(t *testing.T) {
	t.Parallel()
	store := newStore(
		"int",
		func(i int) int { return i },
		map[string]func(i int) string{
			"numbers":    strconv.Itoa,
			"characters": func(i int) string { return strconv.FormatUint(uint64(i), 16) },
		})

	for i := range 100 {
		require.NoError(t, store.put(i))
	}
	require.Equal(t, 100, store.len())

	zero, err := store.get("numbers", "0")
	require.NoError(t, err)
	require.Equal(t, 0, zero)

	n, err := store.get("numbers", "1000")
	require.ErrorIs(t, err, &trace.NotFoundError{Message: `"int" "1000" does not exist`})
	require.Equal(t, 0, n)

	v, err := store.get("characters", "1c")
	require.NoError(t, err)
	require.Equal(t, 28, v)

	out := slices.Collect(store.resources("numbers", "", ""))
	require.Len(t, out, 100)

	out = slices.Collect(store.resources("characters", "", ""))
	require.Len(t, out, 100)

	require.NoError(t, store.delete(0))
	_, err = store.get("numbers", "0")
	require.ErrorIs(t, err, &trace.NotFoundError{Message: `"int" "0" does not exist`})

	require.NoError(t, store.clear())

	_, err = store.get("numbers", "0")
	require.ErrorIs(t, err, &trace.NotFoundError{Message: `"int" "0" does not exist`})

	require.Zero(t, store.len())
}

type numericResource struct {
	ID    string
	Count int
}

func TestResourceStoreWithCustomCompareFns(t *testing.T) {
	t.Parallel()

	customCompareFns := map[string]func(a, b string) bool{
		"count": sortcache.NumericPrefixCompare,
	}

	store := newStore(
		"numericResource",
		func(r numericResource) numericResource { return r },
		map[string]func(numericResource) string{
			"id":    func(r numericResource) string { return r.ID },
			"count": func(r numericResource) string { return fmt.Sprintf("%d/%s", r.Count, r.ID) },
		},
		WithCustomCompareFns[numericResource](customCompareFns))

	items := []numericResource{
		{ID: "apple", Count: 100},
		{ID: "banana", Count: 2},
		{ID: "cherry", Count: 30},
		{ID: "cloud", Count: 30},
		{ID: "elephant", Count: 5},
		{ID: "dog", Count: 5},
		{ID: "fox", Count: 2},
	}

	for _, item := range items {
		require.NoError(t, store.put(item))
	}

	results := slices.Collect(store.resources("count", "", ""))

	require.Equal(t, []numericResource{
		{ID: "banana", Count: 2},
		{ID: "fox", Count: 2},
		{ID: "dog", Count: 5},
		{ID: "elephant", Count: 5},
		{ID: "cherry", Count: 30},
		{ID: "cloud", Count: 30},
		{ID: "apple", Count: 100},
	}, results)
}
