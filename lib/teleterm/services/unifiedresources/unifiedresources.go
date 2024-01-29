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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

var supportedResourceKinds = []string{
	types.KindNode,
	types.KindDatabase,
	types.KindKubernetesCluster,
	types.KindApp,
}

func List(ctx context.Context, cluster *clusters.Cluster, client Client, req *proto.ListUnifiedResourcesRequest) (*ListResponse, error) {
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
	unifiedResourcesResponse, err := client.ListUnifiedResources(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &ListResponse{
		NextKey: unifiedResourcesResponse.NextKey,
	}

	for _, unifiedResource := range unifiedResourcesResponse.Resources {
		switch e := unifiedResource.GetResource().(type) {
		case *proto.PaginatedResource_Node:
			response.Resources = append(response.Resources, UnifiedResource{
				Server: &clusters.Server{
					URI:    cluster.URI.AppendServer(e.Node.GetName()),
					Server: e.Node,
				},
			})
		case *proto.PaginatedResource_DatabaseServer:
			response.Resources = append(response.Resources, UnifiedResource{
				Database: &clusters.Database{
					URI:      cluster.URI.AppendDB(e.DatabaseServer.GetName()),
					Database: e.DatabaseServer.GetDatabase(),
				},
			})
		case *proto.PaginatedResource_KubernetesServer:
			response.Resources = append(response.Resources, UnifiedResource{
				Kube: &clusters.Kube{
					URI:               cluster.URI.AppendKube(e.KubernetesServer.GetCluster().GetName()),
					KubernetesCluster: e.KubernetesServer.GetCluster(),
				},
			})
		case *proto.PaginatedResource_AppServer:
			app := e.AppServer.GetApp()

			response.Resources = append(response.Resources, UnifiedResource{
				App: &clusters.App{
					URI:      cluster.URI.AppendApp(app.GetName()),
					FQDN:     cluster.AssembleAppFQDN(app),
					AWSRoles: cluster.GetAWSRoles(app),
					App:      app,
				},
			})
		case *proto.PaginatedResource_AppServerOrSAMLIdPServiceProvider:
			if e.AppServerOrSAMLIdPServiceProvider.IsAppServer() {
				app := e.AppServerOrSAMLIdPServiceProvider.GetAppServer().GetApp()
				response.Resources = append(response.Resources, UnifiedResource{
					App: &clusters.App{
						URI:      cluster.URI.AppendApp(app.GetName()),
						FQDN:     cluster.AssembleAppFQDN(app),
						AWSRoles: cluster.GetAWSRoles(app),
						App:      app,
					},
				})
			} else {
				provider := e.AppServerOrSAMLIdPServiceProvider.GetSAMLIdPServiceProvider()
				response.Resources = append(response.Resources, UnifiedResource{
					SAMLIdPServiceProvider: &clusters.SAMLIdPServiceProvider{
						URI:      cluster.URI.AppendApp(provider.GetName()),
						Provider: provider,
					},
				})
			}
		}
	}

	return response, nil
}

// Client represents auth.ClientI methods used by [List].
// During a normal operation, auth.ClientI is passed as this interface.
type Client interface {
	// See auth.ClientI.ListUnifiedResources.
	ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error)
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
}
