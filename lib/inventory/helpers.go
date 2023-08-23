/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package inventory

import (
	"sync/atomic"
	"time"

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

// ninthRampingJitter modifies a jitter, applying a linear ramp-up effect to a jitter s.t. the
// first ~n jitters ramp up linearly from 1/9th the base duration to the full base duration.
// For example, say an operation typically uses FullJitter(5m) for backoff. Applying this
// modifier with n=100 would cause the first backoff to be on the range of 0-45s, the 50th
// backoff to be on the range of 0-3m, and so on, with the jitter behaving normally after the
// first 100 calls.
//
// The actual use-case here is pretty niche, we currently only use this for one thing: having
// the first few hundred inventory connections have a shorter initial announce period. This is
// a placeholder until we can get proper dynamically scaling announce rates in place.
func ninthRampingJitter(n uint64, jitter retryutils.Jitter) retryutils.Jitter {
	var counter atomic.Uint64
	n = n * 9 / 8
	counter.Store(n / 9)
	return func(d time.Duration) time.Duration {
		c := counter.Add(1)
		if c < n {
			d = (d * time.Duration(c)) / time.Duration(n)
		}
		return jitter(d)
	}
}
