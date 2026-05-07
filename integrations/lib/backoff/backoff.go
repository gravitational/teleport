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

package backoff

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Backoff is an interface to some (exponential) backoff algorithm.
type Backoff interface {
	Do(context.Context) error
}

// decorr is a "decorrelated jitter" inspired by https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/.
type decorr struct {
	base  int64
	cap   int64
	mul   int64
	sleep int64
	clock clockwork.Clock
}

// NewDecorr initializes an algorithm.
func NewDecorr(base, cap time.Duration, clock clockwork.Clock) Backoff {
	return NewDecorrWithMul(base, cap, 3, clock)
}

// NewDecorrWithMul initializes a backoff algorithm with a given multiplier.
func NewDecorrWithMul(base, cap time.Duration, mul int64, clock clockwork.Clock) Backoff {
	return &decorr{
		base:  int64(base),
		cap:   int64(cap),
		mul:   mul,
		sleep: int64(base),
		clock: clock,
	}
}

func (backoff *decorr) Do(ctx context.Context) error {
	backoff.sleep = backoff.base + rand.N(backoff.sleep*backoff.mul-backoff.base)
	if backoff.sleep > backoff.cap {
		backoff.sleep = backoff.cap
	}
	select {
	case <-backoff.clock.After(time.Duration(backoff.sleep)):
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
