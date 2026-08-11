// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/scopes"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/workloadidentity/v1"
)

// NewWorkloadIdentityDataSourceType returns the workload identity data source type.
func NewWorkloadIdentityDataSourceType() tfdriver.DataSourceType[workloadidentityv1.WorkloadIdentity, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[workloadidentityv1.WorkloadIdentity, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[workloadidentityv1.WorkloadIdentity, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewWorkloadIdentityClient(clientFromProvider(p))
		},
		Kind: types.KindWorkloadIdentity,
		Name: types.KindWorkloadIdentity,
		Codec: tfdriver.DataSourceCodecFuncs[workloadidentityv1.WorkloadIdentity]{
			SchemaFunc:  schemav1.GenSchemaWorkloadIdentity,
			ToStateFunc: schemav1.CopyWorkloadIdentityToTerraform,
		},
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierFromPath(tfdriver.ScopeQualifiedPath{
			Name:  path.Root("metadata").AtName("name"),
			Scope: path.Root("scope"),
		}),
	}
}

// NewWorkloadIdentityResourceType returns the workload identity resource type.
func NewWorkloadIdentityResourceType() tfdriver.ResourceType[workloadidentityv1.WorkloadIdentity, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[workloadidentityv1.WorkloadIdentity, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[workloadidentityv1.WorkloadIdentity, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewWorkloadIdentityClient(clientFromProvider(p))
		},
		Kind: types.KindWorkloadIdentity,
		Name: types.KindWorkloadIdentity,
		Codec: tfdriver.ResourceCodecFuncs[workloadidentityv1.WorkloadIdentity]{
			SchemaFunc:   schemav1.GenSchemaWorkloadIdentity,
			ToStateFunc:  schemav1.CopyWorkloadIdentityToTerraform,
			FromPlanFunc: schemav1.CopyWorkloadIdentityFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[workloadidentityv1.WorkloadIdentity](types.KindWorkloadIdentity),
		Identifier: tfdriver.PossiblyUnscopedScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(workloadIdentity *workloadidentityv1.WorkloadIdentity) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  workloadIdentity.GetMetadata().GetName(),
					Scope: workloadIdentity.GetScope(),
				}
			}),
		ResourceRevision: func(st *workloadidentityv1.WorkloadIdentity) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
