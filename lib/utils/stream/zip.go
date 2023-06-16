/*
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stream

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/internalutils/stream"
)

// ZipStreams is a helper for iterrate two streams and process elements in the
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

// Process processed the streams and returns an error that happened during the
// processing. Processing will stop on the first error.
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
		if z.compareKeys(leaderItem, followerItem) == -1 {
			// leader > follower - follower is missing
			if err := z.onMissing(leaderItem); err != nil {
				return err
			}

			hasLeader = z.leader.Next()
			if hasLeader {
				leaderItem = z.leader.Item()
			}
		} else if z.compareKeys(leaderItem, followerItem) == 1 {
			// leader < follower - advancde
			hasFollower = z.follower.Next()
			if hasFollower {
				followerItem = z.follower.Item()
			}
		} else {
			// leader == follower
			if err := z.onEqualKeys(leaderItem, followerItem); err != nil {
				return err
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
			return err
		}
		hasLeader = z.leader.Next()
		if hasLeader {
			leaderItem = z.leader.Item()
		}
	}

	return trace.NewAggregate(z.leader.Done(), z.follower.Done())
}
