/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	// GetCloudCluster gets the CloudCluster singleton resource.
	GetCloudCluster(ctx context.Context, name string) (*cloudcluster.CloudCluster, error)

	// ListCloudClusters returns a CloudCluster page.
	ListCloudClusters(ctx context.Context, pageSize int, pageToken string) ([]*cloudcluster.CloudCluster, string, error)
}

// CloudClusterService stores the autoupdate service.
type CloudClusterService interface {
	CloudClusterServiceGetter

	// CreateCloudCluster creates the CloudCluster singleton resource.
	CreateCloudCluster(ctx context.Context, config *cloudcluster.CloudCluster) (*cloudcluster.CloudCluster, error)

	// UpdateCloudCluster updates the CloudCluster singleton resource.
	UpdateCloudCluster(ctx context.Context, config *cloudcluster.CloudCluster) (*cloudcluster.CloudCluster, error)

	// UpsertCloudCluster sets the CloudCluster singleton resource.
	UpsertCloudCluster(ctx context.Context, c *cloudcluster.CloudCluster) (*cloudcluster.CloudCluster, error)

	// DeleteCloudCluster deletes the CloudCluster singleton resource.
	DeleteCloudCluster(ctx context.Context, name string) error
}

// MarshalCrownJewel marshals the CrownJewel object into a JSON byte array.
func MarshalCloudCluster(object *cloudcluster.CloudCluster, opts ...MarshalOption) ([]byte, error) {
	return MarshalProtoResource(object, opts...)
}

// UnmarshalCrownJewel unmarshals the CrownJewel object from a JSON byte array.
func UnmarshalCloudCluster(data []byte, opts ...MarshalOption) (*cloudcluster.CloudCluster, error) {
	return UnmarshalProtoResource[*cloudcluster.CloudCluster](data, opts...)
}
