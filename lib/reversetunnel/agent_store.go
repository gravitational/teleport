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
)

// agentStore handles adding and removing agents from an in memory store.
type agentStore struct {
	agents []Agent
	mu     sync.RWMutex
}

// newAgentStore creates a new agentStore instance.
func newAgentStore() *agentStore {
	return &agentStore{}
}

// len returns the number of agents in the store.
func (s *agentStore) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}

// add adds an agent to the store.
func (s *agentStore) add(agent Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents = append(s.agents, agent)
}

// unsafeRemove removes an agent. Warning this is not threadsafe.
func (s *agentStore) unsafeRemove(agent Agent) bool {
	for i := range s.agents {
		if s.agents[i] != agent {
			continue
		}
		s.agents = append(s.agents[:i], s.agents[i+1:]...)
		return true
	}

	return false
}

// remove removes the given agent from the store.
func (s *agentStore) remove(agent Agent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unsafeRemove(agent)
}

// proxyIDs returns a list of proxy ids that each agent is connected to ordered
// from newest to oldest connected proxy id.
func (s *agentStore) proxyIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.agents))

	// New agents are always appended to the store so reversing the list
	// orders the ids from newest to oldest.
	for i := len(s.agents) - 1; i >= 0; i-- {
		if id, ok := s.agents[i].GetProxyID(); ok {
			ids = append(ids, id)
		}
	}
	return ids
}
