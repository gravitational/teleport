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
	"math/rand/v2"
	"time"
)

// Jitter is a function which applies random jitter to a duration. Used to
// randomize backoff values. Must be safe for concurrent usage.
type Jitter func(time.Duration) time.Duration

// NewJitter returns a default jitter (currently [HalfJitter], i.e. a jitter on
// the range [d/2, d), but this is subject to change).
//
// Deprecated: use DefaultJitter directly instead.
func NewJitter() Jitter { return DefaultJitter }

// DefaultJitter is a default jitter (currently [HalfJitter], i.e. a jitter on
// the range [d/2, d), but this is subject to change).
func DefaultJitter(d time.Duration) time.Duration { return HalfJitter(d) }

// NewFullJitter returns [FullJitter], i.e. a jitter on the full [0, d) range.
//
// Deprecated: use FullJitter directly instead.
func NewFullJitter() Jitter { return FullJitter }

// NewShardedFullJitter returns [FullJitter], i.e. a jitter on the full [0, d)
// range.
//
// Deprecated: use FullJitter directly instead.
func NewShardedFullJitter() Jitter { return FullJitter }

// NewHalfJitter returns [HalfJitter], i.e. a jitter on the range [d/2, d).
//
// Deprecated: use HalfJitter directly instead.
func NewHalfJitter() Jitter { return HalfJitter }

// NewShardedHalfJitter returns [HalfJitter], i.e. a jitter on the range [d/2,
// d).
//
// Deprecated: use HalfJitter directly instead.
func NewShardedHalfJitter() Jitter { return HalfJitter }

// NewSeventhJitter returns [SeventhJitter], i.e. a jitter on the range [6d/7,
// d).
//
// Deprecated: use SeventhJitter directly instead.
func NewSeventhJitter() Jitter { return SeventhJitter }

// NewShardedSeventhJitter returns [SeventhJitter], i.e. a jitter on the range
// [6d/7, d).
//
// Deprecated: use SeventhJitter directly instead.
func NewShardedSeventhJitter() Jitter { return SeventhJitter }

// FullJitter is a jitter on the range [0, d). Most use-cases are better served
// by a jitter with a meaningful minimum value, but if the *only* purpose of the
// jitter is to spread out retries to the greatest extent possible (e.g. when
// retrying a ConditionalUpdate operation), a full jitter may be appropriate.
func FullJitter(d time.Duration) time.Duration {
	if d < 1 {
		return 0
	}

	return rand.N(d)
}

// HalfJitter is a jitter on the range [d/2, d). This is a large range and most
// suitable for jittering things like backoff operations where breaking cycles
// quickly is a priority.
func HalfJitter(d time.Duration) time.Duration {
	if d < 1 {
		return 0
	}

	frac := d / 2
	if frac < 1 {
		return d
	}

	return d - frac + rand.N(frac)
}

// SeventhJitter returns a jitter on the range [6d/7, d). Prefer smaller jitters
// such as this when jittering periodic operations (e.g. cert rotation checks)
// since large jitters result in significantly increased load.
func SeventhJitter(d time.Duration) time.Duration {
	if d < 1 {
		return 0
	}

	frac := d / 7
	if frac < 1 {
		return d
	}

	return d - frac + rand.N(frac)
}

// AdditiveSeventhJitter returns a jitter on the range [d, 8d/7).
// Not suitable for use with things that enforce a max duration (like [Linear]).
// Prefer this when jittering a rate-limit delay, to ensure that the caller
// will not be rate-limited again.
func AdditiveSeventhJitter(d time.Duration) time.Duration {
	if d < 1 {
		return 0
	}

	frac := d / 7
	if frac < 1 {
		return d
	}

	return d + rand.N(frac)
}
