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

type sigstorePolicyCollection struct {
	items []*workloadidentityv1pb.SigstorePolicy
}

func (c *sigstorePolicyCollection) Resources() []types.Resource {
	r := make([]types.Resource, 0, len(c.items))
	for _, resource := range c.items {
		r = append(r, types.ProtoResource153ToLegacy(resource))
	}
	return r
}

func (c *sigstorePolicyCollection) WriteText(w io.Writer, verbose bool) error {
	t := asciitable.MakeTable([]string{"Name"})
	for _, policy := range c.items {
		t.AddRow([]string{policy.GetMetadata().GetName()})
	}
	return trace.Wrap(t.WriteTo(w))
}

func sigstorePolicyHandler() Handler {
	return Handler{
		getHandler:    getSigstorePolicy,
		createHandler: createSigstorePolicy,
		updateHandler: updateSigstorePolicy,
		deleteHandler: deleteSigstorePolicy,
		mfaRequired:   false,
		singleton:     false,
		description:   "Configures Sigstore attestation with SPIFFE",
	}
}

func getSigstorePolicy(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
	opts GetOpts,
) (Collection, error) {
	c := client.SigstorePolicyResourceServiceClient()
	if ref.Name != "" {
		r, err := c.GetSigstorePolicy(
			ctx,
			workloadidentityv1pb.GetSigstorePolicyRequest_builder{
				Name: ref.Name,
			}.Build(),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &sigstorePolicyCollection{
			items: []*workloadidentityv1pb.SigstorePolicy{r},
		}, nil
	}

	resources, err := stream.Collect(
		clientutils.Resources(ctx, func(ctx context.Context, limit int, pageToken string) ([]*workloadidentityv1pb.SigstorePolicy, string, error) {
			resp, err := c.ListSigstorePolicies(
				ctx,
				workloadidentityv1pb.ListSigstorePoliciesRequest_builder{
					PageSize:  int32(limit),
					PageToken: pageToken,
				}.Build(),
			)

			return resp.GetSigstorePolicies(), resp.GetNextPageToken(), trace.Wrap(err)
		}),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &sigstorePolicyCollection{items: resources}, nil
}

func deleteSigstorePolicy(
	ctx context.Context,
	client *authclient.Client,
	ref services.Ref,
) error {
	c := client.SigstorePolicyResourceServiceClient()
	if _, err := c.DeleteSigstorePolicy(
		ctx,
		workloadidentityv1pb.DeleteSigstorePolicyRequest_builder{
			Name: ref.Name,
		}.Build(),
	); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		types.KindSigstorePolicy+" %q has been deleted\n",
		ref.Name,
	)
	return nil
}

func createSigstorePolicy(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](
		raw.Raw, services.DisallowUnknown(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SigstorePolicyResourceServiceClient()
	if opts.Force {
		if _, err := c.UpsertSigstorePolicy(
			ctx,
			workloadidentityv1pb.UpsertSigstorePolicyRequest_builder{
				SigstorePolicy: r,
			}.Build(),
		); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if _, err := c.CreateSigstorePolicy(
			ctx,
			workloadidentityv1pb.CreateSigstorePolicyRequest_builder{
				SigstorePolicy: r,
			}.Build(),
		); err != nil {
			return trace.Wrap(err)
		}
	}

	fmt.Printf(
		types.KindSigstorePolicy+" %q has been created\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func updateSigstorePolicy(
	ctx context.Context,
	client *authclient.Client,
	raw services.UnknownResource,
	opts CreateOpts,
) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](
		raw.Raw, services.DisallowUnknown(),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SigstorePolicyResourceServiceClient()
	if _, err = c.UpdateSigstorePolicy(
		ctx,
		workloadidentityv1pb.UpdateSigstorePolicyRequest_builder{
			SigstorePolicy: r,
		}.Build(),
	); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		types.KindSigstorePolicy+" %q has been updated\n",
		r.GetMetadata().GetName(),
	)
	return nil
}
