// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package jitteredticker

import (
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Params contains the parameters for [New].
type Params struct {
	// FirstInterval is the expected time between the creation of the
	// [JitteredTicker] and the first tick. It's not modified by the configured
	// jitter.
	FirstInterval time.Duration
	// FixedInterval is the interval of the ticker, unless VariableInterval is
	// set. If a jitter is configured, the interval will be jittered every tick.
	FixedInterval time.Duration
	// VariableInterval, if set, overrides FixedInterval at every tick.
	VariableInterval *interval.VariableDuration
	// Jitter is a jitter function, applied every tick (if set) to the fixed or
	// variable interval.
	Jitter retryutils.Jitter

	clock clockwork.Clock
}

// New returns a new, running [*JitteredTicker].
func New(p Params) *JitteredTicker {
	if p.clock == nil {
		p.clock = clockwork.NewRealClock()
	}
	return &JitteredTicker{
		clock: p.clock,
		timer: p.clock.NewTimer(p.FirstInterval),

		fixedInterval:    p.FixedInterval,
		variableInterval: p.VariableInterval,

		jitter: p.Jitter,
	}
}

// JitteredTicker is a ticker that ticks with some jitter. Its use requires some
// care as the logic driving the ticks and the jittering must be explicitly
// invoked by the code making use of the ticker, but uses no background
// resources (as it's essentially a small helper around a [*time.Timer]).
type JitteredTicker struct {
	clock clockwork.Clock
	timer clockwork.Timer

	fixedInterval    time.Duration
	variableInterval *interval.VariableDuration

	jitter retryutils.Jitter
}

// Next returns the channel on which the ticks are delivered. This method can be
// called on a nil ticker, resulting in a nil channel. The
// [*JitteredTicker.Advance] method must be called after receiving each tick
// from the channel.
//
//	select {
//		// other cases
//		case now := <-t.Next():
//			t.Advance(now)
//			// business logic here
//	}
func (i *JitteredTicker) Next() <-chan time.Time {
	if i == nil {
		return nil
	}
	return i.timer.Chan()
}

func (i *JitteredTicker) interval() time.Duration {
	ivl := i.fixedInterval
	if i.variableInterval != nil {
		ivl = i.variableInterval.Duration()
	}
	if i.jitter != nil {
		ivl = i.jitter(ivl)
	}
	return ivl
}

// Advance sets up the next tick of the ticker. Must be called after receiving
// from the [*JitteredTicker.Next] channel; specifically, to maintain
// compatibility with [clockwork.Clock], it must only be called with a drained
// timer channel.
func (i *JitteredTicker) Advance(now time.Time) {
	i.timer.Reset(i.interval() - i.clock.Since(now))
}

// Stop stops the ticker. Only needed for [clockwork.Clock] compatibility. Can
// be called on a nil ticker, as a no-op. The ticker should not be used
// afterwards.
func (i *JitteredTicker) Stop() {
	if i == nil {
		return
	}

	i.timer.Stop()
}
