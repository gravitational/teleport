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

package auth

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/terraformcloud"
)

type terraformIDTokenValidator interface {
	Validate(
		ctx context.Context, audience string, token string,
	) (*terraformcloud.IDTokenClaims, error)
}

func (a *Server) checkTerraformJoinRequest(ctx context.Context, req *types.RegisterUsingTokenRequest) (*terraformcloud.IDTokenClaims, error) {
	if req.IDToken == "" {
		return nil, trace.BadParameter("id_token not provided for terraform join request")
	}
	pt, err := a.GetToken(ctx, req.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, ok := pt.(*types.ProvisionTokenV2)
	if !ok {
		return nil, trace.BadParameter("terraform join method only supports ProvisionTokenV2, '%T' was provided", pt)
	}

	if modules.GetModules().BuildType() != modules.BuildEnterprise {
		return nil, fmt.Errorf(
			"terraform joining: %w",
			ErrRequiresEnterprise,
		)
	}

	aud := token.Spec.Terraform.Audience
	if aud == "" {
		clusterName, err := a.GetClusterName()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		aud = clusterName.GetClusterName()
	}

	claims, err := a.terraformIDTokenValidator.Validate(
		ctx, aud, req.IDToken,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.WithFields(logrus.Fields{
		"claims": claims,
		"token":  pt.GetName(),
	}).Info("Terraform run trying to join cluster")

	return claims, trace.Wrap(checkTerraformAllowRules(token, claims))
}

func checkTerraformAllowRules(token *types.ProvisionTokenV2, claims *terraformcloud.IDTokenClaims) error {
	for _, rule := range token.Spec.Terraform.Allow {
		if rule.OrganizationID != "" && claims.OrganizationID != rule.OrganizationID {
			continue
		}
		if rule.OrganizationName != "" && claims.OrganizationName != rule.OrganizationName {
			continue
		}
		if rule.ProjectID != "" && claims.ProjectID != rule.ProjectID {
			continue
		}
		if rule.ProjectName != "" && claims.ProjectName != rule.ProjectName {
			continue
		}
		if rule.WorkspaceID != "" && claims.WorkspaceID != rule.WorkspaceID {
			continue
		}
		if rule.WorkspaceName != "" && claims.WorkspaceName != rule.WorkspaceName {
			continue
		}
		if rule.RunPhase != "" && claims.RunPhase != rule.RunPhase {
			continue
		}

		// All provided rules met.
		return nil
	}

	return trace.AccessDenied("id token claims did not match any allow rules")
}
