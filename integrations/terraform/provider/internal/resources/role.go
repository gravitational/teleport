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

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewRoleDataSourceType returns the role data source type.
func NewRoleDataSourceType() tfdriver.DataSourceType[apitypes.RoleV6, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[apitypes.RoleV6, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.RoleV6, tfdriver.NameIdentifier] {
			return teleport.NewRoleClient(clientFromProvider(p))
		},
		Kind: apitypes.KindRole,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.RoleV6]{
			SchemaFunc:  tfschema.GenSchemaRoleV6,
			ToStateFunc: tfschema.CopyRoleV6ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewRoleResourceType returns the role resource type.
func NewRoleResourceType() tfdriver.ResourceType[apitypes.RoleV6, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[apitypes.RoleV6, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.RoleV6, tfdriver.NameIdentifier] {
			return teleport.NewRoleClient(clientFromProvider(p))
		},
		Kind: apitypes.KindRole,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.RoleV6]{
			SchemaFunc:   tfschema.GenSchemaRoleV6,
			ToStateFunc:  tfschema.CopyRoleV6ToTerraform,
			FromPlanFunc: tfschema.CopyRoleV6FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.RoleV6](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(role *apitypes.RoleV6) string {
			return role.GetMetadata().Name
		}),
		ResourceRevision: func(st *apitypes.RoleV6) string {
			return st.GetMetadata().Revision
		},
	}
}
