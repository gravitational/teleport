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

package awsoidc

import (
	"context"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/modules"
)

const (
	// defaultPolicyNameForAWSAppAccess is the default name for the Inline Policy added to the IntegrationRole.
	defaultPolicyNameForAWSAppAccess = "AWSAppAccess"
)

// AWSAppAccessConfigureRequest is a request to configure the required Policies to use AWS App Access.
// Only IAM Roles with `teleport.dev/integration: Allowed` Tag can be used.
type AWSAppAccessConfigureRequest struct {
	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleAWSAppAccessPolicy is the Policy Name that is created to allow access to call AWS APIs.
	// Defaults to AWSAppAccess
	IntegrationRoleAWSAppAccessPolicy string
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *AWSAppAccessConfigureRequest) CheckAndSetDefaults() error {
	if r == nil {
		return trace.BadParameter("request is nil")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.IntegrationRoleAWSAppAccessPolicy == "" {
		r.IntegrationRoleAWSAppAccessPolicy = defaultPolicyNameForAWSAppAccess
	}

	return nil
}

// AWSAppAccessConfigureClient describes the required methods to create the IAM Policies required for AWS App Access.
type AWSAppAccessConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

// NewAWSAppAccessConfigureClient creates a new AWSAppAccessConfigureClient.
func NewAWSAppAccessConfigureClient(ctx context.Context) (AWSAppAccessConfigureClient, error) {
	var configOptions []func(*config.LoadOptions) error

	if modules.GetModules().IsBoringBinary() {
		configOptions = append(configOptions, config.WithUseFIPSEndpoint(aws.FIPSEndpointStateEnabled))
	}

	cfg, err := config.LoadDefaultConfig(ctx, configOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Region == "" {
		// IAM Service does not support regions, however a value is required:
		// https://github.com/aws/aws-sdk-go-v2/issues/1778#issuecomment-1210031692
		// Providing an invalid region here, ensures the service uses the default AWS Partition.
		cfg.Region = " "
	}

	return iam.NewFromConfig(cfg), nil
}

// ConfigureAWSAppAccess set ups the roles required for AWS App Access.
// It creates an embedded policy with the following permissions:
// - sts:AssumeRole
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:PutRolePolicy
func ConfigureAWSAppAccess(ctx context.Context, awsClient AWSAppAccessConfigureClient, req AWSAppAccessConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	awsAppAccessPolicyDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForAWSAppAccess(),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = awsClient.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     &req.IntegrationRoleAWSAppAccessPolicy,
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &awsAppAccessPolicyDocument,
	})
	if err != nil {
		if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
			return trace.NotFound("role %q not found.", req.IntegrationRole)
		}
		return trace.Wrap(err)
	}
	slog.InfoContext(ctx, "IAM Inline Policy added to IAM Role",
		"policy", req.IntegrationRoleAWSAppAccessPolicy,
		"role", req.IntegrationRole,
	)

	return nil
}
