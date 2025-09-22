/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// RoleCreator can create an IAM role.
type RoleCreator interface {
	// CreateRole creates a new IAM Role.
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
}

// RoleGetter can get an IAM role.
type RoleGetter interface {
	// GetRole retrieves information about the specified role, including the
	// role's path, GUID, ARN, and the role's trust policy that grants
	// permission to assume the role.
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
}

// AssumeRolePolicyUpdater can update an IAM role's trust policy.
type AssumeRolePolicyUpdater interface {
	// UpdateAssumeRolePolicy updates the policy that grants an IAM entity
	// permission to assume a role.
	// This is typically referred to as the "role trust policy".
	UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error)
}

// RoleTagger can tag an AWS IAM role.
type RoleTagger interface {
	// TagRole adds one or more tags to an IAM role. The role can be a regular
	// role or a service-linked role. If a tag with the same key name already
	// exists, then that tag is overwritten with the new value.
	TagRole(ctx context.Context, params *iam.TagRoleInput, optFns ...func(*iam.Options)) (*iam.TagRoleOutput, error)
}

// CreateRole returns a [provisioning.Action] that creates or updates an IAM
// role when invoked.
func CreateRole(
	clt interface {
		AssumeRolePolicyUpdater
		RoleCreator
		RoleGetter
		RoleTagger
	},
	roleName string,
	description string,
	trustPolicy *awslib.PolicyDocument,
	tags tags.AWSTags,
) (*provisioning.Action, error) {
	trustPolicyJSON, err := trustPolicy.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := &iam.CreateRoleInput{
		RoleName:                 &roleName,
		Description:              &description,
		AssumeRolePolicyDocument: &trustPolicyJSON,
		Tags:                     tags.ToIAMTags(),
	}
	type createRoleInput struct {
		// AssumeRolePolicyDocument shadows the input's field of the same name
		// to marshal the trust policy doc as unescaped JSON.
		AssumeRolePolicyDocument *awslib.PolicyDocument
		*iam.CreateRoleInput
	}
	details, err := formatDetails(createRoleInput{
		AssumeRolePolicyDocument: trustPolicy,
		CreateRoleInput:          input,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "CreateRole",
		Summary: fmt.Sprintf("Create IAM role %q with a custom trust policy", roleName),
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			slog.InfoContext(ctx, "Checking for existing IAM role",
				"role", roleName,
			)
			getRoleOut, err := clt.GetRole(ctx, &iam.GetRoleInput{
				RoleName: &roleName,
			})
			if err != nil {
				convertedErr := awslib.ConvertIAMError(err)
				if !trace.IsNotFound(convertedErr) {
					return trace.Wrap(convertedErr)
				}
				slog.InfoContext(ctx, "Creating IAM role", "role", roleName)
				_, err = clt.CreateRole(ctx, input)
				if err != nil {
					return trace.Wrap(awslib.ConvertIAMError(err))
				}
				return nil
			}

			slog.InfoContext(ctx, "IAM role already exists",
				"role", roleName,
			)
			existingTrustPolicy, err := awslib.ParsePolicyDocument(aws.ToString(getRoleOut.Role.AssumeRolePolicyDocument))
			if err != nil {
				return trace.Wrap(err)
			}
			err = ensureTrustPolicy(ctx, clt, roleName, trustPolicy, existingTrustPolicy)
			if err != nil {
				return trace.Wrap(err)
			}

			err = ensureTags(ctx, clt, roleName, tags, getRoleOut.Role.Tags)
			if err != nil {
				// Tagging an existing role after we update it is a
				// nice-to-have, but not a need-to-have.
				slog.WarnContext(ctx, "Failed to update IAM role tags",
					"role", roleName,
					"error", err,
					"tags", tags.ToMap(),
				)
			}
			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}

func ensureTrustPolicy(
	ctx context.Context,
	clt AssumeRolePolicyUpdater,
	roleName string,
	trustPolicy *awslib.PolicyDocument,
	existingTrustPolicy *awslib.PolicyDocument,
) error {
	slog.InfoContext(ctx, "Checking IAM role trust policy",
		"role", roleName,
	)

	if !existingTrustPolicy.EnsureStatements(trustPolicy.Statements...) {
		slog.InfoContext(ctx, "IAM role trust policy does not require update",
			"role", roleName,
		)
		return nil
	}

	slog.InfoContext(ctx, "Updating IAM role trust policy",
		"role", roleName,
	)
	trustPolicyJSON, err := existingTrustPolicy.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName:       &roleName,
		PolicyDocument: &trustPolicyJSON,
	})
	return trace.Wrap(err)
}

func ensureTags(
	ctx context.Context,
	clt RoleTagger,
	roleName string,
	tags tags.AWSTags,
	existingTags []iamtypes.Tag,
) error {
	slog.InfoContext(ctx, "Checking for tags on IAM role",
		"role", roleName,
	)
	if tags.MatchesIAMTags(existingTags) {
		slog.InfoContext(ctx, "IAM role is already tagged",
			"role", roleName,
		)
		return nil
	}

	slog.InfoContext(ctx, "Updating IAM role tags",
		"role", roleName,
	)
	_, err := clt.TagRole(ctx, &iam.TagRoleInput{
		RoleName: &roleName,
		Tags:     tags.ToIAMTags(),
	})
	return trace.Wrap(err)
}
