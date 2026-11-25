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

	apitypes "github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewRoleDataSourceType returns the role data source type.
func NewRoleDataSourceType() tfdriver.DataSourceType[apitypes.RoleV6, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[apitypes.RoleV6, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[apitypes.RoleV6, tfdriver.NameIdentifier] {
			return teleport.NewRoleClient(clientFromProvider(p))
		},
		Kind: apitypes.KindRole,
		Name: apitypes.KindRole,
		Codec: tfdriver.DataSourceCodecFuncs[apitypes.RoleV6]{
			SchemaFunc:  tfschema.GenSchemaRoleV6DataSource,
			ToStateFunc: tfschema.CopyRoleV6ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewRoleResourceType returns the role resource type.
func NewRoleResourceType() tfdriver.ResourceType[apitypes.RoleV6, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[apitypes.RoleV6, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[apitypes.RoleV6, tfdriver.NameIdentifier] {
			return teleport.NewRoleClient(clientFromProvider(p))
		},
		Kind: apitypes.KindRole,
		Name: apitypes.KindRole,
		Codec: tfdriver.ResourceCodecFuncs[apitypes.RoleV6]{
			SchemaFunc:   tfschema.GenSchemaRoleV6Resource,
			ToStateFunc:  tfschema.CopyRoleV6ToTerraform,
			FromPlanFunc: tfschema.CopyRoleV6FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[apitypes.RoleV6](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(role *apitypes.RoleV6) string {
			return role.GetMetadata().Name
		}),
		ResourceRevision: func(st *apitypes.RoleV6) string {
			return st.GetMetadata().Revision
		},
	}
}
