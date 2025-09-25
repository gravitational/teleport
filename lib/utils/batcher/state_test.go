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

	sm := batcher.NewStateMonitor(config)

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
