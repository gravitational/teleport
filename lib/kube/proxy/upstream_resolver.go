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

// upstreamResolver decides, per request, how the Forwarder should treat a
// target kubernetes cluster: serve it locally with the returned details, or
// forward the request to the next hop without enforcing RBAC.
//
// Three return shapes:
//   - (details, nil)  : this service serves the cluster directly. Use the
//     details for credentials and enforce kube RBAC at this hop.
//   - (nil, nil)      : this service is acting as a passthrough; the next hop
//     is responsible for credentials and RBAC.
//   - (nil, err)      : this service is supposed to serve the cluster but
//     does not know about it. Caller should surface the error.
//
// Implementations correspond to the three deployment shapes today (kubernetes
// service, proxy service, legacy proxy service), but the Forwarder no longer
// needs to switch on the shape — it asks the resolver.
type upstreamResolver interface {
	resolveDetails(kubeClusterName string) (*kubeDetails, error)
	component() string

	// servesLocalClusters reports whether this service can hold details for
	// kube clusters directly. KubeService and LegacyProxyService do;
	// ProxyService does not.
	servesLocalClusters() bool

	// forwardsToOtherAgents reports whether this service queries the
	// kube_servers watcher and forwards to another agent when the target
	// cluster is not served locally. ProxyService always does; LegacyProxyService
	// does as a fallback; KubeService does not.
	forwardsToOtherAgents() bool

	// store returns the cluster store for resolvers that serve clusters
	// locally, or nil otherwise. Callers must nil-check.
	store() *clusterStore
}

type kubeServiceResolver struct {
	clusters *clusterStore
}

func (r *kubeServiceResolver) resolveDetails(name string) (*kubeDetails, error) {
	return r.clusters.find(name)
}

func (*kubeServiceResolver) component() string          { return KubeService }
func (*kubeServiceResolver) servesLocalClusters() bool  { return true }
func (*kubeServiceResolver) forwardsToOtherAgents() bool { return false }
func (r *kubeServiceResolver) store() *clusterStore     { return r.clusters }

type proxyServiceResolver struct{}

func (proxyServiceResolver) resolveDetails(string) (*kubeDetails, error) { return nil, nil }

func (proxyServiceResolver) component() string           { return ProxyService }
func (proxyServiceResolver) servesLocalClusters() bool   { return false }
func (proxyServiceResolver) forwardsToOtherAgents() bool { return true }
func (proxyServiceResolver) store() *clusterStore        { return nil }

type legacyProxyResolver struct {
	clusters *clusterStore
}

func (r *legacyProxyResolver) resolveDetails(name string) (*kubeDetails, error) {
	if d, err := r.clusters.find(name); err == nil {
		return d, nil
	}
	// LegacyProxyService falls back to passthrough when the cluster is not
	// served locally. The next hop will enforce RBAC.
	return nil, nil
}

func (*legacyProxyResolver) component() string           { return LegacyProxyService }
func (*legacyProxyResolver) servesLocalClusters() bool   { return true }
func (*legacyProxyResolver) forwardsToOtherAgents() bool { return true }
func (r *legacyProxyResolver) store() *clusterStore      { return r.clusters }

// newUpstreamResolver builds the resolver matching the given KubeServiceType.
// Resolvers that serve clusters locally are given a fresh, empty store; the
// proxy resolver has none.
func newUpstreamResolver(svc KubeServiceType) (upstreamResolver, error) {
	switch svc {
	case KubeService:
		return &kubeServiceResolver{clusters: newClusterStore()}, nil
	case ProxyService:
		return proxyServiceResolver{}, nil
	case LegacyProxyService:
		return &legacyProxyResolver{clusters: newClusterStore()}, nil
	default:
		return nil, trace.BadParameter("unknown KubeServiceType %q", svc)
	}
}
