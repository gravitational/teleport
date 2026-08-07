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

	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/userprovisioning/v2"
)

// NewStaticHostUserDataSourceType returns the static host user data source type.
func NewStaticHostUserDataSourceType() tfdriver.DataSourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
			return teleport.NewStaticHostUserClient(clientFromProvider(p))
		},
		Kind: types.KindStaticHostUser,
		Codec: tfdriver.DataSourceCodecFuncs[userprovisioningv2.StaticHostUser]{
			SchemaFunc:  schemav1.GenSchemaStaticHostUser,
			ToStateFunc: schemav1.CopyStaticHostUserToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewStaticHostUserResourceType returns the static host user resource type.
func NewStaticHostUserResourceType() tfdriver.ResourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
			return teleport.NewStaticHostUserClient(clientFromProvider(p))
		},
		Kind: types.KindStaticHostUser,
		Codec: tfdriver.ResourceCodecFuncs[userprovisioningv2.StaticHostUser]{
			SchemaFunc:   schemav1.GenSchemaStaticHostUser,
			ToStateFunc:  schemav1.CopyStaticHostUserToTerraform,
			FromPlanFunc: schemav1.CopyStaticHostUserFromTerraform,
		},
		Normalizer: tfdriver.ForceKind[userprovisioningv2.StaticHostUser](types.KindStaticHostUser),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(staticHostUser *userprovisioningv2.StaticHostUser) string {
			return staticHostUser.GetMetadata().GetName()
		}),
		ResourceRevision: func(st *userprovisioningv2.StaticHostUser) string {
			return st.GetMetadata().GetRevision()
		},
	}
}
