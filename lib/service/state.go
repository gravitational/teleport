/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package service

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client/debug"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/metrics"
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
func newProcessState(process *TeleportProcess) (*processState, error) {
	err := metrics.RegisterPrometheusCollectors(stateGauge)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &processState{
		process: process,
		states:  make(map[string]*componentState),
	}, nil
}

// update the state of a Teleport component.
func (f *processState) update(event Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	defer f.updateGauge()

	component, ok := event.Payload.(string)
	if !ok {
		f.process.logger.ErrorContext(f.process.ExitContext(), "Received event broadcast without component name, this is a bug!", "event", event.Name)
		return
	}
	s, ok := f.states[component]
	if !ok {
		// Register a new component.
		s = &componentState{recoveryTime: f.process.Clock.Now(), state: stateStarting}
		f.states[component] = s
	}

	switch event.Name {
	// If a degraded event was received, always change the state to degraded.
	case TeleportDegradedEvent:
		s.state = stateDegraded
		f.process.logger.InfoContext(f.process.ExitContext(), "Detected Teleport component is running in a degraded state.", "component", component)
	// If the current state is degraded, and a OK event has been
	// received, change the state to recovering. If the current state is
	// recovering and a OK events is received, if it's been longer
	// than the recovery time (2 time the server keep alive ttl), change
	// state to OK.
	case TeleportOKEvent:
		switch s.state {
		case stateStarting:
			s.state = stateOK
			f.process.logger.DebugContext(f.process.ExitContext(), "Teleport component has started.", "component", component)
		case stateDegraded:
			s.state = stateRecovering
			s.recoveryTime = f.process.Clock.Now()
			f.process.logger.InfoContext(f.process.ExitContext(), "Teleport component is recovering from a degraded state.", "component", component)
		case stateRecovering:
			if f.process.Clock.Since(s.recoveryTime) > defaults.HeartbeatCheckPeriod*2 {
				s.state = stateOK
				f.process.logger.InfoContext(f.process.ExitContext(), "Teleport component has recovered from a degraded state.", "component", component)
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

func (f *processState) readinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch f.getState() {
		// 503
		case stateDegraded:
			roundtrip.ReplyJSON(w, http.StatusServiceUnavailable, debug.Readiness{
				Status: "teleport is in a degraded state, check logs for details",
				PID:    os.Getpid(),
			})
		// 400
		case stateRecovering:
			roundtrip.ReplyJSON(w, http.StatusBadRequest, debug.Readiness{
				Status: "teleport is recovering from a degraded state, check logs for details",
				PID:    os.Getpid(),
			})
		case stateStarting:
			roundtrip.ReplyJSON(w, http.StatusBadRequest, debug.Readiness{
				Status: "teleport is starting and hasn't joined the cluster yet",
				PID:    os.Getpid(),
			})
		// 200
		case stateOK:
			roundtrip.ReplyJSON(w, http.StatusOK, debug.Readiness{
				Status: "ok",
				PID:    os.Getpid(),
			})
		}
	}
}
