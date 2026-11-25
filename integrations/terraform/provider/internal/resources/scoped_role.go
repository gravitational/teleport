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
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/scopes/access/v1"
)

// NewScopedRoleDataSourceType returns the scoped role data source type.
func NewScopedRoleDataSourceType() tfdriver.DataSourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedRoleClient(clientFromProvider(p))
		},
		Identifier: tfdriver.ScopeQualifiedNameIdentifierFromPath(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			}),
		Kind: scopedaccess.KindScopedRole,
		Name: scopedaccess.KindScopedRole,
		Codec: tfdriver.DataSourceCodecFuncs[accessv1.ScopedRole]{
			SchemaFunc:  schemav1.GenSchemaScopedRoleDataSource,
			ToStateFunc: schemav1.CopyScopedRoleToTerraform,
		},
	}
}

// NewScopedRoleResourceType returns the scoped role resource type.
func NewScopedRoleResourceType() tfdriver.ResourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedRoleClient(clientFromProvider(p))
		},
		Kind: scopedaccess.KindScopedRole,
		Name: scopedaccess.KindScopedRole,
		Codec: tfdriver.ResourceCodecFuncs[accessv1.ScopedRole]{
			SchemaFunc:   schemav1.GenSchemaScopedRoleResource,
			ToStateFunc:  schemav1.CopyScopedRoleToTerraform,
			FromPlanFunc: schemav1.CopyScopedRoleFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyScopedRoleFromTerraform,
				schemav1.CopyScopedRoleToTerraformPreserveUnknown,
			),
		},
		Normalizer: tfdriver.ForceKind[accessv1.ScopedRole](scopedaccess.KindScopedRole),
		Identifier: tfdriver.ScopeQualifiedNameIdentifierPolicy(
			tfdriver.ScopeQualifiedPath{
				Name:  path.Root("metadata").AtName("name"),
				Scope: path.Root("scope"),
			},
			func(st *accessv1.ScopedRole) scopes.QualifiedName {
				return scopes.QualifiedName{
					Name:  st.GetMetadata().GetName(),
					Scope: st.GetScope(),
				}
			}),
		ResourceRevision: func(st *accessv1.ScopedRole) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
