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
// along with this program.  If not, see <http://www.gnu.org/licenses/>

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

type workloadIdentityX509IssuerOverrideCollection struct {
	items []*workloadidentityv1pb.X509IssuerOverride
}

func (c *workloadIdentityX509IssuerOverrideCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *workloadIdentityX509IssuerOverrideCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, override := range c.items {
		t.AddRow([]string{override.Metadata.Name})
	}
	return trace.Wrap(t.WriteTo(w))
}

func workloadIdentityX509IssuerOverrideHandler() Handler {
	return Handler{
		getHandler:    getWorkloadIdentityX509IssuerOverride,
		createHandler: createWorkloadIdentityX509IssuerOverride,
		updateHandler: updateWorkloadIdentityX509IssuerOverride,
		deleteHandler: deleteWorkloadIdentityX509IssuerOverride,
		singleton:     false,
		mfaRequired:   false,
		description:   "Overrides the issuers used for X.509 SVIDs",
	}
}

func getWorkloadIdentityX509IssuerOverride(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.WorkloadIdentityX509OverridesClient()
	if ref.Name != "" {
		r, err := c.GetX509IssuerOverride(
			ctx,
			&workloadidentityv1pb.GetX509IssuerOverrideRequest{
				Name: ref.Name,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &workloadIdentityX509IssuerOverrideCollection{
			items: []*workloadidentityv1pb.X509IssuerOverride{
				r,
			},
		}, nil
	}

	resources, err := stream.Collect(
		clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*workloadidentityv1pb.X509IssuerOverride, string, error) {
			resp, err := c.ListX509IssuerOverrides(
				ctx,
				&workloadidentityv1pb.ListX509IssuerOverridesRequest{
					PageSize:  int32(limit),
					PageToken: pageToken,
				},
			)

			return resp.GetX509IssuerOverrides(), resp.GetNextPageToken(), trace.Wrap(err)
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &workloadIdentityX509IssuerOverrideCollection{
		items: resources,
	}, nil
}

func createWorkloadIdentityX509IssuerOverride(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityX509OverridesClient()
	if opts.Force {
		if _, err := c.UpsertX509IssuerOverride(
			ctx,
			&workloadidentityv1pb.UpsertX509IssuerOverrideRequest{
				X509IssuerOverride: r,
			},
		); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if _, err := c.CreateX509IssuerOverride(
			ctx,
			&workloadidentityv1pb.CreateX509IssuerOverrideRequest{
				X509IssuerOverride: r,
			},
		); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf(
		types.KindWorkloadIdentityX509IssuerOverride+" %q has been created\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func updateWorkloadIdentityX509IssuerOverride(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityX509OverridesClient()
	if _, err = c.UpdateX509IssuerOverride(
		ctx,
		&workloadidentityv1pb.UpdateX509IssuerOverrideRequest{
			X509IssuerOverride: r,
		},
	); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		types.KindWorkloadIdentityX509IssuerOverride+" %q has been updated\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func deleteWorkloadIdentityX509IssuerOverride(
	ctx context.Context, client *authclient.Client, ref services.Ref,
) error {
	c := client.WorkloadIdentityX509OverridesClient()
	if _, err := c.DeleteX509IssuerOverride(
		ctx,
		&workloadidentityv1pb.DeleteX509IssuerOverrideRequest{
			Name: ref.Name,
		},
	); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		types.KindWorkloadIdentityX509IssuerOverride+" %q has been deleted\n",
		ref.Name,
	)
	return nil
}
