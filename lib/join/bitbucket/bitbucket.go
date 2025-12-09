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

package bitbucket

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "bitbucket")

// Validator is a validator for Bitbucket OIDC tokens.
type Validator interface {
	Validate(
		ctx context.Context, idpURL, audience, token string,
	) (*IDTokenClaims, error)
}

// IDTokenClaims
// See the following for the structure:
// https://support.atlassian.com/bitbucket-cloud/docs/integrate-pipelines-with-resource-servers-using-oidc/
type IDTokenClaims struct {
	oidc.TokenClaims

	// Sub provides some information about the Bitbucket Pipelines run that
	// generated this token. Format: {RepositoryUUID}:{StepUUID}
	Sub string `json:"sub"`

	// StepUUID is the UUID of the pipeline step for which this token was
	// issued. Bitbucket UUIDs must begin and end with braces, e.g. '{...}'
	StepUUID string `json:"stepUuid"`

	// RepositoryUUID is the UUID of the repository for which this token was
	// issued. Bitbucket UUIDs must begin and end with braces, e.g. '{...}'.
	// This value may be found in the Pipelines -> OpenID Connect section of the
	// repository settings.
	RepositoryUUID string `json:"repositoryUuid"`

	// PipelineUUID is the UUID of the pipeline for which this token was issued.
	// Bitbucket UUIDs must begin and end with braces, e.g. '{...}'
	PipelineUUID string `json:"pipelineUuid"`

	// WorkspaceUUID is the UUID of the workspace for which this token was
	// issued. Bitbucket UUIDs must begin and end with braces, e.g. '{...}'.
	// This value may be found in the Pipelines -> OpenID Connect section of the
	// repository settings.
	WorkspaceUUID string `json:"workspaceUuid"`

	// DeploymentEnvironmentUUID is the name of the deployment environment for
	// which this pipeline was executed. Bitbucket UUIDs must begin and end with
	// braces, e.g. '{...}'.
	DeploymentEnvironmentUUID string `json:"deploymentEnvironmentUuid"`

	// BranchName is the name of the branch on which this pipeline executed.
	BranchName string `json:"branchName"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsBitbucket {
	return &workloadidentityv1pb.JoinAttrsBitbucket{
		Sub:                       c.Sub,
		StepUuid:                  c.StepUUID,
		RepositoryUuid:            c.RepositoryUUID,
		PipelineUuid:              c.PipelineUUID,
		WorkspaceUuid:             c.WorkspaceUUID,
		DeploymentEnvironmentUuid: c.DeploymentEnvironmentUUID,
		BranchName:                c.BranchName,
	}
}

// CheckIDTokenParams are parameters used to validate Bitbucket OIDC tokens.
type CheckIDTokenParams struct {
	ProvisionToken provision.Token
	IDToken        []byte
	Clock          clockwork.Clock
	Validator      Validator
}

func (p *CheckIDTokenParams) checkAndSetDefaults() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case len(p.IDToken) == 0:
		return trace.BadParameter("IDToken is required")
	case p.Validator == nil:
		return trace.BadParameter("Validator is required")
	case p.Clock == nil:
		p.Clock = clockwork.NewRealClock()
	}
	return nil
}

// CheckIDToken validates a Bitbucket OIDC token, verifying both the validity of
// the OIDC token itself, as well as ensuring claims match any configured allow
// rules in the provided provision token.
func CheckIDToken(
	ctx context.Context,
	params *CheckIDTokenParams,
) (*IDTokenClaims, error) {
	if err := params.checkAndSetDefaults(); err != nil {
		return nil, trace.AccessDenied("%s", err.Error())
	}

	token, ok := params.ProvisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("bitbucket join method only supports ProvisionTokenV2, '%T' was provided", params.ProvisionToken)
	}

	claims, err := params.Validator.Validate(
		ctx, token.Spec.Bitbucket.IdentityProviderURL, token.Spec.Bitbucket.Audience, string(params.IDToken),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "Bitbucket run trying to join cluster",
		"claims", claims,
		"token", params.ProvisionToken.GetName(),
	)

	return claims, trace.Wrap(checkBitbucketAllowRules(token, claims))
}

func checkBitbucketAllowRules(token *types.ProvisionTokenV2, claims *IDTokenClaims) error {
	// If a single rule passes, accept the IDToken
	for _, rule := range token.Spec.Bitbucket.Allow {
		// Please consider keeping these field validators in the same order they
		// are defined within the ProvisionTokenSpecV2Bitbucket proto spec.

		if rule.WorkspaceUUID != "" && claims.WorkspaceUUID != rule.WorkspaceUUID {
			continue
		}

		if rule.RepositoryUUID != "" && claims.RepositoryUUID != rule.RepositoryUUID {
			continue
		}

		if rule.DeploymentEnvironmentUUID != "" && claims.DeploymentEnvironmentUUID != rule.DeploymentEnvironmentUUID {
			continue
		}

		if rule.BranchName != "" && claims.BranchName != rule.BranchName {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
