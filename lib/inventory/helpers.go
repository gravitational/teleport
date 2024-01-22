/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package inventory

import (
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// we use dedicated global jitters for all the intervals/retries in this
// package. we do this because our jitter usage in this package can scale by
// the number of concurrent connections to auth, making dedicated jitters a
// poor choice (high memory usage for all the rngs).
var (
	seventhJitter = retryutils.NewShardedSeventhJitter()
	halfJitter    = retryutils.NewShardedHalfJitter()
	fullJitter    = retryutils.NewShardedFullJitter()
)
