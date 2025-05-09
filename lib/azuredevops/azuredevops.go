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

import workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"

// IDTokenClaims for the pipeline OIDC ID Token issued by Azure Devops
type IDTokenClaims struct {
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
	// RepositoryReference is the reference that the pipeline is running
	// against. Example:
	// refs/heads/main
	RepositoryReference string `json:"rpo_ref"`
	// RunID is the ID of the pipeline run that the token belongs to.
	RunID string `json:"run_id"`
}

// JoinAttrs returns the protobuf representation of the attested identity.
// This is used for auditing and for evaluation of WorkloadIdentity rules and
// templating.
func (c *IDTokenClaims) JoinAttrs() *workloadidentityv1pb.JoinAttrsBitbucket {
	// TODO
	return nil
}
