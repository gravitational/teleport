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
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/devicetrust/v1"
)

// NewTrustedDeviceDataSourceType returns the trusted device data source type.
func NewTrustedDeviceDataSourceType() tfdriver.DataSourceType[types.DeviceV1, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.DeviceV1, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.DeviceV1, tfdriver.NameIdentifier] {
			return teleport.NewTrustedDeviceClient(clientFromProvider(p))
		},
		Kind: types.KindDevice,
		Name: "trusted_device",
		Codec: tfdriver.DataSourceCodecFuncs[types.DeviceV1]{
			SchemaFunc:  schemav1.GenSchemaDeviceV1DataSource,
			ToStateFunc: schemav1.CopyDeviceV1ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewTrustedDeviceResourceType returns the trusted device resource type.
func NewTrustedDeviceResourceType() tfdriver.ResourceType[types.DeviceV1, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.DeviceV1, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.DeviceV1, tfdriver.NameIdentifier] {
			return teleport.NewTrustedDeviceClient(clientFromProvider(p))
		},
		Kind: types.KindDevice,
		Name: "trusted_device",
		Codec: tfdriver.ResourceCodecFuncs[types.DeviceV1]{
			SchemaFunc:   schemav1.GenSchemaDeviceV1Resource,
			ToStateFunc:  schemav1.CopyDeviceV1ToTerraform,
			FromPlanFunc: schemav1.CopyDeviceV1FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.DeviceV1](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(trustedDevice *types.DeviceV1) string {
			return trustedDevice.GetMetadata().Name
		}),
	}
}
