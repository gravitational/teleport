/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"sync"
	"time"
)

// tickJitter is the Jitter instance which backs instances
// of JitterTicker.
var tickJitter = NewJitter()

// JitterTicker is a drop-in replacement for time.Ticker which
// applies the default Jitter to each tick interval.  Useful for
// avoiding contention in periodic operations such as CA rotation.
type JitterTicker struct {
	// C is the cannel on which the ticks are delivered
	C         <-chan time.Time
	closeOnce sync.Once
	done      chan struct{}
}

// NewJitterTicker creates a new JitterTicker instance with the
// specified base duration.  The base duration must be a positive
// value, and Stop must be called on the resulting ticker in
// order to release the assoicated resources.
func NewJitterTicker(d time.Duration) *JitterTicker {
	if d <= 0 {
		panic("non-positive interval for NewJitterTicker")
	}

	// note that `c` is never closed.  This mirrors the behavior of time.Ticker
	// and is intended to prevent errors due to spurious ticks.
	c, done := make(chan time.Time, 1), make(chan struct{})

	go func() {
		timer := time.NewTimer(tickJitter(d))
		defer timer.Stop()

		for {
			// precompute next jittered duration while timer is running to
			// reduce play between timer firing and reset.
			nextD := tickJitter(d)
			select {
			case t := <-timer.C:
				timer.Reset(nextD)
				// if the previous tick has not been consumed,
				// drop it from the channel.
				select {
				case <-c:
				default:
				}
				// channel is empty; this never blocks.
				c <- t
			case <-done:
				return
			}
		}
	}()

	return &JitterTicker{
		C:    c,
		done: done,
	}
}

// Stop turns off the ticker.  After calling stop, no more ticks will be sent.
// Stop does not close the tick channel to prevent concurrent goroutines from
// reading from the channel and seeing an erroneous "tick".
func (t *JitterTicker) Stop() {
	t.closeOnce.Do(func() {
		close(t.done)
	})
}
