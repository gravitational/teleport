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

	userprovisioningv2 "github.com/gravitational/teleport/api/gen/proto/go/teleport/userprovisioning/v2"
	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	schemav1 "github.com/gravitational/teleport/integrations/terraform/tfschema/userprovisioning/v2"
)

// NewStaticHostUserDataSourceType returns the static host user data source type.
func NewStaticHostUserDataSourceType() tfdriver.DataSourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
			return teleport.NewStaticHostUserClient(clientFromProvider(p))
		},
		Kind: types.KindStaticHostUser,
		Name: types.KindStaticHostUser,
		Codec: tfdriver.DataSourceCodecFuncs[userprovisioningv2.StaticHostUser]{
			SchemaFunc:  schemav1.GenSchemaStaticHostUserDataSource,
			ToStateFunc: schemav1.CopyStaticHostUserToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewStaticHostUserResourceType returns the static host user resource type.
func NewStaticHostUserResourceType() tfdriver.ResourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[userprovisioningv2.StaticHostUser, tfdriver.NameIdentifier] {
			return teleport.NewStaticHostUserClient(clientFromProvider(p))
		},
		Kind: types.KindStaticHostUser,
		Name: types.KindStaticHostUser,
		Codec: tfdriver.ResourceCodecFuncs[userprovisioningv2.StaticHostUser]{
			SchemaFunc:   schemav1.GenSchemaStaticHostUserResource,
			ToStateFunc:  schemav1.CopyStaticHostUserToTerraform,
			FromPlanFunc: schemav1.CopyStaticHostUserFromTerraform,
			ModifyPlanFunc: tfdriver.NullToZeroModifyPlan(
				schemav1.CopyStaticHostUserFromTerraform,
				schemav1.CopyStaticHostUserToTerraformPreserveUnknown,
			),
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
