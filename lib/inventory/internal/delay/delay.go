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

package delay

import (
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

// Params contains the parameters for [New].
type Params struct {
	// FirstInterval is the expected time between the creation of the [Delay]
	// and the first tick. It's not modified by the configured jitter.
	FirstInterval time.Duration
	// FixedInterval is the interval of the delay, unless VariableInterval is
	// set. If a jitter is configured, the interval will be jittered every tick.
	FixedInterval time.Duration
	// VariableInterval, if set, overrides FixedInterval at every tick.
	VariableInterval *interval.VariableDuration
	// Jitter is a jitter function, applied every tick (if set) to the fixed or
	// variable interval.
	Jitter retryutils.Jitter

	clock clockwork.Clock
}

// New returns a new, running [*Delay].
func New(p Params) *Delay {
	if p.clock == nil {
		p.clock = clockwork.NewRealClock()
	}
	return &Delay{
		clock: p.clock,
		timer: p.clock.NewTimer(p.FirstInterval),

		fixedInterval:    p.FixedInterval,
		variableInterval: p.VariableInterval,

		jitter: p.Jitter,
	}
}

// Delay is a ticker-like abstraction around a [*time.Timer] that's made to tick
// periodically with a potentially varying interval and optionally some jitter.
// Its use requires some care as the logic driving the ticks and the jittering
// must be explicitly invoked by the code making use of it, but uses no
// background resources.
type Delay struct {
	clock clockwork.Clock
	timer clockwork.Timer

	fixedInterval    time.Duration
	variableInterval *interval.VariableDuration

	jitter retryutils.Jitter
}

// Elapsed returns the channel on which the ticks are delivered. This method can
// be called on a nil delay, resulting in a nil channel. The [Delay.Advance]
// method must be called after receiving a tick from the channel.
//
//	select {
//		// other cases
//		case now := <-t.Elapsed():
//			t.Advance(now)
//			// business logic here
//	}
func (i *Delay) Elapsed() <-chan time.Time {
	if i == nil {
		return nil
	}
	return i.timer.Chan()
}

func (i *Delay) interval() time.Duration {
	ivl := i.fixedInterval
	if i.variableInterval != nil {
		ivl = i.variableInterval.Duration()
	}
	if i.jitter != nil {
		ivl = i.jitter(ivl)
	}
	return ivl
}

// Advance sets up the next tick of the delay. Must be called after receiving
// from the [Delay.Elapsed] channel; specifically, to maintain compatibility
// with [clockwork.Clock], it must only be called with a drained timer channel.
// For consistency, the value passed to Advance should be the value received
// from the Elapsed channel (passing the current time will also work, but will
// not compensate for the time that passed since the last tick).
func (i *Delay) Advance(now time.Time) {
	i.timer.Reset(i.interval() - i.clock.Since(now))
}

// Reset restarts the ticker from the current time. Must only be called while
// the timer is running (i.e. it must not be called between receiving from
// [Delay.Elapsed] and calling [Delay.Advance]).
func (i *Delay) Reset() {
	// the drain is for Go earlier than 1.23 and for [clockwork.Clock]
	if !i.timer.Stop() {
		<-i.timer.Chan()
	}
	i.timer.Reset(i.interval())
}

// Stop stops the delay. Only needed for Go 1.22 and [clockwork.Clock]
// compatibility. Can be called on a nil delay, as a no-op. The delay should not
// be used afterwards.
func (i *Delay) Stop() {
	if i == nil {
		return
	}

	i.timer.Stop()
}
