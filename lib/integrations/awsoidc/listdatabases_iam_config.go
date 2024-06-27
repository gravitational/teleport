/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

var (
	// defaultPolicyNameForListDatabases is the default name for the Inline Policy added to the IntegrationRole.
	defaultPolicyNameForListDatabases = "ListDatabases"
)

// ConfigureIAMListDatabasesRequest is a request to configure the required Policy to use the List Databases action.
type ConfigureIAMListDatabasesRequest struct {
	// Region is the AWS Region.
	// Used to set up the AWS SDK Client.
	Region string

	// IntegrationRole is the Integration's AWS Role used by the integration.
	IntegrationRole string
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *ConfigureIAMListDatabasesRequest) CheckAndSetDefaults() error {
	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	return nil
}

// ListDatabasesIAMConfigureClient describes the required methods to create the IAM Policies required for Listing Databases.
type ListDatabasesIAMConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

// ConfigureListDatabasesIAM set ups the policy required for accessing an RDS DB Instances and RDS DB Clusters.
// It creates an inline policy with the following permissions:
//   - rds:DescribeDBInstances
//   - rds:DescribeDBClusters
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:PutRolePolicy
func ConfigureListDatabasesIAM(ctx context.Context, clt ListDatabasesIAMConfigureClient, req ConfigureIAMListDatabasesRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	listDatabasesPolicyDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForListRDSDatabases(),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     &defaultPolicyNameForListDatabases,
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &listDatabasesPolicyDocument,
	})
	if err != nil {
		if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
			return trace.NotFound("role %q not found.", req.IntegrationRole)
		}
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Added Inline Policy to IAM Role",
		"policy", defaultPolicyNameForListDatabases,
		"role", req.IntegrationRole,
	)
	return nil
}
