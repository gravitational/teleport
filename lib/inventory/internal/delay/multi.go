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
	"fmt"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

type entry[T any] struct {
	tick time.Time
	key  T
}

func (e entry[T]) String() string {
	return fmt.Sprintf("entry{tick: %v, key: %v}", e.tick.Format(time.RFC3339Nano), e.key)
}

func entryLess[T any](a, b entry[T]) bool {
	return a.tick.Before(b.tick)
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
	// FirstJitter is the jitter applied to the first interval. It's not applied
	// to the interval after the first tick. If unset, the standard jitter is
	// applied to the first interval.
	FirstJitter retryutils.Jitter
	// Jitter is a jitter function, applied every tick (if set) to the fixed or
	// variable interval (except for the first tick if FirstJitter is set).
	Jitter retryutils.Jitter
	// ResetJitter is a jitter function, applied when a tick is reset before it
	// fires.
	ResetJitter retryutils.Jitter

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

	heap heap[entry[T]]

	fixedInterval    time.Duration
	variableInterval *interval.VariableDuration

	firstJitter retryutils.Jitter
	jitter      retryutils.Jitter
	resetJitter retryutils.Jitter
}

// NewMulti returns a new [*Multi]. Note that the delay starts with no subintervals
// and will not tick until at least one subinterval is added.
func NewMulti[T comparable](p MultiParams) *Multi[T] {
	if p.clock == nil {
		p.clock = clockwork.NewRealClock()
	}
	if p.Jitter == nil {
		p.Jitter = func(d time.Duration) time.Duration { return d }
	}
	if p.FirstJitter == nil {
		p.FirstJitter = p.Jitter
	}
	if p.ResetJitter == nil {
		p.ResetJitter = p.Jitter
	}
	return &Multi[T]{
		clock: p.clock,

		heap: heap[entry[T]]{
			Less: entryLess[T],
		},

		fixedInterval:    p.FixedInterval,
		variableInterval: p.VariableInterval,

		firstJitter: p.FirstJitter,
		jitter:      p.Jitter,
		resetJitter: p.ResetJitter,
	}
}

func (h *Multi[T]) Add(key T) {
	// add new target to the heap
	now := h.clock.Now()
	entry := entry[T]{
		tick: now.Add(h.interval(h.firstJitter)),
		key:  key,
	}
	h.heap.Push(entry)

	// trigger reset in case the new entry should be the next target
	h.reset(now, false /* fired */)
}

func (h *Multi[T]) Remove(key T) {
	// key is not the current target, remove it from the heap
	for i, entry := range h.heap.Slice {
		if entry.key == key {
			h.heap.Remove(i)
			if i == 0 {
				// if the removed entry was the root of the heap, then our target
				// has changed and we need to reset the timer to a new target.
				h.reset(h.clock.Now(), false /* fired */)
			}
			return
		}
	}
}

// Reset resets the next tick for the given key to the current time plus a delay.
func (h *Multi[T]) Reset(key T, delay time.Duration) {
	for i, item := range h.heap.Slice {
		if item.key == key {
			h.heap.Slice[i] = entry[T]{
				key:  key,
				tick: h.clock.Now().Add(h.resetJitter(delay)),
			}
			h.heap.Fix(i)
			if i == 0 {
				// if the adjusted entry was the root of the heap, then our target
				// has changed and we need to reset the timer to a new target.
				h.reset(h.clock.Now(), false /* fired */)
			}
			return
		}
	}
}

// Tick *must* be called exactly once for each firing observed on the Elapsed channel, with the time
// of the firing. Tick will advance the internal state of the multi to start targeting the next interval,
// and return the key associated with the interval that just fired.
func (h *Multi[T]) Tick(now time.Time) (key T) {
	// advance the current root entry (source of the tick), and record its
	// key for later return.
	root := h.heap.Root()
	key = root.key
	root.tick = now.Add(h.interval(h.jitter))

	// fix the heap ordering to reflect the updated state
	h.heap.FixRoot()

	// reset timer to match the new state
	h.reset(now, true /* fired */)

	return
}

// reset configures the appropriate timer/channel for the current state given the
// current time. reset must be called after any addition, removal, or advancement.
// the fired parameter must be true if the call context is one in which a timer firing
// has been *observed* (i.e. the channel alread drained) and false otherwise.
func (h *Multi[T]) reset(now time.Time, fired bool) {
	// if reset isn't in *response* to firing timer may need to be reset
	if h.timer != nil && !fired && !h.timer.Stop() {
		<-h.timer.Chan()
	}

	root := h.heap.Root()
	if root == nil {
		// no targets, fully reset state to free resources and ensure that we're
		// in the expected state if/when new targets are added in the future.
		h.timer = nil
		h.heap.Clear()
		return
	}

	d := root.tick.Sub(now)

	if h.timer == nil {
		h.timer = h.clock.NewTimer(d)
	} else {
		h.timer.Reset(d)
	}
}

func (h *Multi[T]) Elapsed() <-chan time.Time {
	if h == nil || h.timer == nil {
		return nil
	}

	return h.timer.Chan()
}

func (h *Multi[T]) interval(jitter retryutils.Jitter) time.Duration {
	ivl := h.fixedInterval
	if h.variableInterval != nil {
		ivl = h.variableInterval.Duration()
	}
	return jitter(ivl)
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
