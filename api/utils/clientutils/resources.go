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
	"iter"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
)

// IterateResources is a helper that iterates through each resource from all
// pages and passes them one by one to the provided callback.
// Deprecated: Prefer using [Resources] instead.
// TODO(tross): DELETE IN 19.0.0
func IterateResources[T any](
	ctx context.Context,
	pageFunc func(context.Context, int, string) ([]T, string, error),
	callback func(T) error,
) error {
	for item, err := range Resources(ctx, pageFunc) {
		if err != nil {
			return trace.Wrap(err)
		}

		if err := callback(item); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// rangeParams are parameters provided to [rangeInternal]
type rangeParams[T any] struct {
	// start is the minimum inclusive key in the range yielded by the iteration.
	// Empty string means start of the range.
	start string
	// end is the upper bound (exclusive) key in the range yielded by the iteration.
	// Empty string means full remainder of the range.
	end string
	// pageSize is an optional maximum number of items to retrieve via [rangeParams.pageFunc]
	// Default value is 0, in this case it is assumed the backend will impose a page size.
	pageSize int
	// pageFunc is a user provided function to retrieve a single page of items.
	pageFunc func(context.Context, int, string) ([]T, string, error)
	// keyFunc is a user provided function to retrieve a backend key for a given item.
	// This key is used when a given range end key is given to compare against.
	// Backend keys are assumed to be sorted lexigraphically.
	keyFunc func(item T) string
}

// rangeInternal is the internal implementation of resource range getter.
// The iterator will only produce an error if one is encountered retrieving a page.
func rangeInternal[T any](ctx context.Context, params rangeParams[T]) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		pageToken := params.start
		pageSize := params.pageSize
		isLookingForEnd := params.end != "" && params.keyFunc != nil

		for {
			page, nextToken, err := params.pageFunc(ctx, pageSize, pageToken)
			if err != nil {
				if trace.IsLimitExceeded(err) {
					// Cut chunkSize in half if gRPC max message size is exceeded.
					pageSize /= 2
					// This is an extremely unlikely scenario, but better to cover it anyways.
					if pageSize == 0 {
						yield(*new(T), trace.Wrap(err, "resource is too large to retrieve"))
						return
					}

					continue
				}

				yield(*new(T), trace.Wrap(err))
				return
			}

			for _, resource := range page {
				if isLookingForEnd && params.keyFunc(resource) >= params.end {
					return
				}

				if !yield(resource, nil) {
					return
				}
			}

			pageToken = nextToken
			if nextToken == "" {
				return
			}
		}
	}

}

// ResourcesWithPageSize returns an iterator over all resources from every page, limited to pageSize, produced from the pageFunc.
// The iterator will only produce an error if one is encountered retrieving a page.
func ResourcesWithPageSize[T any](ctx context.Context, pageFunc func(context.Context, int, string) ([]T, string, error), pageSize int) iter.Seq2[T, error] {
	return rangeInternal(ctx, rangeParams[T]{
		pageSize: pageSize,
		pageFunc: pageFunc,
	})
}

// Resources returns an iterator over all resources from every page produced from the pageFunc.
// The iterator will only produce an error if one is encountered retrieving a page.
func Resources[T any](ctx context.Context, pageFunc func(context.Context, int, string) ([]T, string, error)) iter.Seq2[T, error] {
	return rangeInternal(ctx, rangeParams[T]{
		pageFunc: pageFunc,
		pageSize: defaults.DefaultChunkSize,
	})
}

// RangeResources returns resources within the range [start, end).
//
// Example use:
//
//	func (c *Client) RangeFoos(ctx context.Context, start, end string) iter.Seq2[Foo, error] {
//		return clientutils.RangeResources(ctx, start, end, c.ListFoos, Foo.GetName)
//	}
func RangeResources[T any](ctx context.Context, start, end string,
	pageFunc func(context.Context, int, string) ([]T, string, error),
	keyFunc func(item T) string) iter.Seq2[T, error] {

	return rangeInternal(ctx, rangeParams[T]{
		start:    start,
		end:      end,
		pageFunc: pageFunc,
		keyFunc:  keyFunc,
		pageSize: defaults.DefaultChunkSize,
	})
}
