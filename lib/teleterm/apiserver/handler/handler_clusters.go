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

package handler

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/constants"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
)

// ListRootClusters lists root clusters
func (s *Handler) ListRootClusters(ctx context.Context, r *api.ListClustersRequest) (*api.ListClustersResponse, error) {
	clusters, err := s.DaemonService.ListRootClusters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := []*api.Cluster{}
	for _, cluster := range clusters {
		result = append(result, newAPIRootCluster(cluster))
	}

	return &api.ListClustersResponse{
		Clusters: result,
	}, nil
}

// ListLeafClusters lists leaf clusters
func (s *Handler) ListLeafClusters(ctx context.Context, req *api.ListLeafClustersRequest) (*api.ListClustersResponse, error) {
	leaves, err := s.DaemonService.ListLeafClusters(ctx, req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &api.ListClustersResponse{}
	for _, leaf := range leaves {
		response.Clusters = append(response.Clusters, newAPILeafCluster(leaf))
	}

	return response, nil
}

// AddCluster creates a new cluster
func (s *Handler) AddCluster(ctx context.Context, req *api.AddClusterRequest) (*api.Cluster, error) {
	cluster, err := s.DaemonService.AddCluster(ctx, req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newAPIRootCluster(cluster), nil
}

// RemoveCluster removes a cluster from local system
func (s *Handler) RemoveCluster(ctx context.Context, req *api.RemoveClusterRequest) (*api.EmptyResponse, error) {
	if err := s.DaemonService.RemoveCluster(ctx, req.ClusterUri); err != nil {
		return nil, trace.Wrap(err)
	}

	return &api.EmptyResponse{}, nil
}

// GetCluster returns a cluster
func (s *Handler) GetCluster(ctx context.Context, req *api.GetClusterRequest) (*api.Cluster, error) {
	cluster, _, err := s.DaemonService.ResolveClusterWithDetails(ctx, req.ClusterUri)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apiRootClusterWithDetails, err := newAPIRootClusterWithDetails(cluster)

	return apiRootClusterWithDetails, trace.Wrap(err)
}

func newAPIRootCluster(cluster *clusters.Cluster) *api.Cluster {
	loggedInUser := cluster.GetLoggedInUser()

	apiCluster := &api.Cluster{
		Uri:       cluster.URI.String(),
		Name:      cluster.Name,
		ProxyHost: cluster.GetProxyHost(),
		Connected: cluster.Connected(),
		LoggedInUser: &api.LoggedInUser{
			Name:            loggedInUser.Name,
			SshLogins:       loggedInUser.SSHLogins,
			Roles:           loggedInUser.Roles,
			ActiveRequests:  loggedInUser.ActiveRequests,
			IsDeviceTrusted: cluster.HasDeviceTrustExtensions(),
		},
		SsoHost: cluster.SSOHost,
	}

	if cluster.GetProfileStatusError() != nil {
		apiCluster.ProfileStatusError = cluster.GetProfileStatusError().Error()
	}

	return apiCluster
}

func newAPIRootClusterWithDetails(cluster *clusters.ClusterWithDetails) (*api.Cluster, error) {
	apiCluster := newAPIRootCluster(cluster.Cluster)

	apiCluster.Features = &api.Features{
		AdvancedAccessWorkflows: cluster.Features.GetAdvancedAccessWorkflows(),
		IsUsageBasedBilling:     cluster.Features.GetIsUsageBased(),
	}
	apiCluster.LoggedInUser.RequestableRoles = cluster.RequestableRoles
	apiCluster.LoggedInUser.SuggestedReviewers = cluster.SuggestedReviewers
	apiCluster.AuthClusterId = cluster.AuthClusterID
	apiCluster.LoggedInUser.Acl = cluster.ACL
	userType, err := clusters.UserTypeFromString(cluster.UserType)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apiCluster.LoggedInUser.UserType = userType
	apiCluster.LoggedInUser.TrustedDeviceRequirement = cluster.TrustedDeviceRequirement
	apiCluster.ProxyVersion = cluster.ProxyVersion

	switch cluster.ShowResources {
	case constants.ShowResourcesaccessibleOnly:
		apiCluster.ShowResources = api.ShowResources_SHOW_RESOURCES_ACCESSIBLE_ONLY
	case constants.ShowResourcesRequestable:
		apiCluster.ShowResources = api.ShowResources_SHOW_RESOURCES_REQUESTABLE
	default:
		// If the UI config for ShowResources is not set, the default is `requestable`.
		apiCluster.ShowResources = api.ShowResources_SHOW_RESOURCES_REQUESTABLE
	}

	return apiCluster, nil
}

func newAPILeafCluster(leaf clusters.LeafCluster) *api.Cluster {
	return &api.Cluster{
		Name:      leaf.Name,
		Uri:       leaf.URI.String(),
		Connected: leaf.Connected,
		Leaf:      true,
		LoggedInUser: &api.LoggedInUser{
			Name:      leaf.LoggedInUser.Name,
			SshLogins: leaf.LoggedInUser.SSHLogins,
			Roles:     leaf.LoggedInUser.Roles,
		},
	}
}
