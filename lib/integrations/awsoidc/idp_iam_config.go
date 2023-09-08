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

	// Region is the AWS Region.
	// Used to set up the AWS SDK Client.
	Region string

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

	if r.Region == "" {
		return trace.BadParameter("region is required")
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
func NewIdPIAMConfigureClient(ctx context.Context, region string) (IdPIAMConfigureClient, error) {
	if region == "" {
		return nil, trace.BadParameter("region is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, trace.Wrap(err)
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

	if err := createIdPIAMRole(ctx, clt, req); err != nil {
		return trace.Wrap(err)
	}
	log.Printf("IAM Role %q created.", req.IntegrationRole)

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
		Tags:                     defaultResourceCreationTags(req.Cluster, req.IntegrationName).ToIAMTags(),
	})
	if err != nil {
		convertedErr := awslib.ConvertIAMv2Error(err)
		if trace.IsAlreadyExists(convertedErr) {
			return trace.AlreadyExists("Role %q already exists, please remove it and try again.", req.IntegrationRole)
		}
		return trace.Wrap(convertedErr)
	}

	return nil
}
