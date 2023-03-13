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
		resp *types.ListResourcesResponse

		authClient  auth.ClientI
		proxyClient *client.ProxyClient
		err         error
	)

	sortBy := types.GetSortByFromString(r.SortBy)
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

		resp, err = authClient.ListResources(ctx, proto.ListResourcesRequest{
			Namespace:           defaults.Namespace,
			ResourceType:        types.KindKubernetesCluster,
			Limit:               r.Limit,
			SortBy:              sortBy,
			StartKey:            r.StartKey,
			PredicateExpression: r.Query,
			SearchKeywords:      client.ParseSearchKeywords(r.Search, ' '),
			UseSearchAsRoles:    r.SearchAsRoles == "yes",
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubeClusters, err := types.ResourcesWithLabels(resp.Resources).AsKubeClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	results := []Kube{}
	for _, cluster := range kubeClusters {
		results = append(results, Kube{
			URI:               c.URI.AppendKube(cluster.GetName()),
			KubernetesCluster: cluster,
		})
	}

	return &GetKubesResponse{
		Kubes:      results,
		StartKey:   resp.NextKey,
		TotalCount: resp.TotalCount,
	}, nil
}

type GetKubesResponse struct {
	Kubes []Kube
	// StartKey is the next key to use as a starting point.
	StartKey string
	// // TotalCount is the total number of resources available as a whole.
	TotalCount int
}
