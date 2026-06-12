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

package azuredevops

import (
	"github.com/zitadel/oidc/v3/pkg/oidc"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

// IDTokenClaims for the pipeline OIDC ID Token issued by Azure Devops
type IDTokenClaims struct {
	oidc.TokenClaims
	// Sub provides some information about the Azure Devops pipeline run.
	// Example:
	// p://noahstride0304/testing-azure-devops-join/strideynet.azure-devops-testing
	Sub string `json:"sub"`
	// OrganizationName is the name of the Azure Devops organization the project
	// and pipeline belongs to. This name is extracted from the Sub.
	OrganizationName string `json:"-"`
	// ProjectName is the name of the Azure Devops project the pipeline belongs
	// to. This name is extracted from the Sub.
	ProjectName string `json:"-"`
	// PipelineName is the name of the Azure Devops pipeline that the token
	// belongs to. This name is extracted from the Sub.
	PipelineName string `json:"-"`

	// OrganizationID is the ID of the organization the pipeline belongs to.
	OrganizationID string `json:"org_id"`
	// ProjectID is the ID of the project the pipeline belongs to.
	ProjectID string `json:"prj_id"`
	// DefinitionID is the ID of the pipeline definition.
	DefinitionID string `json:"def_id"`
	// RepositoryID is the ID of the repository. This is not a UUID as the other
	// ID fields. Example:
	// strideynet/azure-devops-testing
	RepositoryID string `json:"rpo_id"`
	// RepositoryURI is the URI of the repository.
	RepositoryURI string `json:"rpo_uri"`
	// RepositoryVersion is the "version" of the repository the pipeline is
	// running against. For a git repo, this is the commit sha.
	RepositoryVersion string `json:"rpo_ver"`
	// RepositoryRef is the reference that the pipeline is running
	// against. Example:
	// refs/heads/main
	RepositoryRef string `json:"rpo_ref"`
	// RunID is the ID of the pipeline run that the token belongs to.
	RunID string `json:"run_id"`
}

func (c *IDTokenClaims) GetSubject() string {
	return c.Sub
}

// ForAudit returns a map of the claims with the names adjusted to match the
// definition in the JoinToken allow rules. This allows us to provide a better
// UX by using the same names in the allow rules as are presented in the audit
// logs.
func (c *IDTokenClaims) ForAudit() map[string]any {
	return map[string]any{
		"sub":               c.Sub,
		"organization_name": c.OrganizationName,
		"project_name":      c.ProjectName,
		"pipeline_name":     c.PipelineName,
		"organization_id":   c.OrganizationID,
		"project_id":        c.ProjectID,
		"definition_id":     c.DefinitionID,
		"repository_id":     c.RepositoryID,
		"repository_uri":    c.RepositoryURI,
		"repository_ver":    c.RepositoryVersion,
		"repository_ref":    c.RepositoryRef,
		"raw":               c,
	}
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsAzureDevops {
	return &workloadidentityv1pb.JoinAttrsAzureDevops{
		Pipeline: &workloadidentityv1pb.JoinAttrsAzureDevopsPipeline{
			Sub:               c.Sub,
			OrganizationName:  c.OrganizationName,
			ProjectName:       c.ProjectName,
			PipelineName:      c.PipelineName,
			OrganizationId:    c.OrganizationID,
			ProjectId:         c.ProjectID,
			DefinitionId:      c.DefinitionID,
			RepositoryId:      c.RepositoryID,
			RepositoryVersion: c.RepositoryVersion,
			RepositoryRef:     c.RepositoryRef,
			RunId:             c.RunID,
		},
	}
}
