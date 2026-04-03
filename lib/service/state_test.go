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
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestProcessStateGetState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		states map[string]*componentState
		want   componentStateEnum
	}{
		{
			desc:   "no components",
			states: map[string]*componentState{},
			want:   stateStarting,
		},
		{
			desc: "one component in stateOK",
			states: map[string]*componentState{
				"one": {state: stateOK},
			},
			want: stateOK,
		},
		{
			desc: "multiple components in stateOK",
			states: map[string]*componentState{
				"one":   {state: stateOK},
				"two":   {state: stateOK},
				"three": {state: stateOK},
			},
			want: stateOK,
		},
		{
			desc: "multiple components, one is degraded",
			states: map[string]*componentState{
				"one":   {state: stateRecovering},
				"two":   {state: stateDegraded},
				"three": {state: stateOK},
			},
			want: stateDegraded,
		},
		{
			desc: "multiple components, one is recovering",
			states: map[string]*componentState{
				"one":   {state: stateOK},
				"two":   {state: stateRecovering},
				"three": {state: stateOK},
			},
			want: stateRecovering,
		},
		{
			desc: "multiple components, one is starting",
			states: map[string]*componentState{
				"one":   {state: stateOK},
				"two":   {state: stateStarting},
				"three": {state: stateOK},
			},
			want: stateStarting,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ps := &processState{states: tt.states}
			got := ps.getState()
			require.Equal(t, tt.want, got)
		})
	}
}

// TestProcessStateStarting validates that we correctly keep track of the starting services.
func TestProcessStateStarting(t *testing.T) {
	t.Parallel()
	component := teleport.Component("test-component")
	slowComponent := teleport.Component("slow-component")
	log := logtest.NewLogger()

	fakeClock := clockwork.NewFakeClock()
	supervisor, err := NewSupervisor("test-process-state", log, fakeClock)
	require.NoError(t, err)
	process := &TeleportProcess{
		Supervisor: supervisor,
		Clock:      fakeClock,
		logger:     log,
	}
	ps := &process.Supervisor.(*LocalSupervisor).processState

	require.Equal(t, stateStarting, ps.getState(), "no services are running, we are starting")
	process.OnHeartbeat(component)(nil)
	require.Equal(t, stateOK, ps.getState(), "a single service is running, we are healthy")

	process.ExpectService(slowComponent)
	require.Equal(t, stateStarting, ps.getState(), "we know about a second service starting, we should be in starting state")

	process.OnHeartbeat(component)(nil)
	require.Equal(t, stateStarting, ps.getState(), "we know about a second service starting, we should still be in starting state")

	process.OnHeartbeat(slowComponent)(nil)
	require.Equal(t, stateOK, ps.getState(), "two services are running, we are healthy")
}

func TestProcessStateCallback(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		initial componentStateEnum
		expect  []bool
		events  []string
	}{
		{
			name:    "callback receives initial state",
			initial: stateDegraded,
			expect:  []bool{false},
		},
		{
			name:    "callback receives multiple state changes",
			initial: stateOK,
			expect:  []bool{true, false, false},
			events:  []string{TeleportDegradedEvent, TeleportOKEvent},
		},
		{
			name:    "callback skips non-state change events",
			initial: stateOK,
			expect:  []bool{true},
			events:  []string{TeleportOKEvent, TeleportOKEvent, TeleportOKEvent},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			component := "test-component"
			ps := &processState{
				states: map[string]*componentState{
					component: {state: tt.initial},
				},
			}

			var got []bool
			ps.registerCallback(func(h bool) {
				got = append(got, h)
			})
			require.Equal(t, tt.expect[:1], got)

			for _, event := range tt.events {
				ps.update(time.Now(), event, component)
			}
			require.Equal(t, tt.expect, got)
		})
	}
}
