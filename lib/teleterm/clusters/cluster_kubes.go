/*
Copyright 2021 Gravitational, Inc.

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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
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

	err = addMetadataToRetryableError(ctx, func() error {
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

// reissueKubeCert issue new certificates for kube cluster and saves then to disk.
func (c *Cluster) reissueKubeCert(ctx context.Context, kubeCluster string) error {
	return trace.Wrap(addMetadataToRetryableError(ctx, func() error {
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
