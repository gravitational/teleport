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

package resources

import (
	"context"
	"fmt"
	"io"

	"github.com/gravitational/trace"

	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/itertools/stream"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/slices"
)

func delegationProfileHandler() Handler {
	return Handler{
		getHandler:    getDelegationProfile,
		createHandler: createDelegationProfile,
		updateHandler: updateDelegationProfile,
		deleteHandler: deleteDelegationProfile,
		description:   "A set of pre-selected parameters from which a delegation session can be created.",
	}
}

type delegationProfileCollection struct {
	profiles []*delegationv1.DelegationProfile
}

func (c *delegationProfileCollection) Resources() []types.Resource {
	resources := make([]types.Resource, len(c.profiles))
	for idx, profile := range c.profiles {
		resources[idx] = types.ProtoResource153ToLegacy(profile)
	}
	return resources
}

func (c *delegationProfileCollection) WriteText(w io.Writer, _ bool) error {
	rows := slices.Map(c.profiles, func(profile *delegationv1.DelegationProfile) []string {
		return []string{profile.GetMetadata().GetName()}
	})
	table := asciitable.MakeTable([]string{"Name"}, rows...)
	return trace.Wrap(table.WriteTo(w))
}

func getDelegationProfile(ctx context.Context, client *authclient.Client, ref services.Ref, opts GetOpts) (Collection, error) {
	profileClient := client.DelegationProfileServiceClient()

	if ref.Name == "" {
		profileStream := clientutils.Resources(ctx,
			func(ctx context.Context, pageSize int, nextToken string) ([]*delegationv1.DelegationProfile, string, error) {
				rsp, err := profileClient.ListDelegationProfiles(ctx, &delegationv1.ListDelegationProfilesRequest{
					PageSize:  int32(pageSize),
					PageToken: nextToken,
				})
				if err != nil {
					return nil, "", trace.Wrap(err)
				}
				return rsp.GetDelegationProfiles(), rsp.GetNextPageToken(), nil
			},
		)
		profiles, err := stream.Collect(profileStream)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &delegationProfileCollection{
			profiles: profiles,
		}, nil
	}

	profile, err := profileClient.GetDelegationProfile(ctx, &delegationv1.GetDelegationProfileRequest{
		Name: ref.Name,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &delegationProfileCollection{
		profiles: []*delegationv1.DelegationProfile{profile},
	}, nil
}

func createDelegationProfile(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	profile, err := services.UnmarshalDelegationProfile(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	profileClient := client.DelegationProfileServiceClient()

	if opts.Force {
		_, err = profileClient.UpsertDelegationProfile(ctx, &delegationv1.UpsertDelegationProfileRequest{
			DelegationProfile: profile,
		})
	} else {
		_, err = profileClient.CreateDelegationProfile(ctx, &delegationv1.CreateDelegationProfileRequest{
			DelegationProfile: profile,
		})
	}
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("delegation profile %q has been created\n", profile.GetMetadata().GetName())
	return nil
}

func updateDelegationProfile(ctx context.Context, client *authclient.Client, raw services.UnknownResource, opts CreateOpts) error {
	profile, err := services.UnmarshalDelegationProfile(raw.Raw, services.DisallowUnknown())
	if err != nil {
		return trace.Wrap(err)
	}

	profileClient := client.DelegationProfileServiceClient()
	_, err = profileClient.UpdateDelegationProfile(ctx, &delegationv1.UpdateDelegationProfileRequest{
		DelegationProfile: profile,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("delegation profile %q has been updated\n", profile.GetMetadata().GetName())
	return nil
}

func deleteDelegationProfile(ctx context.Context, client *authclient.Client, ref services.Ref) error {
	profileClient := client.DelegationProfileServiceClient()
	if _, err := profileClient.DeleteDelegationProfile(ctx, &delegationv1.DeleteDelegationProfileRequest{
		Name: ref.Name,
	}); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("delegation profile %q has been deleted\n", ref.Name)
	return nil
}
