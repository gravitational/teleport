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

// NewTrustedClusterDataSourceType returns the trusted cluster data source type.
func NewTrustedClusterDataSourceType() tfdriver.DataSourceType[types.TrustedClusterV2, tfdriver.NameIdentifier] {
	return tfdriver.DataSourceType[types.TrustedClusterV2, tfdriver.NameIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.TrustedClusterV2, tfdriver.NameIdentifier] {
			return teleport.NewTrustedClusterClient(clientFromProvider(p))
		},
		Kind: types.KindTrustedCluster,
		Name: types.KindTrustedCluster,
		Codec: tfdriver.DataSourceCodecFuncs[types.TrustedClusterV2]{
			SchemaFunc:  tfschema.GenSchemaTrustedClusterV2DataSource,
			ToStateFunc: tfschema.CopyTrustedClusterV2ToTerraform,
		},
		Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
	}
}

// NewTrustedClusterResourceType returns the trusted cluster resource type.
func NewTrustedClusterResourceType() tfdriver.ResourceType[types.TrustedClusterV2, tfdriver.NameIdentifier] {
	return tfdriver.ResourceType[types.TrustedClusterV2, tfdriver.NameIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.TrustedClusterV2, tfdriver.NameIdentifier] {
			return teleport.NewTrustedClusterClient(clientFromProvider(p))
		},
		Kind: types.KindTrustedCluster,
		Name: types.KindTrustedCluster,
		Codec: tfdriver.ResourceCodecFuncs[types.TrustedClusterV2]{
			SchemaFunc:   tfschema.GenSchemaTrustedClusterV2Resource,
			ToStateFunc:  tfschema.CopyTrustedClusterV2ToTerraform,
			FromPlanFunc: tfschema.CopyTrustedClusterV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.TrustedClusterV2](),
		Identifier: tfdriver.NameIdentifierPolicy(path.Root("metadata").AtName("name"), func(trustedCluster *types.TrustedClusterV2) string {
			return trustedCluster.GetMetadata().Name
		}),
		ResourceRevision: func(st *types.TrustedClusterV2) string {
			return st.GetMetadata().Revision
		},
	}
}
