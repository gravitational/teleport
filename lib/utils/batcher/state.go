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

package batcher

import (
	"context"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// State represents the current processing mode based on batch volume.
type State int

const (
	// StateNormal indicates regular processing mode with normal event volume.
	StateNormal State = iota
	// StateOverloaded indicates high-volume processing mode when batch sizes
	// when threshold is reached.
	StateOverloaded
)

// String returns a human-readable representation of the State.
func (s State) String() string {
	switch s {
	case StateNormal:
		return "Normal"
	case StateOverloaded:
		return "Overloaded"
	default:
		return "Unknown"
	}
}

// StateConfig configures state transitions and callbacks for a StateMonitor.
type StateConfig struct {
	// OnEnterOverloaded is called when transitioning from Normal to Overloaded state.
	// This callback is invoked when batch size reaches or exceeds the threshold.
	// Is not provided, no action is taken on state transition.
	OnEnterOverloaded func()

	// OnExitOverloaded is called when transitioning from the Overloaded state back to the Normal state.
	//
	// This callback is invoked in either of the following cases:
	//   - When the batch size drops below half of the configured Threshold.
	//   - In background goroutine when the OverloadedIdleTimeout period elapses without
	//     receiving any new events, causing the state to automatically revert to Normal.
	//
	// If not provided, no action is taken on state transition.
	OnExitOverloaded func()

	// Threshold is the batch size threshold for state transitions.
	// Normal -> Overloaded: when batchSize >= Threshold
	// Overloaded -> Normal: when batchSize < Threshold/2
	Threshold int

	// OverloadedIdleTimeout is the quiet-period after which the monitor will (if still in
	// StateOverloaded) automatically return to StateNormal even if no new batches arrive.
	// If zero, the timer-based transition is disabled and the state will only return
	// to Normal via an explicit size drop (< Threshold/2).
	OverloadedIdleTimeout time.Duration

	// Clock is used to control time. If nil, the system clock is used.
	Clock clockwork.Clock
}

func (cfg *StateConfig) CheckAndSetDefaults() error {
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	if cfg.Threshold == 0 {
		cfg.Threshold = 100
	}
	if cfg.OverloadedIdleTimeout != 0 && cfg.OverloadedIdleTimeout < time.Second {
		return trace.BadParameter("OverloadedIdleTimeout must be at least 1s")
	}
	return nil
}

// StateMonitor monitors batch sizes and manages state transitions based on configured thresholds.
type StateMonitor struct {
	StateConfig

	mu    sync.RWMutex
	state State

	lastReceived time.Time
	stop         chan struct{}
}

// NewStateMonitor creates a new StateMonitor with the given configuration.
// The monitor starts in Normal state by default.
func NewStateMonitor(config StateConfig) (*StateMonitor, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &StateMonitor{
		state:       StateNormal,
		StateConfig: config,
		stop:        make(chan struct{}),
	}, nil
}

// GetState returns the current state of the monitor.
func (sm *StateMonitor) GetState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

func (sm *StateMonitor) Start(ctx context.Context) {
	if sm.OverloadedIdleTimeout == 0 {
		return
	}
	go sm.monitorIdle(ctx)
}

func (sm *StateMonitor) monitorIdle(ctx context.Context) {
	ticker := sm.Clock.NewTicker(sm.OverloadedIdleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stop:
			return
		case <-ticker.Chan():
			sm.mu.Lock()
			if sm.state == StateOverloaded && sm.lastReceived.Add(sm.OverloadedIdleTimeout).Before(sm.Clock.Now()) {
				sm.state = StateNormal
				sm.mu.Unlock()
				if sm.OnExitOverloaded != nil {
					sm.OnExitOverloaded()
				}
			} else {
				sm.mu.Unlock()
			}
		}
	}
}

// UpdateState evaluates the current batch size and updates the state if necessary.
func (sm *StateMonitor) UpdateState(batchSize int) State {
	sm.mu.Lock()
	oldState := sm.state
	newState := sm.evaluateState(batchSize)
	sm.state = newState
	sm.lastReceived = sm.Clock.Now()
	sm.mu.Unlock()

	if oldState != newState {
		switch {
		case oldState != StateOverloaded && newState == StateOverloaded:
			if sm.StateConfig.OnEnterOverloaded != nil {
				sm.StateConfig.OnEnterOverloaded()
			}
		case oldState == StateOverloaded && newState == StateNormal:
			if sm.StateConfig.OnExitOverloaded != nil {
				sm.StateConfig.OnExitOverloaded()
			}
		}
	}
	return newState
}

func (sm *StateMonitor) evaluateState(batchSize int) State {
	if batchSize >= sm.StateConfig.Threshold {
		// Threshold is reached, enter Overloaded state.
		return StateOverloaded
	}
	if batchSize < sm.StateConfig.Threshold/2 {
		// The batch event size dropped below half the threshold, return to Normal state.
		return StateNormal
	}
	return sm.state
}

// Stop stops any internal timers used by the StateMonitor.
func (sm *StateMonitor) Stop() {
	close(sm.stop)
}
