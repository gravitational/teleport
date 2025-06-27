/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package utils

import (
	"context"
	"strconv"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/pagination"
)

func TestCollect(t *testing.T) {
	mock := &mockBackendLister{items: []int{1, 2, 3, 4, 5}}
	ctx := context.Background()
	results, err := CollectResources(ctx, mock.List)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2, 3, 4, 5}, results)
}

func TestCollect_WithAdaptedLister(t *testing.T) {
	mock := &mockBackendLister{items: []int{1, 2, 3, 4, 5}}
	ctx := context.Background()
	results, err := CollectResources(ctx, AdaptPageTokenLister(mock.ListWithPagination))
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3, 4, 5}, results)
}

func TestForEachResource(t *testing.T) {
	mock := &mockBackendLister{items: []int{1, 2, 3, 4, 5}}

	ctx := context.Background()
	var count int

	err := ForEachResource(ctx, mock.List, func(item int) error {
		count++
		return nil
	}, WithPageSize(2))

	require.NoError(t, err)
	require.Len(t, mock.items, 5)
}

func TestForEachResource_StopIteration(t *testing.T) {
	mock := &mockBackendLister{items: []int{1, 2, 3, 4, 5}}

	ctx := context.Background()
	var count int

	err := ForEachResource(ctx, mock.List, func(item int) error {
		count++
		if item == 3 {
			return ErrStopIteration
		}
		return nil
	}, WithPageSize(2))

	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func TestForEachResource_AdaptPageTokenLister(t *testing.T) {
	mock := &mockBackendLister{items: []int{1, 2, 3, 4, 5}}

	ctx := context.Background()
	var count int

	err := ForEachResource(ctx, AdaptPageTokenLister(mock.ListWithPagination), func(item int) error {
		count++
		return nil
	}, WithPageSize(2))

	require.NoError(t, err)
	require.Equal(t, 5, count)
}

func TestMockBackendLister_List(t *testing.T) {
	mock := &mockBackendLister{items: []int{1, 2, 3, 4, 5}}
	ctx := context.Background()

	expectedResults := [][]int{
		{1}, {2}, {3}, {4}, {5},
	}
	pageToken := ""

	for _, expected := range expectedResults {
		results, nextToken, err := mock.List(ctx, 1, pageToken)
		require.NoError(t, err)
		require.Equal(t, expected, results)
		pageToken = nextToken
	}

	require.Empty(t, pageToken)

	pageToken = ""
	results, nextToken, err := mock.List(ctx, 2, pageToken)
	require.NoError(t, err)
	require.Equal(t, []int{1, 2}, results)
	require.NotEmpty(t, nextToken)

	results, nextToken, err = mock.List(ctx, 2, nextToken)
	require.NoError(t, err)
	require.Equal(t, []int{3, 4}, results)
	require.NotEmpty(t, nextToken)

	results, nextToken, err = mock.List(ctx, 2, nextToken)
	require.NoError(t, err)
	require.Equal(t, []int{5}, results)
	require.Empty(t, nextToken)
}

type mockBackendLister struct {
	items []int
}

func (s *mockBackendLister) List(ctx context.Context, pageSize int, pageToken string) ([]int, string, error) {
	if pageToken == "" {
		pageToken = "0"
	}
	if pageSize <= 0 {
		pageSize = 2
	}
	startIndex, err := strconv.Atoi(pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	endIndex := min(startIndex+pageSize, len(s.items))
	items := s.items[startIndex:endIndex]
	if endIndex < len(s.items) {
		return items, strconv.Itoa(endIndex), nil
	}
	return items, "", nil
}

func (s *mockBackendLister) ListWithPagination(ctx context.Context, pageSize int, page *pagination.PageRequestToken) ([]int, pagination.NextPageToken, error) {
	if pageSize == 0 {
		pageSize = 1
	}
	pageToken, err := page.Consume()
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	resp, nextPage, err := s.List(ctx, pageSize, pageToken)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}
	return resp, pagination.NextPageToken(nextPage), nil
}
