/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/cloud/provisioning/awsactions"
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	// defaultPolicyNameForBedrockSessionSummaries is the default name for the Inline Policy added to the IntegrationRole.
	defaultPolicyNameForBedrockSessionSummaries = "TeleportBedrockSessionSummariesAccess"
)

// BedrockSessionSummariesIAMConfigureRequest is a request to configure the required Policies to use Bedrock for Session Summaries.
type BedrockSessionSummariesIAMConfigureRequest struct {
	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleBedrockPolicy is the Policy Name that is created to allow access to call Bedrock APIs.
	// Defaults to "BedrockSessionSummariesAccess"
	IntegrationRoleBedrockPolicy string

	// Resource is the AWS Bedrock resource to grant access to.
	// Can be a full ARN or a model ID (e.g., 'anthropic.claude-v2' or '*' for all models).
	Resource string

	// AccountID is the AWS account ID.
	AccountID string

	// AutoConfirm skips user confirmation of the operation plan if true.
	AutoConfirm bool

	// stdout is used to override stdout output in tests.
	stdout io.Writer
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *BedrockSessionSummariesIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.Resource == "" {
		return trace.BadParameter("resource is required")
	}

	if r.AccountID == "" {
		return trace.BadParameter("account ID is required")
	}

	if r.IntegrationRoleBedrockPolicy == "" {
		r.IntegrationRoleBedrockPolicy = defaultPolicyNameForBedrockSessionSummaries
	}

	return nil
}

// BedrockSessionSummariesIAMConfigureClient describes the required methods to create the IAM Policies
// required for enabling Bedrock Session Summaries in Teleport.
type BedrockSessionSummariesIAMConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)
	// GetCallerIdentity retrieves details about the IAM identity used to call the API.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

type defaultBedrockSessionSummariesIAMConfigureClient struct {
	*iam.Client
	stsClient *sts.Client
}

func (c *defaultBedrockSessionSummariesIAMConfigureClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return c.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

// NewBedrockSessionSummariesIAMConfigureClient creates a new BedrockSessionSummariesIAMConfigureClient.
func NewBedrockSessionSummariesIAMConfigureClient(ctx context.Context) (BedrockSessionSummariesIAMConfigureClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("" /* region */))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultBedrockSessionSummariesIAMConfigureClient{
		Client:    iamutils.NewFromConfig(cfg),
		stsClient: stsutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureBedrockSessionSummariesIAM sets up the IAM policy required for Teleport to invoke Bedrock models
// for the Session Summaries feature.
// The following actions must be allowed by the IAM Role assigned in the Client:
//   - iam:PutRolePolicy
func ConfigureBedrockSessionSummariesIAM(ctx context.Context, clt BedrockSessionSummariesIAMConfigureClient, req BedrockSessionSummariesIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	accountID := req.AccountID

	statement := awslib.StatementForBedrockSessionSummaries(accountID, req.Resource)
	policy := awslib.NewPolicyDocument(statement)

	putRolePolicy, err := awsactions.PutRolePolicy(clt, req.IntegrationRoleBedrockPolicy, req.IntegrationRole, policy)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name: "bedrock-session-summaries",
		Actions: []provisioning.Action{
			*putRolePolicy,
		},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}))
}
