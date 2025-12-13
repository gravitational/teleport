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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	cloudclusterv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/cloudcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type cloudClusterCollection struct {
	cloudClusters []*cloudclusterv1pb.CloudCluster
}

func NewCloudClusterCollection(cloudClusters []*cloudclusterv1pb.CloudCluster) Collection {
	return &cloudClusterCollection{
		cloudClusters: cloudClusters,
	}
}

func (c *cloudClusterCollection) Resources() []types.Resource {
	resources := make([]types.Resource, 0, len(c.cloudClusters))

	for _, cc := range c.cloudClusters {
		resources = append(resources, types.ProtoResource153ToLegacy(cc))
	}

	return resources
}

func (c *cloudClusterCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, cc := range c.cloudClusters {
		t.AddRow([]string{
			cc.GetMetadata().GetName(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func cloudClusterHandler() Handler {
	return Handler{
		getHandler:    getCloudCluster,
		createHandler: createCloudCluster,
		updateHandler: updateCloudCluster,
		deleteHandler: deleteCloudCluster,
		singleton:     false,
		mfaRequired:   true,
		description:   "A cloud cluster managed by Teleport",
	}
}

func getCloudCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		clusters, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*cloudclusterv1pb.CloudCluster, string, error) {
			resp, nextToken, err := client.CloudClusterServiceClient().ListCloudClusters(ctx, limit, token)

			return resp, nextToken, trace.Wrap(err)
		}))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &cloudClusterCollection{cloudClusters: clusters}, nil
	}

	cloudCluster, err := client.CloudClusterServiceClient().GetCloudCluster(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cloudClusterCollection{
		cloudClusters: []*cloudclusterv1pb.CloudCluster{
			cloudCluster,
		},
	}, nil
}

func createCloudCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cc, err := services.UnmarshalProtoResource[*cloudclusterv1pb.CloudCluster](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		_, err = client.CloudClusterServiceClient().UpsertCloudCluster(ctx, cc)
	} else {
		_, err = client.CloudClusterServiceClient().CreateCloudCluster(ctx, cc)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("cloud_cluster %q has been created", cc.Metadata.GetName())
	return nil
}

func updateCloudCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cc, err := services.UnmarshalProtoResource[*cloudclusterv1pb.CloudCluster](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.CloudClusterServiceClient().UpdateCloudCluster(ctx, cc)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("cloud cluster %q has been updated\n", cc.Metadata.GetName())
	return nil
}

func deleteCloudCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.CloudClusterServiceClient().DeleteCloudCluster(ctx, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("cloud_cluster %q has been deleted", ref.Name)
	return nil
}
