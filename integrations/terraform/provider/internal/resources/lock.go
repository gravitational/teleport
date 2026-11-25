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

// NewLockDataSourceType returns the lock data source type.
func NewLockDataSourceType() tfdriver.DataSourceType[types.LockV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.LockV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.LockV2, tfdriver.NameIdentifier] {
			return teleport.NewLockClient(clientFromProvider(p))
		},
		Kind: types.KindLock,
		Name: types.KindLock,
		Codec: tfdriver.DataSourceCodecFuncs[types.LockV2]{
			SchemaFunc:  tfschema.GenSchemaLockV2DataSource,
			ToStateFunc: tfschema.CopyLockV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewLockResourceType returns the lock resource type.
func NewLockResourceType() tfdriver.ResourceType[types.LockV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.LockV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.LockV2, tfdriver.NameIdentifier] {
			return teleport.NewLockClient(clientFromProvider(p))
		},
		Kind: types.KindLock,
		Name: types.KindLock,
		Codec: tfdriver.ResourceCodecFuncs[types.LockV2]{
			SchemaFunc:   tfschema.GenSchemaLockV2Resource,
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
