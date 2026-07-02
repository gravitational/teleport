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
	"time"

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

func TestAgentStoreDuplicateAgents(t *testing.T) {
	store := newAgentStore()
	agent := &testStoreAgent{proxyID: "proxy-1"}

	store.add(agent)
	store.add(agent)

	require.Equal(t, 1, store.len())
	require.Equal(t, []string{"proxy-1"}, store.proxyIDs())

	require.True(t, store.remove(agent))
	require.False(t, store.remove(agent))
	require.Zero(t, store.len())

	waitDone := make(chan struct{})
	go func() {
		store.wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("agent store did not drain")
	}
}

func TestAgentStoreProxyIDsNewestFirst(t *testing.T) {
	oldest := &testStoreAgent{proxyID: "proxy-1"}
	newest := &testStoreAgent{proxyID: "proxy-2"}
	middle := &testStoreAgent{proxyID: "proxy-3"}
	store := &agentStore{
		agents: map[Agent]agentStoreMeta{
			oldest: {addedAt: time.Unix(1, 0)},
			newest: {addedAt: time.Unix(3, 0)},
			middle: {addedAt: time.Unix(2, 0)},
		},
	}

	require.Equal(t, []string{"proxy-2", "proxy-3", "proxy-1"}, store.proxyIDsByJoinTime())
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
