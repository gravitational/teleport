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
package batcher_test

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/batcher"
)

func TestStateManage(t *testing.T) {
	enterStormCount := 0
	exitStormCount := 0

	config := batcher.StateConfig{
		Threshold: 100,
		OnEnterOverloaded: func() {
			enterStormCount++
		},
		OnExitOverloaded: func() {
			exitStormCount++
		},
	}

	sm, err := batcher.NewStateMonitor(config)
	require.NoError(t, err)
	defer sm.Stop()

	sm.UpdateState(50)
	require.Equal(t, 0, enterStormCount)
	require.Equal(t, 0, exitStormCount)

	sm.UpdateState(100)
	require.Equal(t, 1, enterStormCount)
	require.Equal(t, 0, exitStormCount)

	sm.UpdateState(150)
	require.Equal(t, 1, enterStormCount)
	require.Equal(t, 0, exitStormCount)

	sm.UpdateState(40)
	require.Equal(t, 1, enterStormCount)
	require.Equal(t, 1, exitStormCount)

	sm.UpdateState(20)
	require.Equal(t, 1, enterStormCount)
	require.Equal(t, 1, exitStormCount)

	sm.UpdateState(100)
	require.Equal(t, 2, enterStormCount)
	require.Equal(t, 1, exitStormCount)
}

func TestStateMonitor_IdleTimer(t *testing.T) {
	fc := clockwork.NewFakeClock()

	enterStormCount := 0
	exitStormCount := 0
	exitCh := make(chan struct{}, 1)

	cfg := batcher.StateConfig{
		Threshold:             100,
		OverloadedIdleTimeout: 10 * time.Second,
		Clock:                 fc,
		OnEnterOverloaded: func() {
			enterStormCount++
		},
		OnExitOverloaded: func() {
			exitStormCount++
			select {
			case exitCh <- struct{}{}:
			default:
			}
		},
	}

	sm, err := batcher.NewStateMonitor(cfg)
	require.NoError(t, err)
	sm.Start(t.Context())
	defer sm.Stop()

	sm.UpdateState(200)
	require.Equal(t, batcher.StateOverloaded, sm.GetState())
	require.Equal(t, 1, enterStormCount)
	require.Equal(t, 0, exitStormCount)

	fc.Advance(500 * time.Millisecond)
	require.Equal(t, batcher.StateOverloaded, sm.GetState())
	require.Equal(t, 0, exitStormCount)

	sm.UpdateState(150)
	require.Equal(t, batcher.StateOverloaded, sm.GetState())

	// Advance another 700ms not new events but this state is still overloaded.
	fc.Advance(700 * time.Millisecond)
	require.Equal(t, batcher.StateOverloaded, sm.GetState())
	require.Equal(t, 0, exitStormCount)

	require.NoError(t, fc.BlockUntilContext(t.Context(), 1))
	fc.Advance(21 * time.Second)

	select {
	case <-exitCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected OnExitOverloaded to fire after idle timeout")
	}

	require.Equal(t, batcher.StateNormal, sm.GetState())
	require.Equal(t, 1, exitStormCount)
}
