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

// UpstreamResolver decides, per request, how the Forwarder should treat a
// target kubernetes cluster: serve it locally with the returned details, or
// forward the request to the next hop without enforcing RBAC.
//
// Callers construct one via NewKubeServiceUpstream, NewProxyServiceUpstream,
// or NewLegacyProxyUpstream and pass it to ForwarderConfig.Upstream. All
// methods are package-private — the interface exists only so the Forwarder
// can be configured with one of the three built-in shapes.
type UpstreamResolver interface {
	resolveDetails(kubeClusterName string) (*kubeDetails, error)
	component() string
	servesLocalClusters() bool
	forwardsToOtherAgents() bool
	store() *clusterStore
}

// NewKubeServiceUpstream returns the resolver for a teleport kubernetes_service
// instance: serves only its own clusters, never forwards.
func NewKubeServiceUpstream() UpstreamResolver {
	return &kubeServiceResolver{clusters: newClusterStore()}
}

// NewProxyServiceUpstream returns the resolver for a proxy_service instance:
// holds no clusters, always forwards to a kubernetes_service agent.
func NewProxyServiceUpstream() UpstreamResolver {
	return proxyServiceResolver{}
}

// NewLegacyProxyUpstream returns the resolver for the legacy proxy_service
// shape: serves its own clusters and falls back to forwarding for any cluster
// it does not serve directly.
func NewLegacyProxyUpstream() UpstreamResolver {
	return &legacyProxyResolver{clusters: newClusterStore()}
}

type kubeServiceResolver struct {
	clusters *clusterStore
}

func (r *kubeServiceResolver) resolveDetails(name string) (*kubeDetails, error) {
	return r.clusters.find(name)
}

func (*kubeServiceResolver) component() string           { return KubeService }
func (*kubeServiceResolver) servesLocalClusters() bool   { return true }
func (*kubeServiceResolver) forwardsToOtherAgents() bool { return false }
func (r *kubeServiceResolver) store() *clusterStore      { return r.clusters }

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

// newUpstreamResolver returns a resolver matching the given service type
// string. Internal helper used by tests that drive Forwarder behavior off the
// legacy KubeServiceType string.
func newUpstreamResolver(svc KubeServiceType) (UpstreamResolver, error) {
	switch svc {
	case KubeService:
		return NewKubeServiceUpstream(), nil
	case ProxyService:
		return NewProxyServiceUpstream(), nil
	case LegacyProxyService:
		return NewLegacyProxyUpstream(), nil
	default:
		return nil, trace.BadParameter("unknown KubeServiceType %q", svc)
	}
}
