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
	"github.com/gravitational/teleport/api/types"
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// uiConfigID is the static identifier used in the terraform state.
// Note that types.MetaNameUIConfig is "ui-config" which can't be used here
// without causing churn.
const uiConfigID = "ui_config"

// NewUIConfigDataSourceType returns the web ui config data source type.
func NewUIConfigDataSourceType() tfdriver.DataSourceType[types.UIConfigV1, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[types.UIConfigV1, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.UIConfigV1, tfdriver.SingletonIdentifier] {
			return teleport.NewUIConfigClient(clientFromProvider(p))
		},
		Kind: types.KindUIConfig,
		Name: types.KindUIConfig,
		Codec: tfdriver.DataSourceCodecFuncs[types.UIConfigV1]{
			SchemaFunc:  tfschema.GenSchemaUIConfigV1,
			ToStateFunc: tfschema.CopyUIConfigV1ToTerraform,
		},
		Identifier: tfdriver.SingletonIdentifierFromName(uiConfigID),
	}
}

// NewUIConfigResourceType returns the web ui config resource type.
func NewUIConfigResourceType() tfdriver.ResourceType[types.UIConfigV1, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[types.UIConfigV1, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.UIConfigV1, tfdriver.SingletonIdentifier] {
			return teleport.NewUIConfigClient(clientFromProvider(p))
		},
		Kind: types.KindUIConfig,
		Name: types.KindUIConfig,
		Codec: tfdriver.ResourceCodecFuncs[types.UIConfigV1]{
			SchemaFunc:   tfschema.GenSchemaUIConfigV1,
			ToStateFunc:  tfschema.CopyUIConfigV1ToTerraform,
			FromPlanFunc: tfschema.CopyUIConfigV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.UIConfigV1](),
		Identifier: tfdriver.SingletonIdentifierPolicy[types.UIConfigV1](uiConfigID),
		ResourceRevision: func(st *types.UIConfigV1) string {
			return st.GetMetadata().Revision
		},
	}
}
