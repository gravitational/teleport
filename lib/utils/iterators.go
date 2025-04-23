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
	"errors"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/utils/pagination"
)

// ErrStopIteration is value that signals to stop iteration from the caller injected function.
var ErrStopIteration = errors.New("stop iteration")

// ForEachOptions specifies options for ForEachResource.
type ForEachOptions struct {
	// PageSize is the number of items to fetch per page.
	PageSize int
}

// ForEachOptionFunc is a function that sets an option on ForEachOptions.
type ForEachOptionFunc func(*ForEachOptions)

// WithPageSize sets the page size option.
func WithPageSize(pageSize int) ForEachOptionFunc {
	return func(opts *ForEachOptions) {
		opts.PageSize = pageSize
	}
}

// TokenLister is a function that lists resources with a page token.
type TokenLister[T any] func(context.Context, int, string) ([]T, string, error)

// ForEachResource iterates over resources.
// Example:
//
//	count := 0
//	err := ForEachResource(ctx, svc.ListAccessLists, func(acl accesslist.AccessList) error {
//	   count++
//	   return nil
//	})
//	if err != nil {
//	   return trace.Wrap(err)
//	}
//	fmt.Printf("Total access lists: %v", count)
func ForEachResource[T any](ctx context.Context, listFn TokenLister[T], fn func(T) error, opts ...ForEachOptionFunc) error {
	var options ForEachOptions
	for _, opt := range opts {
		opt(&options)
	}
	pageToken := ""
	for {
		items, nextToken, err := listFn(ctx, options.PageSize, pageToken)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, item := range items {
			if err := fn(item); err != nil {
				if errors.Is(err, ErrStopIteration) {
					return nil
				}
				return trace.Wrap(err)
			}
		}
		if nextToken == "" {
			return nil
		}
		pageToken = nextToken
	}
}

// ListerWithPageToken is a function that lists resources with a page token.
type ListerWithPageToken[T any] func(context.Context, int, *pagination.PageRequestToken) ([]T, pagination.NextPageToken, error)

// AdaptPageTokenLister adapts a listener with page token to a lister.
func AdaptPageTokenLister[T any](listFn ListerWithPageToken[T]) TokenLister[T] {
	return func(ctx context.Context, pageSize int, pageToken string) ([]T, string, error) {
		var pageRequestToken pagination.PageRequestToken
		pageRequestToken.Update(pagination.NextPageToken(pageToken))
		resources, nextPageToken, err := listFn(ctx, pageSize, &pageRequestToken)
		if err != nil {
			return nil, "", trace.Wrap(err)
		}
		return resources, string(nextPageToken), nil
	}
}

// CollectResources collects resources.
// Example usage:
//
//	accessLists err := ForEachResource(ctx, svc.ListAccessLists)
//	fmt.Printf("Total access lists: %v", len(accessLists))
func CollectResources[T any](ctx context.Context, listFn TokenLister[T], opts ...ForEachOptionFunc) ([]T, error) {
	var results []T
	err := ForEachResource(ctx, listFn, func(item T) error {
		results = append(results, item)
		return nil
	}, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return results, nil
}
