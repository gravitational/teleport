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

// NewInstallerDataSourceType returns the installer data source type.
func NewInstallerDataSourceType() tfdriver.DataSourceType[types.InstallerV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.InstallerV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.InstallerV1, tfdriver.NameIdentifier] {
			return teleport.NewInstallerClient(clientFromProvider(p))
		},
		Kind: types.KindInstaller,
		Name: types.KindInstaller,
		Codec: tfdriver.DataSourceCodecFuncs[types.InstallerV1]{
			SchemaFunc:  tfschema.GenSchemaInstallerV1DataSource,
			ToStateFunc: tfschema.CopyInstallerV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewInstallerResourceType returns the installer resource type.
func NewInstallerResourceType() tfdriver.ResourceType[types.InstallerV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.InstallerV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.InstallerV1, tfdriver.NameIdentifier] {
			return teleport.NewInstallerClient(clientFromProvider(p))
		},
		Kind: types.KindInstaller,
		Name: types.KindInstaller,
		Codec: tfdriver.ResourceCodecFuncs[types.InstallerV1]{
			SchemaFunc:   tfschema.GenSchemaInstallerV1Resource,
			ToStateFunc:  tfschema.CopyInstallerV1ToTerraform,
			FromPlanFunc: tfschema.CopyInstallerV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.InstallerV1](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(installer *types.InstallerV1) string {
			return installer.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.InstallerV1) string {
			return st.GetMetadata().Revision
		},
	}
}
