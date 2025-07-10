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

// ResourcesWithPageSize returns an iterator over all resources from every page, limited to pageSize, produced from the pageFunc.
// The iterator will only produce an error if one is encountered retrieving a page.
func ResourcesWithPageSize[T any](ctx context.Context, pageFunc func(context.Context, int, string) ([]T, string, error), pageSize int) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var pageToken string
		for {
			page, nextToken, err := pageFunc(ctx, pageSize, pageToken)
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

				yield(*new(T), err)
				return
			}
			for _, resource := range page {
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

// Resources returns an iterator over all resources from every page produced from the pageFunc.
// The iterator will only produce an error if one is encountered retrieving a page.
func Resources[T any](ctx context.Context, pageFunc func(context.Context, int, string) ([]T, string, error)) iter.Seq2[T, error] {
	return ResourcesWithPageSize(ctx, pageFunc, defaults.DefaultChunkSize)
}
