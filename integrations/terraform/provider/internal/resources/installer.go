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

// NewInstallerDataSourceType returns the installer data source type.
func NewInstallerDataSourceType() tfdriver.DataSourceType[types.InstallerV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.InstallerV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.InstallerV1, tfdriver.NameIdentifier] {
			return teleport.NewInstallerClient(clientFromProvider(p))
		},
		Kind: types.KindInstaller,
		Name: types.KindInstaller,
		Codec: tfdriver.DataSourceCodecFuncs[types.InstallerV1]{
			SchemaFunc:  tfschema.GenSchemaInstallerV1,
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
			SchemaFunc:   tfschema.GenSchemaInstallerV1,
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
