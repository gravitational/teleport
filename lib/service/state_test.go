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
	"context"
	"testing"

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

	supervisor := NewSupervisor("test-process-state", log)
	process := &TeleportProcess{
		Supervisor: supervisor,
		Clock:      clockwork.NewFakeClock(),
		logger:     log,
	}
	ps, err := newProcessState(process)
	require.NoError(t, err)
	process.state = ps

	eventProcessor := newFakeEventProcessor(t.Context(), process)

	require.Equal(t, stateStarting, ps.getState(), "no services are running, we are starting")
	process.OnHeartbeat(component)(nil)
	eventProcessor.processEvent()
	require.Equal(t, stateOK, ps.getState(), "a single service is running, we are healthy")

	process.ExpectService(slowComponent)
	eventProcessor.processEvent()
	require.Equal(t, stateStarting, ps.getState(), "we know about a second service starting, we should be in starting state")

	process.OnHeartbeat(component)(nil)
	eventProcessor.processEvent()
	require.Equal(t, stateStarting, ps.getState(), "we know about a second service starting, we should still be in starting state")

	process.OnHeartbeat(slowComponent)(nil)
	eventProcessor.processEvent()
	require.Equal(t, stateOK, ps.getState(), "two services are running, we are healthy")
}

// fakeEventProcessor synchronously processes events in tests. The real TeleportProcess
// has a dedicated routine for that, but testing with asynchronous event processing
// would be troublesome and flaky.
type fakeEventProcessor struct {
	eventCh chan Event
	ps      *processState
}

func newFakeEventProcessor(ctx context.Context, process *TeleportProcess) *fakeEventProcessor {
	eventCh := make(chan Event, 1024)
	process.ListenForEvents(ctx, TeleportDegradedEvent, eventCh)
	process.ListenForEvents(ctx, TeleportOKEvent, eventCh)
	process.ListenForEvents(ctx, TeleportStartingEvent, eventCh)
	return &fakeEventProcessor{
		eventCh: eventCh,
		ps:      process.state,
	}
}

func (f *fakeEventProcessor) processEvent() {
	evt := <-f.eventCh
	f.ps.update(evt)
}
