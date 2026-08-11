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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewDynamicWindowsDesktopDataSourceType returns the dynamic Windows desktop data source type.
func NewDynamicWindowsDesktopDataSourceType() tfdriver.DataSourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
			return teleport.NewDynamicWindowsDesktopClient(clientFromProvider(p))
		},
		Kind: types.KindDynamicWindowsDesktop,
		Codec: tfdriver.DataSourceCodecFuncs[types.DynamicWindowsDesktopV1]{
			SchemaFunc:  tfschema.GenSchemaDynamicWindowsDesktopV1,
			ToStateFunc: tfschema.CopyDynamicWindowsDesktopV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewDynamicWindowsDesktopResourceType returns the dynamic Windows desktop resource type.
func NewDynamicWindowsDesktopResourceType() tfdriver.ResourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
			return teleport.NewDynamicWindowsDesktopClient(clientFromProvider(p))
		},
		Kind: types.KindDynamicWindowsDesktop,
		Codec: tfdriver.ResourceCodecFuncs[types.DynamicWindowsDesktopV1]{
			SchemaFunc:   tfschema.GenSchemaDynamicWindowsDesktopV1,
			ToStateFunc:  tfschema.CopyDynamicWindowsDesktopV1ToTerraform,
			FromPlanFunc: tfschema.CopyDynamicWindowsDesktopV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.DynamicWindowsDesktopV1](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(desktop *types.DynamicWindowsDesktopV1) string {
			return desktop.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.DynamicWindowsDesktopV1) string {
			return st.GetMetadata().Revision
		},
	}
}
