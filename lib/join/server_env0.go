// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package join

import (
	"context"

	"github.com/gravitational/trace"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/env0"
	"github.com/gravitational/teleport/lib/join/provision"
)

type Env0TokenValidator interface {
	ValidateToken(
		ctx context.Context,
		token []byte,
	) (*env0.IDTokenClaims, error)
}

// validateEnv0Token performs OIDC token verification for Env0-type JWTs,
// suitable for use in `handleOIDCJoin`
func (s *Server) validateEnv0Token(
	ctx context.Context,
	provisionToken provision.Token,
	idToken []byte,
) (any, *workloadidentityv1.JoinAttrs, error) {
	verifiedIdentity, err := s.cfg.AuthService.GetEnv0IDTokenValidator().ValidateToken(ctx, idToken)
	if err != nil {
		return nil, nil, trace.Wrap(err, "validating Env0 OIDC token")
	}

	ptv2, ok := provisionToken.(*types.ProvisionTokenV2)
	if !ok {
		return nil, nil, trace.BadParameter("expected *types.ProvisionTokenV2, got %T", provisionToken)
	}

	if err := checkEnv0AllowRules(ptv2, verifiedIdentity); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return verifiedIdentity, &workloadidentityv1.JoinAttrs{
		Env0: verifiedIdentity.JoinAttrs(),
	}, nil
}

func checkEnv0AllowRules(token *types.ProvisionTokenV2, claims *env0.IDTokenClaims) error {
	for _, rule := range token.Spec.Env0.Allow {
		if rule.OrganizationID != "" && claims.OrganizationID != rule.OrganizationID {
			continue
		}
		if rule.ProjectID != "" && claims.ProjectID != rule.ProjectID {
			continue
		}
		if rule.ProjectName != "" && claims.ProjectName != rule.ProjectName {
			continue
		}
		if rule.TemplateID != "" && claims.TemplateID != rule.TemplateID {
			continue
		}
		if rule.TemplateName != "" && claims.TemplateName != rule.TemplateName {
			continue
		}
		if rule.EnvironmentID != "" && claims.EnvironmentID != rule.EnvironmentID {
			continue
		}
		if rule.EnvironmentName != "" && claims.EnvironmentName != rule.EnvironmentName {
			continue
		}
		if rule.WorkspaceName != "" && claims.WorkspaceName != rule.WorkspaceName {
			continue
		}
		if rule.DeploymentType != "" && claims.DeploymentType != rule.DeploymentType {
			continue
		}
		if rule.DeployerEmail != "" && claims.DeployerEmail != rule.DeployerEmail {
			continue
		}
		if rule.Env0Tag != "" && claims.Env0Tag != rule.Env0Tag {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
