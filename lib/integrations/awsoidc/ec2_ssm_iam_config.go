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
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/cloud/provisioning/awsactions"
	"github.com/gravitational/teleport/lib/integrations/awsoidc/tags"
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

const (
	// defaultPolicyNameForEC2SSM is the default name for the Inline Policy added to the IntegrationRole.
	defaultPolicyNameForEC2SSM = "EC2DiscoverWithSSM"
)

// EC2SSMIAMConfigureRequest is a request to configure the required Policies to use the EC2 Auto Discover with SSM.
type EC2SSMIAMConfigureRequest struct {
	// Region is the AWS Region.
	// Used to set up the AWS SDK Client.
	Region string

	// IntegrationRole is the Integration's AWS Role used to set up Teleport as an OIDC IdP.
	IntegrationRole string

	// IntegrationRoleEC2SSMPolicy is the Policy Name that is created to allow access to call AWS APIs.
	// Defaults to EC2DiscoverWithSSM.
	IntegrationRoleEC2SSMPolicy string

	// SSMDocumentName is the SSM Document to be created.
	// This document calls the installer scripts in the target host.
	SSMDocumentName string

	// ProxyPublicURL is Proxy's Public URL.
	// This is used fetch the installer script.
	// No trailing / is expected.
	// Eg https://tenant.teleport.sh
	ProxyPublicURL string

	// ClusterName is the Teleport cluster name.
	// Used for resource tagging.
	ClusterName string
	// IntegrationName is the Teleport AWS OIDC Integration name.
	// Used for resource tagging.
	IntegrationName string
	// AccountID is the AWS Account ID.
	AccountID string
	// AutoConfirm skips user confirmation of the operation plan if true.
	AutoConfirm bool
	// stdout is used to override stdout output in tests.
	stdout io.Writer
	// insecureSkipInstallPathRandomization is set to true under output test to
	// produce consistent output.
	insecureSkipInstallPathRandomization bool
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *EC2SSMIAMConfigureRequest) CheckAndSetDefaults() error {
	if r.Region == "" {
		return trace.BadParameter("region is required")
	}

	if r.IntegrationRole == "" {
		return trace.BadParameter("integration role is required")
	}

	if r.IntegrationRoleEC2SSMPolicy == "" {
		r.IntegrationRoleEC2SSMPolicy = defaultPolicyNameForEC2SSM
	}

	if r.SSMDocumentName == "" {
		return trace.BadParameter("ssm document name is required")
	}

	if r.ProxyPublicURL == "" {
		return trace.BadParameter("proxy public url is required")
	}

	if r.ClusterName == "" {
		return trace.BadParameter("cluster name is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	return nil
}

// EC2SSMConfigureClient describes the required methods to create the IAM Policies and SSM Document required for installing Teleport in EC2 instances.
type EC2SSMConfigureClient interface {
	CallerIdentityGetter
	awsactions.RolePolicyPutter
	awsactions.DocumentCreator
}

type defaultEC2SSMConfigureClient struct {
	*iam.Client
	ssmClient *ssm.Client
	CallerIdentityGetter
}

// CreateDocument creates a Amazon Web Services Systems Manager (SSM document).
func (d *defaultEC2SSMConfigureClient) CreateDocument(ctx context.Context, params *ssm.CreateDocumentInput, optFns ...func(*ssm.Options)) (*ssm.CreateDocumentOutput, error) {
	return d.ssmClient.CreateDocument(ctx, params, optFns...)
}

// NewEC2SSMConfigureClient creates a new EC2SSMConfigureClient.
func NewEC2SSMConfigureClient(ctx context.Context, region string) (EC2SSMConfigureClient, error) {
	if region == "" {
		return nil, trace.BadParameter("region is required")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultEC2SSMConfigureClient{
		Client:               iamutils.NewFromConfig(cfg),
		ssmClient:            ssm.NewFromConfig(cfg),
		CallerIdentityGetter: stsutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureEC2SSM creates the required resources in AWS to enable EC2 Auto Discover using script mode..
// It creates an inline policy with the following permissions:
//
// Action: List EC2 instances where teleport is going to be installed.
//   - ec2:DescribeInstances
//
// Action: Get SSM Agent Status
//   - ssm:DescribeInstanceInformation
//
// Action: Run a command and get its output.
//   - ssm:SendCommand
//   - ssm:GetCommandInvocation
//   - ssm:ListCommandInvocations
//
// Besides setting up the required IAM policies, this method also adds the SSM Document.
// This SSM Document downloads and runs the Teleport Installer Script, which installs teleport in the target EC2 instance.
func ConfigureEC2SSM(ctx context.Context, clt EC2SSMConfigureClient, req EC2SSMIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := CheckAccountID(ctx, clt, req.AccountID); err != nil {
		return trace.Wrap(err)
	}

	policy := awslib.NewPolicyDocument(
		awslib.StatementForEC2SSMAutoDiscover(),
	)
	putRolePolicy, err := awsactions.PutRolePolicy(clt, req.IntegrationRoleEC2SSMPolicy, req.IntegrationRole, policy)
	if err != nil {
		return trace.Wrap(err)
	}

	content := awslib.EC2DiscoverySSMDocument(req.ProxyPublicURL,
		awslib.WithInsecureSkipInstallPathRandomization(req.insecureSkipInstallPathRandomization),
	)
	tags := tags.DefaultResourceCreationTags(req.ClusterName, req.IntegrationName)
	createDoc, err := awsactions.CreateDocument(clt, req.SSMDocumentName, content, ssmtypes.DocumentTypeCommand, ssmtypes.DocumentFormatYaml, tags)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name: "ec2-ssm-iam",
		Actions: []provisioning.Action{
			*putRolePolicy,
			*createDoc,
		},
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}))
}
