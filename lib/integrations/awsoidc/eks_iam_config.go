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

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

const (
	// defaultPolicyNameForEKS is the default name for the Inline EKS Policy added to the IntegrationRole.
	defaultPolicyNameForEKS = "EKSAccess"
)

// EKSIAMConfigureRequest is a request to configure the required Policies to use the EKS.
type EKSIAMConfigureRequest struct {
	// Region is the AWS Region.
	// Used to set up the AWS SDK Client.
	Region string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleEKSPolicy is the Policy Name that is created to allow access to call AWS APIs.
	// Defaults to "EKSAccess"
	IntegrationRoleEKSPolicy string
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *EKSIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.IntegrationRoleEKSPolicy == "" {
		r.IntegrationRoleEKSPolicy = defaultPolicyNameForEKS
	}

	return nil
}

// EKSIAMConfigureClient describes the required methods to create the IAM Policies required for enrolling EKS clusters into Teleport.
type EKSIAMConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

type defaultEKSEIAMConfigureClient struct {
	*iam.Client
}

// NewEKSIAMConfigureClient creates a new EKSIAMConfigureClient.
func NewEKSIAMConfigureClient(ctx context.Context, region string) (EKSIAMConfigureClient, error) {
	if region == "" {
		return nil, trace.BadParameter("region is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultEKSEIAMConfigureClient{
		Client: iam.NewFromConfig(cfg),
	}, nil
}

// ConfigureEKSIAM sets up the roles required for enrolling EKS clusters into Teleport.
// It creates an embedded policy with the following permissions:
// - eks:ListClusters
// - eks:DescribeCluster
// - eks:ListAccessEntries
// - eks:CreateAccessEntry
// - eks:DeleteAccessEntry
// - eks:AssociateAccessPolicy
//
// For more info about EKS access entries see:
// https://aws.amazon.com/blogs/containers/a-deep-dive-into-simplified-amazon-eks-access-management-controls/
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:PutRolePolicy
func ConfigureEKSIAM(ctx context.Context, clt EKSIAMConfigureClient, req EKSIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	eksPolicyDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForEKSAccess(),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     &req.IntegrationRoleEKSPolicy,
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &eksPolicyDocument,
	})
	if err != nil {
		if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
			return trace.NotFound("role %q not found.", req.IntegrationRole)
		}
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "IntegrationRole: IAM Policy added to role",
		"policy", req.IntegrationRoleEKSPolicy,
		"role", req.IntegrationRole,
	)
	return nil
}
