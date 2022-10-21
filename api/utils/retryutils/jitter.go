// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package retryutils

import (
	"math/rand"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

// Jitter is a function which applies random jitter to a
// duration.  Used to randomize backoff values.  Must be
// safe for concurrent usage.
type Jitter func(time.Duration) time.Duration

// NewJitter builds a new default jitter (currently jitters on
// the range [d/2,d), but this is subject to change).
func NewJitter() Jitter {
	return NewHalfJitter()
}

// NewFullJitter builds a new jitter on the range [0,d). Most use-cases
// are better served by a jitter with a meaningful minimum value, but if
// the *only* purpose of the jitter is to spread out retries to the greatest
// extent possible (e.g. when retrying a CompareAndSwap operation), a full jitter
// may be appropriate.
func NewFullJitter() Jitter {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitter, _ := newJitter(1, rng)
	return jitter
}

// NewHalfJitter returns a new jitter on the range [d/2,d).  This is
// a large range and most suitable for jittering things like backoff
// operations where breaking cycles quickly is a priority.
func NewHalfJitter() Jitter {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitter, _ := newJitter(2, rng)
	return jitter
}

// NewSeventhJitter builds a new jitter on the range [6d/7,d). Prefer smaller
// jitters such as this when jittering periodic operations (e.g. cert rotation
// checks) since large jitters result in significantly increased load.
func NewSeventhJitter() Jitter {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	jitter, _ := newJitter(7, rng)
	return jitter
}

// rng is an interface implemented by math/rand.Rand. This interface
// is used in testting.
type rng interface {
	// Int63n returns, as an int64, a non-negative pseudo-random number
	// in the half-open interval [0,n). It panics if n <= 0.
	Int63n(n int64) int64
}

// newJitter builds a new jitter on the range [d*(n-1)/n,d)
// newJitter only returns an error if n < 1.
func newJitter(n time.Duration, rng rng) (Jitter, error) {
	if n < 1 {
		return nil, trace.BadParameter("newJitter expects n>=1, but got %v", n)
	}
	var mu sync.Mutex
	return func(d time.Duration) time.Duration {
		// values less than 1 cause rng to panic, and some logic
		// relies on treating zero duration as non-blocking case.
		if d < 1 {
			return 0
		}
		mu.Lock()
		defer mu.Unlock()
		return d*(n-1)/n + time.Duration(rng.Int63n(int64(d))/int64(n))
	}, nil
}
