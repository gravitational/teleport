// Teleport
// Copyright (C) 2025 Gravitational, Inc.
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

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/jonboulle/clockwork"
)

var never <-chan time.Time

func immediate(t time.Time) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- t
	return ch
}

// MultiParams contains the parameters for [NewMulti].
type MultiParams struct {
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

// Multi is a ticker-like abstraction around a [*time.Timer] that's made to tick
// periodically with a potentially varying interval and optionally some jitter.
// Its use requires some care as the logic driving the ticks and the jittering
// must be explicitly invoked by the code making use of it, but uses no
// background resources. It tracks an arbitrary number of sub-intervals by key,
// allowing a single delay to be applied to multiple overlapping intervals.
type Multi[T comparable] struct {
	clock clockwork.Clock
	timer clockwork.Timer
	ch    <-chan time.Time

	heap   tickHeap[T]
	target *entry[T]

	firstInterval    time.Duration
	fixedInterval    time.Duration
	variableInterval *interval.VariableDuration

	jitter retryutils.Jitter
}

// NewMulti returns a new [*Multi]. Note that the delay starts with no subintervals
// and will not tick until at least one subinterval is added.
func NewMulti[T comparable](p MultiParams) *Multi[T] {
	if p.clock == nil {
		p.clock = clockwork.NewRealClock()
	}
	return &Multi[T]{
		clock: p.clock,

		firstInterval:    p.FirstInterval,
		fixedInterval:    p.FixedInterval,
		variableInterval: p.VariableInterval,

		jitter: p.Jitter,
	}
}

func (h *Multi[T]) Add(key T) {
	if h.target != nil {
		// move current target back into heap without advancing it
		h.heap.Push(h.target)
		h.target = nil
	}

	// add new target to the heap
	now := h.clock.Now()
	interval := h.firstInterval
	if interval < 1 {
		interval = h.interval()
	}
	entry := &entry[T]{
		tick: now.Add(interval),
		key:  key,
	}
	h.heap.Push(entry)

	// trigger the 'advance' logic to recalculate current target
	h.Advance(now)
}

func (h *Multi[T]) Remove(key T) {
	if h.target != nil && h.target.key == key {
		// if the target is the one being removed, clear it and
		// trigger the 'advance' logic to recalculate the next target
		h.target = nil
		h.Advance(h.clock.Now())
		return
	}

	// key is not the current target, remove it from the heap
	h.heap.Remove(key)
}

func (h *Multi[T]) Advance(now time.Time) {
	if h.target != nil {
		h.target.tick = now.Add(h.interval())
		h.heap.Push(h.target)
	}

	if h.heap.Len() == 0 {
		h.target = nil
		h.ch = never
		return
	}

	h.target = h.heap.Pop()
	if h.target == nil {
		h.ch = never
		return
	}

	d := h.target.tick.Sub(now)

	if d < 1 {
		h.ch = immediate(now)
		return
	}

	if h.timer == nil {
		h.timer = h.clock.NewTimer(d)
	} else {
		h.timer.Stop()
		select {
		case <-h.timer.Chan():
		default:
		}
		h.timer.Reset(d)
	}

	h.ch = h.timer.Chan()
}

func (h *Multi[T]) Elapsed() <-chan time.Time {
	if h == nil {
		return never
	}

	return h.ch
}

func (h *Multi[T]) Current() (key T) {
	if h.target == nil {
		return
	}
	return h.target.key
}

func (h *Multi[T]) interval() time.Duration {
	ivl := h.fixedInterval
	if h.variableInterval != nil {
		ivl = h.variableInterval.Duration()
	}
	if h.jitter != nil {
		ivl = h.jitter(ivl)
	}
	return ivl
}

// Stop stops the delay. Only needed for Go 1.22 and [clockwork.Clock]
// compatibility. Can be called on a nil delay, as a no-op. The delay should not
// be used afterwards.
func (h *Multi[T]) Stop() {
	if h == nil || h.timer == nil {
		return
	}

	h.timer.Stop()
}
