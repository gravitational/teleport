/*
Copyright 2023 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awsoidc

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
)

const (
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

	// ListDatabasesPolicy is the Policy Name that is created to allow access to call AWS APIs.
	// Defaults to ListDatabases
	ListDatabasesPolicy string
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *ConfigureIAMListDatabasesRequest) CheckAndSetDefaults() error {
	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.ListDatabasesPolicy == "" {
		r.ListDatabasesPolicy = defaultPolicyNameForListDatabases
	}

	return nil
}

// ListDatabasesIAMConfigureClient describes the required methods to create the IAM Policies required for Listing Databases.
type ListDatabasesIAMConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
}

// NewListDatabasesIAMConfigureClient creates a new ListDatabasesIAMConfigureClient.
func NewListDatabasesIAMConfigureClient(ctx context.Context, region string) (ListDatabasesIAMConfigureClient, error) {
	if region == "" {
		return nil, trace.BadParameter("region is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return iam.NewFromConfig(cfg), nil
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
		PolicyName:     &req.ListDatabasesPolicy,
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &listDatabasesPolicyDocument,
	})
	if err != nil {
		if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
			return trace.NotFound("role %q not found.", req.IntegrationRole)
		}
		return trace.Wrap(err)
	}

	log.Printf("IntegrationRole: IAM Policy %q added to Role %q\n", req.ListDatabasesPolicy, req.IntegrationRole)
	return nil
}
