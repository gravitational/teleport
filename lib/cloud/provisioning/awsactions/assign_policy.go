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

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// PolicyCreator can assign a policy to an existing IAM role.
type PolicyCreator interface {
	// PutRolePolicy updates AWS IAM role with the given policy document.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

// RolePolicy defines policy
type RolePolicy struct {
	RoleName        string
	PolicyName      string
	PolicyStatement *awslib.Statement
}

// AssignRolePolicy assigns AWS OIDC integration role with a preset policy.
func AssignRolePolicy(
	clt interface {
		PolicyCreator
		RoleGetter
	},
	req RolePolicy,
) (*provisioning.Action, error) {
	policyDocument, err := awslib.NewPolicyDocument(
		req.PolicyStatement,
	).Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	details, err := formatDetails(iam.PutRolePolicyInput{
		PolicyName:     &req.PolicyName,
		RoleName:       &req.RoleName,
		PolicyDocument: &policyDocument,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name:    "AssignPolicy",
		Summary: fmt.Sprintf("Assign IAM role %q with an inline policy %q", req.RoleName, req.PolicyName),
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			_, err = clt.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
				PolicyName:     &req.PolicyName,
				RoleName:       &req.RoleName,
				PolicyDocument: &policyDocument,
			})
			if err != nil {
				if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
					return trace.NotFound("role %q not found.", req.RoleName)
				}
				return trace.Wrap(err)
			}
			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}
