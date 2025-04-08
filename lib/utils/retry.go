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

package utils

import (
	"time"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// HalfJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls, and use a dedicated sharded jitter instance for
// any usecases that might scale with cluster size or request count.
var HalfJitter = retryutils.NewHalfJitter()

// SeventhJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls, and use a dedicated sharded jitter instance for
// any usecases that might scale with cluster size or request count.
var SeventhJitter = retryutils.NewSeventhJitter()

// FullJitter is a global jitter instance used for one-off jitters.
// Prefer instantiating a new jitter instance for operations that require
// repeated calls, and use a dedicated sharded jitter instance for
// any usecases that might scale with cluster size or request count.
var FullJitter = retryutils.NewFullJitter()

// NewDefaultLinear creates a linear retry with reasonable default parameters for
// attempting to restart "critical but potentially load-inducing" operations, such
// as watcher or control stream resume. Exact parameters are subject to change,
// but this retry will always be configured for automatic reset.
func NewDefaultLinear() *retryutils.Linear {
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:     FullJitter(time.Second * 10),
		Step:      time.Second * 15,
		Max:       time.Second * 90,
		Jitter:    retryutils.NewHalfJitter(),
		AutoReset: 5,
	})
	if err != nil {
		panic("default linear retry misconfigured (this is a bug)")
	}
	return retry
}
