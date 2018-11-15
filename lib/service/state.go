/*
Copyright 2018 Gravitational, Inc.

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

package service

import (
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
)

const (
	// stateOK means Teleport is operating normally.
	stateOK = 0

	// stateRecovering means Teleport has begun recovering from a degraded state.
	stateRecovering = 1

	// stateDegraded means some kind of connection error has occurred to put
	// Teleport into a degraded state.
	stateDegraded = 2
)

// processState tracks the state of the Teleport process.
type processState struct {
	process      *TeleportProcess
	recoveryTime time.Time
	currentState int64
}

// newProcessState returns a new FSM that tracks the state of the Teleport process.
func newProcessState(process *TeleportProcess) *processState {
	return &processState{
		process:      process,
		recoveryTime: process.Clock.Now(),
		currentState: stateOK,
	}
}

// Process updates the state of Teleport.
func (f *processState) Process(event Event) {
	switch event.Name {
	// Ready event means Teleport has started successfully.
	case TeleportReadyEvent:
		atomic.StoreInt64(&f.currentState, stateOK)
		f.process.Infof("Detected that service started and joined the cluster successfully.")
	// If a degraded event was received, always change the state to degraded.
	case TeleportDegradedEvent:
		atomic.StoreInt64(&f.currentState, stateDegraded)
		f.process.Infof("Detected Teleport is running in a degraded state.")
	// If the current state is degraded, and a OK event has been
	// received, change the state to recovering. If the current state is
	// recovering and a OK events is received, if it's been longer
	// than the recovery time (2 time the server heartbeat ttl), change
	// state to OK.
	case TeleportOKEvent:
		switch atomic.LoadInt64(&f.currentState) {
		case stateDegraded:
			atomic.StoreInt64(&f.currentState, stateRecovering)
			f.recoveryTime = f.process.Clock.Now()
			f.process.Infof("Teleport is recovering from a degraded state.")
		case stateRecovering:
			if f.process.Clock.Now().Sub(f.recoveryTime) > defaults.ServerHeartbeatTTL*2 {
				atomic.StoreInt64(&f.currentState, stateOK)
				f.process.Infof("Teleport has recovered from a degraded state.")
			}
		}
	}
}

// GetState returns the current state of the system.
func (f *processState) GetState() int64 {
	return atomic.LoadInt64(&f.currentState)
}
