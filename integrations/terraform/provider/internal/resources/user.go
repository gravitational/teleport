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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewUserDataSourceType returns the user data source type.
func NewUserDataSourceType() tfdriver.DataSourceType[types.UserV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.UserV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.UserV2, tfdriver.NameIdentifier] {
			return teleport.NewUserClient(clientFromProvider(p))
		},
		Kind: types.KindUser,
		Name: types.KindUser,
		Codec: tfdriver.DataSourceCodecFuncs[types.UserV2]{
			SchemaFunc:  tfschema.GenSchemaUserV2,
			ToStateFunc: tfschema.CopyUserV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewUserResourceType returns the user resource type.
func NewUserResourceType() tfdriver.ResourceType[types.UserV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.UserV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.UserV2, tfdriver.NameIdentifier] {
			return teleport.NewUserClient(clientFromProvider(p))
		},
		Kind: types.KindUser,
		Name: types.KindUser,
		Codec: tfdriver.ResourceCodecFuncs[types.UserV2]{
			SchemaFunc:   tfschema.GenSchemaUserV2,
			ToStateFunc:  tfschema.CopyUserV2ToTerraform,
			FromPlanFunc: tfschema.CopyUserV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.UserV2](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(user *types.UserV2) string {
			return user.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.UserV2) string {
			return st.GetMetadata().Revision
		},
	}
}
