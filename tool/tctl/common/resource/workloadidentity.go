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

package resource

import (
	"context"
	"fmt"
	"slices"

	"github.com/gravitational/trace"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/tool/tctl/common/resource/collections"
)

var spiffeFederation = resource{
	getHandler:    getSPIFFEFederation,
	createHandler: createSPIFFEFederation,
	deleteHandler: deleteSPIFFEFederation,
}

func createSPIFFEFederation(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	in, err := services.UnmarshalSPIFFEFederation(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SPIFFEFederationServiceClient()
	if _, err := c.CreateSPIFFEFederation(ctx, &machineidv1pb.CreateSPIFFEFederationRequest{
		SpiffeFederation: in,
	}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SPIFFE Federation %q has been created\n", in.GetMetadata().GetName())

	return nil
}

func getSPIFFEFederation(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		resource, err := client.SPIFFEFederationServiceClient().GetSPIFFEFederation(ctx, &machineidv1pb.GetSPIFFEFederationRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewSpiffeFederationCollection([]*machineidv1pb.SPIFFEFederation{resource}), nil
	}

	var resources []*machineidv1pb.SPIFFEFederation
	pageToken := ""
	for {
		resp, err := client.SPIFFEFederationServiceClient().ListSPIFFEFederations(ctx, &machineidv1pb.ListSPIFFEFederationsRequest{
			PageToken: pageToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resp.SpiffeFederations...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return collections.NewSpiffeFederationCollection(resources), nil
}

func deleteSPIFFEFederation(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.SPIFFEFederationServiceClient().DeleteSPIFFEFederation(
		ctx, &machineidv1pb.DeleteSPIFFEFederationRequest{
			Name: ref.Name,
		},
	); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("SPIFFE federation %q has been deleted\n", ref.Name)
	return nil
}

var workloadIdentity = resource{
	getHandler:    getWorkloadIdentity,
	createHandler: createWorkloadIdentity,
	deleteHandler: deleteWorkloadIdentity,
}

func createWorkloadIdentity(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	in, err := services.UnmarshalWorkloadIdentity(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityResourceServiceClient()
	if opts.force {
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

	fmt.Printf("Workload identity %q has been created\n", in.GetMetadata().GetName())

	return nil
}

func getWorkloadIdentity(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		resource, err := client.WorkloadIdentityResourceServiceClient().GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name: ref.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewWorkloadIdentityCollection([]*workloadidentityv1pb.WorkloadIdentity{resource}), nil
	}

	var resources []*workloadidentityv1pb.WorkloadIdentity
	pageToken := ""
	for {
		resp, err := client.WorkloadIdentityResourceServiceClient().ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{
			PageToken: pageToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resp.WorkloadIdentities...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return collections.NewWorkloadIdentityCollection(resources), nil
}

func deleteWorkloadIdentity(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.WorkloadIdentityResourceServiceClient().DeleteWorkloadIdentity(
		ctx, &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
			Name: ref.Name,
		}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Workload identity %q has been deleted\n", ref.Name)
	return nil
}

var workloadIdentityX509Revocation = resource{
	getHandler:    getWorkloadIdentityX509Revocation,
	deleteHandler: deleteWorkloadIdentityX509Revocation,
}

func getWorkloadIdentityX509Revocation(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	if ref.Name != "" {
		resource, err := client.
			WorkloadIdentityRevocationServiceClient().
			GetWorkloadIdentityX509Revocation(ctx, &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
				Name: ref.Name,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewWorkloadIdentityX509RevocationCollection([]*workloadidentityv1pb.WorkloadIdentityX509Revocation{resource}), nil
	}

	var resources []*workloadidentityv1pb.WorkloadIdentityX509Revocation
	pageToken := ""
	for {
		resp, err := client.
			WorkloadIdentityRevocationServiceClient().
			ListWorkloadIdentityX509Revocations(ctx, &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest{
				PageToken: pageToken,
			})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resources = append(resources, resp.WorkloadIdentityX509Revocations...)

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return collections.NewWorkloadIdentityX509RevocationCollection(resources), nil
}

func deleteWorkloadIdentityX509Revocation(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	if _, err := client.WorkloadIdentityRevocationServiceClient().DeleteWorkloadIdentityX509Revocation(
		ctx, &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
			Name: ref.Name,
		}); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("Workload identity X509 revocation %q has been deleted\n", ref.Name)
	return nil
}

var workloadIdentityX509IssuerOverride = resource{
	getHandler:    getWorkloadIdentityX509IssuerOverride,
	createHandler: createWorkloadIdentityX509IssuerOverride,
	updateHandler: updateWorkloadIdentityX509IssuerOverride,
	deleteHandler: deleteWorkloadIdentityX509IssuerOverride,
}

func createWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityX509OverridesClient()
	if opts.force {
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

func getWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
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
		return collections.NewNamedResourceCollection([]types.Resource{types.ProtoResource153ToLegacy(r)}), nil
	}
	var resources []types.Resource
	var pageToken string
	for {
		resp, err := c.ListX509IssuerOverrides(
			ctx,
			&workloadidentityv1pb.ListX509IssuerOverridesRequest{
				PageToken: pageToken,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = slices.Grow(resources, len(resp.GetX509IssuerOverrides()))
		for _, r := range resp.GetX509IssuerOverrides() {
			resources = append(resources, types.ProtoResource153ToLegacy(r))
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	return collections.NewNamedResourceCollection(resources), nil
}

func updateWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
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

func deleteWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client, ref services.Ref) error {
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

var sigstorePolicy = resource{
	getHandler:    getSigstorePolicy,
	createHandler: createSigstorePolicy,
	updateHandler: updateSigstorePolicy,
	deleteHandler: deleteSigstorePolicy,
}

func createSigstorePolicy(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SigstorePolicyResourceServiceClient()
	if opts.force {
		if _, err := c.UpsertSigstorePolicy(
			ctx,
			&workloadidentityv1pb.UpsertSigstorePolicyRequest{
				SigstorePolicy: r,
			},
		); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if _, err := c.CreateSigstorePolicy(
			ctx,
			&workloadidentityv1pb.CreateSigstorePolicyRequest{
				SigstorePolicy: r,
			},
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

func getSigstorePolicy(ctx context.Context, client *authclient.Client, ref services.Ref, opts getOpts) (collections.ResourceCollection, error) {
	c := client.SigstorePolicyResourceServiceClient()
	if ref.Name != "" {
		r, err := c.GetSigstorePolicy(
			ctx,
			&workloadidentityv1pb.GetSigstorePolicyRequest{
				Name: ref.Name,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return collections.NewNamedResourceCollection([]types.Resource{types.ProtoResource153ToLegacy(r)}), nil
	}
	var resources []types.Resource
	var pageToken string
	for {
		resp, err := c.ListSigstorePolicies(
			ctx,
			&workloadidentityv1pb.ListSigstorePoliciesRequest{
				PageToken: pageToken,
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = slices.Grow(resources, len(resp.GetSigstorePolicies()))
		for _, r := range resp.GetSigstorePolicies() {
			resources = append(resources, types.ProtoResource153ToLegacy(r))
		}
		pageToken = resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}
	return collections.NewNamedResourceCollection(resources), nil
}

func updateSigstorePolicy(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts createOpts) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SigstorePolicyResourceServiceClient()
	if _, err = c.UpdateSigstorePolicy(
		ctx,
		&workloadidentityv1pb.UpdateSigstorePolicyRequest{
			SigstorePolicy: r,
		},
	); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		types.KindSigstorePolicy+" %q has been updated\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func deleteSigstorePolicy(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	c := client.SigstorePolicyResourceServiceClient()
	if _, err := c.DeleteSigstorePolicy(
		ctx,
		&workloadidentityv1pb.DeleteSigstorePolicyRequest{
			Name: ref.Name,
		},
	); err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf(
		types.KindSigstorePolicy+" %q has been deleted\n",
		ref.Name,
	)
	return nil
}
