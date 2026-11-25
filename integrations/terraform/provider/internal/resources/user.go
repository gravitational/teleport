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

// NewUserDataSourceType returns the user data source type.
func NewUserDataSourceType() tfdriver.DataSourceType[types.UserV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.UserV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.UserV2, tfdriver.NameIdentifier] {
			return teleport.NewUserClient(clientFromProvider(p))
		},
		Kind: types.KindUser,
		Name: types.KindUser,
		Codec: tfdriver.DataSourceCodecFuncs[types.UserV2]{
			SchemaFunc:  tfschema.GenSchemaUserV2DataSource,
			ToStateFunc: tfschema.CopyUserV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewUserResourceType returns the user resource type.
func NewUserResourceType() tfdriver.ResourceType[types.UserV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.UserV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.UserV2, tfdriver.NameIdentifier] {
			return teleport.NewUserClient(clientFromProvider(p))
		},
		Kind: types.KindUser,
		Name: types.KindUser,
		Codec: tfdriver.ResourceCodecFuncs[types.UserV2]{
			SchemaFunc:   tfschema.GenSchemaUserV2Resource,
			ToStateFunc:  tfschema.CopyUserV2ToTerraform,
			FromPlanFunc: tfschema.CopyUserV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.UserV2](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(user *types.UserV2) string {
			return user.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.UserV2) string {
			return st.GetMetadata().Revision
		},
	}
}
