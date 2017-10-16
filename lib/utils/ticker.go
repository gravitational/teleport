/*
Copyright 2017 Gravitational, Inc.

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
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
)

// NewSwitchTicker returns new instance of the switch ticker
func NewSwitchTicker(threshold int, slowPeriod time.Duration, fastPeriod time.Duration) (*SwitchTicker, error) {
	if threshold == 0 {
		return nil, trace.BadParameter("missing threshold")
	}
	if slowPeriod <= 0 || fastPeriod <= 0 {
		return nil, trace.BadParameter("bad slow period or fast period parameters")
	}
	return &SwitchTicker{
		threshold:  int64(threshold),
		slowTicker: time.NewTicker(slowPeriod),
		fastTicker: time.NewTicker(fastPeriod),
	}, nil
}

// SwitchTicker switches between slow and fast
// ticker based on the number of failures
type SwitchTicker struct {
	threshold  int64
	failCount  int64
	slowTicker *time.Ticker
	fastTicker *time.Ticker
}

// IncrementFailureCount increments internal failure count
func (c *SwitchTicker) IncrementFailureCount() {
	atomic.AddInt64(&c.failCount, 1)
}

// Channel returns either channel with fast ticker or slow ticker
// based on whether failure count exceeds threshold or not
func (c *SwitchTicker) Channel() <-chan time.Time {
	failCount := atomic.LoadInt64(&c.failCount)
	if failCount > c.threshold {
		return c.fastTicker.C
	}
	return c.slowTicker.C
}

// Reset resets internal failure counter and switches back to fast retry period
func (c *SwitchTicker) Reset() {
	atomic.StoreInt64(&c.failCount, 0)
}

// Stop stops tickers and has to be called to prevent timer leaks
func (c *SwitchTicker) Stop() {
	c.slowTicker.Stop()
	c.fastTicker.Stop()
}
