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

package unifiedresources

import (
	"context"
	"slices"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

var supportedResourceKinds = []string{
	types.KindNode,
	types.KindDatabase,
	types.KindKubernetesCluster,
	types.KindApp,
	types.KindSAMLIdPServiceProvider,
}

func List(ctx context.Context, cluster *clusters.Cluster, client apiclient.ListUnifiedResourcesClient, req *proto.ListUnifiedResourcesRequest) (*ListResponse, error) {
	kinds := req.GetKinds()
	if len(kinds) == 0 {
		kinds = supportedResourceKinds
	} else {
		for _, kind := range kinds {
			if !slices.Contains(supportedResourceKinds, kind) {
				return nil, trace.BadParameter("unsupported resource kind: %s", kind)
			}
		}
	}

	req.Kinds = kinds
	enrichedResources, nextKey, err := apiclient.GetUnifiedResourcePage(ctx, client, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &ListResponse{
		NextKey: nextKey,
	}

	for _, enrichedResource := range enrichedResources {
		requiresRequest := enrichedResource.RequiresRequest
		switch r := enrichedResource.ResourceWithLabels.(type) {
		case types.Server:
			response.Resources = append(response.Resources, UnifiedResource{
				Server: &clusters.Server{
					URI:    cluster.URI.AppendServer(r.GetName()),
					Server: r,
				},
				RequiresRequest: requiresRequest,
			})
		case types.DatabaseServer:
			db := r.GetDatabase()
			response.Resources = append(response.Resources, UnifiedResource{
				Database: &clusters.Database{
					URI:      cluster.URI.AppendDB(db.GetName()),
					Database: db,
				},
				RequiresRequest: requiresRequest,
			})
		case types.AppServer:
			app := r.GetApp()

			response.Resources = append(response.Resources, UnifiedResource{
				App: &clusters.App{
					URI:      cluster.URI.AppendApp(app.GetName()),
					FQDN:     cluster.AssembleAppFQDN(app),
					AWSRoles: cluster.GetAWSRoles(app),
					App:      app,
				},
				RequiresRequest: requiresRequest,
			})
		case types.AppServerOrSAMLIdPServiceProvider:
			//nolint:staticcheck // SA1019. TODO(sshah) DELETE IN 17.0
			if r.IsAppServer() {
				app := r.GetAppServer().GetApp()
				response.Resources = append(response.Resources, UnifiedResource{
					App: &clusters.App{
						URI:      cluster.URI.AppendApp(app.GetName()),
						FQDN:     cluster.AssembleAppFQDN(app),
						AWSRoles: cluster.GetAWSRoles(app),
						App:      app,
					},
					RequiresRequest: requiresRequest,
				})
			} else {
				provider := r.GetSAMLIdPServiceProvider()
				response.Resources = append(response.Resources, UnifiedResource{
					SAMLIdPServiceProvider: &clusters.SAMLIdPServiceProvider{
						URI:      cluster.URI.AppendApp(provider.GetName()),
						Provider: provider,
					},
					RequiresRequest: requiresRequest,
				})
			}
		case types.SAMLIdPServiceProvider:
			response.Resources = append(response.Resources, UnifiedResource{
				SAMLIdPServiceProvider: &clusters.SAMLIdPServiceProvider{
					URI:      cluster.URI.AppendApp(r.GetName()),
					Provider: r,
				},
				RequiresRequest: requiresRequest,
			})
		case types.KubeCluster:
			kubeCluster := r
			response.Resources = append(response.Resources, UnifiedResource{
				Kube: &clusters.Kube{
					URI:               cluster.URI.AppendKube(kubeCluster.GetName()),
					KubernetesCluster: kubeCluster,
				},
				RequiresRequest: requiresRequest,
			})
		case types.KubeServer:
			kubeCluster := r.GetCluster()
			response.Resources = append(response.Resources, UnifiedResource{
				Kube: &clusters.Kube{
					URI:               cluster.URI.AppendKube(kubeCluster.GetName()),
					KubernetesCluster: kubeCluster,
				},
				RequiresRequest: requiresRequest,
			})
		}
	}

	return response, nil
}

type ListResponse struct {
	Resources []UnifiedResource
	NextKey   string
}

// UnifiedResource combines all resource types into a single struct.
// Only one filed should be set at a time.
type UnifiedResource struct {
	Server                 *clusters.Server
	Database               *clusters.Database
	Kube                   *clusters.Kube
	App                    *clusters.App
	SAMLIdPServiceProvider *clusters.SAMLIdPServiceProvider
	RequiresRequest        bool
}
