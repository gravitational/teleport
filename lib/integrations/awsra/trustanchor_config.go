/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package awsra

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/rolesanywhere"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"

	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/cloud/aws/tags"
	"github.com/gravitational/teleport/lib/cloud/provisioning"
	"github.com/gravitational/teleport/lib/cloud/provisioning/awsactions"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/aws/iamutils"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

// TrustAnchorConfigureRequest represents a request to configure AWS IAM Roles Anywhere integration.
type TrustAnchorConfigureRequest struct {
	// Cluster is the Teleport Cluster.
	// Used for tagging the created RolesAnywhere Trust Anchor/Profile and IAM Role.
	Cluster string

	// AccountID is the AWS Account ID.
	// Optional. sts.GetCallerIdentity is used if not provided.
	AccountID string

	// IntegrationName is the Integration Name.
	// Used for tagging the created RolesAnywhere Trust Anchor/Profile and IAM Role.
	IntegrationName string

	// TrustAnchorName is the name of the trust anchor.
	TrustAnchorName string

	// TrustAnchorCertBase64 is the base64 encoded PEM certificate of the trust anchor.
	TrustAnchorCertBase64 string

	// SyncProfileName is the name of the AWS IAM Roles Anywhere Profile to create, used to sync profiles.
	SyncProfileName string

	// SyncRoleName is the name of the AWS IAM Role to create, used to sync profiles.
	SyncRoleName string

	// AutoConfirm skips user confirmation of the operation plan if true.
	AutoConfirm bool

	ownershipTags tags.AWSTags

	// stdout is used to override stdout output in tests.
	stdout io.Writer
}

// CheckAndSetDefaults ensures the required fields are present.
func (r *TrustAnchorConfigureRequest) CheckAndSetDefaults() error {
	if r.Cluster == "" {
		return trace.BadParameter("cluster is required")
	}

	if r.IntegrationName == "" {
		return trace.BadParameter("integration name is required")
	}

	if r.TrustAnchorName == "" {
		return trace.BadParameter("trust anchor name is required")
	}

	if r.TrustAnchorCertBase64 == "" {
		return trace.BadParameter("trust anchor's certificate is required")
	}

	if r.SyncProfileName == "" {
		return trace.BadParameter("roles anywhere profile name used for the sync process is required")
	}

	if r.SyncRoleName == "" {
		return trace.BadParameter("role name used for the sync process is required")
	}

	r.ownershipTags = defaultResourceCreationTags(r.Cluster, r.IntegrationName)

	if r.stdout == nil {
		r.stdout = os.Stdout
	}

	return nil
}

// CallerIdentityGetter is an interface that defines the method to get the caller identity from AWS STS.
type CallerIdentityGetter interface {
	// GetCallerIdentity retrieves the AWS caller identity.
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// RolesAnywhereIAMConfigurationClient describes the required methods to create the AWS IAM Roles Anywhere Trust Anchor, Profile and the IAM Role.
// There is no guarantee that the client is thread safe.
type RolesAnywhereIAMConfigurationClient interface {
	CallerIdentityGetter

	awsactions.RolesAnywhereTrustAnchorLister
	awsactions.RolesAnywhereTrustAnchorCreator
	awsactions.RolesAnywhereTrustAnchorUpdater

	awsactions.RolesAnywhereProfileLister
	awsactions.RolesAnywhereProfileCreator
	awsactions.RolesAnywhereProfileUpdater

	awsactions.RolesAnywhereResourceTagsGetter

	awsactions.AssumeRolePolicyUpdater
	awsactions.RoleCreator
	awsactions.RoleGetter
	awsactions.RoleTagger
	awsactions.PolicyAssigner
}

type defaultRolesAnywhereIAMConfigurationClient struct {
	CallerIdentityGetter
	*rolesanywhere.Client
	iamClient *iam.Client

	httpClient *http.Client
}

func (c *defaultRolesAnywhereIAMConfigurationClient) CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error) {
	return c.iamClient.CreateRole(ctx, params, optFns...)
}
func (c *defaultRolesAnywhereIAMConfigurationClient) GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error) {
	return c.iamClient.GetRole(ctx, params, optFns...)
}
func (c *defaultRolesAnywhereIAMConfigurationClient) PutRolePolicy(ctx context.Context, params *iam.PutRolePolicyInput, optFns ...func(*iam.Options)) (*iam.PutRolePolicyOutput, error) {
	return c.iamClient.PutRolePolicy(ctx, params, optFns...)
}
func (c *defaultRolesAnywhereIAMConfigurationClient) TagRole(ctx context.Context, params *iam.TagRoleInput, optFns ...func(*iam.Options)) (*iam.TagRoleOutput, error) {
	return c.iamClient.TagRole(ctx, params, optFns...)
}
func (c *defaultRolesAnywhereIAMConfigurationClient) UpdateAssumeRolePolicy(ctx context.Context, params *iam.UpdateAssumeRolePolicyInput, optFns ...func(*iam.Options)) (*iam.UpdateAssumeRolePolicyOutput, error) {
	return c.iamClient.UpdateAssumeRolePolicy(ctx, params, optFns...)
}

