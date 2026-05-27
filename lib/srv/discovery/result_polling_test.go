/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package discovery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestPollerManagerPollOnce(t *testing.T) {
	tests := []struct {
		name           string
		pollers        []*testPoller
		wantPending    bool
		wantDoneCount  int
		wantPendingLen int
	}{
		{
			name: "empty queue",
		},
		{
			name: "completed poller",
			pollers: []*testPoller{
				{doneAfterPolls: 1},
			},
			wantDoneCount: 1,
		},
		{
			name: "pending poller",
			pollers: []*testPoller{
				{doneAfterPolls: 2},
			},
			wantPending:    true,
			wantPendingLen: 1,
		},
		{
			name: "poll error is terminal",
			pollers: []*testPoller{
				{
					doneAfterPolls: 2,
					pollErrors:     []error{errors.New("transient error")},
				},
			},
			wantDoneCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pollers := make([]poller, 0, len(tt.pollers))
			for _, poller := range tt.pollers {
				pollers = append(pollers, poller)
			}
			manager := newPollerManager(logtest.NewLogger(), clockwork.NewFakeClock(), pollers...)

			require.Equal(t, tt.wantPending, manager.pollOnce(context.Background()))
			require.Equal(t, tt.wantDoneCount, manager.doneCount)
			require.Len(t, manager.pending, tt.wantPendingLen)

			for _, poller := range tt.pollers {
				require.Equal(t, 1, poller.pollCount())
			}
		})
	}

	t.Run("retries pending pollers", func(t *testing.T) {
		poller := &testPoller{
			doneAfterPolls: 2,
		}
		manager := newPollerManager(logtest.NewLogger(), clockwork.NewFakeClock(), poller)

		require.True(t, manager.pollOnce(context.Background()))
		require.Len(t, manager.pending, 1)
		require.Zero(t, manager.doneCount)
		require.Equal(t, 1, poller.pollCount())

		require.False(t, manager.pollOnce(context.Background()))
		require.Empty(t, manager.pending)
		require.Equal(t, 1, manager.doneCount)
		require.Equal(t, 2, poller.pollCount())
	})
}

func TestPollerManagerRun(t *testing.T) {
	t.Run("polls until all pollers are done", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			poller := &testPoller{doneAfterPolls: 3}
			manager := newPollerManager(logtest.NewLogger(), clockwork.NewRealClock(), poller)

			manager.run(t.Context())

			require.Equal(t, 3, poller.pollCount())
			require.Empty(t, manager.pending)
			require.Equal(t, 1, manager.doneCount)
		})
	})

	t.Run("stops when context is canceled", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			poller := &testPoller{
				onPoll: func(polls int) {
					if polls == 2 {
						cancel()
					}
				},
			}
			manager := newPollerManager(logtest.NewLogger(), clockwork.NewRealClock(), poller)

			manager.run(ctx)

			require.Equal(t, 2, poller.pollCount())
			require.Len(t, manager.pending, 1)
			require.Zero(t, manager.doneCount)
		})
	})
}

func TestPollerManagerRunWithTimeout(t *testing.T) {
	t.Run("polls until all pollers are done before timeout", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			poller := &testPoller{doneAfterPolls: 2}
			manager := newPollerManager(logtest.NewLogger(), clockwork.NewRealClock(), poller)

			manager.runWithTimeout(t.Context(), 2*azureResultPollingFrequency)

			require.Equal(t, 2, poller.pollCount())
			require.Empty(t, manager.pending)
			require.Equal(t, 1, manager.doneCount)
		})
	})

	t.Run("stops when timeout expires", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			poller := &testPoller{}
			manager := newPollerManager(logtest.NewLogger(), clockwork.NewRealClock(), poller)

			manager.runWithTimeout(t.Context(), azureResultPollingFrequency-time.Nanosecond)

			require.Equal(t, 1, poller.pollCount())
			require.Len(t, manager.pending, 1)
			require.Zero(t, manager.doneCount)
		})
	})
}

type testPoller struct {
	mu sync.Mutex

	doneAfterPolls int
	pollErrors     []error
	onPoll         func(polls int)
	polls          int
}

func (p *testPoller) Poll(context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.polls++
	if p.onPoll != nil {
		p.onPoll(p.polls)
	}
	if p.polls <= len(p.pollErrors) {
		return true
	}
	return p.doneAfterPolls > 0 && p.polls >= p.doneAfterPolls
}

func (p *testPoller) pollCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.polls
}
