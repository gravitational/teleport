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
	"github.com/hashicorp/terraform-plugin-framework/provider"

	"github.com/gravitational/teleport/api/types"

	"github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
	"github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
	"github.com/gravitational/teleport/integrations/terraform/tfschema"
)

// clusterNetworkingConfigID is the static identifier used in the terraform state.
// Note that types.MetaNameClusterNetworkingConfig is "cluster-networking-config"
// which can't be used here without causing churn.
const clusterNetworkingConfigID = "cluster_networking_config"

// NewClusterNetworkingConfigDataSourceType returns the cluster networking config data source type.
func NewClusterNetworkingConfigDataSourceType() tfdriver.DataSourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
	return tfdriver.DataSourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier]{
		NewDataSourceClient: func(p provider.Provider) tfdriver.DataSourceClient[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
			return teleport.NewClusterNetworkingConfigClient(clientFromProvider(p))
		},
		Kind: types.KindClusterNetworkingConfig,
		Name: types.KindClusterNetworkingConfig,
		Codec: tfdriver.DataSourceCodecFuncs[types.ClusterNetworkingConfigV2]{
			SchemaFunc:  tfschema.GenSchemaClusterNetworkingConfigV2DataSource,
			ToStateFunc: tfschema.CopyClusterNetworkingConfigV2ToTerraform,
		},
		Identifier: tfdriver.SingletonIdentifierFromName(clusterNetworkingConfigID),
	}
}

// NewClusterNetworkingConfigResourceType returns the cluster networking config resource type.
func NewClusterNetworkingConfigResourceType() tfdriver.ResourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
	return tfdriver.ResourceType[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier]{
		NewResourceClient: func(p provider.Provider) tfdriver.ResourceClient[types.ClusterNetworkingConfigV2, tfdriver.SingletonIdentifier] {
			return teleport.NewClusterNetworkingConfigClient(clientFromProvider(p))
		},
		Kind: types.KindClusterNetworkingConfig,
		Name: types.KindClusterNetworkingConfig,
		Codec: tfdriver.ResourceCodecFuncs[types.ClusterNetworkingConfigV2]{
			SchemaFunc:   tfschema.GenSchemaClusterNetworkingConfigV2Resource,
			ToStateFunc:  tfschema.CopyClusterNetworkingConfigV2ToTerraform,
			FromPlanFunc: tfschema.CopyClusterNetworkingConfigV2FromTerraform,
		},
		Normalizer: tfdriver.CheckAndSetDefaults[types.ClusterNetworkingConfigV2](),
		Identifier: tfdriver.SingletonIdentifierPolicy[types.ClusterNetworkingConfigV2](clusterNetworkingConfigID),
		ResourceRevision: func(st *types.ClusterNetworkingConfigV2) string {
			return st.GetMetadata().Revision
		},
	}
}
