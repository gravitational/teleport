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
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
)

const (
	// defaultPolicyNameForTAGSync is the default name for the Inline TAG Policy added to the IntegrationRole.
	defaultPolicyNameForTAGSync = "AccessGraphSyncAccess"
)

// AccessGraphAWSIAMConfigureRequest is a request to configure the required Policies to use the TAG AWS Sync.
type AccessGraphAWSIAMConfigureRequest struct {
	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleTAGPolicy is the Policy Name that is created to allow access to call AWS APIs.
	// Defaults to "AccessGraphSyncAccess"
	IntegrationRoleTAGPolicy string
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *AccessGraphAWSIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.IntegrationRoleTAGPolicy == "" {
		r.IntegrationRoleTAGPolicy = defaultPolicyNameForTAGSync
	}

	return nil
}

// AccessGraphIAMConfigureClient describes the required methods to create the IAM Policies
// required for enrolling Access Graph AWS Sync into Teleport.
type AccessGraphIAMConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

type defaultTAGIAMConfigureClient struct {
	*iam.Client
}

// NewAccessGraphIAMConfigureClient creates a new TAGIAMConfigureClient.
func NewAccessGraphIAMConfigureClient(ctx context.Context) (AccessGraphIAMConfigureClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("" /* region */))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultTAGIAMConfigureClient{
		Client: iamutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureAccessGraphSyncIAM sets up the roles required for Teleport to be able to pool
// AWS resources into Teleport.
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:PutRolePolicy
func ConfigureAccessGraphSyncIAM(ctx context.Context, clt AccessGraphIAMConfigureClient, req AccessGraphAWSIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	policyDocument, err := awslib.NewPolicyDocument(
		awslib.StatementAccessGraphAWSSync(),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     &req.IntegrationRoleTAGPolicy,
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &policyDocument,
	})
	if err != nil {
		if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
			return trace.NotFound("role %q not found.", req.IntegrationRole)
		}
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "IntegrationRole: IAM Policy added to IAM Role",
		"policy", req.IntegrationRoleTAGPolicy,
		"role", req.IntegrationRole,
	)
	return nil
}
