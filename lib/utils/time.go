// Copyright 2021 Gravitational, Inc
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
	"time"

	"github.com/jonboulle/clockwork"
)

// MinTTL finds min non 0 TTL duration,
// if both durations are 0, fails
func MinTTL(a, b time.Duration) time.Duration {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

// ToTTL converts expiration time to TTL duration
// relative to current time as provided by clock
func ToTTL(c clockwork.Clock, tm time.Time) time.Duration {
	now := c.Now().UTC()
	if tm.IsZero() || tm.Before(now) {
		return 0
	}
	return tm.Sub(now)
}
