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

package services

import (
	"context"

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
)

// WorkloadClusterServiceGetter defines only read-only service methods.
type WorkloadClusterServiceGetter interface {
	// GetWorkloadCluster gets the requested WorkloadCluster resource.
	GetWorkloadCluster(ctx context.Context, name string) (*workloadcluster.WorkloadCluster, error)

	// ListWorkloadClusters returns a WorkloadCluster page.
	ListWorkloadClusters(ctx context.Context, pageSize int, pageToken string) ([]*workloadcluster.WorkloadCluster, string, error)
}

// WorkloadClusterService manges WorkloadCluster resources.
type WorkloadClusterService interface {
	WorkloadClusterServiceGetter

	// CreateWorkloadCluster creates the requested WorkloadCluster resource.
	CreateWorkloadCluster(ctx context.Context, config *workloadcluster.WorkloadCluster) (*workloadcluster.WorkloadCluster, error)

	// UpdateWorkloadCluster updates the requested WorkloadCluster resource.
	UpdateWorkloadCluster(ctx context.Context, config *workloadcluster.WorkloadCluster) (*workloadcluster.WorkloadCluster, error)

	// UpsertWorkloadCluster sets the requested WorkloadCluster resource.
	UpsertWorkloadCluster(ctx context.Context, c *workloadcluster.WorkloadCluster) (*workloadcluster.WorkloadCluster, error)

	// DeleteWorkloadCluster deletes the requested WorkloadCluster resource.
	DeleteWorkloadCluster(ctx context.Context, name string) error
}

// MarshalWorkloadCluster marshals the WorkloadCluster object into a JSON byte array.
func MarshalWorkloadCluster(object *workloadcluster.WorkloadCluster, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalWorkloadCluster unmarshals the WorkloadCluster object from a JSON byte array.
func UnmarshalWorkloadCluster(data []byte, opts ...MarshalOption) (*workloadcluster.WorkloadCluster, error) {
	return UnmarshalProtoResource[*workloadcluster.WorkloadCluster](data, opts...)
}
