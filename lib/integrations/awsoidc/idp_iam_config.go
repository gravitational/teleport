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
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	descriptionOIDCIdPRole = "Used by Teleport to provide access to AWS resources."
)

// IdPIAMConfigureRequest is a request to configure the required Policies to use the EC2 Instance Connect Endpoint feature.
type IdPIAMConfigureRequest struct {
	// Cluster is the Teleport Cluster.
	// Used for tagging the created Roles/IdP.
	Cluster string

	// AccountID is the AWS Account ID.
	// Optional. sts.GetCallerIdentity is used if not provided.
	AccountID string

	// IntegrationName is the Integration Name.
	// Used for tagging the created Roles/IdP.
	IntegrationName string

	// ProxyPublicAddress is the URL to use as provider URL.
	// This must be a valid URL (ie, url.Parse'able)
	// Eg, https://<tenant>.teleport.sh, https://proxy.example.org:443, https://teleport.ec2.aws:3080
	ProxyPublicAddress string

	// issuer is the above value but only contains the host.
	// Eg, <tenant>.teleport.sh, proxy.example.org
	issuer string
	// issuerURL is the full url for the issuer
	// Eg, https://<tenant>.teleport.sh, https://proxy.example.org
	issuerURL string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	ownershipTags tags.AWSTags
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *IdPIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.Cluster == "" {
		return trace.BadParameter("cluster is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.ProxyPublicAddress == "" {
		return trace.BadParameter("argument --proxy-public-url is required")
	}

	issuerURL, err := url.Parse(r.ProxyPublicAddress)
	if err != nil {
		return trace.BadParameter("--proxy-public-url is not a valid url: %v", err)
	}
	r.issuer = issuerURL.Host
	if issuerURL.Port() == "443" {
		r.issuer = issuerURL.Hostname()
	}
	r.issuerURL = issuerURL.String()

	r.ownershipTags = tags.DefaultResourceCreationTags(r.Cluster, r.IntegrationName)

	return nil
}

// IdPIAMConfigureClient describes the required methods to create the AWS OIDC IdP and a Role that trusts that identity provider.
// There is no guarantee that the client is thread safe.
type IdPIAMConfigureClient interface {
	// GetCallerIdentity returns information about the caller identity.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)

	// CreateOpenIDConnectProvider creates an IAM OIDC IdP.
	CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput, optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error)

	// CreateRole creates a new IAM Role.
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)

	// GetRole retrieves information about the specified role, including the role's path,
	// GUID, ARN, and the role's trust policy that grants permission to assume the
	// role.
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)

	// UpdateAssumeRolePolicy updates the policy that grants an IAM entity permission to assume a role.
	// This is typically referred to as the "role trust policy".
	UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error)
}

type defaultIdPIAMConfigureClient struct {
	httpClient *http.Client

	*iam.Client
	awsConfig aws.Config
	stsClient *sts.Client
}

// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
func (d *defaultIdPIAMConfigureClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return d.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

// NewIdPIAMConfigureClient creates a new IdPIAMConfigureClient.
// The client is not thread safe.
func NewIdPIAMConfigureClient(ctx context.Context) (IdPIAMConfigureClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Region == "" {
		return nil, trace.BadParameter("failed to resolve local AWS region from environment, please set the AWS_REGION environment variable")
	}

	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultIdPIAMConfigureClient{
		httpClient: httpClient,
		awsConfig:  cfg,
		Client:     iam.NewFromConfig(cfg),
		stsClient:  stsutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureIdPIAM creates a new IAM OIDC IdP in AWS.
//
// The provider URL is Teleport's public address.
// It also creates a new Role configured to trust the recently created IdP.
// If the role already exists, it will create another trust relationship for the IdP (if it doesn't exist).
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:CreateOpenIDConnectProvider
//   - iam:CreateRole
//   - iam:GetRole
//   - iam:UpdateAssumeRolePolicy
func ConfigureIdPIAM(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if req.AccountID == "" {
		callerIdentity, err := clt.GetCallerIdentity(ctx, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		req.AccountID = aws.ToString(callerIdentity.Account)
	}

	slog.InfoContext(ctx, "Creating IAM OpenID Connect Provider", "url", req.issuerURL)
	if err := ensureOIDCIdPIAM(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "Creating IAM Role", "role", req.IntegrationRole)
	if err := upsertIdPIAMRole(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func ensureOIDCIdPIAM(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	thumbprint, err := ThumbprintIdP(ctx, req.ProxyPublicAddress)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.CreateOpenIDConnectProvider(ctx, &iam.CreateOpenIDConnectProviderInput{
		ThumbprintList: []string{thumbprint},
		Url:            &req.issuerURL,
		ClientIDList:   []string{types.IntegrationAWSOIDCAudience},
		Tags:           req.ownershipTags.ToIAMTags(),
	})
	if err != nil {
		awsErr := awslib.ConvertIAMv2Error(err)
		if trace.IsAlreadyExists(awsErr) {
			return nil
		}

		return trace.Wrap(err)
	}

	return nil
}

func createIdPIAMRole(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	integrationRoleAssumeRoleDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForAWSOIDCRoleTrustRelationship(req.AccountID, req.issuer, []string{types.IntegrationAWSOIDCAudience}),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 &req.IntegrationRole,
		Description:              aws.String(descriptionOIDCIdPRole),
		AssumeRolePolicyDocument: &integrationRoleAssumeRoleDocument,
		Tags:                     req.ownershipTags.ToIAMTags(),
	})
	return trace.Wrap(err)
}

func upsertIdPIAMRole(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) error {
	getRoleOut, err := clt.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &req.IntegrationRole,
	})
	if err != nil {
		convertedErr := awslib.ConvertIAMv2Error(err)
		if !trace.IsNotFound(convertedErr) {
			return trace.Wrap(convertedErr)
		}

		return trace.Wrap(createIdPIAMRole(ctx, clt, req))
	}

	if !req.ownershipTags.MatchesIAMTags(getRoleOut.Role.Tags) {
		return trace.BadParameter("IAM Role %q already exists but is not managed by Teleport. "+
			"Add the following tags to allow Teleport to manage this Role: %s", req.IntegrationRole, req.ownershipTags)
	}

	trustRelationshipDoc, err := awslib.ParsePolicyDocument(aws.ToString(getRoleOut.Role.AssumeRolePolicyDocument))
	if err != nil {
		return trace.Wrap(err)
	}

	trustRelationshipForIdP := awslib.StatementForAWSOIDCRoleTrustRelationship(req.AccountID, req.issuer, []string{types.IntegrationAWSOIDCAudience})
	for _, existingStatement := range trustRelationshipDoc.Statements {
		if existingStatement.EqualStatement(trustRelationshipForIdP) {
			return nil
		}
	}

	trustRelationshipDoc.Statements = append(trustRelationshipDoc.Statements, trustRelationshipForIdP)
	trustRelationshipDocString, err := trustRelationshipDoc.Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &trustRelationshipDocString,
	})
	return trace.Wrap(err)
}
