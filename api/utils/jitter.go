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

package utils

import (
	"math/rand"
	"sync"
	"time"
)

type Jitter func(time.Duration) time.Duration

// NewSeventhJitter builds a new jitter on the range [6n/7,n). Prefer smaller
// jitters such as this when jittering periodic operations (e.g. cert rotation
// checks) since large jitters result in significantly increased load.
func NewSeventhJitter() Jitter {
	var mu sync.Mutex
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(d time.Duration) time.Duration {
		// values less than 1 cause rng to panic, and some logic
		// relies on treating zero duration as non-blocking case.
		if d < 1 {
			return 0
		}
		mu.Lock()
		defer mu.Unlock()
		return (6 * d / 7) + time.Duration(rng.Int63n(int64(d))/7)
	}
}
