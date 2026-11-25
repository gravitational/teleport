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

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewDynamicWindowsDesktopDataSourceType returns the dynamic Windows desktop data source type.
func NewDynamicWindowsDesktopDataSourceType() tfdriver.DataSourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
			return teleport.NewDynamicWindowsDesktopClient(clientFromProvider(p))
		},
		Kind: types.KindDynamicWindowsDesktop,
		Name: types.KindDynamicWindowsDesktop,
		Codec: tfdriver.DataSourceCodecFuncs[types.DynamicWindowsDesktopV1]{
			SchemaFunc:  tfschema.GenSchemaDynamicWindowsDesktopV1DataSource,
			ToStateFunc: tfschema.CopyDynamicWindowsDesktopV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewDynamicWindowsDesktopResourceType returns the dynamic Windows desktop resource type.
func NewDynamicWindowsDesktopResourceType() tfdriver.ResourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.DynamicWindowsDesktopV1, tfdriver.NameIdentifier] {
			return teleport.NewDynamicWindowsDesktopClient(clientFromProvider(p))
		},
		Kind: types.KindDynamicWindowsDesktop,
		Name: types.KindDynamicWindowsDesktop,
		Codec: tfdriver.ResourceCodecFuncs[types.DynamicWindowsDesktopV1]{
			SchemaFunc:   tfschema.GenSchemaDynamicWindowsDesktopV1Resource,
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
