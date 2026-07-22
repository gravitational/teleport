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
	"cmp"
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	ratypes "github.com/aws/aws-sdk-go-v2/service/rolesanywhere/types"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// RolesAnywhereTrustAnchorLister is an interface that defines methods for listing IAM Roles Anywhere Trust Anchors.
type RolesAnywhereTrustAnchorLister interface {
	// ListTrustAnchors lists IAM Roles Anywhere Trust Anchors.
	ListTrustAnchors(ctx context.Context, params *rolesanywhere.ListTrustAnchorsInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTrustAnchorsOutput, error)
}

// RolesAnywhereResourceTagsGetter is an interface that defines methods for getting IAM Roles Anywhere resource tags.
type RolesAnywhereResourceTagsGetter interface {
	// ListTagsForResource lists IAM Roles Anywhere resource tags.
	ListTagsForResource(ctx context.Context, params *rolesanywhere.ListTagsForResourceInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.ListTagsForResourceOutput, error)
}

// RolesAnywhereTrustAnchorCreator is an interface that defines methods for creating IAM Roles Anywhere Trust Anchors.
type RolesAnywhereTrustAnchorCreator interface {
	// CreateTrustAnchor creates an IAM Roles Anywhere Trust Anchor in AWS IAM.
	CreateTrustAnchor(ctx context.Context, params *rolesanywhere.CreateTrustAnchorInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.CreateTrustAnchorOutput, error)
}

// RolesAnywhereTrustAnchorUpdater is an interface that defines methods for updating IAM Roles Anywhere Trust Anchors.
type RolesAnywhereTrustAnchorUpdater interface {
	// UpdateTrustAnchor updates an IAM Roles Anywhere Trust Anchor in AWS IAM.
	UpdateTrustAnchor(ctx context.Context, params *rolesanywhere.UpdateTrustAnchorInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.UpdateTrustAnchorOutput, error)
	// EnableTrustAnchor enables temporary credential requests for a trust anchor.
	EnableTrustAnchor(ctx context.Context, params *rolesanywhere.EnableTrustAnchorInput, optFns ...func(*rolesanywhere.Options)) (*rolesanywhere.EnableTrustAnchorOutput, error)
}

// CreateRolesAnywhereTrustAnchorProvider wraps a [RolesAnywhereTrustAnchorCreator] in a
// [provisioning.Action] that creates an IAM Roles Anywhere Trust Anchor in AWS IAM when invoked.
func CreateRolesAnywhereTrustAnchorProvider(
	clt interface {
		RolesAnywhereTrustAnchorLister
		RolesAnywhereTrustAnchorCreator
		RolesAnywhereTrustAnchorUpdater
		RolesAnywhereResourceTagsGetter
	},
	name string,
	trustAnchorCertificate string,
	tags tags.AWSTags,
) (*provisioning.Action, error) {
	input := &rolesanywhere.CreateTrustAnchorInput{
		Name: aws.String(name),
		Source: &ratypes.Source{
			SourceType: ratypes.TrustAnchorTypeCertificateBundle,
			SourceData: &ratypes.SourceDataMemberX509CertificateData{
				Value: trustAnchorCertificate,
			},
		},
		Enabled: aws.Bool(true),
		Tags:    tags.ToRolesAnywhereTags(),
	}
	details, err := formatDetails(input)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "CreateRolesAnywhereTrustAnchorProvider",
		Summary: "Create a Roles Anywhere Trust Anchor in AWS IAM for your Teleport cluster",
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			trustAnchorDetails, err := trustAnchorDetails(ctx, name, clt)
			switch {
			case trace.IsNotFound(err):
				slog.InfoContext(ctx, "Creating a new Roles Anywhere Trust Anchor")
				_, err = clt.CreateTrustAnchor(ctx, input)
				return trace.Wrap(err)

			case err != nil:
				return trace.Wrap(err)
			}

			resourceTags, err := clt.ListTagsForResource(ctx, &rolesanywhere.ListTagsForResourceInput{
				ResourceArn: trustAnchorDetails.TrustAnchorArn,
			})
			if err != nil {
				return trace.Wrap(err)
			}

			if !tags.MatchesRolesAnywhereTags(resourceTags.Tags) {
				return trace.AccessDenied("Roles Anywhere Trust Anchor %q is not owned by this integration", name)
			}

			requiresUpdate := false

			trustAnchorSource := cmp.Or(trustAnchorDetails.Source, &ratypes.Source{})
			if trustAnchorSource.SourceType != input.Source.SourceType {
				trustAnchorDetails.Source.SourceType = input.Source.SourceType
				requiresUpdate = true
			}

			if trustAnchorSource.SourceData != input.Source.SourceData {
				trustAnchorDetails.Source.SourceData = input.Source.SourceData
				requiresUpdate = true
			}

			if requiresUpdate {
				slog.InfoContext(ctx, "Updating the existing Roles Anywhere Profile")
				_, err = clt.UpdateTrustAnchor(ctx, &rolesanywhere.UpdateTrustAnchorInput{
					Name:          trustAnchorDetails.Name,
					TrustAnchorId: trustAnchorDetails.TrustAnchorId,
					Source:        input.Source,
				})
				if err != nil {
					return trace.Wrap(err)
				}
			}

			if !aws.ToBool(trustAnchorDetails.Enabled) {
				_, err := clt.EnableTrustAnchor(ctx, &rolesanywhere.EnableTrustAnchorInput{
					TrustAnchorId: trustAnchorDetails.TrustAnchorId,
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

func trustAnchorDetails(ctx context.Context, trustAnchorName string, clt RolesAnywhereTrustAnchorLister) (*ratypes.TrustAnchorDetail, error) {
	var nextToken *string
	for {
		trustAnchorResp, err := clt.ListTrustAnchors(ctx, &rolesanywhere.ListTrustAnchorsInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, trustAnchor := range trustAnchorResp.TrustAnchors {
			if aws.ToString(trustAnchor.Name) == trustAnchorName {
				return &trustAnchor, nil
			}
		}

		if aws.ToString(trustAnchorResp.NextToken) == "" {
			return nil, trace.NotFound("Roles Anywhere Trust Anchor %q not found", trustAnchorName)
		}

		nextToken = trustAnchorResp.NextToken
	}
}
