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
// Example token claims:
//
//	{
//	  "apiKeyType": "oidc",
//	  "aud": "https://prod.env0.com",
//	  "azp": "hoMiq9PdkRh9LUvVpH4wIErWg50VSG1b",
//	  "deployerEmail": "user@email.com",
//	  "deploymentLogId": "4224831d-05c4-4548-841d-d61988e179cb",
//	  "deploymentType": "deploy",
//	  "environmentId": "50aa6e65-2956-4984-aab3-27af9dcc06dc",
//	  "environmentName": "teleport-env0-demo",
//	  "exp": 1761193071,
//	  "gty": "password",
//	  "https://aws.amazon.com/tags": {
//	    "principal_tags": {
//	      "deployerEmail": ["user@email.com"],
//	      "deploymentType": ["deploy"],
//	      "environmentId": ["50aa6e65-2956-4984-aab3-27af9dcc06dc"],
//	      "organizationId": ["948de0c4-94d6-4ad6-8e56-1374353e9a38"],
//	      "projectId": ["df15f983-808a-49c7-bc91-910cb10411ec"],
//	      "templateId": ["00d81864-757c-4449-a91c-cf6bee07e44a"]
//	    }
//	  },
//	  "https://env0.com/apiKeyType": "oidc",
//	  "https://env0.com/deployerEmail": "user@email.com",
//	  "https://env0.com/deploymentLogId": "4224831d-05c4-4548-841d-d61988e179cb",
//	  "https://env0.com/deploymentType": "deploy",
//	  "https://env0.com/environmentId": "50aa6e65-2956-4984-aab3-27af9dcc06dc",
//	  "https://env0.com/environmentName": "teleport-env0-demo",
//	  "https://env0.com/organization": "948de0c4-94d6-4ad6-8e56-1374353e9a38",
//	  "https://env0.com/organizationId": "948de0c4-94d6-4ad6-8e56-1374353e9a38",
//	  "https://env0.com/projectId": "df15f983-808a-49c7-bc91-910cb10411ec",
//	  "https://env0.com/projectName": "My First Project",
//	  "https://env0.com/templateId": "00d81864-757c-4449-a91c-cf6bee07e44a",
//	  "https://env0.com/templateName": "single-use-template-for-teleport-env0-demo",
//	  "https://env0.com/workspaceName": "teleport-env0-demo-000000",
//	  "iat": 1761106671,
//	  "iss": "https://login.app.env0.com/",
//	  "organization": "948de0c4-94d6-4ad6-8e56-1374353e9a38",
//	  "organizationId": "948de0c4-94d6-4ad6-8e56-1374353e9a38",
//	  "projectId": "df15f983-808a-49c7-bc91-910cb10411ec",
//	  "projectName": "My First Project",
//	  "sub": "auth0|68f8497fe94b94dab2324697",
//	  "templateId": "00d81864-757c-4449-a91c-cf6bee07e44a",
//	  "templateName": "single-use-template-for-teleport-env0-demo",
//	  "workspaceName": "teleport-env0-demo-000000"
//	}
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
