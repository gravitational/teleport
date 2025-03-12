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
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	descriptionOIDCIdPRole = "Used by Teleport to provide access to AWS resources."
)

// IdPIAMConfigureRequest represents a request to configure AWS OIDC integration.
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

	// IntegrationRole is the name of the AWS IAM role that will be created by the
	// AWS OIDC integration.
	IntegrationRole string

	// IntegrationPolicyPreset is the name of a policy preset to be applied to the IntegrationRole.
	// Optional. If empty, no policy is assigned to the newly created IAM Role.
	IntegrationPolicyPreset PolicyPreset

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

	ownershipTags tags.AWSTags

	// stdout is used to override stdout output in tests.
	stdout io.Writer
	// fakeThumbprint is used to override thumbprint in output tests, to produce
	// consistent output.
	fakeThumbprint string
}

// PolicyPreset defines a preset policy type for the AWS IAM role
// created by the Teleport AWS OIDC integration.
type PolicyPreset string

const (
	// PolicyPresetUnspecified specifies no preset policy to apply.
	PolicyPresetUnspecified PolicyPreset = ""
	// PolicyPresetAWSIdentityCenter specifies poicy required for the AWS identity center integration.
	PolicyPresetAWSIdentityCenter PolicyPreset = "aws-identity-center"
)

// ErrAWSOIDCInvalidPolicyPreset is issued if provided policy preset
// value is not supported.
var ErrAWSOIDCInvalidPolicyPreset = &trace.BadParameterError{
	Message: "--preset-policy defines an unknown preset value",
}

// ValidatePolicyPreset validates if a given policy preset is supported or not.
func ValidatePolicyPreset(input PolicyPreset) error {
	switch input {
	case PolicyPresetUnspecified, PolicyPresetAWSIdentityCenter:
		return nil
	default:
		return ErrAWSOIDCInvalidPolicyPreset
	}
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

	switch r.IntegrationPolicyPreset {
	case PolicyPresetUnspecified, PolicyPresetAWSIdentityCenter:
	default:
		return ErrAWSOIDCInvalidPolicyPreset
	}

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
	awsactions.PolicyAssigner
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
		Client:               iamutils.NewFromConfig(cfg),
		CallerIdentityGetter: stsutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureIdPIAM creates a new AWS IAM OIDC IdP, IAM role and optionally updates
// the role with the given policy preset.
//
// The provider URL is Teleport's public address.
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

	// --policy-preset is an optional flag so the assignIdPIAMPolicyAction
	// should only be appended if assignIdPIAMPolicyAction returns a non-nil value.
	actions := []provisioning.Action{
		*createOIDCIdP,
		*createIdPIAMRole,
	}
	assignIdPIAMPolicy, err := assignIdPIAMPolicyAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}
	if assignIdPIAMPolicy != nil {
		actions = append(actions, *assignIdPIAMPolicy)
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name:        "awsoidc-idp",
		Actions:     actions,
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

func assignIdPIAMPolicyAction(clt IdPIAMConfigureClient, req IdPIAMConfigureRequest) (*provisioning.Action, error) {
	var policyName string
	var policyStatement *awslib.Statement

	switch req.IntegrationPolicyPreset {
	case PolicyPresetAWSIdentityCenter:
		policyName = "TeleportAWSIdentityCenterIntegration"
		policyStatement = awslib.StatementForAWSIdentityCenterAccess()
	default:
		return nil, nil
	}

	return awsactions.AssignRolePolicy(
		clt,
		awsactions.RolePolicy{
			RoleName:        req.IntegrationRole,
			PolicyName:      policyName,
			PolicyStatement: policyStatement,
		})
}
