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

package awsactions

import (
	"context"
	"log/slog"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// RolesAnywhereProfileLister can list IAM Roles Anywhere Profiles.
type RolesAnywhereProfileLister interface {
	// ListProfiles lists IAM Roles Anywhere Profiles.
	ListProfiles(ctx context.Context, params *rolesanywhere.ListProfilesInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListProfilesOutput, error)
}

// RolesAnywhereProfileCreator can create an IAM Roles Anywhere Profiles.
type RolesAnywhereProfileCreator interface {
	// CreateProfile creates an IAM Roles Anywhere Profile in AWS IAM.
	CreateProfile(ctx context.Context, params *rolesanywhere.CreateProfileInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.CreateProfileOutput, error)
}

// RolesAnywhereProfileUpdater can update an IAM Roles Anywhere Profiles.
type RolesAnywhereProfileUpdater interface {
	// UpdateProfile updates an IAM Roles Anywhere Profile in AWS IAM.
	UpdateProfile(ctx context.Context, params *rolesanywhere.UpdateProfileInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.UpdateProfileOutput, error)
	// Enables temporary credential requests for a profile.
	EnableProfile(ctx context.Context, params *rolesanywhere.EnableProfileInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.EnableProfileOutput, error)
}

// CreateRolesAnywhereProfileProvider wraps a [RolesAnywhereProfileCreator] in a
// [provisioning.Action] that creates an IAM Roles Anywhere Profile in AWS IAM when invoked.
func CreateRolesAnywhereProfileProvider(
	clt interface {
		RolesAnywhereProfileLister
		RolesAnywhereProfileCreator
		RolesAnywhereProfileUpdater
		RolesAnywhereResourceTagsGetter
	},
	name string,
	roleARN string,
	tags tags.AWSTags,
) (*provisioning.Action, error) {
	input := &rolesanywhere.CreateProfileInput{
		Name:                  aws.String(name),
		Enabled:               aws.Bool(true),
		RoleArns:              []string{roleARN},
		AcceptRoleSessionName: aws.Bool(true),
		Tags:                  tags.ToRolesAnywhereTags(),
	}
	details, err := formatDetails(input)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "CreateRolesAnywhereProfileProvider",
		Summary: "Create a Roles Anywhere Profile in AWS IAM for your Teleport cluster",
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			profileDetails, err := profileDetails(ctx, name, clt)
			switch {
			case trace.IsNotFound(err):
				slog.InfoContext(ctx, "Creating a new Roles Anywhere Profile")
				_, err = clt.CreateProfile(ctx, input)
				return trace.Wrap(err)

			case err != nil:
				return trace.Wrap(err)

			}

			resourceTags, err := clt.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
				ResourceArn: profileDetails.ProfileArn,
			})
			if err != nil {
				return trace.Wrap(err)
			}

			if !tags.MatchesRolesAnywhereTags(resourceTags.Tags) {
				return trace.AccessDenied("Roles Anywhere Profile %q is not owned by this integration", name)
			}

			requiresUpdate := false
			if !aws.ToBool(profileDetails.AcceptRoleSessionName) {
				profileDetails.AcceptRoleSessionName = aws.Bool(true)
				requiresUpdate = true
			}

			if !slices.Contains(profileDetails.RoleArns, roleARN) {
				requiresUpdate = true
				profileDetails.RoleArns = append(profileDetails.RoleArns, roleARN)
			}

			if requiresUpdate {
				slog.InfoContext(ctx, "Updating the existing Roles Anywhere Profile")
				_, err = clt.UpdateProfile(ctx, &rolesanywhere.UpdateProfileInput{
					Name:                  profileDetails.Name,
					ProfileId:             profileDetails.ProfileId,
					AcceptRoleSessionName: profileDetails.AcceptRoleSessionName,
					RoleArns:              profileDetails.RoleArns,
				})
				if err != nil {
					return trace.Wrap(err)
				}
			}

			if !aws.ToBool(profileDetails.Enabled) {
				_, err := clt.EnableProfile(ctx, &rolesanywhere.EnableProfileInput{
					ProfileId: profileDetails.ProfileId,
				})
				if err != nil {
					return trace.Wrap(err)
				}
			}

			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}

func profileDetails(ctx context.Context, profileName string, clt RolesAnywhereProfileLister) (*ratypes.ProfileDetail, error) {
	var nextToken *string
	for {
		profilesResp, err := clt.ListProfiles(ctx, &rolesanywhere.ListProfilesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, profile := range profilesResp.Profiles {
			if aws.ToString(profile.Name) == profileName {
				return &profile, nil
			}
		}

		if aws.ToString(profilesResp.NextToken) == "" {
			return nil, trace.NotFound("Roles Anywhere Profile %q not found", profileName)
		}

		nextToken = profilesResp.NextToken
	}
}
