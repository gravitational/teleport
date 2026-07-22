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

// NewAppDataSourceType returns the app data source type.
func NewAppDataSourceType() tfdriver.DataSourceType[apitypes.AppV3, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[apitypes.AppV3, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.AppV3, tfdriver.NameIdentifier] {
			return teleport.NewAppClient(clientFromProvider(p))
		},
		Kind: apitypes.KindApp,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.AppV3]{
			SchemaFunc:  tfschema.GenSchemaAppV3,
			ToStateFunc: tfschema.CopyAppV3ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewAppResourceType returns the app resource type.
func NewAppResourceType() tfdriver.ResourceType[apitypes.AppV3, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[apitypes.AppV3, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.AppV3, tfdriver.NameIdentifier] {
			return teleport.NewAppClient(clientFromProvider(p))
		},
		Kind: apitypes.KindApp,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.AppV3]{
			SchemaFunc:   tfschema.GenSchemaAppV3,
			ToStateFunc:  tfschema.CopyAppV3ToTerraform,
			FromPlanFunc: tfschema.CopyAppV3FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.AppV3](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(av *apitypes.AppV3) string {
			return av.GetMetadata().Name
		}),
		ResourceRevision: func(st *apitypes.AppV3) string {
			return st.GetMetadata().Revision
		},
	}
}
