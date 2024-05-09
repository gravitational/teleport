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
	"errors"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
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

	return nil
}

// EC2SSMConfigureClient describes the required methods to create the IAM Policies and SSM Document required for installing Teleport in EC2 instances.
type EC2SSMConfigureClient interface {
	// PutRolePolicy creates or replaces a Policy by its name in a IAM Role.
	PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error)

	// CreateDocument creates a Amazon Web Services Systems Manager (SSM document).
	CreateDocument(ctx context.Context, params *ssm.CreateDocumentInput, optFns ...func(*ssm.Options)) (*ssm.CreateDocumentOutput, error)
}

type defaultEC2SSMConfigureClient struct {
	*iam.Client
	ssmClient *ssm.Client
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
		Client:    iam.NewFromConfig(cfg),
		ssmClient: ssm.NewFromConfig(cfg),
	}, nil
}

// ConfigureEC2SSM creates the required resources in AWS to enable EC2 Auto Discover using script mode..
// It creates an embedded policy with the following permissions:
//
// Action: List EC2 instances where teleport is going to be installed.
//   - ec2:DescribeInstances
//
// Action: Run a command and get its output.
//   - ssm:SendCommand
//   - ssm:GetCommandInvocation
//
// Besides setting up the required IAM policies, this method also adds the SSM Document.
// This SSM Document downloads and runs the Teleport Installer Script, which installs teleport in the target EC2 instance.
func ConfigureEC2SSM(ctx context.Context, clt EC2SSMConfigureClient, req EC2SSMIAMConfigureRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	ec2ICEPolicyDocument, err := awslib.NewPolicyDocument(
		awslib.StatementForEC2SSMAutoDiscover(),
	).Marshal()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = clt.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		PolicyName:     &req.IntegrationRoleEC2SSMPolicy,
		RoleName:       &req.IntegrationRole,
		PolicyDocument: &ec2ICEPolicyDocument,
	})
	if err != nil {
		if trace.IsNotFound(awslib.ConvertIAMv2Error(err)) {
			return trace.NotFound("role %q not found", req.IntegrationRole)
		}
		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "IntegrationRole: IAM Policy added to Role", "policy", req.IntegrationRoleEC2SSMPolicy, "role", req.IntegrationRole)

	_, err = clt.CreateDocument(ctx, &ssm.CreateDocumentInput{
		Name:           aws.String(req.SSMDocumentName),
		DocumentType:   ssmtypes.DocumentTypeCommand,
		DocumentFormat: ssmtypes.DocumentFormatYaml,
		Content:        aws.String(awslib.EC2DiscoverySSMDocument(req.ProxyPublicURL)),
	})
	if err != nil {
		var docAlreadyExistsError *ssmtypes.DocumentAlreadyExists
		if errors.As(err, &docAlreadyExistsError) {
			slog.InfoContext(ctx, "SSM Document already exists", "name", req.SSMDocumentName)
			return nil
		}

		return trace.Wrap(err)
	}

	slog.InfoContext(ctx, "SSM Document created", "name", req.SSMDocumentName)

	return nil
}
