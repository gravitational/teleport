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

package terraformcloud

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/zitadel/oidc/v3/pkg/oidc"

	"github.com/gravitational/teleport"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/join/provision"
	"github.com/gravitational/teleport/lib/modules"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var log = logutils.NewPackageLogger(teleport.ComponentKey, "terraformcloud")

type Validator interface {
	Validate(
		ctx context.Context, audience, hostname, token string,
	) (*IDTokenClaims, error)
}

// IDTokenClaims
// See the following for the structure:
// https://developer.hashicorp.com/terraform/enterprise/workspaces/dynamic-provider-credentials/workload-identity-tokens
type IDTokenClaims struct {
	oidc.TokenClaims
	// Sub provides some information about the Spacelift run that generated this
	// token.
	// organization:<org name>:project:<project name>:workspace:<workspace name>:run_phase:<phase>
	Sub string `json:"sub"`

	// OrganizationID is the ID of the HCP Terraform organization
	OrganizationID string `json:"terraform_organization_id"`
	// OrganizationName is the human-readable name of the HCP Terraform organization
	OrganizationName string `json:"terraform_organization_name"`
	// ProjectID is the ID of the HCP Terraform project
	ProjectID string `json:"terraform_project_id"`
	// ProjectName is the human-readable name of the HCP Terraform project
	ProjectName string `json:"terraform_project_name"`
	// WorkspaceID is the ID of the HCP Terraform project
	WorkspaceID string `json:"terraform_workspace_id"`
	// WorkspaceName is the human-readable name of the HCP Terraform workspace
	WorkspaceName string `json:"terraform_workspace_name"`
	// FullWorkspace is the full path to the workspace, e.g. `organization:<name>:project:<name>:workspace:<name>`
	FullWorkspace string `json:"terraform_full_workspace"`
	// RunID is the ID of the run the token was generated for.
	RunID string `json:"terraform_run_id"`
	// RunPhase is the phase of the run the token was issued for, e.g. `plan` or `apply`
	RunPhase string `json:"terraform_run_phase"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsTerraformCloud {
	return &workloadidentityv1pb.JoinAttrsTerraformCloud{
		Sub:              c.Sub,
		OrganizationName: c.OrganizationName,
		ProjectName:      c.ProjectName,
		WorkspaceName:    c.WorkspaceName,
		FullWorkspace:    c.FullWorkspace,
		RunId:            c.RunID,
		RunPhase:         c.RunPhase,
	}
}

// CheckIDTokenParams are parameters used to validate CircleCI OIDC tokens.
type CheckIDTokenParams struct {
	ProvisionToken provision.Token
	IDToken        []byte
	Validator      Validator
	ClusterName    string
}

func (p *CheckIDTokenParams) checkAndSetDefaults() error {
	switch {
	case p.ProvisionToken == nil:
		return trace.BadParameter("ProvisionToken is required")
	case len(p.IDToken) == 0:
		return trace.BadParameter("IDToken is required")
	case p.Validator == nil:
		return trace.BadParameter("Validator is required")
	case p.ClusterName == "":
		return trace.BadParameter("ClusterName is required")
	}
	return nil
}

// CheckIDToken validates a CircleCI OIDC token, verifying both the validity of
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
		return nil, trace.BadParameter("terraform_cloud join method only supports ProvisionTokenV2, '%T' was provided", params.ProvisionToken)
	}

	hostnameOverride := token.Spec.TerraformCloud.Hostname
	if hostnameOverride != "" {
		if err := modules.GetModules().RequireEnterpriseBuild("terraform_cloud joining for Terraform Enterprise"); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	aud := token.Spec.TerraformCloud.Audience
	if aud == "" {
		aud = params.ClusterName
	}

	claims, err := params.Validator.Validate(
		ctx, aud, hostnameOverride, string(params.IDToken),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.InfoContext(ctx, "Terraform Cloud run trying to join cluster",
		"claims", claims,
		"token", token.GetName(),
	)

	return claims, trace.Wrap(checkTerraformCloudAllowRules(token, claims))
}

func checkTerraformCloudAllowRules(token *types.ProvisionTokenV2, claims *IDTokenClaims) error {
	for _, rule := range token.Spec.TerraformCloud.Allow {
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
