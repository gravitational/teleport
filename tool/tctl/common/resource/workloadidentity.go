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

func (rc *ResourceCommand) createSPIFFEFederation(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

func (rc *ResourceCommand) getSPIFFEFederation(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		resource, err := client.SPIFFEFederationServiceClient().GetSPIFFEFederation(ctx, &machineidv1pb.GetSPIFFEFederationRequest{
			Name: rc.ref.Name,
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

func (rc *ResourceCommand) createWorkloadIdentity(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	in, err := services.UnmarshalWorkloadIdentity(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityResourceServiceClient()
	if rc.force {
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

func (rc *ResourceCommand) getWorkloadIdentity(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		resource, err := client.WorkloadIdentityResourceServiceClient().GetWorkloadIdentity(ctx, &workloadidentityv1pb.GetWorkloadIdentityRequest{
			Name: rc.ref.Name,
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

func (rc *ResourceCommand) getWorkloadIdentityX509Revocation(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	if rc.ref.Name != "" {
		resource, err := client.
			WorkloadIdentityRevocationServiceClient().
			GetWorkloadIdentityX509Revocation(ctx, &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
				Name: rc.ref.Name,
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

func (rc *ResourceCommand) createWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.X509IssuerOverride](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.WorkloadIdentityX509OverridesClient()
	if rc.IsForced() {
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

	fmt.Fprintf(
		rc.stdout,
		types.KindWorkloadIdentityX509IssuerOverride+" %q has been created\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func (rc *ResourceCommand) getWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	c := client.WorkloadIdentityX509OverridesClient()
	if rc.ref.Name != "" {
		r, err := c.GetX509IssuerOverride(
			ctx,
			&workloadidentityv1pb.GetX509IssuerOverrideRequest{
				Name: rc.ref.Name,
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

func (rc *ResourceCommand) updateWorkloadIdentityX509IssuerOverride(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

	fmt.Fprintf(
		rc.stdout,
		types.KindWorkloadIdentityX509IssuerOverride+" %q has been updated\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func (rc *ResourceCommand) createSigstorePolicy(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
	r, err := services.UnmarshalProtoResource[*workloadidentityv1pb.SigstorePolicy](raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	c := client.SigstorePolicyResourceServiceClient()
	if rc.IsForced() {
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

	fmt.Fprintf(
		rc.stdout,
		types.KindSigstorePolicy+" %q has been created\n",
		r.GetMetadata().GetName(),
	)
	return nil
}

func (rc *ResourceCommand) getSigstorePolicy(ctx context.Context, client *authclient.Client) (collections.ResourceCollection, error) {
	c := client.SigstorePolicyResourceServiceClient()
	if rc.ref.Name != "" {
		r, err := c.GetSigstorePolicy(
			ctx,
			&workloadidentityv1pb.GetSigstorePolicyRequest{
				Name: rc.ref.Name,
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

func (rc *ResourceCommand) updateSigstorePolicy(ctx context.Context, client *authclient.Client, raw services.UnknownResource) error {
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

	fmt.Fprintf(
		rc.stdout,
		types.KindSigstorePolicy+" %q has been updated\n",
		r.GetMetadata().GetName(),
	)
	return nil
}
