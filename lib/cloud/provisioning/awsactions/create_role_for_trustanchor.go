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
	"fmt"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// CreateRoleForTrustAnchorRequest are the request parameters for creating a Role which can be used by the Trust Anchor.
type CreateRoleForTrustAnchorRequest struct {
	// AccountID is the AWS account ID
	AccountID string
	// RoleName is the name of the IAM role to create.
	RoleName string
	// RoleDescription is the description of the IAM role to create.
	RoleDescription string
	// TrustAnchorName is the name of the Trust Anchor to associate with the role.
	TrustAnchorName string
	// Tags are the tags to apply to the IAM role.
	Tags tags.AWSTags
}

type createRoleInput struct {
	// AssumeRolePolicyDocument shadows the input's field of the same name
	// to marshal the trust policy doc as unescaped JSON.
	AssumeRolePolicyDocument *awslib.PolicyDocument
	*iam.CreateRoleInput
}

func createRoleInputForTrustAnchorWithPlaceholders(req CreateRoleForTrustAnchorRequest) (*iam.CreateRoleInput, *awslib.PolicyDocument, error) {
	return createRoleInputForTrustAnchor(req, "_Region_", "_TrustAnchorID_")
}

func createRoleInputForTrustAnchor(req CreateRoleForTrustAnchorRequest, region, trustAnhcorID string) (*iam.CreateRoleInput, *awslib.PolicyDocument, error) {
	trustPolicy := awslib.NewPolicyDocument(
		awslib.StatementForAWSRolesAnywhereSyncRoleTrustRelationship(region, req.AccountID, trustAnhcorID),
	)
	trustPolicyJSON, err := trustPolicy.Marshal()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	input := &iam.CreateRoleInput{
		RoleName:                 &req.RoleName,
		Description:              &req.RoleDescription,
		AssumeRolePolicyDocument: &trustPolicyJSON,
		Tags:                     req.Tags.ToIAMTags(),
	}

	return input, trustPolicy, nil
}

// CreateRoleForTrustAnchor returns a [provisioning.Action] that creates or updates an IAM
// role when invoked.
func CreateRoleForTrustAnchor(
	clt interface {
		AssumeRolePolicyUpdater
		RoleCreator
		RoleGetter
		RoleTagger
		RolesAnywhereTrustAnchorLister
	},
	req CreateRoleForTrustAnchorRequest,
) (*provisioning.Action, error) {

	// At this point, we don't know the TrustAnchorID or region.
	// We will use placeholders in the action description to indicate that they will be replaced when the action runs.
	createRoleInputWithPlaceholders, trustPolicyWithPlaceholders, err := createRoleInputForTrustAnchorWithPlaceholders(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	details, err := formatDetails(createRoleInput{
		AssumeRolePolicyDocument: trustPolicyWithPlaceholders,
		CreateRoleInput:          createRoleInputWithPlaceholders,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "CreateRole",
		Summary: fmt.Sprintf("Create IAM role %q which can be used by IAM Roles Anywhere", req.RoleName),
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			slog.InfoContext(ctx, "Getting the Trust Anchor ID",
				"trust_anchor", req.TrustAnchorName,
			)

			trustAnchorDetails, err := trustAnchorDetails(ctx, req.TrustAnchorName, clt)
			if err != nil {
				return trace.Wrap(err)
			}
			trustAnchorID := aws.ToString(trustAnchorDetails.TrustAnchorId)

			trustAnchorParsedARN, err := arn.Parse(aws.ToString(trustAnchorDetails.TrustAnchorArn))
			if err != nil {
				return trace.Wrap(err)
			}
			region := trustAnchorParsedARN.Region

			createRoleInput, trustPolicy, err := createRoleInputForTrustAnchor(req, region, trustAnchorID)
			if err != nil {
				return trace.Wrap(err)
			}

			getRoleOut, err := clt.GetRole(ctx, &iam.GetRoleInput{
				RoleName: aws.String(req.RoleName),
			})
			if err != nil {
				convertedErr := awslib.ConvertIAMError(err)
				if !trace.IsNotFound(convertedErr) {
					return trace.Wrap(convertedErr)
				}
				slog.InfoContext(ctx, "Creating IAM role", "role", req.RoleName)
				_, err = clt.CreateRole(ctx, createRoleInput)
				if err != nil {
					return trace.Wrap(awslib.ConvertIAMError(err))
				}
				return nil
			}

			slog.InfoContext(ctx, "IAM role already exists",
				"role", req.RoleName,
			)
			existingTrustPolicy, err := awslib.ParsePolicyDocument(aws.ToString(getRoleOut.Role.AssumeRolePolicyDocument))
			if err != nil {
				return trace.Wrap(err)
			}
			err = ensureTrustPolicy(ctx, clt, req.RoleName, trustPolicy, existingTrustPolicy)
			if err != nil {
				return trace.Wrap(err)
			}

			err = ensureTags(ctx, clt, req.RoleName, req.Tags, getRoleOut.Role.Tags)
			if err != nil {
				// Tagging an existing role after we update it is a
				// nice-to-have, but not a need-to-have.
				slog.WarnContext(ctx, "Failed to update IAM role tags",
					"role", req.RoleName,
					"error", err,
					"tags", req.Tags.ToMap(),
				)
			}
			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}
