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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	accessv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/access/v1"
	scopedaccess "github.com/gravitational/teleport/lib/scopes/access"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/scopes/access/v1"
)

// NewScopedRoleDataSourceType returns the scoped role data source type.
func NewScopedRoleDataSourceType() tfdriver.DataSourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.DataSourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedRoleClient(clientFromProvider(p))
		},
		Identifier: tfdriver.ScopeQualifiedNameIdentifierFromPath(path.Root("metadata").AtName("name"), path.Root("scope")),
		Kind:       scopedaccess.KindScopedRole,
		Codec: tfdriver.DataSourceCodecFuncs[accessv1.ScopedRole]{
			SchemaFunc:  schemav1.GenSchemaScopedRole,
			ToStateFunc: schemav1.CopyScopedRoleToTerraform,
		},
	}
}

// NewScopedRoleResourceType returns the scoped role resource type.
func NewScopedRoleResourceType() tfdriver.ResourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
	return tfdriver.ResourceType[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[accessv1.ScopedRole, tfdriver.ScopeQualifiedNameIdentifier] {
			return teleport.NewScopedRoleClient(clientFromProvider(p))
		},
		Kind: scopedaccess.KindScopedRole,
		Codec: tfdriver.ResourceCodecFuncs[accessv1.ScopedRole]{
			SchemaFunc:   schemav1.GenSchemaScopedRole,
			ToStateFunc:  schemav1.CopyScopedRoleToTerraform,
			FromPlanFunc: schemav1.CopyScopedRoleFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[accessv1.ScopedRole](scopedaccess.KindScopedRole),
		Identifier: tfdriver.ScopeQualifiedNameIdentifierPolicy(
			path.Root("metadata").AtName("name"),
			path.Root("scope"),
			func(st *accessv1.ScopedRole) (string, string) {
				return st.GetMetadata().GetName(), st.GetScope()
			}),
		ResourceRevision: func(st *accessv1.ScopedRole) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
