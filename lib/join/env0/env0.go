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

package env0

import (
	"github.com/zitadel/oidc/v3/pkg/oidc"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// IDTokenClaims are the additional custom claims provided by Env0 for their
// issued OIDC tokens. See also:
// https://docs.envzero.com/guides/integrations/oidc-integrations
type IDTokenClaims struct {
	oidc.TokenClaims

	// OrganizationID is a unique identifier for the organization. Corresponds
	// to `organizationId`.
	OrganizationID string `json:"organizationId"`
	// ProjectID is a unique identifier for the project. Corresponds to
	// `projectId`.
	ProjectID string `json:"projectId"`
	// ProjectName is the name of the project. Corresponds to `projectName`.
	ProjectName string `json:"projectName"`
	// TemplateID is a unique identifier for the template. Corresponds to
	// `templateId`.
	TemplateID string `json:"templateId"`
	// TemplateName is the name of the template. Corresponds to `templateName`.
	TemplateName string `json:"templateName"`
	// EnvironmentID is a unique identifier for the environment. Corresponds to
	// `environmentId`.
	EnvironmentID string `json:"environmentId"`
	// EnvironmentName is the name of the environment. Corresponds to
	// `environmentName`.
	EnvironmentName string `json:"environmentName"`
	// WorkspaceName is the name of the workspace. Corresponds to
	// `workspaceName`.
	WorkspaceName string `json:"workspaceName"`
	// DeploymentLogID is a unique identifier for this deployment. Corresponds
	// to `deploymentLogId`.
	DeploymentLogID string `json:"deploymentLogId"`
	// DeploymentType is the type of this deployment. Corresponds to
	// `deploymentType`.
	DeploymentType string `json:"deploymentType"`
	// DeployerEmail is the email of the user that started this deployment.
	// Corresponds to `deployerEmail`.
	DeployerEmail string `json:"deployerEmail"`
	// Env0Tag is an optional custom tag. Corresponds to `env0Tag`.
	Env0Tag string `json:"env0Tag"`
}

func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsEnv0 {
	return &workloadidentityv1pb.JoinAttrsEnv0{
		Sub:             c.Subject,
		OrganizationId:  c.OrganizationID,
		ProjectId:       c.ProjectID,
		ProjectName:     c.ProjectName,
		TemplateId:      c.TemplateID,
		TemplateName:    c.TemplateName,
		EnvironmentId:   c.EnvironmentID,
		EnvironmentName: c.EnvironmentName,
		WorkspaceName:   c.WorkspaceName,
		DeploymentLogId: c.DeploymentLogID,
		DeploymentType:  c.DeploymentType,
		DeployerEmail:   c.DeployerEmail,
		Env0Tag:         c.Env0Tag,
	}
}
