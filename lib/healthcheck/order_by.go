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

package healthcheck

import (
	"iter"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
)

// OrderByTargetHealthStatus returns an iterator over resources ordered by
// health status: healthy, unknown, and unhealthy. Each group is shuffled
// to distributing load on resources.
func OrderByTargetHealthStatus[T types.TargetHealthStatusGetter](resources []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		groups := types.GroupByTargetHealthStatus(resources)
		for _, group := range [][]T{groups.Healthy, groups.Unknown, groups.Unhealthy} {
			// ShuffleVisit is used for its efficient early return and partial shuffle.
			// The whole healthy group is likely not shuffled or visited.
			// And the unknown and unhealthy groups are likely not shuffled or visited.
			for _, resource := range utils.ShuffleVisit(group) {
				if !yield(resource) {
					return
				}
			}
		}
	}
}
