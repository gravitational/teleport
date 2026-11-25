// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	workloadidentityv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"

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
			SchemaFunc:  schemav1.GenSchemaWorkloadIdentityDataSourceWithExtraClaims,
			ToStateFunc: schemav1.CopyWorkloadIdentityToTerraformWithExtraClaims,
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
			SchemaFunc:   schemav1.GenSchemaWorkloadIdentityResourceWithExtraClaims,
			ToStateFunc:  schemav1.CopyWorkloadIdentityToTerraformWithExtraClaims,
			FromPlanFunc: schemav1.CopyWorkloadIdentityFromTerraformWithExtraClaims,
			ModifyPlanFunc: schemav1.ModifyWorkloadIdentityPlanWithExtraClaims(tfdriver.NullToZeroModifyPlan(
				schemav1.CopyWorkloadIdentityFromTerraform,
				schemav1.CopyWorkloadIdentityToTerraformPreserveUnknown,
			)),
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
