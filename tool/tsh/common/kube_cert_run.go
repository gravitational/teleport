/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"crypto/tls"
	"slices"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
)

// kubeCertRun is the state of one issuance burst.
// The per-session MFA requirement of every cluster it covers, and the certs issued so far.
// Its own lock makes it safe for the concurrent fan-out,
// so callers never have to work out whether a given path needs one.
type kubeCertRun struct {
	issuer    *kubeCertIssuer
	mfaChecks map[string]*proto.IsMFARequiredResponse
	certs     alpnproxy.KubeClientCerts
	mu        sync.Mutex
}

func (issuer *kubeCertIssuer) newCertRun(mfaChecks map[string]*proto.IsMFARequiredResponse) *kubeCertRun {
	return &kubeCertRun{
		issuer:    issuer,
		mfaChecks: mfaChecks,
		certs:     make(alpnproxy.KubeClientCerts),
	}
}

// PartitionByMFA splits clusters by whether they need per-session MFA.
func (r *kubeCertRun) PartitionByMFA(clusters kubeconfig.LocalProxyClusters) (mfaOn, mfaOff kubeconfig.LocalProxyClusters) {
	for _, cluster := range clusters {
		if r.mfaChecks[localProxyClusterKey(cluster)].GetRequired() {
			mfaOn = append(mfaOn, cluster)
		} else {
			mfaOff = append(mfaOff, cluster)
		}
	}
	return mfaOn, mfaOff
}

// IssueOne issues the cert for a single cluster and collects it.
func (r *kubeCertRun) IssueOne(ctx context.Context, cluster kubeconfig.LocalProxyCluster) error {
	cert, err := r.issuer.IssueCert(ctx, cluster.TeleportCluster, cluster.KubeCluster, r.mfaChecks[localProxyClusterKey(cluster)])
	if err != nil {
		return trace.Wrap(err)
	}
	logger.DebugContext(ctx, "Client cert issued for cluster", "cluster", cluster)
	r.add(cluster.TeleportCluster, cluster.KubeCluster, *cert)
	return nil
}

// IssuePerCluster issues one routed cert per cluster, serially.
func (r *kubeCertRun) IssuePerCluster(ctx context.Context, clusters kubeconfig.LocalProxyClusters) error {
	for _, cluster := range clusters {
		if err := r.IssueOne(ctx, cluster); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// IssueShared issues one unrouted cert per Teleport cluster covering the given clusters.
func (r *kubeCertRun) IssueShared(ctx context.Context, clusters kubeconfig.LocalProxyClusters) error {
	for _, teleportCluster := range slices.Sorted(slices.Values(clusters.TeleportClusters())) {
		cert, err := r.issuer.IssueCert(ctx, teleportCluster, "" /*kubeCluster*/, nil /*mfaCheck*/)
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(ctx, "Shared client cert issued for Teleport cluster", "teleport_cluster", teleportCluster)
		r.add(teleportCluster, "", *cert)
	}
	return nil
}

// DropShared removes the shared certs issued so far,
// so a fallback to per-cluster certs cannot leave one behind for the proxy to serve.
func (r *kubeCertRun) DropShared() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.certs {
		if key.KubeCluster == "" {
			delete(r.certs, key)
		}
	}
}

func (r *kubeCertRun) add(teleportCluster, kubeCluster string, cert tls.Certificate) {
	r.mu.Lock()
	r.certs.Add(teleportCluster, kubeCluster, cert)
	r.mu.Unlock()
}
