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

package clusters

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	kubeclient "github.com/gravitational/teleport/lib/client/kube"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/utils"
)

// Kube describes kubernetes service
type Kube struct {
	// URI is the kube URI
	URI uri.ResourceURI

	KubernetesCluster types.KubeCluster
}

// reissueKubeCert issue new certificates for kube cluster and saves them to disk.
func (c *Cluster) reissueKubeCert(ctx context.Context, clusterClient *client.ClusterClient, kubeCluster string) (tls.Certificate, error) {
	// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
	err := clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		AccessRequests: c.status.ActiveRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	result, err := clusterClient.IssueUserCertsWithMFA(
		ctx, client.ReissueParams{
			RouteToCluster:    c.clusterClient.SiteName,
			KubernetesCluster: kubeCluster,
			RequesterName:     proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY,
			TTL:               c.clusterClient.KeyTTL,
		},
	)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	// Make sure the cert is allowed to access the cluster.
	// At this point we already know that the user has access to the cluster
	// via the RBAC rules, but we also need to make sure that the user has
	// access to the cluster with at least one kubernetes_user or kubernetes_group
	// defined.
	if err := kubeclient.CheckIfCertsAreAllowedToAccessCluster(
		result.KeyRing,
		clusterClient.RootClusterName(),
		c.Name,
		kubeCluster); err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, err := result.KeyRing.KubeTLSCert(kubeCluster)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	// Set leaf so we don't have to parse it on each request.
	leaf, err := utils.TLSCertLeaf(cert)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	cert.Leaf = leaf

	return cert, nil
}

func (c *Cluster) getKube(ctx context.Context, authClient authclient.ClientI, kubeCluster string) (types.KubeCluster, error) {
	var kubeClusters []types.KubeCluster
	err := AddMetadataToRetryableError(ctx, func() error {
		var err error
		kubeClusters, err = kubeutils.ListKubeClustersWithFilters(ctx, authClient, proto.ListResourcesRequest{
			PredicateExpression: fmt.Sprintf("name == %q", kubeCluster),
		})

		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, cluster := range kubeClusters {
		if cluster.GetName() == kubeCluster {
			return cluster, nil
		}
	}
	return nil, trace.NotFound("kubernetes cluster %q not found", kubeCluster)
}
