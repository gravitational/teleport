// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package watcher

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
)

// WindowedGateConfig configures a WindowedGate.
type WindowedGateConfig struct {
	// Window defines the minimum duration between consecutive executions of the
	// wrapped function. Calls within this window are suppressed.
	Window time.Duration
	// StartupGracePeriod defines an initial delay after construction during which
	// executions are suppressed. This is useful to avoid immediate upstream calls
	// during startup.
	StartupGracePeriod time.Duration
}

// CheckAndSetDefaults validates the configuration.
func (cfg *WindowedGateConfig) CheckAndSetDefaults() error {
	if cfg.Window <= 0 {
		return trace.BadParameter("Window cannot be <= 0")
	}
	return nil
}

// WindowedGate provides a time-windowed execution gate.
//
//   - At most one execution of the provided function is in-flight at any time
//     (single-flight behavior).
//   - Subsequent calls within a configured time window are suppressed.
//   - Callers arriving while a function is in-flight will wait for the same
//     result and receive the same error.
type WindowedGate struct {
	WindowedGateConfig
	ctx context.Context

	// mu protects variables below
	mu sync.Mutex
	// current holds the in-flight call, if any.
	current *call
	// next is the earliest time a new execution is allowed.
	next time.Time
}

// call represents a single in-flight execution.
type call struct {
	done chan struct{}
	// err stores the error of this result to propagate to callers.
	err error
}

// NewWindowedGate creates a new WindowedGate.
func NewWindowedGate(ctx context.Context, cfg WindowedGateConfig) (*WindowedGate, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &WindowedGate{
		WindowedGateConfig: cfg,
		ctx:                ctx,
		next:               time.Now().Add(cfg.StartupGracePeriod),
	}, nil
}

// Do executes fn if the gate allows it, otherwise suppresses or coalesces calls.
//
//   - If another execution is already in progress, Do will wait for it to finish
//     and return the same result.
//   - If called before the next allowed execution time, Do returns nil without
//     invoking fn.
//   - Otherwise, fn is executed and its result is shared with any concurrent callers.
//
// The execution window is advanced after fn completes, regardless of success or failure,
// with jitter applied to reduce synchronization across callers.
// Do returns true if it has been the driver of the current call or false if nothing has been ran.
func (g *WindowedGate) Do(ctx context.Context, fn func(context.Context) error) (bool, error) {
	g.mu.Lock()

	if c := g.current; c != nil {
		done := c.done
		g.mu.Unlock()

		select {
		case <-g.ctx.Done():
			return false, trace.Wrap(g.ctx.Err(), "parent context closed")
		case <-ctx.Done():
			return false, trace.Wrap(ctx.Err(), "caller context closed")
		case <-done:
			return false, c.err
		}
	}

	if time.Now().Before(g.next) {
		g.mu.Unlock()
		return false, nil
	}

	c := &call{done: make(chan struct{})}
	g.current = c

	g.mu.Unlock()

	err := trace.Wrap(fn(ctx))
	now := time.Now()

	g.mu.Lock()
	c.err = err
	g.current = nil
	// Advance window with jitter, this happens regardless of result.
	g.next = now.Add(retryutils.SeventhJitter(g.Window))
	g.mu.Unlock()

	close(c.done)
	return true, err
}
