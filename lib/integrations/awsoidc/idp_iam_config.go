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
	"io"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/cloud/provisioning/awsactions"
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

	// AutoConfirm skips user confirmation of the operation plan if true.
	AutoConfirm bool

	// issuer is the above value but only contains the host.
	// Eg, <tenant>.teleport.sh, proxy.example.org
	issuer string
	// issuerURL is the full url for the issuer
	// Eg, https://<tenant>.teleport.sh, https://proxy.example.org
	issuerURL string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	ownershipTags tags.AWSTags

	// stdout is used to override stdout output in tests.
	stdout io.Writer
	// fakeThumbprint is used to override thumbprint in output tests, to produce
	// consistent output.
	fakeThumbprint string
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
	CallerIdentityGetter
	awsactions.AssumeRolePolicyUpdater
	awsactions.OpenIDConnectProviderCreator
	awsactions.RoleCreator
	awsactions.RoleGetter
	awsactions.RoleTagger
}

type defaultIdPIAMConfigureClient struct {
	httpClient *http.Client

	*iam.Client
	awsConfig aws.Config
	CallerIdentityGetter
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
		httpClient:           httpClient,
		awsConfig:            cfg,
		Client:               iam.NewFromConfig(cfg),
		CallerIdentityGetter: stsutils.NewFromConfig(cfg),
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

	createOIDCIdP, err := createOIDCIdPAction(ctx, clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	createIdPIAMRole, err := createIdPIAMRoleAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name: "awsoidc-idp",
		Actions: []provisioning.Action{
			*createOIDCIdP,
			*createIdPIAMRole,
		},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}))
}

func createOIDCIdPAction(ctx context.Context, clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) (*provisioning.Action, error) {
	var thumbprint string
	if req.fakeThumbprint != "" {
		// only happens in tests.
		thumbprint = req.fakeThumbprint
	} else {
		var err error
		thumbprint, err = ThumbprintIdP(ctx, req.ProxyPublicAddress)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	clientIDs := []string{types.IntegrationAWSOIDCAudience}
	thumbprints := []string{thumbprint}
	return awsactions.CreateOIDCProvider(clt, thumbprints, req.issuerURL, clientIDs, req.ownershipTags)
}

func createIdPIAMRoleAction(clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) (*provisioning.Action, error) {
	integrationRoleAssumeRoleDocument := awslib.NewPolicyDocument(
		awslib.StatementForAWSOIDCRoleTrustRelationship(req.AccountID, req.issuer, []string{types.IntegrationAWSOIDCAudience}),
	)
	return awsactions.CreateRole(clt,
		req.IntegrationRole,
		descriptionOIDCIdPRole,
		integrationRoleAssumeRoleDocument,
		req.ownershipTags,
	)
}
