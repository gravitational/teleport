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

package stream

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/internalutils/stream"
)

// ZipStreams is a helper for iterate two streams and process elements in the
// leader stream only if they don't already exists in the follower stream.
// The streams must be sorted and comparable.
type ZipStreams[T, V any] struct {
	// leader is the stream that will be leading the iteration.
	leader stream.Stream[T]
	// follower is the stream that will be following the iteration.
	follower stream.Stream[V]
	// onMissing is the function that will be called when the leader element is
	// missing in the follower stream.
	onMissing func(elem T) error
	// onEqualKeys is the function that will be called when the leader element
	// has the same key as the follower element. It allows additional processing
	// of the element.
	onEqualKeys func(leader T, follower V) error
	// compareKeys is the function that will be used to compare the keys of the
	// leader and follower elements.
	// It should return 0 if leader == follower, -1 if leader < follower, and +1 if leader > follower.
	compareKeys func(leader T, follower V) int
}

// NewZipStreams returns a new instance of ZipStreams.
func NewZipStreams[T, V any](leader stream.Stream[T], follower stream.Stream[V],
	onMissing func(elem T) error,
	onEqualKeys func(leader T, follower V) error,
	compare func(leader T, follower V) int,
) *ZipStreams[T, V] {
	return &ZipStreams[T, V]{
		leader:      leader,
		follower:    follower,
		onMissing:   onMissing,
		onEqualKeys: onEqualKeys,
		compareKeys: compare,
	}
}

// Process consumes the streams and returns an error reported by handler functions.
// Processing will stop on the first error.
func (z *ZipStreams[T, V]) Process() error {
	var leaderItem T
	var followerItem V
	hasLeader := z.leader.Next()
	hasFollower := z.follower.Next()

	if hasLeader {
		leaderItem = z.leader.Item()
	}
	if hasFollower {
		followerItem = z.follower.Item()
	}

	for hasLeader && hasFollower {
		cmp := z.compareKeys(leaderItem, followerItem)
		switch cmp {
		case -1:
			// leader > follower - follower is missing
			if err := z.onMissing(leaderItem); err != nil {
				return trace.Wrap(err)
			}

			hasLeader = z.leader.Next()
			if hasLeader {
				leaderItem = z.leader.Item()
			}
		case 1:
			// leader < follower - advancde
			hasFollower = z.follower.Next()
			if hasFollower {
				followerItem = z.follower.Item()
			}
		default:
			// leader == follower
			if err := z.onEqualKeys(leaderItem, followerItem); err != nil {
				return trace.Wrap(err)
			}
			hasLeader = z.leader.Next()
			hasFollower = z.follower.Next()
			if hasLeader {
				leaderItem = z.leader.Item()
			}
			if hasFollower {
				followerItem = z.follower.Item()
			}
		}
	}

	for hasLeader {
		if err := z.onMissing(leaderItem); err != nil {
			return trace.Wrap(err)
		}
		hasLeader = z.leader.Next()
		if hasLeader {
			leaderItem = z.leader.Item()
		}
	}

	return trace.NewAggregate(z.leader.Done(), z.follower.Done())
}
