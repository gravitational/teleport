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
	"sort"
	"sync"
	"time"
)

// agentStore handles adding and removing agents from an in memory store.
type agentStore struct {
	agents map[Agent]agentStoreMeta
	mu     sync.RWMutex
	wg     sync.WaitGroup
}

type agentStoreMeta struct {
	addedAt time.Time
}

// newAgentStore creates a new agentStore instance.
func newAgentStore() *agentStore {
	return &agentStore{
		agents: make(map[Agent]agentStoreMeta),
	}
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

	if _, ok := s.agents[agent]; ok {
		return
	}

	s.wg.Add(1)
	s.agents[agent] = agentStoreMeta{
		addedAt: time.Now(),
	}
}

// unsafeRemove removes an agent. Warning this is not threadsafe.
func (s *agentStore) unsafeRemove(agent Agent) bool {
	if _, ok := s.agents[agent]; !ok {
		return false
	}

	delete(s.agents, agent)
	return true
}

// remove removes the given agent from the store.
func (s *agentStore) remove(agent Agent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.unsafeRemove(agent) {
		return false
	}

	s.wg.Done()
	return true
}

// wait blocks until all agents have been removed from the store.
func (s *agentStore) wait() {
	if s == nil {
		return
	}
	s.wg.Wait()
}

// proxyIDs returns a list of proxy ids that each agent is connected to.
func (s *agentStore) proxyIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0, len(s.agents))

	for agent := range s.agents {
		if id, ok := agent.GetProxyID(); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// proxyIDsByJoinTime returns a list of proxy ids ordered from newest to oldest
// connected proxy id.
func (s *agentStore) proxyIDsByJoinTime() []string {
	s.mu.RLock()
	agents := make([]struct {
		agent Agent
		entry agentStoreMeta
	}, 0, len(s.agents))
	for agent, entry := range s.agents {
		agents = append(agents, struct {
			agent Agent
			entry agentStoreMeta
		}{agent: agent, entry: entry})
	}
	s.mu.RUnlock()

	sort.Slice(agents, func(i, j int) bool {
		return agents[i].entry.addedAt.After(agents[j].entry.addedAt)
	})

	ids := make([]string, 0, len(agents))
	for _, agent := range agents {
		if id, ok := agent.agent.GetProxyID(); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func (s *agentStore) getByProxyID(proxyID string) (Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for agent := range s.agents {
		if id, ok := agent.GetProxyID(); ok && id == proxyID {
			return agent, true
		}
	}
	return nil, false
}
