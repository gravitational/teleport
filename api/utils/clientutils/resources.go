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

// ListAllResources is a helper that fetches all resources by iterating all
// pages.
func ListAllResources[T any](
	ctx context.Context,
	listPageFunc func(context.Context, int, string) ([]T, string, error),
) (all []T, err error) {
	var page []T
	var nextToken string
	for {
		page, nextToken, err = listPageFunc(ctx, defaults.DefaultChunkSize, nextToken)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		all = append(all, page...)
		if nextToken == "" {
			return all, nil
		}
	}
}
