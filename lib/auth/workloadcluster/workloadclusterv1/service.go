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

package workloadclusterv1

import (
	"context"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/lib/services"
)

// Backend interface for manipulating WorkloadCluster resources.
type Backend interface {
	services.WorkloadClusterService
}

// Service implements the gRPC API layer for the WorkloadCluster.
type Service struct {
	workloadcluster.UnimplementedWorkloadClusterServiceServer
}

// NewService returns a new service that returns a license error for every RPC
func NewService() *Service {
	return &Service{}
}

// GetWorkloadCluster gets the requested WorkloadCluster.
func (s *Service) GetWorkloadCluster(ctx context.Context, req *workloadcluster.GetWorkloadClusterRequest) (*workloadcluster.WorkloadCluster, error) {
	return nil, requireCloud()
}

// CreateWorkloadCluster creates a new WorkloadCluster.
func (s *Service) CreateWorkloadCluster(ctx context.Context, req *workloadcluster.CreateWorkloadClusterRequest) (*workloadcluster.WorkloadCluster, error) {
	return nil, requireCloud()
}

// UpdateWorkloadCluster updates the requested WorkloadCluster.
func (s *Service) UpdateWorkloadCluster(ctx context.Context, req *workloadcluster.UpdateWorkloadClusterRequest) (*workloadcluster.WorkloadCluster, error) {
	return nil, requireCloud()
}

// UpsertWorkloadCluster updates or creates the requested WorkloadCluster.
func (s *Service) UpsertWorkloadCluster(ctx context.Context, req *workloadcluster.UpsertWorkloadClusterRequest) (*workloadcluster.WorkloadCluster, error) {
	return nil, requireCloud()
}

// DeleteWorkloadCluster deletes the requested WorkloadCluster.
func (s *Service) DeleteWorkloadCluster(ctx context.Context, req *workloadcluster.DeleteWorkloadClusterRequest) (*emptypb.Empty, error) {
	return nil, requireCloud()
}

// ListWorkloadClusters returns a list of workload clusters.
func (s *Service) ListWorkloadClusters(ctx context.Context, req *workloadcluster.ListWorkloadClustersRequest) (*workloadcluster.ListWorkloadClustersResponse, error) {
	return nil, requireCloud()
}

func requireCloud() error {
	return trace.AccessDenied(
		"workload_cluster resources are only available for Teleport Cloud users")
}
