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

package backend

import (
	"time"
)

const (
	// DefaultBufferCapacity is a default circular buffer size
	// used by backends to fan out events
	DefaultBufferCapacity = 1024
	// DefaultBacklogGracePeriod is the default amount of time that the circular buffer
	// will tolerate an event backlog in one of its watchers. Value was selected to be
	// just under 1m since 1m is typically the highest rate that high volume events
	// (e.g. heartbeats) are be created. If a watcher can't catch up in under a minute,
	// it probably won't catch up.
	DefaultBacklogGracePeriod = time.Second * 59
	// DefaultCreationGracePeriod is the default amount of time time that the circular buffer
	// will wait before enforcing the backlog grace period. This is intended to give downstream
	// caches time to initialize before they start receiving events. Without this, large caches
	// may be unable to successfully initialize even if they would otherwise be able to keep up
	// with the event stream once established.
	DefaultCreationGracePeriod = DefaultBacklogGracePeriod * 3
	// DefaultPollStreamPeriod is a default event poll stream period
	DefaultPollStreamPeriod = time.Second
	// DefaultEventsTTL is a default events TTL period
	DefaultEventsTTL = 10 * time.Minute
	// DefaultRangeLimit is used to specify some very large limit when limit is not specified
	// explicitly to prevent OOM due to infinite loops or other issues along those lines.
	DefaultRangeLimit = 2_000_000
)
