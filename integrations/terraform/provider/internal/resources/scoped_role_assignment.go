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

	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	"github.com/gravitational/teleport/api/scopes"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/scopes/access/assignment/v1"
)

// NewScopedRoleAssignmentDataSourceType returns the scoped token data source type.
func NewScopedRoleAssignmentDataSourceType() tfdriver.DataSourceType[accessv1.ScopedRoleAssignment, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[accessv1.ScopedRoleAssignment, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[accessv1.ScopedRoleAssignment, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedRoleAssignmentClient(clientFromProvider(p))
		},
		Identifier: tfdriver.ScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			}),
		Kind: scopedaccess.KindScopedRoleAssignment,
		Name: scopedaccess.KindScopedRoleAssignment,
		Codec: tfdriver.DataSourceCodecFuncs[accessv1.ScopedRoleAssignment]{
			SchemaFunc:  schemav1.GenSchemaScopedRoleAssignmentDataSource,
			ToStateFunc: schemav1.CopyScopedRoleAssignmentToTerraform,
		},
	}
}

// NewScopedRoleAssignmentResourceType returns the scoped token resource type.
func NewScopedRoleAssignmentResourceType() tfdriver.ResourceType[accessv1.ScopedRoleAssignment, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[accessv1.ScopedRoleAssignment, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[accessv1.ScopedRoleAssignment, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedRoleAssignmentClient(clientFromProvider(p))
		},
		Kind: scopedaccess.KindScopedRoleAssignment,
		Name: scopedaccess.KindScopedRoleAssignment,
		Codec: tfdriver.ResourceCodecFuncs[accessv1.ScopedRoleAssignment]{
			SchemaFunc:   schemav1.GenSchemaScopedRoleAssignmentResource,
			ToStateFunc:  schemav1.CopyScopedRoleAssignmentToTerraform,
			FromPlanFunc: schemav1.CopyScopedRoleAssignmentFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyScopedRoleAssignmentFromTerraform,
				schemav1.CopyScopedRoleAssignmentToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[accessv1.ScopedRoleAssignment](scopedaccess.KindScopedRoleAssignment),
		Identifier: tfdriver.ScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(st *accessv1.ScopedRoleAssignment) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  st.GetMetadata().GetName(),
					Scope: st.GetScope(),
				}

			}),
		ResourceRevision: func(st *accessv1.ScopedRoleAssignment) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
