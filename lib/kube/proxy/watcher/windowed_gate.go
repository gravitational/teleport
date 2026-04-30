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
type WindowedGate struct {
	WindowedGateConfig

	// mu protects variables below
	mu sync.Mutex

	// next is the earliest time a new execution is allowed.
	next time.Time

	// pending marks if a request is in flight.
	pending bool
}

// NewWindowedGate creates a new WindowedGate.
func NewWindowedGate(cfg WindowedGateConfig) (*WindowedGate, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &WindowedGate{
		WindowedGateConfig: cfg,
		next:               time.Now().Add(cfg.StartupGracePeriod),
	}, nil
}

// MaybeDo executes fn if the gate allows it, otherwise suppresses calls.
//
//   - If called before the next allowed execution time or another call is pending returns nil without
//     invoking fn.
//   - Otherwise, fn is executed and its error is returned
//
// The execution window is advanced after fn completes, regardless of success or failure,
// with jitter applied to reduce synchronization across callers.
// Do returns true if it has been the driver of the current call or false if nothing has been ran.
func (g *WindowedGate) MaybeDo(ctx context.Context, fn func(context.Context) error) (bool, error) {
	now := time.Now()

	g.mu.Lock()
	if g.pending || now.Before(g.next) {
		g.mu.Unlock()
		return false, nil
	}
	g.pending = true
	g.mu.Unlock()

	err := trace.Wrap(fn(ctx))
	done := time.Now()

	g.mu.Lock()
	g.pending = false
	// Advance window with jitter, this happens regardless of result.
	g.next = done.Add(retryutils.SeventhJitter(g.Window))
	g.mu.Unlock()
	return true, err
}
