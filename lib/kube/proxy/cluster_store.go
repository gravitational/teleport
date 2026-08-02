// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package proxy

import (
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
)

// clusterStore holds the kube clusters served by a teleport service.
// Safe for concurrent use. Only the resolvers that serve clusters locally
// (kube_service, legacy proxy_service) embed a store; the proxy_service
// resolver does not.
type clusterStore struct {
	mu      sync.RWMutex
	details map[string]*kubeDetails
}

func newClusterStore() *clusterStore {
	return &clusterStore{details: make(map[string]*kubeDetails)}
}

func (s *clusterStore) find(name string) (*kubeDetails, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if d, ok := s.details[name]; ok {
		return d, nil
	}
	return nil, trace.NotFound("cluster %s not found", name)
}

func (s *clusterStore) upsert(name string, details *kubeDetails) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.details[name]; ok {
		old.Close()
	}
	s.details[name] = details
}

func (s *clusterStore) remove(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.details[name]; ok {
		old.Close()
	}
	delete(s.details, name)
}

func (s *clusterStore) clusters() types.KubeClusters {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(types.KubeClusters, 0, len(s.details))
	for _, d := range s.details {
		res = append(res, d.kubeCluster.Copy())
	}
	return res
}
