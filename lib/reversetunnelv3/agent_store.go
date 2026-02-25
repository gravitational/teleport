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

package reversetunnelv3

import "sync"

// agentStore is a concurrency-safe store of active Agent connections. Agents
// are appended in connection order; the newest agent is last.
type agentStore struct {
	mu     sync.RWMutex
	agents []Agent
}

func newAgentStore() *agentStore {
	return &agentStore{}
}

// add appends a to the store.
func (s *agentStore) add(a Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents = append(s.agents, a)
}

// remove deletes a from the store. Returns true if it was present.
func (s *agentStore) remove(a Agent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, v := range s.agents {
		if v == a {
			s.agents = append(s.agents[:i], s.agents[i+1:]...)
			return true
		}
	}
	return false
}

// len returns the number of agents in the store.
func (s *agentStore) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}

// proxyIDs returns the proxy IDs of all stored agents, ordered newest to
// oldest (matching the preference order used when dialing).
func (s *agentStore) proxyIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.agents))
	for i := len(s.agents) - 1; i >= 0; i-- {
		ids = append(ids, s.agents[i].GetProxyID())
	}
	return ids
}

// thrivingCount returns the number of agents that are not terminating.
func (s *agentStore) thrivingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, a := range s.agents {
		if !a.IsTerminating() {
			n++
		}
	}
	return n
}
