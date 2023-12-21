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
	"log"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
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
	// Eg, <tenant>.teleport.sh, proxy.example.org, teleport.ec2.aws:3080
	issuer string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string
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
		return trace.BadParameter("proxy public address is required")
	}

	issuerURL, err := url.Parse(r.ProxyPublicAddress)
	if err != nil {
		return trace.BadParameter("proxy public address is not a valid url: %v", err)
	}
	r.issuer = issuerURL.Host
	if issuerURL.Port() == "443" {
		r.issuer = issuerURL.Hostname()
	}

	return nil
}

// IdPIAMConfigureClient describes the required methods to create the AWS OIDC IdP and a Role that trusts that identity provider.
type IdPIAMConfigureClient interface {
	// GetCallerIdentity returns information about the caller identity.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)

	// CreateOpenIDConnectProvider creates an IAM OIDC IdP.
	CreateOpenIDConnectProvider(ctx context.Context, params *iam.CreateOpenIDConnectProviderInput, optFns ...func(*iam.Options)) (*iam.CreateOpenIDConnectProviderOutput, error)

	// CreateRole creates a new IAM Role.
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
}

type defaultIdPIAMConfigureClient struct {
	*iam.Client
	stsClient *sts.Client
}

// GetCallerIdentity returns details about the IAM user or role whose credentials are used to call the operation.
func (d defaultIdPIAMConfigureClient) GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error) {
	return d.stsClient.GetCallerIdentity(ctx, params, optFns...)
}

// NewIdPIAMConfigureClient creates a new IdPIAMConfigureClient.
func NewIdPIAMConfigureClient(ctx context.Context) (IdPIAMConfigureClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Region == "" {
		return nil, trace.BadParameter("failed to resolve local AWS region from environment, please set the AWS_REGION environment variable")
	}

	return &defaultIdPIAMConfigureClient{
		Client:    iam.NewFromConfig(cfg),
		stsClient: sts.NewFromConfig(cfg),
	}, nil
}

// ConfigureIdPIAM creates a new IAM OIDC IdP in AWS.
//
// The Provider URL is Teleport's Public Address.
// It also creates a new Role configured to trust the recently created IdP.
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - iam:CreateOpenIDConnectProvider
//   - iam:CreateRole
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

	thumbprint, err := ThumbprintIdP(ctx, req.ProxyPublicAddress)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Printf("Using the following thumbprint: %s", thumbprint)

	createOIDCResp, err := clt.CreateOpenIDConnectProvider(ctx, &iam.CreateOpenIDConnectProviderInput{
		ThumbprintList: []string{thumbprint},
		Url:            &req.ProxyPublicAddress,
		ClientIDList:   []string{types.IntegrationAWSOIDCAudience},
		Tags:           defaultResourceCreationTags(req.Cluster, req.IntegrationName).ToIAMTags(),
	})
	if err != nil {
		if trace.IsAlreadyExists(awslib.ConvertIAMv2Error(err)) {
			return trace.AlreadyExists("identity provider for the same URL (%s) already exists, please remove it and try again", req.ProxyPublicAddress)
		}
		return trace.Wrap(err)
	}
	log.Printf("IAM OpenID Connect Provider created: url=%q arn=%q.", req.ProxyPublicAddress, aws.ToString(createOIDCResp.OpenIDConnectProviderArn))

	createdIdpIAMRoleArn, err := createIdPIAMRole(ctx, clt, req)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Printf("IAM Role created: name=%q arn=%q", req.IntegrationRole, aws.ToString(createdIdpIAMRoleArn))

	return nil
}

func createIdPIAMRole(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) (*string, error) {
	integrationRoleAssumeRoleDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForAWSOIDCRoleTrustRelationship(req.AccountID, req.issuer, []string{types.IntegrationAWSOIDCAudience}),
	).Marshal()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	createRoleOutput, err := clt.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 &req.IntegrationRole,
		Description:              aws.String(descriptionOIDCIdPRole),
		AssumeRolePolicyDocument: &integrationRoleAssumeRoleDocument,
		Tags:                     defaultResourceCreationTags(req.Cluster, req.IntegrationName).ToIAMTags(),
	})
	if err != nil {
		convertedErr := awslib.ConvertIAMv2Error(err)
		if trace.IsAlreadyExists(convertedErr) {
			return nil, trace.AlreadyExists("Role %q already exists, please remove it and try again.", req.IntegrationRole)
		}
		return nil, trace.Wrap(convertedErr)
	}

	return createRoleOutput.Role.Arn, nil
}
