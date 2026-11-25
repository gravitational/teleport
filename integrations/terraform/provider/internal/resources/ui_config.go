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
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/gravitational/teleport/api/types"

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
			SchemaFunc:  tfschema.GenSchemaUIConfigV1DataSource,
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
			SchemaFunc:   tfschema.GenSchemaUIConfigV1Resource,
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
