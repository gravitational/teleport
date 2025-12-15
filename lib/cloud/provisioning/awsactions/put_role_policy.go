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

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
)

// RolePolicyPutter can upsert an IAM inline role policy.
type RolePolicyPutter interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(context.Context, *iam.PutRolePolicyInput, ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

// PutRolePolicy wraps a [RolePolicyPutter] in a [provisioning.Action] that
// upserts an inline IAM policy when invoked.
func PutRolePolicy(
	clt RolePolicyPutter,
	policyName string,
	roleName string,
	policy *awslib.PolicyDocument,
) (*provisioning.Action, error) {
	policyJSON, err := policy.Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	input := &iam.PutRolePolicyInput{
		PolicyName:     &policyName,
		RoleName:       &roleName,
		PolicyDocument: &policyJSON,
	}
	type putRolePolicyInput struct {
		// PolicyDocument shadows the input's field of the same name
		// to marshal the trust policy doc as unescaped JSON.
		PolicyDocument *awslib.PolicyDocument
		*iam.PutRolePolicyInput
	}
	details, err := formatDetails(putRolePolicyInput{
		PolicyDocument:     policy,
		PutRolePolicyInput: input,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := provisioning.ActionConfig{
		Name: "PutRolePolicy",
		Summary: fmt.Sprintf("Attach an inline IAM policy named %q to IAM role %q",
			policyName,
			roleName,
		),
		Details: details,
		RunnerFn: func(ctx context.Context) error {
			_, err = clt.PutRolePolicy(ctx, input)
			if err != nil {
				if trace.IsNotFound(awslib.ConvertIAMError(err)) {
					return trace.NotFound("role %q not found.", roleName)
				}
				return trace.Wrap(err)
			}

			slog.InfoContext(ctx, "Added inline policy to IAM role",
				"policy", policyName,
				"role", roleName,
			)
			return nil
		},
	}
	action, err := provisioning.NewAction(config)
	return action, trace.Wrap(err)
}
