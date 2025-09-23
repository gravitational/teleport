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

import "sync"

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

	// OnExitOverloaded is called when transitioning from Overloaded to Normal state.
	// This callback is invoked when batch size drops below threshold/2.
	//
	// WARN: For simplicity there is not timer-based transition back to Normal state,
	// it only happens when batch size drops below threshold/2.
	// So if the last batch is larger and there are no new events, the state will remain Overloaded.
	// Is not provided, no action is taken on state transition.
	OnExitOverloaded func()

	// Threshold is the batch size threshold for state transitions.
	// Normal -> Overloaded: when batchSize >= Threshold
	// Overloaded -> Normal: when batchSize < Threshold/2
	Threshold int
}

// StateMonitor monitors batch sizes and manages state transitions based on configured thresholds.
type StateMonitor struct {
	StateConfig

	mu    sync.RWMutex
	state State
}

// NewStateMonitor creates a new StateMonitor with the given configuration.
// The monitor starts in Normal state by default.
func NewStateMonitor(config StateConfig) *StateMonitor {
	return &StateMonitor{
		state:       StateNormal,
		StateConfig: config,
	}
}

// GetState returns the current state of the monitor.
func (sm *StateMonitor) GetState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// UpdateState evaluates the current batch size and updates the state if necessary.
func (sm *StateMonitor) UpdateState(batchSize int) State {
	sm.mu.Lock()
	oldState := sm.state
	newState := sm.evaluateState(batchSize)
	sm.state = newState
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
