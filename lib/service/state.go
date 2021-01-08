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
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/prometheus/client_golang/prometheus"
)

type componentStateEnum byte

// Note: these consts are not using iota because they get exposed via a
// Prometheus metric. Using iota makes it possible to accidentally change the
// values.
const (
	// stateOK means Teleport is operating normally.
	stateOK = componentStateEnum(0)
	// stateRecovering means Teleport has begun recovering from a degraded state.
	stateRecovering = componentStateEnum(1)
	// stateDegraded means some kind of connection error has occurred to put
	// Teleport into a degraded state.
	stateDegraded = componentStateEnum(2)
	// stateStarting means the process is starting but hasn't joined the
	// cluster yet.
	stateStarting = componentStateEnum(3)
)

var stateGauge = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: teleport.MetricState,
	Help: fmt.Sprintf("State of the teleport process: %d - ok, %d - recovering, %d - degraded, %d - starting", stateOK, stateRecovering, stateDegraded, stateStarting),
})

func init() {
	prometheus.MustRegister(stateGauge)
	stateGauge.Set(float64(stateStarting))
}

// processState tracks the state of the Teleport process.
type processState struct {
	process *TeleportProcess
	mu      sync.Mutex
	states  map[string]*componentState
}

type componentState struct {
	recoveryTime time.Time
	state        componentStateEnum
}

// newProcessState returns a new FSM that tracks the state of the Teleport process.
func newProcessState(process *TeleportProcess) *processState {
	return &processState{
		process: process,
		states:  make(map[string]*componentState),
	}
}

// update the state of a Teleport component.
func (f *processState) update(event Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer f.updateGauge()

	component, ok := event.Payload.(string)
	if !ok {
		f.process.log.Errorf("TeleportDegradedEvent broadcasted without component name, this is a bug!")
		return
	}
	s, ok := f.states[component]
	if !ok {
		// Register a new component.
		s = &componentState{recoveryTime: f.process.GetClock().Now(), state: stateStarting}
		f.states[component] = s
	}

	switch event.Name {
	// If a degraded event was received, always change the state to degraded.
	case TeleportDegradedEvent:
		s.state = stateDegraded
		f.process.log.Infof("Detected Teleport component %q is running in a degraded state.", component)
	// If the current state is degraded, and a OK event has been
	// received, change the state to recovering. If the current state is
	// recovering and a OK events is received, if it's been longer
	// than the recovery time (2 time the server keep alive ttl), change
	// state to OK.
	case TeleportOKEvent:
		switch s.state {
		case stateStarting:
			s.state = stateOK
			f.process.log.Debugf("Teleport component %q has started.", component)
		case stateDegraded:
			s.state = stateRecovering
			s.recoveryTime = f.process.GetClock().Now()
			f.process.log.Infof("Teleport component %q is recovering from a degraded state.", component)
		case stateRecovering:
			if f.process.GetClock().Now().Sub(s.recoveryTime) > defaults.HeartbeatCheckPeriod*2 {
				s.state = stateOK
				f.process.log.Infof("Teleport component %q has recovered from a degraded state.", component)
			}
		}
	}
}

// getStateLocked returns the overall process state based on the state of
// individual components. If no components sent updates yet, returns
// stateStarting.
//
// Order of importance:
// 1. degraded
// 2. recovering
// 3. starting
// 4. ok
//
// Note: f.mu must be locked by the caller!
func (f *processState) getStateLocked() componentStateEnum {
	state := stateStarting
	numNotOK := len(f.states)
	for _, s := range f.states {
		switch s.state {
		case stateDegraded:
			return stateDegraded
		case stateRecovering:
			state = stateRecovering
		case stateOK:
			numNotOK--
		}
	}
	// Only return stateOK if *all* components are in stateOK.
	if numNotOK == 0 && len(f.states) > 0 {
		state = stateOK
	}
	return state
}

// Note: f.mu must be locked by the caller!
func (f *processState) updateGauge() {
	stateGauge.Set(float64(f.getStateLocked()))
}

// GetState returns the current state of the system.
func (f *processState) getState() componentStateEnum {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getStateLocked()
}
