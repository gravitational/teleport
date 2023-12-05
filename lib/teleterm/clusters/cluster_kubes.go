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
	"fmt"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	kubeutils "github.com/gravitational/teleport/lib/kube/utils"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
)

// Kube describes kubernetes service
type Kube struct {
	// URI is the kube URI
	URI uri.ResourceURI

	KubernetesCluster types.KubeCluster
}

// GetKubes returns a paginated kubes list
func (c *Cluster) GetKubes(ctx context.Context, r *api.GetKubesRequest) (*GetKubesResponse, error) {
	var (
		page        apiclient.ResourcePage[types.KubeCluster]
		authClient  auth.ClientI
		proxyClient *client.ProxyClient
		err         error
	)

	req := &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindKubernetesCluster,
		Limit:               r.Limit,
		SortBy:              types.GetSortByFromString(r.SortBy),
		StartKey:            r.StartKey,
		PredicateExpression: r.Query,
		SearchKeywords:      client.ParseSearchKeywords(r.Search, ' '),
		UseSearchAsRoles:    r.SearchAsRoles == "yes",
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		page, err = apiclient.GetResourcePage[types.KubeCluster](ctx, authClient, req)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := make([]Kube, 0, len(page.Resources))
	for _, cluster := range page.Resources {
		results = append(results, Kube{
			URI:               c.URI.AppendKube(cluster.GetName()),
			KubernetesCluster: cluster,
		})
	}

	return &GetKubesResponse{
		Kubes:      results,
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}, nil
}

type GetKubesResponse struct {
	Kubes []Kube
	// StartKey is the next key to use as a starting point.
	StartKey string
	// // TotalCount is the total number of resources available as a whole.
	TotalCount int
}

// reissueKubeCert issue new certificates for kube cluster and saves them to disk.
func (c *Cluster) reissueKubeCert(ctx context.Context, kubeCluster string) error {
	return trace.Wrap(AddMetadataToRetryableError(ctx, func() error {
		// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
		err := c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
			RouteToCluster: c.clusterClient.SiteName,
			AccessRequests: c.status.ActiveRequests.AccessRequests,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Fetch the certs for the kube cluster.
		return trace.Wrap(c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
			RouteToCluster:    c.clusterClient.SiteName,
			KubernetesCluster: kubeCluster,
			AccessRequests:    c.status.ActiveRequests.AccessRequests,
		}))
	}))
}

func (c *Cluster) getKube(ctx context.Context, kubeCluster string) (types.KubeCluster, error) {
	var kubeClusters []types.KubeCluster
	err := AddMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err := proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

		kubeClusters, err = kubeutils.ListKubeClustersWithFilters(ctx, authClient, proto.ListResourcesRequest{
			PredicateExpression: fmt.Sprintf("name == %q", kubeCluster),
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
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
