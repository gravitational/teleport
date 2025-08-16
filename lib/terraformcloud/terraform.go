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
	"github.com/zitadel/oidc/v3/pkg/oidc"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

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
