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

import "github.com/gravitational/trace"

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
}

type kubeServiceResolver struct {
	lookup func(name string) (*kubeDetails, error)
}

func (r *kubeServiceResolver) resolveDetails(name string) (*kubeDetails, error) {
	return r.lookup(name)
}

func (*kubeServiceResolver) component() string        { return KubeService }
func (*kubeServiceResolver) servesLocalClusters() bool { return true }
func (*kubeServiceResolver) forwardsToOtherAgents() bool { return false }

type proxyServiceResolver struct{}

func (proxyServiceResolver) resolveDetails(string) (*kubeDetails, error) { return nil, nil }

func (proxyServiceResolver) component() string          { return ProxyService }
func (proxyServiceResolver) servesLocalClusters() bool  { return false }
func (proxyServiceResolver) forwardsToOtherAgents() bool { return true }

type legacyProxyResolver struct {
	lookup func(name string) (*kubeDetails, error)
}

func (r *legacyProxyResolver) resolveDetails(name string) (*kubeDetails, error) {
	d, err := r.lookup(name)
	if err != nil {
		// LegacyProxyService falls back to passthrough when the cluster is not
		// served locally. The next hop will enforce RBAC.
		return nil, nil
	}
	return d, nil
}

func (*legacyProxyResolver) component() string          { return LegacyProxyService }
func (*legacyProxyResolver) servesLocalClusters() bool  { return true }
func (*legacyProxyResolver) forwardsToOtherAgents() bool { return true }

// newUpstreamResolver builds the resolver matching the given KubeServiceType,
// wiring it to lookup for credential/details resolution.
func newUpstreamResolver(svc KubeServiceType, lookup func(string) (*kubeDetails, error)) (upstreamResolver, error) {
	switch svc {
	case KubeService:
		return &kubeServiceResolver{lookup: lookup}, nil
	case ProxyService:
		return proxyServiceResolver{}, nil
	case LegacyProxyService:
		return &legacyProxyResolver{lookup: lookup}, nil
	default:
		return nil, trace.BadParameter("unknown KubeServiceType %q", svc)
	}
}
