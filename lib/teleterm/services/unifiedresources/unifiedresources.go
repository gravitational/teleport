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
	"log/slog"
	"slices"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/componentfeatures"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/utils/aws"
)

var supportedResourceKinds = []string{
	types.KindNode,
	types.KindDatabase,
	types.KindKubernetesCluster,
	types.KindApp,
	types.KindSAMLIdPServiceProvider,
	types.KindWindowsDesktop,
	types.KindMCP,
}

type listUnifiedResourcesClient interface {
	apiclient.ListUnifiedResourcesClient
	// GetProxies returns a list of proxy servers registered in the cluster
	//
	// Deprecated: Prefer paginated variant [ListProxyServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetProxies() ([]types.Server, error)
	// ListProxyServers returns a paginated list of proxy servers registered in the cluster
	ListProxyServers(ctx context.Context, pageSize int, nextToken string) ([]types.Server, string, error)
	// GetAuthServers returns a list of auth servers registered in the cluster
	//
	// Deprecated: Prefer paginated variant [ListAuthServers].
	//
	// TODO(kiosion): DELETE IN 21.0.0
	GetAuthServers() ([]types.Server, error)
	// ListAuthServers returns a paginated list of auth servers registered in the cluster
	ListAuthServers(ctx context.Context, pageSize int, nextToken string) ([]types.Server, string, error)
}

func List(ctx context.Context, cluster *clusters.Cluster, client listUnifiedResourcesClient, req *proto.ListUnifiedResourcesRequest, logger *slog.Logger) (*ListResponse, error) {
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
	req.IncludeLogins = true
	enrichedResources, nextKey, err := apiclient.GetUnifiedResourcePage(ctx, client, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &ListResponse{
		NextKey: nextKey,
	}

	clusterAuthProxyServerFeatures := componentfeatures.GetClusterAuthProxyServerFeatures(ctx, client, logger)

	for _, enrichedResource := range enrichedResources {
		requiresRequest := enrichedResource.RequiresRequest
		switch r := enrichedResource.ResourceWithLabels.(type) {
		case types.Server:
			logins, err := libclient.CalculateSSHLogins(cluster.GetLoggedInUser().SSHLogins, enrichedResource.Logins)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			response.Resources = append(response.Resources, UnifiedResource{
				Server: &clusters.Server{
					URI:    cluster.URI.AppendServer(r.GetName()),
					Server: r,
					Logins: logins,
				},
				RequiresRequest: requiresRequest,
			})
		case types.DatabaseServer:
			db := r.GetDatabase()
			response.Resources = append(response.Resources, UnifiedResource{
				Database: &clusters.Database{
					URI:          cluster.URI.AppendDB(db.GetName()),
					Database:     db,
					TargetHealth: r.GetTargetHealth(),
				},
				RequiresRequest: requiresRequest,
			})
		// TODO(kiosion): Much of this logic could be shared between apiserver's clusterUnifiedResourcesGet and here.
		case types.AppServer:
			app := r.GetApp()

			// Compute AWS roles if present/applicable.
			awsRoles := computeAWSRolesWithRequiresRequest(
				enrichedResource.Logins,
				cluster.GetAWSRoles(app),
				app.GetAWSAccountID(),
				req.IncludeRequestable,
			)

			// Compute end-to-end feature support for this app: only features that are supported by the AppServer *and*
			// by all required cluster hops (Auth + Proxy), so clients can hide features that would fail somewhere
			// along the request path.
			appComponentFeatures := componentfeatures.Intersect(r.GetComponentFeatures(), clusterAuthProxyServerFeatures)

			response.Resources = append(response.Resources, UnifiedResource{
				App: &clusters.App{
					URI:                 cluster.URI.AppendApp(app.GetName()),
					FQDN:                cluster.AssembleAppFQDN(app),
					AWSRoles:            awsRoles,
					SupportedFeatureIDs: componentfeatures.ToIntegers(appComponentFeatures),
					App:                 app,
				},
				RequiresRequest: requiresRequest,
			})
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
			ur := UnifiedResource{
				Kube: &clusters.Kube{
					URI:               cluster.URI.AppendKube(kubeCluster.GetName()),
					KubernetesCluster: kubeCluster,
				},
				RequiresRequest: requiresRequest,
			}
			targetHealth := r.GetTargetHealth()
			if targetHealth != nil {
				ur.Kube.TargetHealth = *targetHealth
			}
			response.Resources = append(response.Resources, ur)
		case types.WindowsDesktop:
			response.Resources = append(response.Resources, UnifiedResource{
				WindowsDesktop: &clusters.WindowsDesktop{
					URI:            cluster.URI.AppendWindowsDesktop(r.GetName()),
					WindowsDesktop: r,
					Logins:         enrichedResource.Logins,
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
	WindowsDesktop         *clusters.WindowsDesktop
	RequiresRequest        bool
}

// computeAWSRolesWithRequiresRequest computes AWS roles with the RequiresRequest field set.
func computeAWSRolesWithRequiresRequest(visibleRoleARNs []string, grantedRoles aws.Roles, accountID string, includeRequestable bool) aws.Roles {
	// Filter visible roles by account ID and convert to aws.Roles.
	visibleRoles := aws.FilterAWSRoles(visibleRoleARNs, accountID)
	grantedSet := make(map[string]struct{}, len(grantedRoles))
	for _, role := range grantedRoles {
		grantedSet[role.ARN] = struct{}{}
	}

	// Mark each visible role as requiring request if not in granted set.
	result := make(aws.Roles, 0, len(visibleRoles))
	for _, role := range visibleRoles {
		_, isGranted := grantedSet[role.ARN]
		// If req does not include requestable resources, skip non-granted roles
		if !isGranted && !includeRequestable {
			continue
		}
		result = append(result, aws.Role{
			Name:            role.Name,
			Display:         role.Display,
			ARN:             role.ARN,
			AccountID:       role.AccountID,
			RequiresRequest: !isGranted,
		})
	}

	return result
}
