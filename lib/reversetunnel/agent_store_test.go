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

package reversetunnel

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type testStoreAgent struct {
	Agent
	proxyID string
}

func (a *testStoreAgent) GetProxyID() (string, bool) {
	return a.proxyID, true
}

func TestAgentStoreRace(t *testing.T) {
	store := newAgentStore()
	agents := []*testStoreAgent{{}, {}, {}, {}, {}}

	wg := &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			store.add(agents[i])
			wg.Done()
		}(i)
	}

	wg.Wait()

	wg = &sync.WaitGroup{}
	for i := range agents {
		wg.Add(1)
		go func(i int) {
			ok := store.remove(agents[i])
			require.True(t, ok)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

func TestAgentStoreGetByProxyID(t *testing.T) {
	store := newAgentStore()

	first := &testStoreAgent{proxyID: "proxy-1"}
	second := &testStoreAgent{proxyID: "proxy-2"}

	store.add(first)
	store.add(second)

	got, ok := store.getByProxyID("proxy-2")
	require.True(t, ok)
	require.Same(t, second, got)

	got, ok = store.getByProxyID("proxy-1")
	require.True(t, ok)
	require.Same(t, first, got)

	got, ok = store.getByProxyID("missing")
	require.False(t, ok)
	require.Nil(t, got)
}
