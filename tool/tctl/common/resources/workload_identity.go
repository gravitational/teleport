// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
)

type workloadIdentityCollection struct {
	items []*workloadidentityv1pb.WorkloadIdentity
}

func (c *workloadIdentityCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *workloadIdentityCollection) WriteText(w io.Writer, verbose bool) error {
	headers := []string{"Name", "SPIFFE ID"}

	var rows [][]string
	for _, item := range c.items {
		rows = append(rows, []string{
			item.Metadata.Name,
			item.GetSpec().GetSpiffe().GetId(),
		})
	}

	t := asciitable.MakeTable(headers, rows...)

	// stable sort by name.
	t.SortRowsBy([]int{0}, true)
	_, err := t.AsBuffer().WriteTo(w)
	return trace.Wrap(err)
}

func workloadIdentityHandler() Handler {
	return Handler{
		getHandler:    getWorkloadIdentity,
		deleteHandler: deleteWorkloadIdentity,
		createHandler: createWorkloadIdentity,
		singleton:     false,
		mfaRequired:   false,
		description:   "Configures the issuance of SPIFFE SVIDs to workloads.",
	}
}

func getWorkloadIdentity(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.WorkloadIdentityResourceServiceClient()

	if ref.Name != "" {
		resource, err := c.GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &workloadIdentityCollection{items: []*workloadidentityv1pb.WorkloadIdentity{resource}}, nil
	}

	resources, err := stream.Collect(clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*workloadidentityv1pb.WorkloadIdentity, string, error) {
		resp, err := client.WorkloadIdentityResourceServiceClient().ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{
			PageSize:  int32(limit),
			PageToken: pageToken,
		})

		return resp.GetWorkloadIdentities(), resp.GetNextPageToken(), trace.Wrap(err)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadIdentityCollection{items: resources}, nil
}

func createWorkloadIdentity(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	in, err := services.UnmarshalWorkloadIdentity(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityResourceServiceClient()
	if opts.Force {
		if _, err := c.UpsertWorkloadIdentity(ctx, &workloadidentityv1pb.UpsertWorkloadIdentityRequest{
			WorkloadIdentity: in,
		}); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if _, err := c.CreateWorkloadIdentity(ctx, &workloadidentityv1pb.CreateWorkloadIdentityRequest{
			WorkloadIdentity: in,
		}); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf("Workload Identity %q has been created\n", in.GetMetadata().GetName())

	return nil
}

func deleteWorkloadIdentity(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	c := client.WorkloadIdentityResourceServiceClient()
	_, err := c.DeleteWorkloadIdentity(ctx, &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
		Name: ref.Name,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Workload Identity %q has been deleted\n", ref.Name)
	return nil
}
