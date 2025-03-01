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
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	awsutils "github.com/gravitational/teleport/api/utils/aws"
)

const (
	// awsSTSShortName is the service name of the AWS STS
	// Used to build the ARN matcher in the IAM Join Token.
	awsSTSShortName = "sts"
)

// TokenService defines the required methods to upsert the Provision Token used by the Deploy Service.
type TokenService interface {
	// GetToken returns a provision token by name.
	GetToken(ctx context.Context, name string) (types.ProvisionToken, error)

	// UpsertToken creates or updates a provision token.
	UpsertToken(ctx context.Context, token types.ProvisionToken) error
}

// upsertIAMJoinTokenRequest is the request to create or update a Provision Token that has a rule which allows AWS Identities to join the cluster.
// Allowed AWS Identities are the ones described
// It gives access for those identities to join as system roles.
type upsertIAMJoinTokenRequest struct {
	// tokenName is the Token's name to create or update.
	tokenName string

	// accountID is the allowed AWS Account ID.
	accountID string

	// region is the allowed AWS Region.
	region string

	// iamRole is the allowed IAM Role.
	iamRole string

	// deploymentMode is the service configuration that is going to be deployed
	deploymentMode string

	// awsARN is the IAM Role's ARN.
	// This value is calculated using the AWS account, region and IAM role.
	awsARN string
}

// CheckAndSetDefaults verifies the required fields are present.
func (u *upsertIAMJoinTokenRequest) CheckAndSetDefaults() error {
	if u.tokenName == "" {
		return trace.BadParameter("token name is required")
	}

	if u.accountID == "" {
		return trace.BadParameter("account id is required")
	}

	if u.region == "" {
		return trace.BadParameter("region is required")
	}

	if u.iamRole == "" {
		return trace.BadParameter("iam role is required")
	}

	if !slices.Contains(DeploymentModes, u.deploymentMode) {
		return trace.BadParameter("invalid deployment mode, please use one of the following: %v", DeploymentModes)
	}

	partition := awsutils.GetPartitionFromRegion(u.region)

	// Example ARN:
	// arn:aws:sts::278576220453:assumed-role/MarcoTestRoleOIDCProvider/1688143717490080920
	u.awsARN = arn.ARN{
		Partition: partition,
		Service:   awsSTSShortName,
		AccountID: u.accountID,
		Resource:  fmt.Sprintf("assumed-role/%s/*", u.iamRole),
	}.String()

	return nil
}

// upsertIAMJoinToken creates or updates a Provision Token with a rule that allows Teleport Services with given AWS Identities to join the cluster.
// The allowed Identities are the ones provided in the request (Account, Region and AWSRole).
// It gives access for those identities to join as system roles.
func upsertIAMJoinToken(ctx context.Context, req upsertIAMJoinTokenRequest, clt TokenService) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	rule := &types.TokenRule{
		AWSAccount: req.accountID,
		AWSARN:     req.awsARN,
	}

	token, err := clt.GetToken(ctx, req.tokenName)
	switch {
	case err != nil && !trace.IsNotFound(err):
		return trace.Wrap(err)

	case trace.IsNotFound(err):
		token = &types.ProvisionTokenV2{
			Metadata: types.Metadata{
				Name: req.tokenName,
			},
			Spec: types.ProvisionTokenSpecV2{
				JoinMethod: types.JoinMethodIAM,
			},
		}
		fallthrough
	default:
		existingToken, ok := token.(*types.ProvisionTokenV2)
		if !ok {
			return trace.BadParameter("expected token to be of type ProvisionTokenV2, got %T", token)
		}

		token, err = updateTokenIAMJoin(ctx, req, rule, existingToken)
		if trace.IsAlreadyExists(err) {
			// Early return when the required rule already exists.
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := clt.UpsertToken(ctx, token); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
func updateTokenIAMJoin(ctx context.Context, req upsertIAMJoinTokenRequest, rule *types.TokenRule, existingToken *types.ProvisionTokenV2) (types.ProvisionToken, error) {
	if existingToken.GetJoinMethod() != types.JoinMethodIAM {
		return nil, trace.BadParameter("Token %q already exists but has the wrong join method %q. "+
			"Please remove it before continuing.",
			req.tokenName, existingToken.GetJoinMethod(),
		)
	}

	allRoles := existingToken.GetRoles()

	switch req.deploymentMode {
	case DatabaseServiceDeploymentMode:
		allRoles = append(allRoles, types.RoleDatabase)

	default:
		return nil, trace.BadParameter("invalid deployment mode %q, supported modes: %v", req.deploymentMode, DeploymentModes)
	}

	uniqueRoles := utils.Deduplicate(allRoles)
	existingToken.SetRoles(uniqueRoles)

	for _, rule := range existingToken.GetAllowRules() {
		if rule.AWSAccount == req.accountID && rule.AWSARN == req.awsARN {

			return nil, trace.AlreadyExists("rule already exists")
		}
	}

	existingToken.SetAllowRules(append(existingToken.GetAllowRules(), rule))

	return existingToken, nil
}
