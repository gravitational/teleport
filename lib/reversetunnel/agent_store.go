/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reversetunnel

import (
	"strings"
	"sync"
)

// agentStore handles adding and removing agents from an in memory store.
type agentStore struct {
	agents []*Agent
	mu     sync.RWMutex
}

// newAgentStore creates a new agentStore instance.
func newAgentStore() *agentStore {
	return &agentStore{
		agents: make([]*Agent, 0),
	}
}

// len returns the number of agents in the store.
func (s *agentStore) len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.agents)
}

// add adds an agent to the store.
func (s *agentStore) add(agent *Agent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents = append(s.agents, agent)
}

// unsafeRemove removes an agent. Warning this is not threadsafe.
func (s *agentStore) unsafeRemove(agent *Agent) bool {
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
func (s *agentStore) remove(agent *Agent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unsafeRemove(agent)
}

// poplen pops an agent from the store if there are more agents in the store
// than the the given value. The oldest agent is always returned first.
func (s *agentStore) poplen(l int) (*Agent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if l < 0 || len(s.agents) == 0 {
		return nil, false
	}
	if len(s.agents) <= l {
		return nil, false
	}

	agent := s.agents[0]
	s.agents = s.agents[1:]
	return agent, true
}

// ids returns a list of proxy ids that each agent is connected to.
func (s *agentStore) proxyIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, 0)

	for i := len(s.agents) - 1; i >= 0; i-- {
		if id, ok := proxyIDFromPrincipals(s.agents[i].client.Principals()); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

// proxyIDFromPrincipals gets the proxy id from a list of principals.
func proxyIDFromPrincipals(principals []string) (string, bool) {
	if len(principals) == 0 {
		return "", false
	}

	// The proxy id will always be the first principal.
	id := principals[0]

	// Return the uuid from the format "<uuid>.<cluster-name>".
	split := strings.Split(id, ".")
	id = split[0]

	if id == "" {
		return "", false
	}

	return id, true
}
