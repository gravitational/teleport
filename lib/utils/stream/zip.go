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

type ZipStreams[T, V any] struct {
	leader      stream.Stream[T]
	follower    stream.Stream[V]
	onMissing   func(elem T) error
	onEqualKeys func(leader T, follower V) error

	// The result will be 0 if a == b, -1 if a < b, and +1 if a > b.
	compareKeys func(leader T, follower V) int
}

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
