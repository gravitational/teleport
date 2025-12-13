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

	"github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
)

// CloudClusterServiceGetter defines only read-only service methods.
type CloudClusterServiceGetter interface {
	// GetCloudCluster gets the requested CloudCluster resource.
	GetCloudCluster(ctx context.Context, name string) (*cloudcluster.CloudCluster, error)

	// ListCloudClusters returns a CloudCluster page.
	ListCloudClusters(ctx context.Context, pageSize int, pageToken string) ([]*cloudcluster.CloudCluster, string, error)
}

// CloudClusterService manges CloudCluster resources.
type CloudClusterService interface {
	CloudClusterServiceGetter

	// CreateCloudCluster creates the requested CloudCluster resource.
	CreateCloudCluster(ctx context.Context, config *cloudcluster.CloudCluster) (*cloudcluster.CloudCluster, error)

	// UpdateCloudCluster updates the requested CloudCluster resource.
	UpdateCloudCluster(ctx context.Context, config *cloudcluster.CloudCluster) (*cloudcluster.CloudCluster, error)

	// UpsertCloudCluster sets the requested CloudCluster resource.
	UpsertCloudCluster(ctx context.Context, c *cloudcluster.CloudCluster) (*cloudcluster.CloudCluster, error)

	// DeleteCloudCluster deletes the requested CloudCluster resource.
	DeleteCloudCluster(ctx context.Context, name string) error
}

// MarshalCloudCluster marshals the CloudCluster object into a JSON byte array.
func MarshalCloudCluster(object *cloudcluster.CloudCluster, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalCloudCluster unmarshals the CloudCluster object from a JSON byte array.
func UnmarshalCloudCluster(data []byte, opts ...MarshalOption) (*cloudcluster.CloudCluster, error) {
	return UnmarshalProtoResource[*cloudcluster.CloudCluster](data, opts...)
}
