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
	"log/slog"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
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

func TestProcessStateProgress(t *testing.T) {
	clock := clockwork.NewFakeClock()
	startTime := clock.Now()

	testComponent := "test"

	type stateProgression struct {
		event   Event
		advance time.Duration
		expect  componentState
	}
	for _, tc := range []struct {
		desc     string
		progress []stateProgression
	}{
		{
			desc: "recover from degraded state after recovery period",
			progress: []stateProgression{
				{
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component:      testComponent,
							recoveryPeriod: defaults.HeartbeatCheckPeriod * 2,
						},
					},
					expect: componentState{
						state:        stateOK,
						recoveryTime: clock.Now(),
					},
				},
				{
					event: Event{
						Name: TeleportDegradedEvent,
						Payload: servicePayload{
							component:      testComponent,
							recoveryPeriod: defaults.HeartbeatCheckPeriod * 2,
						},
					},
					expect: componentState{
						state:        stateDegraded,
						recoveryTime: clock.Now(),
					},
				},
				{
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component:      testComponent,
							recoveryPeriod: defaults.HeartbeatCheckPeriod * 2,
						},
					},
					expect: componentState{
						state:        stateRecovering,
						recoveryTime: clock.Now(),
					},
				},
				{
					advance: defaults.HeartbeatCheckPeriod,
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component:      testComponent,
							recoveryPeriod: defaults.HeartbeatCheckPeriod * 2,
						},
					},
					expect: componentState{
						state:        stateRecovering,
						recoveryTime: clock.Now(),
					},
				},
				{
					advance: defaults.HeartbeatCheckPeriod + time.Second,
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component:      testComponent,
							recoveryPeriod: defaults.HeartbeatCheckPeriod * 2,
						},
					},
					expect: componentState{
						state:        stateOK,
						recoveryTime: clock.Now(),
					},
				},
			},
		},
		{
			desc: "recover from degraded state with no recovery period",
			progress: []stateProgression{
				{
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component: testComponent,
						},
					},
					expect: componentState{
						state:        stateOK,
						recoveryTime: clock.Now(),
					},
				},
				{
					event: Event{
						Name: TeleportDegradedEvent,
						Payload: servicePayload{
							component: testComponent,
						},
					},
					expect: componentState{
						state:        stateDegraded,
						recoveryTime: clock.Now(),
					},
				},
				{
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component: testComponent,
						},
					},
					expect: componentState{
						state:        stateRecovering,
						recoveryTime: clock.Now(),
					},
				},
				{
					event: Event{
						Name: TeleportOKEvent,
						Payload: servicePayload{
							component: testComponent,
						},
					},
					expect: componentState{
						state:        stateOK,
						recoveryTime: clock.Now(),
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			clock := clockwork.NewFakeClockAt(startTime)
			supervisor := NewSupervisor("1", slog.Default())
			process := &TeleportProcess{
				Supervisor: supervisor,
				logger:     slog.Default(),
				Clock:      clock,
			}
			ps, err := newProcessState(process)
			require.NoError(t, err)

			for i, p := range tc.progress {
				clock.Advance(p.advance)
				ps.update(p.event)
				got := ps.states[testComponent]
				require.Equal(t, p.expect.recoveryTime.String(), got.recoveryTime.String(), "unexpected recovery time at step %d", i)
				require.Equal(t, p.expect.state, got.state, "unexpected state at step %d", i)
			}
		})
	}
}
