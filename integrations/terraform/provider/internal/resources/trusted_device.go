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
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/devicetrust/v1"
)

// NewTrustedDeviceDataSourceType returns the trusted device data source type.
func NewTrustedDeviceDataSourceType() tfdriver.DataSourceType[types.DeviceV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.DeviceV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.DeviceV1, tfdriver.NameIdentifier] {
			return teleport.NewTrustedDeviceClient(clientFromProvider(p))
		},
		Kind: types.KindDevice,
		Codec: tfdriver.DataSourceCodecFuncs[types.DeviceV1]{
			SchemaFunc:  schemav1.GenSchemaDeviceV1,
			ToStateFunc: schemav1.CopyDeviceV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewTrustedDeviceResourceType returns the trusted device resource type.
func NewTrustedDeviceResourceType() tfdriver.ResourceType[types.DeviceV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.DeviceV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.DeviceV1, tfdriver.NameIdentifier] {
			return teleport.NewTrustedDeviceClient(clientFromProvider(p))
		},
		Kind: types.KindDevice,
		Codec: tfdriver.ResourceCodecFuncs[types.DeviceV1]{
			SchemaFunc:   schemav1.GenSchemaDeviceV1,
			ToStateFunc:  schemav1.CopyDeviceV1ToTerraform,
			FromPlanFunc: schemav1.CopyDeviceV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.DeviceV1](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(trustedDevice *types.DeviceV1) string {
			return trustedDevice.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.DeviceV1) string {
			return st.GetMetadata().Revision
		},
	}
}