// NewRolesAnywhereIAMConfigurationClient creates a new RolesAnywhereIAMConfigurationClient.
// The client is not thread safe.
func NewRolesAnywhereIAMConfigurationClient(ctx context.Context) (RolesAnywhereIAMConfigurationClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClient, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &defaultRolesAnywhereIAMConfigurationClient{
		httpClient:           httpClient,
		Client:               rolesanywhere.NewFromConfig(cfg),
		iamClient:            iamutils.NewFromConfig(cfg),
		CallerIdentityGetter: stsutils.NewFromConfig(cfg),
	}, nil
}

// ConfigureRolesAnywhereIAM creates a new AWS IAM Roles Anywhere Trust Anchor.
// It also creates a new AWS IAM Roles Anywhere Profile and a new AWS IAM Role which is used to sync Profiles into Teleport resources (AWS Apps).
//
// The following actions must be allowed by the IAM Role assigned in the Client.
//   - rolesanywhere:CreateTrustAnchor
//   - rolesanywhere:CreateProfile
//   - iam:CreateRole
//   - iam:GetRole
//   - iam:UpdateAssumeRolePolicy
func ConfigureRolesAnywhereIAM(ctx context.Context, clt RolesAnywhereIAMConfigurationClient, req TrustAnchorConfigureRequest) error {
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

	createRolesAnywhereTrustAnchorAction, err := createIAMRolesAnywhereTrustAnchor(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	createIAMSyncRoleAction, err := createIAMSyncRoleAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	assignListProfilesPolicyToSyncRoleAction, err := assignProfileSyncPolicyToRoleAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	createIAMSyncProfileAction, err := createIAMSyncProfileAction(clt, req)
	if err != nil {
		return trace.Wrap(err)
	}

	trustAnchorSetUpARNsAction, err := awsactions.TrustAnchorSetUpARNs(clt, req.TrustAnchorName, req.SyncProfileName, req.SyncRoleName, req.stdout)
	if err != nil {
		return trace.Wrap(err)
	}

	actions := []provisioning.Action{
		*createRolesAnywhereTrustAnchorAction,
		*createIAMSyncRoleAction,
		*assignListProfilesPolicyToSyncRoleAction,
		*createIAMSyncProfileAction,
		*trustAnchorSetUpARNsAction,
	}

	return trace.Wrap(provisioning.Run(ctx, provisioning.OperationConfig{
		Name:        "awsra-trust-anchor",
		Actions:     actions,
		AutoConfirm: req.AutoConfirm,
		Output:      req.stdout,
	}))
}

func createIAMRolesAnywhereTrustAnchor(clt RolesAnywhereIAMConfigurationClient, req TrustAnchorConfigureRequest) (*provisioning.Action, error) {
	return awsactions.CreateRolesAnywhereTrustAnchorProvider(clt, req.TrustAnchorName, req.TrustAnchorCertBase64, req.ownershipTags)
}

func createIAMSyncRoleAction(clt RolesAnywhereIAMConfigurationClient, req TrustAnchorConfigureRequest) (*provisioning.Action, error) {
	return awsactions.CreateRoleForTrustAnchor(clt, awsactions.CreateRoleForTrustAnchorRequest{
		AccountID:       req.AccountID,
		RoleName:        req.SyncRoleName,
		RoleDescription: "Used by Teleport to provide access to AWS resources.",
		TrustAnchorName: req.TrustAnchorName,
		Tags:            req.ownershipTags,
	})
}

func createIAMSyncProfileAction(clt RolesAnywhereIAMConfigurationClient, req TrustAnchorConfigureRequest) (*provisioning.Action, error) {
	syncRoleARN := fmt.Sprintf("arn:aws:iam::%s:role/%s", req.AccountID, req.SyncRoleName)

	return awsactions.CreateRolesAnywhereProfileProvider(clt,
		req.SyncProfileName,
		syncRoleARN,
		req.ownershipTags,
	)
}

func assignProfileSyncPolicyToRoleAction(clt RolesAnywhereIAMConfigurationClient, req TrustAnchorConfigureRequest) (*provisioning.Action, error) {
	policyStatement := awslib.StatementForAWSRolesAnywhereSyncRolePolicy()

	return awsactions.AssignRolePolicy(
		clt,
		awsactions.RolePolicy{
			RoleName:        req.SyncRoleName,
			PolicyName:      "TeleportRolesAnywhereProfileSync",
			PolicyStatement: policyStatement,
		})
}
