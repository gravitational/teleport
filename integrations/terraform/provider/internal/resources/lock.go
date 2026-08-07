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
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// NewLockDataSourceType returns the lock data source type.
func NewLockDataSourceType() tfdriver.DataSourceType[types.LockV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.LockV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[types.LockV2, tfdriver.NameIdentifier] {
			return teleport.NewLockClient(clientFromProvider(p))
		},
		Kind: types.KindLock,
		Codec: tfdriver.DataSourceCodecFuncs[types.LockV2]{
			SchemaFunc:  tfschema.GenSchemaLockV2,
			ToStateFunc: tfschema.CopyLockV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewLockResourceType returns the lock resource type.
func NewLockResourceType() tfdriver.ResourceType[types.LockV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.LockV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[types.LockV2, tfdriver.NameIdentifier] {
			return teleport.NewLockClient(clientFromProvider(p))
		},
		Kind: types.KindLock,
		Codec: tfdriver.ResourceCodecFuncs[types.LockV2]{
			SchemaFunc:   tfschema.GenSchemaLockV2,
			ToStateFunc:  tfschema.CopyLockV2ToTerraform,
			FromPlanFunc: tfschema.CopyLockV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.LockV2](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(lock *types.LockV2) string {
			return lock.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.LockV2) string {
			return st.GetMetadata().Revision
		},
	}
}
