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

	workloadclusterv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadcluster/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type workloadClusterCollection struct {
	workloadClusters []*workloadclusterv1pb.WorkloadCluster
}

func (c *workloadClusterCollection) Resources() []types.Resource {
	resources := make([]types.Resource, 0, len(c.workloadClusters))

	for _, cc := range c.workloadClusters {
		resources = append(resources, types.ProtoResource153ToLegacy(cc))
	}

	return resources
}

func (c *workloadClusterCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, cc := range c.workloadClusters {
		t.AddRow([]string{
			cc.GetMetadata().GetName(),
		})
	}
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func workloadClusterHandler() Handler {
	return Handler{
		getHandler:    getWorkloadCluster,
		createHandler: createWorkloadCluster,
		updateHandler: updateWorkloadCluster,
		deleteHandler: deleteWorkloadCluster,
		singleton:     false,
		mfaRequired:   true,
		description:   "A workload cluster managed by Teleport",
	}
}

func getWorkloadCluster(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	if ref.Name == "" {
		clusters, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, token string) ([]*workloadclusterv1pb.WorkloadCluster, string, error) {
			resp, nextToken, err := client.ListWorkloadClusters(ctx, limit, token)

			return resp, nextToken, trace.Wrap(err)
		}))
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return &workloadClusterCollection{workloadClusters: clusters}, nil
	}

	workloadCluster, err := client.GetWorkloadCluster(ctx, ref.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &workloadClusterCollection{
		workloadClusters: []*workloadclusterv1pb.WorkloadCluster{
			workloadCluster,
		},
	}, nil
}

func createWorkloadCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cc, err := services.UnmarshalProtoResource[*workloadclusterv1pb.WorkloadCluster](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	if opts.Force {
		_, err = client.UpsertWorkloadCluster(ctx, cc)
	} else {
		_, err = client.CreateWorkloadCluster(ctx, cc)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("workload cluster %q has been created\n", cc.Metadata.GetName())
	return nil
}

func updateWorkloadCluster(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	cc, err := services.UnmarshalProtoResource[*workloadclusterv1pb.WorkloadCluster](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.UpdateWorkloadCluster(ctx, cc)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("workload cluster %q has been updated\n", cc.Metadata.GetName())
	return nil
}

func deleteWorkloadCluster(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	err := client.DeleteWorkloadCluster(ctx, ref.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("workload cluster %q has been deleted\n", ref.Name)
	return nil
}
