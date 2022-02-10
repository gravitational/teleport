/*
Copyright 2021 Gravitational, Inc.

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
	"time"

	"github.com/jonboulle/clockwork"
)

// TimedCounter is essentially a lightweight rate calculator. It counts events
// that happen over a period of time, e.g. have there been more than 4 errors
// in the last 30 seconds. Automatically expires old events so they are not
// included in the count. Not safe for concurrent use.
type TimedCounter struct {
	clock   clockwork.Clock
	timeout time.Duration
	events  []time.Time
}

// NewTimedCounter creates a new timed counter with the specified timeout
func NewTimedCounter(clock clockwork.Clock, timeout time.Duration) *TimedCounter {
	return &TimedCounter{
		clock:   clock,
		timeout: timeout,
		events:  nil,
	}
}

// Increment adds a new item into the counter, returning the current count.
func (c *TimedCounter) Increment() int {
	c.trim()
	c.events = append(c.events, c.clock.Now())
	return len(c.events)
}

// Count fetches the number of recorded events currently in the measurement
// time window.
func (c *TimedCounter) Count() int {
	c.trim()
	return len(c.events)
}

func (c *TimedCounter) trim() {
	deadline := c.clock.Now().Add(-c.timeout)
	lastExpiredEvent := -1
	for i := range c.events {
		if c.events[i].After(deadline) {
			break
		}
		lastExpiredEvent = i
	}

	if lastExpiredEvent > -1 {
		c.events = c.events[lastExpiredEvent+1:]
	}
}
