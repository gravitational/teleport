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

package clientutils

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/defaults"
)

const totalItems = defaults.DefaultChunkSize*2 + 5

type mockPaginator struct {
	accessDenied  bool
	limitExceeded bool
	pageCalls     int
}

func generatePage(start, count int) []int {
	page := make([]int, count)
	for i := range count {
		page[i] = start + i
	}
	return page
}

func limitCount(start, pageSize int) int {
	if start >= totalItems {
		return 0
	}
	if start+pageSize > totalItems {
		return totalItems - start
	}
	return pageSize
}

func nextToken(start, pageSize int) string {
	if start+pageSize > totalItems {
		return ""
	}
	return strconv.Itoa(start + pageSize)
}

func startIndex(token string) int {
	var start int
	if token != "" {
		start, _ = strconv.Atoi(token)
	}
	return start
}

func (m *mockPaginator) List(_ context.Context, pageSize int, token string) ([]int, string, error) {
	m.pageCalls++
	if m.accessDenied {
		return nil, "", trace.AccessDenied("access denied")
	}

	if m.limitExceeded {
		return nil, "", trace.LimitExceeded("page size %d exceeded the limit", pageSize)
	}

	start := startIndex(token)
	if start >= totalItems {
		return nil, "", trace.BadParameter("invalid token")
	}
	count := limitCount(start, pageSize)
	next := nextToken(start, pageSize)

	return generatePage(start, count), next, nil
}

func TestIterateResources(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		var count int
		paginator := mockPaginator{}
		err := IterateResources(context.Background(), paginator.List, func(int) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, totalItems, count)
	})
	t.Run("paginator error", func(t *testing.T) {
		paginator := mockPaginator{accessDenied: true}
		err := IterateResources(context.Background(), paginator.List, func(int) error {
			return nil
		})
		assert.Error(t, err)
	})
	t.Run("callback error", func(t *testing.T) {
		paginator := mockPaginator{}
		err := IterateResources(context.Background(), paginator.List, func(int) error {
			return trace.BadParameter("error")
		})
		assert.Error(t, err)
	})
}

func TestResources(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		paginator := mockPaginator{}
		var count int
		for _, err := range Resources(context.Background(), paginator.List) {
			count++
			require.NoError(t, err)
		}

		assert.Equal(t, totalItems, count)
		assert.Equal(t, 3, paginator.pageCalls)
	})
	t.Run("paginator error", func(t *testing.T) {
		paginator := mockPaginator{accessDenied: true}
		var count int
		for _, err := range Resources(context.Background(), paginator.List) {
			count++
			require.Error(t, err)
		}
		assert.Equal(t, 1, count)
		assert.Equal(t, 1, paginator.pageCalls)
	})

	t.Run("limit exceeded", func(t *testing.T) {
		paginator := mockPaginator{limitExceeded: true}
		var count int
		for _, err := range Resources(context.Background(), paginator.List) {
			count++
			require.Error(t, err)
		}
		assert.Equal(t, 1, count)
		assert.Equal(t, 10, paginator.pageCalls)
	})
}

func TestResourcesWithPageSize(t *testing.T) {
	t.Run("adjust page size", func(t *testing.T) {
		paginator := mockPaginator{}
		var count int
		for _, err := range ResourcesWithPageSize(context.Background(), paginator.List, 10) {
			count++
			require.NoError(t, err)
		}
		assert.Equal(t, totalItems, count)
		assert.Equal(t, 201, paginator.pageCalls)
	})
}

func TestRangeResources(t *testing.T) {
	tokenFunc := func(item int) string {
		return fmt.Sprintf("%06d", item)
	}
	t.Run("span all", func(t *testing.T) {
		paginator := mockPaginator{}
		var count int

		for _, err := range RangeResources(context.Background(), "", "", paginator.List, tokenFunc) {
			count++
			require.NoError(t, err)
		}

		assert.Equal(t, totalItems, count)
		assert.Equal(t, 3, paginator.pageCalls)
	})
	t.Run("paginator error", func(t *testing.T) {
		paginator := mockPaginator{accessDenied: true}
		var count int
		for _, err := range RangeResources(context.Background(), "", "", paginator.List, tokenFunc) {
			count++
			require.Error(t, err)
		}
		assert.Equal(t, 1, count)
		assert.Equal(t, 1, paginator.pageCalls)
	})

	t.Run("span end", func(t *testing.T) {
		paginator := mockPaginator{}
		var count int
		for _, err := range RangeResources(context.Background(), "", tokenFunc(20), paginator.List, tokenFunc) {
			count++
			require.NoError(t, err)
		}
		assert.Equal(t, 20, count)
		assert.Equal(t, 1, paginator.pageCalls)
	})

	t.Run("span start", func(t *testing.T) {
		paginator := mockPaginator{}
		var count int
		for _, err := range RangeResources(context.Background(), tokenFunc(1337), "", paginator.List, tokenFunc) {
			count++
			require.NoError(t, err)
		}
		assert.Equal(t, totalItems-1337, count)
		assert.Equal(t, 1, paginator.pageCalls)
	})

	t.Run("span range", func(t *testing.T) {
		paginator := mockPaginator{}
		var count int
		for _, err := range RangeResources(context.Background(), tokenFunc(500), tokenFunc(1500), paginator.List, tokenFunc) {
			count++
			require.NoError(t, err)
		}
		assert.Equal(t, 1500-500, count)
		assert.Equal(t, 2, paginator.pageCalls)
	})
}
