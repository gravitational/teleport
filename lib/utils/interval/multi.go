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

package interval

import (
	"errors"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// MultiInterval is equivalent to Interval except that it supports multiple intervals simultanesouly,
// distinguishing them by key. The only real benefit to using this type instead of using multiple
// Intervals is that it only allocates one timer and one background goroutine regardless of the number
// of intervals. There are very few cases where this distinction matters. An example of a place where this
// *does* matter is when you need multiple intervals *per* connected instance on an auth or proxy server.
// In such a case, allocating only one timer and one goroutine per connected instance can be a significant
// saving (see the lib/inventory for an example of this usecase).
//
// Note that MultiInterval behaves differently than time.Ticker or Interval in that it may yield the same
// timestamp multiple times. It will only do this for *different* keys (i.e. K1 and K2 may both tick at T0)
// but it is still a potential source of bugs/confusion when transitioning to using this type from one
// of the single-interval alternatives.
type MultiInterval[T comparable] struct {
	clock     clockwork.Clock
	subs      []subIntervalEntry[T]
	push      chan subIntervalEntry[T]
	ch        chan Tick[T]
	reset     chan T
	fire      chan T
	closeOnce sync.Once
	done      chan struct{}
}

// Tick represents a firing of the interval. The Key field denominates the sub-interval that
// fired, and the Time field represents the time at which the firing occurred.
type Tick[T any] struct {
	Key  T
	Time time.Time
}

// SubInterval configures an interval.  The only required parameters are the Duration
// field which *must* be a positive duration, and the Key field which can be any comparable
// value.
type SubInterval[T any] struct {
	// Key is the key that will uniquely identify this sub-interval.
	Key T
	// Duration is the duration on which the interval "ticks" (if a jitter is
	// applied, this represents the upper bound of the range).
	Duration time.Duration

	// VariableDuration, if supplied, replaces the 'Duration' paramater with a
	// variable duration generator. Variable durations are used to calculate
	// some heartbeat intervals so that the time between heartbeats scales up
	// as concurrent load increases.
	VariableDuration *VariableDuration

	// FirstDuration is an optional special duration to be used for the first
	// "tick" of the interval.  This duration is not jittered.
	FirstDuration time.Duration

	// Jitter is an optional jitter to be applied to each step of the interval.
	// It is usually preferable to use a smaller jitter (e.g. NewSeventhJitter())
	// for this parameter, since periodic operations are typically costly and the
	// effect of the jitter is cumulative.
	Jitter retryutils.Jitter
}

// subIntervalEntry wraps a SubInterval with some internal state.
type subIntervalEntry[T comparable] struct {
	SubInterval[T]
	next time.Time
}

func (s *subIntervalEntry[T]) init(now time.Time) {
	d := s.Duration
	if s.FirstDuration != 0 {
		d = s.FirstDuration
	} else if s.Jitter != nil {
		// jitter is only applied if we aren't using
		// a custom FirstDuration value.
		d = s.Jitter(d)
	}
	s.next = now.Add(d)
}

func (s *subIntervalEntry[T]) duration(now time.Time) time.Duration {
	if now.After(s.next) {
		// we use the smallest possible duration to represent "fire now".
		return time.Duration(1)
	}
	return s.next.Sub(now)
}

func (s *subIntervalEntry[T]) reset(now time.Time) {
	d := s.Duration
	if s.VariableDuration != nil {
		d = s.VariableDuration.Duration()
	}
	if s.Jitter != nil {
		d = s.Jitter(d)
	}
	s.next = now.Add(d)
}

func (s *subIntervalEntry[T]) increment() {
	s.reset(s.next)
}

// NewMulti creates a new multi-interval instance.  This function panics on non-positive
// interval durations (equivalent to time.NewTicker) or if no sub-intervals are provided.
func NewMulti[T comparable](clock clockwork.Clock, intervals ...SubInterval[T]) *MultiInterval[T] {
	if len(intervals) == 0 {
		panic(errors.New("empty sub-interval set for interval.NewMulti"))
	}

	if clock == nil {
		clock = clockwork.NewRealClock()
	}

	interval := &MultiInterval[T]{
		clock: clock,
		subs:  make([]subIntervalEntry[T], 0, len(intervals)),
		push:  make(chan subIntervalEntry[T]),
		ch:    make(chan Tick[T], 1),
		reset: make(chan T),
		fire:  make(chan T),
		done:  make(chan struct{}),
	}

	// check and initialize our sub-intervals.
	now := clock.Now()
	for _, sub := range intervals {
		if sub.Duration <= 0 && (sub.VariableDuration == nil || sub.VariableDuration.Duration() <= 0) {
			panic(errors.New("non-positive sub interval for interval.NewMulti"))
		}
		entry := subIntervalEntry[T]{
			SubInterval: sub,
		}
		entry.init(now)
		interval.pushEntry(entry)
	}

	key, d := interval.duration(now)

	// start the timer in this goroutine to improve
	// consistency of first tick.
	timer := clock.NewTimer(d)

	go interval.run(timer, key)

	return interval
}

// Push adds a new sub-interval, potentially overwriting an existing sub-interval with the same key.
// This method panics on non-positive durations (equivalent to time.NewTicker).
func (i *MultiInterval[T]) Push(sub SubInterval[T]) {
	if sub.Duration <= 0 && (sub.VariableDuration == nil || sub.VariableDuration.Duration() <= 0) {
		panic(errors.New("non-positive sub interval for MultiInterval.Push"))
	}
	entry := subIntervalEntry[T]{
		SubInterval: sub,
	}
	// we initialize here in order to improve consistency of start time
	entry.init(i.clock.Now())
	select {
	case i.push <- entry:
	case <-i.done:
	}
}

// Stop permanently stops the interval.  Note that stopping an interval does not
// close its output channel.  This is done in order to prevent concurrent stops
// from generating erroneous "ticks" and is consistent with the behavior of
// time.Ticker.
func (i *MultiInterval[T]) Stop() {
	i.closeOnce.Do(func() {
		close(i.done)
	})
}

// Reset resets the interval without pausing it (i.e. it will now fire in
// jitter(duration) regardless of current timer progress).
func (i *MultiInterval[T]) Reset(key T) {
	select {
	case i.reset <- key:
	case <-i.done:
	}
}

// FireNow forces the sub-interval to fire immediately regardless of how much time is left on
// the current interval. This also effectively resets the sub-interval.
func (i *MultiInterval[T]) FireNow(key T) {
	select {
	case i.fire <- key:
	case <-i.done:
	}
}

// Next is the channel over which interval ticks are delivered.
func (i *MultiInterval[T]) Next() <-chan Tick[T] {
	return i.ch
}

// duration gets the next duration and its associated key.
func (i *MultiInterval[T]) duration(now time.Time) (key T, d time.Duration) {
	for idx := range i.subs {
		sd := i.subs[idx].duration(now)
		if d == 0 || sd < d {
			key = i.subs[idx].Key
			d = sd
		}
	}
	return
}

// increment increments the sub-interval with the provided key.
func (i *MultiInterval[T]) increment(key T) {
	for idx := range i.subs {
		if i.subs[idx].Key == key {
			i.subs[idx].increment()
			return
		}
	}
}

// resetEntry resets the sub-interval entry with the provided key.
func (i *MultiInterval[T]) resetEntry(now time.Time, key T) {
	for idx := range i.subs {
		if i.subs[idx].Key == key {
			i.subs[idx].reset(now)
			return
		}
	}
}

// pushEntry adds a new subinterval, overwriting any previous sub-intervals with the
// same key.
func (i *MultiInterval[T]) pushEntry(entry subIntervalEntry[T]) {
	for idx := range i.subs {
		if i.subs[idx].Key == entry.Key {
			i.subs[idx] = entry
			return
		}
	}
	i.subs = append(i.subs, entry)
}

func (i *MultiInterval[T]) run(timer clockwork.Timer, key T) {
	defer timer.Stop()

	var pending pendingTicks[T]
	var ch chan Tick[T]
	for {

		// get the next pending tick if one exists
		tick, ok := pending.next()

		// set up outbound channel so that we don't send
		// unless we have a valid tick.
		if ok {
			ch = i.ch
		} else {
			ch = nil
		}

		select {
		case t := <-timer.Chan():
			// increment the sub-interval for the current key
			i.increment(key)

			// add a pending tick for the current key
			pending.add(t, key)

			// calculate next key+duration
			var d time.Duration
			key, d = i.duration(t)

			// timer has fired, so we can safely reset it without
			// stop and drain.
			timer.Reset(d)

		case resetKey := <-i.reset:
			now := i.clock.Now()

			// reset the sub-interval for the target key
			i.resetEntry(now, resetKey)

			// remove any pending tick for the target key
			pending.remove(resetKey)

			// recalulate our next key+duration in case resetting
			// the sub-interval changed things.
			var d time.Duration
			key, d = i.duration(now)

			// stop and drain timer
			if !timer.Stop() {
				<-timer.Chan()
			}

			// apply the new duration
			timer.Reset(d)

		case fireKey := <-i.fire:
			now := i.clock.Now()

			// reset the sub-interval for the key we are firing
			i.resetEntry(now, fireKey)

			// push an "artificial" tick.
			pending.add(now, fireKey)

			// recalculate our next key+duration in case resetting
			// the fired key changes things.
			var d time.Duration
			key, d = i.duration(now)

			// stop and drain timer.
			if !timer.Stop() {
				<-timer.Chan()
			}

			// re-set the timer
			timer.Reset(d)
		case entry := <-i.push:
			now := i.clock.Now()

			// add the new sub-interval entry
			i.pushEntry(entry)

			// remove any pending ticks that may be invalidated
			// by this new entry overwriting an old one.
			pending.remove(entry.Key)

			// recalulate our next key+duration in case adding/overwriting the
			// new entry changes things.
			var d time.Duration
			key, d = i.duration(now)

			// stop and drain timer
			if !timer.Stop() {
				<-timer.Chan()
			}

			// apply the new duration
			timer.Reset(d)

		case ch <- tick:
			// remove the fired tick from pending
			pending.remove(tick.Key)

		case <-i.done:
			// interval has been stopped.
			return
		}
	}
}

// pendingTicks is a helper for managing a backlog of pending ticks. Ideally, there shouldn't be
// a backlog, but its important to not miss a tick if two sub-intervals happen to fire at the
// same time. This helper preserves tick order, but does not store duplicates (i.e. if the tick
// order is A,B,B,C,A, this will result in the sequence A,B,C). All ticks are yielded with the
// most recently observed timestamp.
type pendingTicks[T comparable] struct {
	time time.Time
	keys []T
}

func (p *pendingTicks[T]) add(now time.Time, key T) {
	p.time = now
	for _, k := range p.keys {
		if k == key {
			return
		}
	}
	p.keys = append(p.keys, key)
}

func (p *pendingTicks[T]) next() (tick Tick[T], ok bool) {
	if len(p.keys) == 0 {
		return Tick[T]{}, false
	}
	return Tick[T]{
		Key:  p.keys[0],
		Time: p.time,
	}, true
}

func (p *pendingTicks[T]) remove(key T) {
	for idx := range p.keys {
		if p.keys[idx] == key {
			p.keys = append(p.keys[:idx], p.keys[idx+1:]...)
			return
		}
	}
}
