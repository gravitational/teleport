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

package cloudclusterv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	"github.com/gravitational/teleport/lib/services"
)

// Backend interface for manipulating CloudCluster resources.
type Backend interface {
	services.CloudClusterService
}

// Service implements the gRPC API layer for the CloudCluster.
type Service struct {
	cloudcluster.UnimplementedCloudClusterServiceServer
}

// NewService returns a new service that returns a license error for every RPC
func NewService() *Service {
	return &Service{}
}

// GetCloudCluster gets the requested CloudCluster.
func (s *Service) GetCloudCluster(ctx context.Context, req *cloudcluster.GetCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	return nil, requireEnterprise()
}

// CreateCloudCluster creates a new CloudCluster.
func (s *Service) CreateCloudCluster(ctx context.Context, req *cloudcluster.CreateCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	return nil, requireEnterprise()
}

// UpdateCloudCluster updates the requested CloudCluster.
func (s *Service) UpdateCloudCluster(ctx context.Context, req *cloudcluster.UpdateCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	return nil, requireEnterprise()
}

// UpsertCloudCluster updates or creates the requested Cloud Cluster.
func (s *Service) UpsertCloudCluster(ctx context.Context, req *cloudcluster.UpsertCloudClusterRequest) (*cloudcluster.CloudCluster, error) {
	return nil, requireEnterprise()
}

// DeleteCloudCluster deletes the requested CloudCluster.
func (s *Service) DeleteCloudCluster(ctx context.Context, req *cloudcluster.DeleteCloudClusterRequest) (*emptypb.Empty, error) {
	return nil, requireEnterprise()
}

// ListCloudClusters returns a list of cloud clusters.
func (s *Service) ListCloudClusters(ctx context.Context, req *cloudcluster.ListCloudClustersRequest) (*cloudcluster.ListCloudClustersResponse, error) {
	return nil, requireEnterprise()
}

func requireEnterprise() error {
	return trace.AccessDenied(
		"cloud cluster resources are only available for Teleport Cloud users")
}
