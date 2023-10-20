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

package faketime

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
)

// Clock is a simplified clock interface which facilitates testing of delays and
// timeouts by allowing all waits to be tagged so that test code can react to
// specific waits and respond accordingly.
type Clock interface {
	// NewTicker returns a new Ticker containing a channel that will send
	// the current time on the channel after each tick. The typical period of the
	// ticks is specified by the duration argument, but in the case of a fake
	// ticker ticks may be manually sent. Include a tag so that tests can
	// intercept tickers and handle appropriately.
	NewTicker(d time.Duration, tag any) Ticker
}

// Ticker is an interface to wrap time.Ticker.
type Ticker interface {
	// C returns the ticker channel.
	C() <-chan time.Time

	// Stop releases any resources associated with the ticker and makes sure no
	// future ticks are fired.
	Stop()
}

type realClock struct{}

// NewRealClock returns a clock which just calls out to real time functions.
func NewRealClock() Clock {
	return realClock{}
}

// NewTicker returns time.NewTicker(d), wrapping the returned ticker to conform
// to this package's Ticker interface which uses a function rather than a struct
// field to expose the channel.
func (c realClock) NewTicker(d time.Duration, tag any) Ticker {
	return &realTicker{time.NewTicker(d)}
}

type realTicker struct {
	*time.Ticker
}

// C returns this ticker's tick channel.
func (r *realTicker) C() <-chan time.Time {
	return r.Ticker.C
}

// FakeClock is a type which exposes "faked" time functions which can be useful
// for determinstically (and quickly) testing time-dependent code.
type FakeClock struct {
	tickerListener chan *FakeTicker
}

// NewFakeClock return a new fake clock, and must be passed a tickerListener
// Channel where all new tickers will be sent.
func NewFakeClock(tickerListener chan *FakeTicker) *FakeClock {
	return &FakeClock{
		tickerListener: tickerListener,
	}
}

// FakeTicker is a Ticker implementation which will only send ticks when Tick is
// manually called.
type FakeTicker struct {
	// Tag can be used to identify tickers for tests.
	Tag      any
	ch       chan time.Time
	stopped  chan struct{}
	stopOnce sync.Once
}

// NewTicker returns a new FakeTicker. Every "tick" of the ticker must be
// manually sent by calling the "Tick" method.
func (c *FakeClock) NewTicker(d time.Duration, tag any) Ticker {
	t := &FakeTicker{
		Tag:     tag,
		ch:      make(chan time.Time),
		stopped: make(chan struct{}),
	}

	c.tickerListener <- t
	return t
}

// C returns the ticker channel.
func (t *FakeTicker) C() <-chan time.Time {
	return t.ch
}

// Tick blocks until:
// - the ticker is stopped,
// - ctx expires, or
// - successfully sending on the tick channel.
func (t *FakeTicker) Tick(ctx context.Context) error {
	select {
	case <-t.stopped:
		return trace.BadParameter("ticker already stopped")
	default:
	}
	select {
	case <-t.stopped:
		return trace.BadParameter("ticker concurrently stopped")
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case t.ch <- time.Now():
		return nil
	}
}

// Stop prevents future Tick calls from succeeding, concurrent Tick calls may or
// may not be successful.
func (t *FakeTicker) Stop() {
	t.stopOnce.Do(func() { close(t.stopped) })
}
