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

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/defaults"
)

// IterateResources is a helper that iterates through each resource from all
// pages and passes them one by one to the provided callback.
func IterateResources[T any](
	ctx context.Context,
	listPageFunc func(context.Context, int, string) ([]T, string, error),
	callback func(T) error,
) error {
	var pageToken string
	for {
		page, nextToken, err := listPageFunc(ctx, defaults.DefaultChunkSize, pageToken)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, resource := range page {
			if err := callback(resource); err != nil {
				return trace.Wrap(err)
			}
		}

		if nextToken == "" {
			return nil
		}
		pageToken = nextToken
	}
}
